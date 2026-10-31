package application

import (
	"context"
	"errors"
	"time"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
)

var ErrDocumentAIAccountingInvalid = errors.New("document_ai_accounting_invalid")
var ErrDocumentAIAccountingUnavailable = errors.New("document_ai_accounting_unavailable")
var ErrDocumentAIAccountingConflict = errors.New("document_ai_accounting_conflict")

const DocumentCompletionMaxAttempts int32 = 9

var documentCompletionBackoffs = [...]time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour}

type DocumentCompletionEffect string

const (
	DocumentCompletionConfirmed DocumentCompletionEffect = "CONFIRMED"
	DocumentCompletionRetryable DocumentCompletionEffect = "RETRYABLE_OR_AMBIGUOUS"
	DocumentCompletionConflict  DocumentCompletionEffect = "ACCOUNTING_CONFLICT"
)

type DocumentCommercialAccounting interface {
	VerifyDocumentReservation(context.Context, ProcessingClaim) error
	CompleteDocumentReservation(context.Context, entitlementapp.CompletionRequest) (DocumentCompletionEffect, error)
}

type DocumentCompletionClaim struct {
	Request      entitlementapp.CompletionRequest
	LeaseID      ID
	LeaseExpires time.Time
	Attempts     int32
}

type DocumentCompletionStore interface {
	RecoverExpiredDocumentCompletion(context.Context, ID) (bool, error)
	ClaimDocumentCompletion(context.Context, ID) (DocumentCompletionClaim, bool, error)
	RetryDocumentCompletion(context.Context, DocumentCompletionClaim, ID, string, time.Time, time.Time) error
	HoldDocumentCompletion(context.Context, DocumentCompletionClaim, ID, string, time.Time) error
	AcknowledgeDocumentCompletion(context.Context, DocumentCompletionClaim, [3]ID, ID, time.Time) error
}

type DocumentCompletionPlacement interface {
	ValidateCurrentDocumentCompletionAssignment(context.Context, ID, int64) error
}

type DocumentCompletionCoordinator struct {
	store                DocumentCompletionStore
	commercial           DocumentCommercialAccounting
	placement            DocumentCompletionPlacement
	ids                  CaptureIDGenerator
	clock                CaptureClock
	observer             DocumentCompletionObserver
	tenantID             ID
	assignmentGeneration int64
}

func NewDocumentCompletionCoordinator(store DocumentCompletionStore, commercial DocumentCommercialAccounting, placement DocumentCompletionPlacement, ids CaptureIDGenerator, clock CaptureClock, observer DocumentCompletionObserver, tenantID ID, assignmentGeneration int64) (*DocumentCompletionCoordinator, error) {
	if store == nil || commercial == nil || placement == nil || ids == nil || clock == nil || observer == nil || tenantID.IsZero() || assignmentGeneration < 1 {
		return nil, ErrDocumentAIAccountingInvalid
	}
	return &DocumentCompletionCoordinator{store: store, commercial: commercial, placement: placement, ids: ids, clock: clock, observer: observer, tenantID: tenantID, assignmentGeneration: assignmentGeneration}, nil
}

func (c *DocumentCompletionCoordinator) ProcessNext(ctx context.Context) (bool, error) {
	if c == nil || ctx == nil {
		return false, ErrDocumentAIAccountingInvalid
	}
	recoveryID, err := c.newID()
	if err != nil {
		return false, err
	}
	if recovered, recoverErr := c.store.RecoverExpiredDocumentCompletion(ctx, recoveryID); recoverErr != nil || recovered {
		if recoverErr != nil {
			return false, ErrDocumentAIAccountingUnavailable
		}
		return true, nil
	}
	leaseID, err := c.newID()
	if err != nil {
		return false, err
	}
	claim, found, err := c.store.ClaimDocumentCompletion(ctx, leaseID)
	if err != nil || !found {
		if err != nil {
			return false, ErrDocumentAIAccountingUnavailable
		}
		return false, nil
	}
	request, checkpoint := claim.Request, claim.Request.Checkpoint
	if ctx.Err() != nil {
		return true, ErrDocumentAIAccountingUnavailable
	}
	if err = c.placement.ValidateCurrentDocumentCompletionAssignment(ctx, c.tenantID, c.assignmentGeneration); err != nil {
		return true, ErrDocumentAIAccountingUnavailable
	}
	if ID(checkpoint.TenantID) != c.tenantID || checkpoint.ID != request.CheckpointID || checkpoint.ReservationID != request.ReservationID || checkpoint.RequesterUserID.IsZero() || request.ID.IsZero() || request.ExecutionID.IsZero() {
		return true, c.finishHold(ctx, claim, "INVALID_REQUEST")
	}
	if checkpoint.AssignmentGeneration < c.assignmentGeneration {
		return true, c.finishHold(ctx, claim, "STALE_ASSIGNMENT")
	}
	if checkpoint.AssignmentGeneration > c.assignmentGeneration {
		return true, c.finishHold(ctx, claim, "GENERATION_INTEGRITY_CONFLICT")
	}
	if (request.Operation != entitlementapp.CompletionSettle && request.Operation != entitlementapp.CompletionRelease) || (request.Operation == entitlementapp.CompletionSettle && request.ActualCredits < 1) || (request.Operation == entitlementapp.CompletionRelease && request.ActualCredits != 0) {
		return true, c.finishHold(ctx, claim, "INVALID_REQUEST")
	}
	effect, commercialErr := c.commercial.CompleteDocumentReservation(ctx, request)
	if ctx.Err() != nil {
		return true, ErrDocumentAIAccountingUnavailable
	}
	if effect == DocumentCompletionConflict {
		return true, c.finishHold(ctx, claim, "ACCOUNTING_CONFLICT")
	}
	if commercialErr != nil || effect != DocumentCompletionConfirmed {
		return true, c.finishRetry(ctx, claim, "AMBIGUOUS_INTERNAL_EFFECT")
	}
	if err = c.placement.ValidateCurrentDocumentCompletionAssignment(ctx, c.tenantID, c.assignmentGeneration); err != nil {
		return true, ErrDocumentAIAccountingUnavailable
	}
	ids, err := c.newIDs(4)
	if err != nil {
		return true, c.finishRetry(ctx, claim, "TRANSIENT_UNAVAILABLE")
	}
	at := c.clock.Now().UTC()
	if at.IsZero() {
		return true, ErrDocumentAIAccountingUnavailable
	}
	if err = c.store.AcknowledgeDocumentCompletion(ctx, claim, [3]ID{ids[0], ids[1], ids[2]}, ids[3], at); err != nil {
		if ctx.Err() != nil {
			return true, ErrDocumentAIAccountingUnavailable
		}
		return true, c.finishRetry(ctx, claim, "LOCAL_ACK_UNAVAILABLE")
	}
	c.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "DISPOSITION", Result: "ACKNOWLEDGED"})
	return true, nil
}

func (c *DocumentCompletionCoordinator) finishRetry(ctx context.Context, claim DocumentCompletionClaim, code string) error {
	if ctx.Err() != nil {
		return ErrDocumentAIAccountingUnavailable
	}
	if claim.Attempts >= DocumentCompletionMaxAttempts {
		return c.finishHold(ctx, claim, "RECONCILIATION_EXHAUSTED")
	}
	if claim.Attempts < 1 || int(claim.Attempts) > len(documentCompletionBackoffs) {
		return c.finishHold(ctx, claim, "INVALID_REQUEST")
	}
	outcomeID, err := c.newID()
	if err != nil {
		return err
	}
	at := c.clock.Now().UTC()
	if at.IsZero() {
		return ErrDocumentAIAccountingUnavailable
	}
	if err = c.store.RetryDocumentCompletion(ctx, claim, outcomeID, code, at, at.Add(documentCompletionBackoffs[claim.Attempts-1])); err != nil {
		return ErrDocumentAIAccountingUnavailable
	}
	c.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "DISPOSITION", Result: "RETRY_" + code})
	return ErrDocumentAIAccountingUnavailable
}

func (c *DocumentCompletionCoordinator) finishHold(ctx context.Context, claim DocumentCompletionClaim, code string) error {
	if ctx.Err() != nil {
		return ErrDocumentAIAccountingUnavailable
	}
	outcomeID, err := c.newID()
	if err != nil {
		return err
	}
	at := c.clock.Now().UTC()
	if at.IsZero() {
		return ErrDocumentAIAccountingUnavailable
	}
	if err = c.store.HoldDocumentCompletion(ctx, claim, outcomeID, code, at); err != nil {
		return ErrDocumentAIAccountingUnavailable
	}
	c.observer.ObserveDocumentCompletion(DocumentCompletionObservation{Signal: "DISPOSITION", Result: "HELD_" + code})
	return ErrDocumentAIAccountingConflict
}

func (c *DocumentCompletionCoordinator) newID() (ID, error) {
	id, err := c.ids.NewID()
	if err != nil || id.IsZero() {
		return ID{}, ErrDocumentAIAccountingUnavailable
	}
	return id, nil
}
func (c *DocumentCompletionCoordinator) newIDs(count int) ([]ID, error) {
	ids := make([]ID, count)
	for index := range ids {
		id, err := c.newID()
		if err != nil {
			return nil, err
		}
		ids[index] = id
	}
	return ids, nil
}
