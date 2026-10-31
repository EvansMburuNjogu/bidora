package application

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"
)

// ID is an opaque application-generated UUID value. It carries no authority.
type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

// CaptureSubject is authenticated request provenance. Authorizer must recheck
// current session, membership, context generation, tenant state and capability.
type CaptureSubject struct {
	IdentityID, SelectedTenantID ID
	ContextGeneration            int64
}
type CaptureAuthority struct {
	ActorID, TenantID    ID
	AssignmentGeneration int64
}

type CaptureAuthorizer interface {
	AuthorizeCapture(context.Context, CaptureSubject) (CaptureAuthority, error)
}
type SourceDocumentPolicy interface {
	CheckCaptureSource(context.Context, CaptureAuthority, string) error
}
type CaptureIDGenerator interface{ NewID() (ID, error) }
type CaptureClock interface{ Now() time.Time }

type IdempotencyState uint8

const (
	IdempotencyNew IdempotencyState = iota + 1
	IdempotencyCompleted
	IdempotencyPending
)

type CaptureClaim struct {
	State                          IdempotencyState
	RequestHash                    [32]byte
	OpportunityID, SourceVersionID ID
	CreatedAt                      time.Time
}

type CaptureTx interface {
	Claim(context.Context, ID, ID, [32]byte, [32]byte, time.Time) (CaptureClaim, error)
	InsertOpportunity(context.Context, ID, ID, ID, CaptureCommand, time.Time) error
	AppendAudit(context.Context, ID, ID, ID, ID, time.Time) error
	AppendOutbox(context.Context, ID, ID, ID, ID, time.Time) error
	Complete(context.Context, ID, [32]byte, ID, time.Time) error
}
type CaptureUnitOfWork interface {
	WithinCapture(context.Context, CaptureAuthority, func(CaptureTx) error) error
}

type CaptureExecutor struct {
	Authorizer CaptureAuthorizer
	Documents  SourceDocumentPolicy
	UOW        CaptureUnitOfWork
	IDs        CaptureIDGenerator
	Clock      CaptureClock
}
type ExecuteCaptureInput struct {
	Subject        CaptureSubject
	IdempotencyKey string
	CorrelationID  ID
	Command        CaptureCommand
}
type CaptureResult struct {
	OpportunityID, SourceVersionID ID
	Version                        int64
	CreatedAt                      time.Time
	Replayed                       bool
}

func (e CaptureExecutor) Execute(ctx context.Context, input ExecuteCaptureInput) (CaptureResult, error) {
	if ctx == nil {
		return CaptureResult{}, safe("capture_context_required", nil)
	}
	if e.Authorizer == nil || e.Documents == nil || e.UOW == nil || e.IDs == nil || e.Clock == nil {
		return CaptureResult{}, safe("capture_not_configured", nil)
	}
	if input.CorrelationID.IsZero() {
		return CaptureResult{}, safe("capture_correlation_required", nil)
	}
	if !uuidSyntax(input.IdempotencyKey) {
		return CaptureResult{}, safe("capture_idempotency_key_invalid", nil)
	}
	requestHash, ok := input.Command.Fingerprint()
	if !ok {
		return CaptureResult{}, safe("capture_command_invalid", nil)
	}
	authority, err := e.Authorizer.AuthorizeCapture(ctx, input.Subject)
	if err != nil {
		return CaptureResult{}, safe("capture_not_authorized", err)
	}
	if authority.ActorID.IsZero() || authority.TenantID.IsZero() || authority.AssignmentGeneration < 1 {
		return CaptureResult{}, safe("capture_not_authorized", nil)
	}
	facts, _ := input.Command.Facts()
	if ref := facts.SourceDocumentVersionID(); ref != "" {
		if err = e.Documents.CheckCaptureSource(ctx, authority, ref); err != nil {
			return CaptureResult{}, safe("capture_source_unavailable", err)
		}
	}
	ids := make([]ID, 5)
	for i := range ids {
		ids[i], err = e.IDs.NewID()
		if err != nil || ids[i].IsZero() {
			return CaptureResult{}, safe("capture_id_generation_failed", err)
		}
	}
	at := e.Clock.Now().UTC()
	if at.IsZero() {
		return CaptureResult{}, safe("capture_clock_invalid", nil)
	}
	keyHash := sha256.Sum256([]byte("bidora.capture.key.v1\x00" + strings.ToLower(input.IdempotencyKey)))
	var result CaptureResult
	err = e.UOW.WithinCapture(ctx, authority, func(tx CaptureTx) error {
		if tx == nil {
			return safe("capture_transaction_unavailable", nil)
		}
		claim, err := tx.Claim(ctx, ids[0], authority.ActorID, keyHash, requestHash, at)
		if err != nil {
			return safe("capture_persistence_failed", err)
		}
		if subtle.ConstantTimeCompare(claim.RequestHash[:], requestHash[:]) != 1 {
			return safe("capture_idempotency_conflict", nil)
		}
		switch claim.State {
		case IdempotencyCompleted:
			if claim.OpportunityID.IsZero() || claim.SourceVersionID.IsZero() || claim.CreatedAt.IsZero() {
				return safe("capture_persistence_inconsistent", nil)
			}
			result = CaptureResult{claim.OpportunityID, claim.SourceVersionID, 1, claim.CreatedAt.UTC(), true}
			return nil
		case IdempotencyPending:
			return safe("capture_operation_in_progress", nil)
		case IdempotencyNew:
		default:
			return safe("capture_persistence_inconsistent", nil)
		}
		if err = tx.InsertOpportunity(ctx, ids[1], ids[2], authority.ActorID, input.Command, at); err != nil {
			return safe("capture_persistence_failed", err)
		}
		if err = tx.AppendAudit(ctx, ids[3], authority.ActorID, ids[1], input.CorrelationID, at); err != nil {
			return safe("capture_persistence_failed", err)
		}
		if err = tx.AppendOutbox(ctx, ids[4], authority.ActorID, ids[1], ids[2], at); err != nil {
			return safe("capture_persistence_failed", err)
		}
		if err = tx.Complete(ctx, ids[0], requestHash, ids[1], at); err != nil {
			return safe("capture_persistence_failed", err)
		}
		result = CaptureResult{ids[1], ids[2], 1, at, false}
		return nil
	})
	if err != nil {
		return CaptureResult{}, err
	}
	return result, nil
}

type operationError struct {
	code  string
	cause error
}

func safe(code string, cause error) error               { return &operationError{code, cause} }
func (e *operationError) Error() string                 { return e.code }
func (e *operationError) Unwrap() error                 { return e.cause }
func (e *operationError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.code) }
