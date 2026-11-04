package application

import (
	"bidora/backend/internal/identity/assurance"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrAdjustmentRequestInvalid = errors.New("commercial_adjustment_request_invalid")
var ErrAdjustmentNotFound = errors.New("commercial_adjustment_not_found")
var ErrAdjustmentNotPermitted = errors.New("commercial_refund_not_permitted")
var ErrAdjustmentApprovalRequired = errors.New("commercial_adjustment_approval_required")
var ErrAdjustmentConflict = errors.New("commercial_adjustment_conflict")
var ErrAdjustmentUnavailable = errors.New("commercial_adjustment_unavailable")

type AdjustmentActor struct {
	UserID, SessionID domain.ID
	ContextGeneration int64
	SupportSession    bool
}
type AdjustmentAuthority struct {
	UserID                                  domain.ID
	ContextGeneration, AdministratorVersion int64
	BillingAdministrator, PlatformOwner     bool
	InvoiceReader, PaymentReader            bool
	AssuranceMethod                         string
	AssuredAt                               time.Time
}
type ExpiredAdjustment struct {
	ChangeID, PaymentID                        domain.ID
	Action, ReasonCode, EntitlementDisposition string
	AmountMinorUnits, Version                  int64
}
type AdjustmentCommand struct {
	RequestHash [32]byte
	TargetID    domain.ID
	State       string
	Version     int64
}
type AdjustmentStored struct {
	Proposal                                     domain.AdjustmentProposal
	State, InvoiceState, PaymentState            string
	CheckerUserID, CreditNoteID, RefundID        domain.ID
	CreditNumber                                 string
	CurrentInvoiceVersion, CurrentPaymentVersion int64
	InvoiceTotalMinor, PriorCreditMinor          int64
}
type AdjustmentDecision struct {
	ChangeID, TenantID, CheckerUserID, DecisionID, CreditNoteID, RefundID, CommandID domain.ID
	ExpectedVersion                                                                  int64
	Decision, Reason, CreditNumber, CreditState                                      string
	AuthorityHash, RequestHash, CreditHash, KeyHash                                  [32]byte
	At                                                                               time.Time
}

type AdjustmentTx interface {
	ResolveAdjustmentInvoiceTenant(context.Context, domain.ID) (domain.ID, bool, error)
	ResolveAdjustmentChangeTenant(context.Context, domain.ID) (domain.ID, bool, error)
	ResolveAdjustmentAuthority(context.Context, AdjustmentActor, domain.ID, time.Time) (AdjustmentAuthority, bool, error)
	GetAdjustmentCommand(context.Context, domain.ID, string, [32]byte) (AdjustmentCommand, bool, error)
	GetAdjustment(context.Context, domain.ID, domain.ID, bool) (AdjustmentStored, bool, error)
	ExpirePendingAdjustment(context.Context, domain.ID, domain.ID, time.Time) (ExpiredAdjustment, bool, error)
	LockAdjustmentCandidate(context.Context, domain.ID, domain.ID) (domain.AdjustmentCandidate, bool, error)
	AppendAdjustmentEvidence(context.Context, AdjustmentEvidence) error
	InsertAdjustment(context.Context, domain.AdjustmentProposal, domain.ID, [32]byte, AdjustmentEvidence) error
	LockAdjustment(context.Context, domain.ID, domain.ID) (AdjustmentStored, bool, error)
	LockCreditNoteNumber(context.Context, int, time.Time) (int64, error)
	CommitAdjustmentDecision(context.Context, AdjustmentDecision, AdjustmentStored, AdjustmentEvidence) error
}
type AdjustmentUnitOfWork interface {
	WithinAdjustment(context.Context, func(context.Context, AdjustmentTx) error) error
}

type ProposeAdjustmentInput struct {
	Actor                                                                      AdjustmentActor
	InvoiceID, CorrelationID                                                   domain.ID
	Action, ReasonCode, EntitlementDisposition, ProposalReason, IdempotencyKey string
	AmountMinorUnits, ExpectedInvoiceVersion, ExpectedPaymentVersion           int64
}
type DecideAdjustmentInput struct {
	Actor                            AdjustmentActor
	ChangeID, CorrelationID          domain.ID
	Decision, Reason, IdempotencyKey string
	ExpectedVersion                  int64
}

type AdjustmentEvidence struct {
	AuditID, OutboxID, CorrelationID, ActorID, TenantID, InvoiceID, PaymentID, ChangeID domain.ID
	EventType, State, Action, ReasonCode, EntitlementDisposition                        string
	Version, AmountMinorUnits                                                           int64
	OccurredAt                                                                          time.Time
}

type AdjustmentPolicy struct{ RefundRequestWindow time.Duration }
type AdjustmentService struct {
	uow    AdjustmentUnitOfWork
	ids    InvoiceIDGenerator
	clock  InvoiceClock
	zone   *time.Location
	policy AdjustmentPolicy
}

func NewAdjustmentService(uow AdjustmentUnitOfWork, ids InvoiceIDGenerator, clock InvoiceClock, policy AdjustmentPolicy) (*AdjustmentService, error) {
	if uow == nil || ids == nil || clock == nil || policy.RefundRequestWindow <= 0 || policy.RefundRequestWindow > domain.RefundEligibilityWindow {
		return nil, errors.New("commercial_adjustment_service_invalid")
	}
	z, e := time.LoadLocation("Africa/Nairobi")
	if e != nil {
		return nil, ErrAdjustmentUnavailable
	}
	return &AdjustmentService{uow: uow, ids: ids, clock: clock, zone: z, policy: policy}, nil
}

func (s *AdjustmentService) Propose(ctx context.Context, in ProposeAdjustmentInput) (out AdjustmentStored, err error) {
	if ctx == nil || in.Actor.UserID.IsZero() || in.Actor.SessionID.IsZero() || in.Actor.ContextGeneration < 1 || in.Actor.SupportSession || in.InvoiceID.IsZero() || in.CorrelationID.IsZero() || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 200 || in.ExpectedInvoiceVersion < 1 || in.ExpectedPaymentVersion < 1 {
		return out, ErrAdjustmentRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrAdjustmentUnavailable
	}
	key := sha256.Sum256([]byte("commercial-adjustment-propose:v1:" + in.IdempotencyKey))
	request, err := adjustmentHash("PROPOSE", in)
	if err != nil {
		return out, ErrAdjustmentRequestInvalid
	}
	ids, e := paymentIDs(s.ids, 6)
	if e != nil {
		return out, e
	}
	err = s.uow.WithinAdjustment(ctx, func(c context.Context, tx AdjustmentTx) error {
		tenantID, targetFound, x := tx.ResolveAdjustmentInvoiceTenant(c, in.InvoiceID)
		if x != nil {
			return x
		}
		if !targetFound {
			return ErrAdjustmentNotFound
		}
		auth, tie, x := tx.ResolveAdjustmentAuthority(c, in.Actor, tenantID, at)
		if x != nil {
			return x
		}
		if tie || auth.UserID != in.Actor.UserID || auth.ContextGeneration != in.Actor.ContextGeneration || !auth.BillingAdministrator || !assurance.Platform(auth.AssuranceMethod) || auth.AssuredAt.After(at) || auth.AssuredAt.Before(at.Add(-15*time.Minute)) {
			return ErrAdjustmentNotPermitted
		}
		cmd, found, x := tx.GetAdjustmentCommand(c, in.Actor.UserID, "PROPOSE_ADJUSTMENT", key)
		if x != nil {
			return x
		}
		if found {
			if cmd.RequestHash != request {
				return ErrAdjustmentConflict
			}
			var replayFound bool
			out, replayFound, x = tx.GetAdjustment(c, tenantID, cmd.TargetID, false)
			if x != nil {
				return x
			}
			if !replayFound {
				return ErrAdjustmentConflict
			}
			out.State = cmd.State
			out.Proposal.Version = cmd.Version
			out.CheckerUserID, out.CreditNoteID, out.RefundID, out.CreditNumber = domain.ID{}, domain.ID{}, domain.ID{}, ""
			return nil
		}
		authHash, x := domain.AdjustmentAuthorityHash(auth.UserID, auth.ContextGeneration, auth.AdministratorVersion, "BILLING_ADMINISTRATOR", "platform.billing.adjustment.request", auth.AssuranceMethod, auth.AssuredAt)
		if x != nil {
			return ErrAdjustmentNotPermitted
		}
		expired, expiredFound, x := tx.ExpirePendingAdjustment(c, tenantID, in.InvoiceID, at)
		if x != nil {
			return x
		}
		if expiredFound {
			expiredEvidence := AdjustmentEvidence{AuditID: ids[4], OutboxID: ids[5], CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TenantID: tenantID, InvoiceID: in.InvoiceID, PaymentID: expired.PaymentID, ChangeID: expired.ChangeID, EventType: "commercial.financial_adjustment.expired.v1", State: "EXPIRED", Action: expired.Action, ReasonCode: expired.ReasonCode, EntitlementDisposition: expired.EntitlementDisposition, Version: expired.Version, AmountMinorUnits: expired.AmountMinorUnits, OccurredAt: at}
			if x = tx.AppendAdjustmentEvidence(c, expiredEvidence); x != nil {
				return x
			}
		}
		candidate, ok, x := tx.LockAdjustmentCandidate(c, tenantID, in.InvoiceID)
		if x != nil {
			return x
		}
		if !ok {
			return ErrAdjustmentNotPermitted
		}
		if candidate.InvoiceVersion != in.ExpectedInvoiceVersion || candidate.PaymentVersion != in.ExpectedPaymentVersion {
			return ErrAdjustmentConflict
		}
		proposal, x := domain.NewAdjustmentProposal(ids[0], in.Actor.UserID, in.Action, in.ReasonCode, in.EntitlementDisposition, in.ProposalReason, in.AmountMinorUnits, candidate, authHash, request, s.policy.RefundRequestWindow, at)
		if x != nil {
			return ErrAdjustmentRequestInvalid
		}
		evidence := AdjustmentEvidence{AuditID: ids[2], OutboxID: ids[3], CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TenantID: tenantID, InvoiceID: in.InvoiceID, PaymentID: candidate.PaymentID, ChangeID: proposal.ID, EventType: "commercial.financial_adjustment.proposed.v1", State: "PENDING_APPROVAL", Action: proposal.Action, ReasonCode: proposal.ReasonCode, EntitlementDisposition: proposal.EntitlementDisposition, Version: 1, AmountMinorUnits: proposal.AmountMinorUnits, OccurredAt: at}
		if x = tx.InsertAdjustment(c, proposal, ids[1], key, evidence); x != nil {
			return x
		}
		out = AdjustmentStored{Proposal: proposal, State: "PENDING_APPROVAL"}
		return nil
	})
	if err != nil {
		return AdjustmentStored{}, mapAdjustmentError(err)
	}
	return out, nil
}

func (s *AdjustmentService) Decide(ctx context.Context, in DecideAdjustmentInput) (out AdjustmentStored, err error) {
	if ctx == nil || in.Actor.UserID.IsZero() || in.Actor.SessionID.IsZero() || in.Actor.ContextGeneration < 1 || in.Actor.SupportSession || in.ChangeID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedVersion < 1 || (in.Decision != "APPROVED" && in.Decision != "REJECTED") || in.Reason == "" || len(in.Reason) > 2000 || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 200 {
		return out, ErrAdjustmentRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrAdjustmentUnavailable
	}
	operation := map[string]string{"APPROVED": "APPROVE_ADJUSTMENT", "REJECTED": "REJECT_ADJUSTMENT"}[in.Decision]
	key := sha256.Sum256([]byte("commercial-adjustment-decide:v1:" + operation + ":" + in.IdempotencyKey))
	request, e := adjustmentHash("DECIDE", in)
	if e != nil {
		return out, ErrAdjustmentRequestInvalid
	}
	ids, e := paymentIDs(s.ids, 6)
	if e != nil {
		return out, e
	}
	err = s.uow.WithinAdjustment(ctx, func(c context.Context, tx AdjustmentTx) error {
		tenantID, targetFound, x := tx.ResolveAdjustmentChangeTenant(c, in.ChangeID)
		if x != nil {
			return x
		}
		if !targetFound {
			return ErrAdjustmentNotFound
		}
		auth, tie, x := tx.ResolveAdjustmentAuthority(c, in.Actor, tenantID, at)
		if x != nil {
			return x
		}
		if tie || auth.UserID != in.Actor.UserID || auth.ContextGeneration != in.Actor.ContextGeneration || !auth.PlatformOwner || !assurance.Platform(auth.AssuranceMethod) || auth.AssuredAt.After(at) || auth.AssuredAt.Before(at.Add(-15*time.Minute)) {
			return ErrAdjustmentNotPermitted
		}
		cmd, found, x := tx.GetAdjustmentCommand(c, in.Actor.UserID, operation, key)
		if x != nil {
			return x
		}
		if found {
			if cmd.RequestHash != request {
				return ErrAdjustmentConflict
			}
			var replayFound bool
			out, replayFound, x = tx.GetAdjustment(c, tenantID, cmd.TargetID, true)
			if x != nil {
				return x
			}
			if !replayFound {
				return ErrAdjustmentConflict
			}
			return nil
		}
		stored, ok, x := tx.LockAdjustment(c, tenantID, in.ChangeID)
		if x != nil {
			return x
		}
		if !ok {
			return ErrAdjustmentNotFound
		}
		if stored.State != "PENDING_APPROVAL" || stored.Proposal.Version != in.ExpectedVersion || stored.Proposal.MakerUserID == in.Actor.UserID || at.After(stored.Proposal.ExpiresAt) {
			return ErrAdjustmentApprovalRequired
		}
		if in.Decision == "APPROVED" && (stored.InvoiceState != "PAID" || stored.PaymentState != "SETTLED" || stored.CurrentInvoiceVersion != stored.Proposal.ExpectedInvoiceVersion || stored.CurrentPaymentVersion != stored.Proposal.ExpectedPaymentVersion || stored.PriorCreditMinor < 0 || stored.Proposal.AmountMinorUnits > stored.InvoiceTotalMinor-stored.PriorCreditMinor) {
			return ErrAdjustmentConflict
		}
		authHash, x := domain.AdjustmentAuthorityHash(auth.UserID, auth.ContextGeneration, auth.AdministratorVersion, "PLATFORM_OWNER", "platform.governance.approve", auth.AssuranceMethod, auth.AssuredAt)
		if x != nil {
			return ErrAdjustmentNotPermitted
		}
		decision := AdjustmentDecision{ChangeID: in.ChangeID, TenantID: tenantID, CheckerUserID: in.Actor.UserID, DecisionID: ids[0], CreditNoteID: ids[1], RefundID: ids[2], CommandID: ids[3], ExpectedVersion: in.ExpectedVersion, Decision: in.Decision, Reason: in.Reason, AuthorityHash: authHash, RequestHash: request, KeyHash: key, At: at}
		if in.Decision == "APPROVED" {
			seq, x := tx.LockCreditNoteNumber(c, at.In(s.zone).Year(), at)
			if x != nil {
				return x
			}
			decision.CreditNumber, x = domain.FormatFinancialNumber("CREDIT_NOTE", at.In(s.zone).Year(), seq)
			if x != nil {
				return ErrAdjustmentUnavailable
			}
			decision.CreditState = map[string]string{"ISSUE_CREDIT": "ISSUED", "REQUEST_REFUND": "REFUND_PENDING"}[stored.Proposal.Action]
			decision.CreditHash, x = domain.CreditNoteHash(stored.Proposal, decision.CreditNumber, decision.CreditState, in.Actor.UserID, at)
			if x != nil {
				return ErrAdjustmentConflict
			}
		}
		evidence := AdjustmentEvidence{AuditID: ids[4], OutboxID: ids[5], CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TenantID: tenantID, InvoiceID: stored.Proposal.InvoiceID, PaymentID: stored.Proposal.PaymentID, ChangeID: in.ChangeID, EventType: map[string]string{"APPROVED": "commercial.financial_adjustment.approved.v1", "REJECTED": "commercial.financial_adjustment.rejected.v1"}[in.Decision], State: in.Decision, Action: stored.Proposal.Action, ReasonCode: stored.Proposal.ReasonCode, EntitlementDisposition: stored.Proposal.EntitlementDisposition, Version: in.ExpectedVersion + 1, AmountMinorUnits: stored.Proposal.AmountMinorUnits, OccurredAt: at}
		if x = tx.CommitAdjustmentDecision(c, decision, stored, evidence); x != nil {
			return x
		}
		stored.State = in.Decision
		stored.Proposal.Version++
		stored.CheckerUserID = in.Actor.UserID
		if in.Decision == "APPROVED" {
			stored.CreditNoteID = decision.CreditNoteID
			if stored.Proposal.Action == "REQUEST_REFUND" {
				stored.RefundID = decision.RefundID
			}
		}
		stored.CreditNumber = decision.CreditNumber
		out = stored
		return nil
	})
	if err != nil {
		return AdjustmentStored{}, mapAdjustmentError(err)
	}
	return out, nil
}

func adjustmentHash(kind string, v any) ([32]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return [32]byte{}, e
	}
	return sha256.Sum256(append([]byte("commercial-adjustment:"+kind+":v1:"), b...)), nil
}
func mapAdjustmentError(e error) error {
	switch {
	case e == nil:
		return nil
	case errors.Is(e, ErrAdjustmentRequestInvalid), errors.Is(e, ErrAdjustmentNotFound), errors.Is(e, ErrAdjustmentNotPermitted), errors.Is(e, ErrAdjustmentApprovalRequired), errors.Is(e, ErrAdjustmentConflict):
		return e
	default:
		return ErrAdjustmentUnavailable
	}
}
