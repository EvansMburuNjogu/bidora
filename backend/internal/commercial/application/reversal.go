package application

import (
	"bidora/backend/internal/identity/assurance"
	"bidora/backend/internal/identity/securitypolicy"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
	"github.com/google/uuid"
)

var ErrReversalRequestInvalid = errors.New("commercial_reversal_request_invalid")
var ErrReversalNotFound = errors.New("commercial_reversal_not_found")
var ErrReversalNotPermitted = errors.New("commercial_reversal_not_permitted")
var ErrReversalConflict = errors.New("commercial_reversal_conflict")
var ErrReversalNotReady = errors.New("commercial_reversal_not_ready")
var ErrReversalUnavailable = errors.New("commercial_reversal_unavailable")

type ReconciliationRequestRecord struct {
	RequestHash [32]byte
	PaymentID   domain.ID
	State       string
	Version     int64
}
type ReversalStored struct {
	Decision     domain.ReversalDecision
	AccessHoldID domain.ID
}
type ReversalEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID, PaymentID, SubscriptionID, ObservationID, AccessHoldID domain.ID
	EventType, PaymentState, InvoiceState, SubscriptionState, AccessMode                                                   string
	PaymentVersion, InvoiceVersion, SubscriptionVersion, AmountMinorUnits                                                  int64
	OccurredAt                                                                                                             time.Time
}

type ReversalTx interface {
	LockPaymentForReconciliationRequest(context.Context, domain.ID) (domain.Payment, bool, error)
	ResolveAdjustmentAuthority(context.Context, AdjustmentActor, domain.ID, time.Time) (AdjustmentAuthority, bool, error)
	GetReconciliationRequestCommand(context.Context, domain.ID, [32]byte) (ReconciliationRequestRecord, bool, error)
	SchedulePaymentReconciliation(context.Context, domain.Payment, domain.ID, string, [32]byte, [32]byte, [32]byte, PaymentEvidence) error
	GetPaymentReversal(context.Context, domain.ID) (ReversalStored, bool, error)
	LockPaymentReversalCandidate(context.Context, domain.ID) (domain.ReversalCandidate, bool, error)
	CommitPaymentReversal(context.Context, domain.ReversalDecision, domain.ID, ReversalEvidence) error
}
type ReversalUnitOfWork interface {
	WithinReversal(context.Context, func(context.Context, ReversalTx) error) error
}

type RequestReconciliationInput struct {
	Actor                    AdjustmentActor
	PaymentID, CorrelationID domain.ID
	ExpectedVersion          int64
	Reason, IdempotencyKey   string
}
type CompleteReversalInput struct{ PaymentID, ExecutionID, CorrelationID domain.ID }

type ReversalService struct {
	uow   ReversalUnitOfWork
	ids   InvoiceIDGenerator
	clock InvoiceClock
}

func NewReversalService(uow ReversalUnitOfWork, ids InvoiceIDGenerator, clock InvoiceClock) (*ReversalService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_reversal_service_invalid")
	}
	return &ReversalService{uow: uow, ids: ids, clock: clock}, nil
}

func (s *ReversalService) RequestReconciliation(ctx context.Context, in RequestReconciliationInput) (result domain.Payment, err error) {
	if _, e := uuid.Parse(in.IdempotencyKey); ctx == nil || e != nil || in.Actor.UserID.IsZero() || in.Actor.SessionID.IsZero() || in.Actor.ContextGeneration < 1 || in.Actor.SupportSession || in.PaymentID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedVersion < 1 || !domain.ValidReconciliationReason(in.Reason) {
		return result, ErrReversalRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrReversalUnavailable
	}
	key := sha256.Sum256([]byte("commercial-payment-reconciliation-request:v1:" + in.IdempotencyKey))
	body, e := json.Marshal(struct {
		Actor, Payment    domain.ID
		Context, Expected int64
		Reason            string
	}{in.Actor.UserID, in.PaymentID, in.Actor.ContextGeneration, in.ExpectedVersion, in.Reason})
	if e != nil {
		return result, ErrReversalRequestInvalid
	}
	request := sha256.Sum256(append([]byte("commercial-payment-reconciliation-request:v1:"), body...))
	ids, e := paymentIDs(s.ids, 3)
	if e != nil {
		return result, e
	}
	err = s.uow.WithinReversal(ctx, func(c context.Context, tx ReversalTx) error {
		payment, found, x := tx.LockPaymentForReconciliationRequest(c, in.PaymentID)
		if x != nil {
			return x
		}
		if !found {
			return ErrReversalNotFound
		}
		auth, tie, x := tx.ResolveAdjustmentAuthority(c, in.Actor, payment.TenantID, at)
		if x != nil {
			return x
		}
		if tie || auth.UserID != in.Actor.UserID || auth.ContextGeneration != in.Actor.ContextGeneration || !auth.BillingAdministrator || !assurance.Platform(auth.AssuranceMethod) || auth.AssuredAt.After(at) || !securitypolicy.Recent(ctx, auth.AssuredAt, at) {
			return ErrReversalNotPermitted
		}
		authorityHash, x := domain.AdjustmentAuthorityHash(auth.UserID, auth.ContextGeneration, auth.AdministratorVersion, "BILLING_ADMINISTRATOR", "platform.payment.reconcile", auth.AssuranceMethod, auth.AssuredAt)
		if x != nil {
			return ErrReversalNotPermitted
		}
		command, replayed, x := tx.GetReconciliationRequestCommand(c, in.Actor.UserID, key)
		if x != nil {
			return x
		}
		if replayed {
			if command.RequestHash != request || command.PaymentID != in.PaymentID {
				return ErrReversalConflict
			}
			result = payment
			return nil
		}
		if payment.Version != in.ExpectedVersion || payment.State != "SETTLED" || payment.ReconciliationState != "COMPLETE" {
			return ErrReversalConflict
		}
		evidence := PaymentEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TenantID: payment.TenantID, InvoiceID: payment.InvoiceID, PaymentID: payment.ID, EventType: "commercial.payment.reconciliation_requested.v1", State: "SETTLED", ReconciliationState: "DUE", Version: payment.Version + 1, AmountMinorUnits: payment.AmountMinorUnits, OccurredAt: at}
		if x = tx.SchedulePaymentReconciliation(c, payment, ids[0], in.Reason, key, request, authorityHash, evidence); x != nil {
			return x
		}
		payment.Version++
		payment.ReconciliationState = "DUE"
		payment.ReconciliationReason = "POST_SETTLEMENT_RECHECK"
		payment.ReconciliationAttempts = 0
		payment.ReconciliationDueAt = &at
		payment.UpdatedAt = at
		result = payment
		return nil
	})
	if err != nil {
		return domain.Payment{}, mapReversalError(err)
	}
	return result, nil
}

func (s *ReversalService) Complete(ctx context.Context, in CompleteReversalInput) (result ReversalStored, err error) {
	if ctx == nil || in.PaymentID.IsZero() || in.ExecutionID.IsZero() || in.CorrelationID.IsZero() {
		return result, ErrReversalRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrReversalUnavailable
	}
	ids, e := paymentIDs(s.ids, 3)
	if e != nil {
		return result, e
	}
	err = s.uow.WithinReversal(ctx, func(c context.Context, tx ReversalTx) error {
		stored, found, x := tx.GetPaymentReversal(c, in.PaymentID)
		if x != nil {
			return x
		}
		if found {
			result = stored
			return nil
		}
		candidate, ready, x := tx.LockPaymentReversalCandidate(c, in.PaymentID)
		if x != nil {
			return x
		}
		if !ready {
			return ErrReversalNotReady
		}
		decision, x := domain.DecidePaymentReversal(candidate, at)
		if x != nil {
			return ErrReversalConflict
		}
		evidence := ReversalEvidence{AuditID: ids[1], OutboxID: ids[2], CorrelationID: in.CorrelationID, ActorID: in.ExecutionID, TenantID: decision.TenantID, InvoiceID: decision.InvoiceID, PaymentID: decision.PaymentID, SubscriptionID: decision.SubscriptionID, ObservationID: decision.ReversalObservationID, AccessHoldID: ids[0], EventType: "commercial.payment.reversed.v1", PaymentState: "REVERSED", InvoiceState: "REVIEW_REQUIRED", SubscriptionState: "SUSPENDED", AccessMode: decision.AccessMode, PaymentVersion: decision.ResultingPaymentVersion, InvoiceVersion: decision.ResultingInvoiceVersion, SubscriptionVersion: decision.ResultingSubscriptionVersion, AmountMinorUnits: decision.AmountMinorUnits, OccurredAt: at}
		if x = tx.CommitPaymentReversal(c, decision, ids[0], evidence); x != nil {
			return x
		}
		result = ReversalStored{Decision: decision, AccessHoldID: ids[0]}
		return nil
	})
	if err != nil {
		return ReversalStored{}, mapReversalError(err)
	}
	return result, nil
}

func mapReversalError(e error) error {
	switch {
	case e == nil:
		return nil
	case errors.Is(e, ErrReversalRequestInvalid), errors.Is(e, ErrReversalNotFound), errors.Is(e, ErrReversalNotPermitted), errors.Is(e, ErrReversalConflict), errors.Is(e, ErrReversalNotReady):
		return e
	default:
		return ErrReversalUnavailable
	}
}
