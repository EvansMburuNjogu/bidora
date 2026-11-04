package application

import (
	"bidora/backend/internal/identity/assurance"
	"context"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrFinancialReadInvalid = errors.New("commercial_financial_read_invalid")
var ErrFinancialReadNotAuthorized = errors.New("commercial_financial_read_not_authorized")
var ErrFinancialReadNotFound = errors.New("commercial_financial_read_not_found")
var ErrFinancialReadUnavailable = errors.New("commercial_financial_read_unavailable")

type TenantPaymentRead struct {
	ID, InvoiceID, ReceiptID                                domain.ID
	State, Currency, ReconciliationState, EntitlementEffect string
	Version, AmountMinorUnits                               int64
	UpdatedAt                                               time.Time
	Paid, EntitlementsActivated                             bool
}

type FinancialAdjustmentRead struct {
	ID, TenantID, InvoiceID, PaymentID, MakerUserID, CheckerUserID, CreditNoteID, RefundID domain.ID
	Action, State, ReasonCode, EntitlementDisposition, ProposalReason, DecisionReason      string
	CreditNumber, CreditNoteState, RefundState                                             string
	Version, AmountMinorUnits, NetMinorUnits, TaxMinorUnits                                int64
	CreatedAt, ExpiresAt                                                                   time.Time
	DecidedAt                                                                              *time.Time
}
type PaymentReversalRead struct {
	PaymentID, TenantID, InvoiceID, SubscriptionID, SettlementObservationID, ReversalObservationID, AccessHoldID                                                         domain.ID
	PaymentState, InvoiceState, SubscriptionState, AccessMode, HoldState, Currency                                                                                       string
	PriorPaymentVersion, ResultingPaymentVersion, PriorInvoiceVersion, ResultingInvoiceVersion, PriorSubscriptionVersion, ResultingSubscriptionVersion, AmountMinorUnits int64
	ProviderObservedAt                                                                                                                                                   *time.Time
	RecordedAt, AppliedAt                                                                                                                                                time.Time
}
type FinancialReadEvidence struct {
	AuditID, CorrelationID, ActorID, TenantID, TargetID domain.ID
	Action, TargetType                                  string
	OccurredAt                                          time.Time
}

type FinancialReadTx interface {
	Authorize(context.Context, SubscriptionAccess, string) (bool, error)
	GetTenantPaymentView(context.Context, domain.ID, domain.ID) (TenantPaymentRead, bool, error)
	GetTenantReceipt(context.Context, domain.ID, domain.ID) (domain.Receipt, bool, error)
	GetTenantCreditNote(context.Context, domain.ID, domain.ID) (domain.CreditNote, bool, error)
	GetPlatformAdjustment(context.Context, domain.ID) (FinancialAdjustmentRead, bool, error)
	GetPlatformReversal(context.Context, domain.ID) (PaymentReversalRead, bool, error)
	ResolveAdjustmentAuthority(context.Context, AdjustmentActor, domain.ID, time.Time) (AdjustmentAuthority, bool, error)
	AppendFinancialReadEvidence(context.Context, FinancialReadEvidence) error
}
type FinancialReadUnitOfWork interface {
	WithinFinancialRead(context.Context, func(context.Context, FinancialReadTx) error) error
}
type PlatformFinancialReadInput struct {
	Actor                   AdjustmentActor
	TargetID, CorrelationID domain.ID
}

type FinancialReadService struct {
	uow   FinancialReadUnitOfWork
	ids   InvoiceIDGenerator
	clock InvoiceClock
}

func NewFinancialReadService(uow FinancialReadUnitOfWork, ids InvoiceIDGenerator, clock InvoiceClock) (*FinancialReadService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_financial_read_service_invalid")
	}
	return &FinancialReadService{uow: uow, ids: ids, clock: clock}, nil
}

func (s *FinancialReadService) ReadPayment(ctx context.Context, access SubscriptionAccess, id domain.ID) (result TenantPaymentRead, err error) {
	if invalidSubscriptionAccess(ctx, access) || id.IsZero() {
		return result, ErrFinancialReadInvalid
	}
	err = s.uow.WithinFinancialRead(ctx, func(c context.Context, tx FinancialReadTx) error {
		allowed, e := tx.Authorize(c, access, CapabilitySubscriptionRead)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrFinancialReadNotAuthorized
		}
		var found bool
		result, found, e = tx.GetTenantPaymentView(c, access.TenantID, id)
		if e != nil {
			return e
		}
		if !found {
			return ErrFinancialReadNotFound
		}
		return nil
	})
	if err != nil {
		return TenantPaymentRead{}, mapFinancialReadError(err)
	}
	return result, nil
}

func (s *FinancialReadService) ReadReceipt(ctx context.Context, access SubscriptionAccess, id domain.ID) (result domain.Receipt, err error) {
	if invalidSubscriptionAccess(ctx, access) || id.IsZero() {
		return result, ErrFinancialReadInvalid
	}
	err = s.uow.WithinFinancialRead(ctx, func(c context.Context, tx FinancialReadTx) error {
		allowed, e := tx.Authorize(c, access, CapabilitySubscriptionRead)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrFinancialReadNotAuthorized
		}
		var found bool
		result, found, e = tx.GetTenantReceipt(c, access.TenantID, id)
		if e != nil {
			return e
		}
		if !found {
			return ErrFinancialReadNotFound
		}
		return nil
	})
	if err != nil {
		return domain.Receipt{}, mapFinancialReadError(err)
	}
	return result, nil
}
func (s *FinancialReadService) ReadCreditNote(ctx context.Context, access SubscriptionAccess, id domain.ID) (result domain.CreditNote, err error) {
	if invalidSubscriptionAccess(ctx, access) || id.IsZero() {
		return result, ErrFinancialReadInvalid
	}
	err = s.uow.WithinFinancialRead(ctx, func(c context.Context, tx FinancialReadTx) error {
		allowed, e := tx.Authorize(c, access, CapabilitySubscriptionRead)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrFinancialReadNotAuthorized
		}
		var found bool
		result, found, e = tx.GetTenantCreditNote(c, access.TenantID, id)
		if e != nil {
			return e
		}
		if !found {
			return ErrFinancialReadNotFound
		}
		return nil
	})
	if err != nil {
		return domain.CreditNote{}, mapFinancialReadError(err)
	}
	return result, nil
}
func (s *FinancialReadService) ReadPlatformAdjustment(ctx context.Context, in PlatformFinancialReadInput) (result FinancialAdjustmentRead, err error) {
	if invalidPlatformFinancialRead(ctx, in) {
		return result, ErrFinancialReadInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrFinancialReadUnavailable
	}
	auditID, e := s.ids.NewID()
	if e != nil || auditID.IsZero() {
		return result, ErrFinancialReadUnavailable
	}
	err = s.uow.WithinFinancialRead(ctx, func(c context.Context, tx FinancialReadTx) error {
		record, found, x := tx.GetPlatformAdjustment(c, in.TargetID)
		if x != nil {
			return x
		}
		if !found {
			return ErrFinancialReadNotFound
		}
		if x = authorizePlatformFinancialRead(c, tx, in.Actor, record.TenantID, at, "platform.invoice.read"); x != nil {
			return x
		}
		if x = tx.AppendFinancialReadEvidence(c, FinancialReadEvidence{AuditID: auditID, CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TenantID: record.TenantID, TargetID: record.ID, Action: "CommercialFinancialAdjustmentRead", TargetType: "commercial_financial_adjustment", OccurredAt: at}); x != nil {
			return x
		}
		result = record
		return nil
	})
	if err != nil {
		return FinancialAdjustmentRead{}, mapFinancialReadError(err)
	}
	return result, nil
}
func (s *FinancialReadService) ReadPlatformReversal(ctx context.Context, in PlatformFinancialReadInput) (result PaymentReversalRead, err error) {
	if invalidPlatformFinancialRead(ctx, in) {
		return result, ErrFinancialReadInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrFinancialReadUnavailable
	}
	auditID, e := s.ids.NewID()
	if e != nil || auditID.IsZero() {
		return result, ErrFinancialReadUnavailable
	}
	err = s.uow.WithinFinancialRead(ctx, func(c context.Context, tx FinancialReadTx) error {
		record, found, x := tx.GetPlatformReversal(c, in.TargetID)
		if x != nil {
			return x
		}
		if !found {
			return ErrFinancialReadNotFound
		}
		if x = authorizePlatformFinancialRead(c, tx, in.Actor, record.TenantID, at, "platform.payment.read"); x != nil {
			return x
		}
		if x = tx.AppendFinancialReadEvidence(c, FinancialReadEvidence{AuditID: auditID, CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TenantID: record.TenantID, TargetID: record.PaymentID, Action: "CommercialPaymentReversalRead", TargetType: "commercial_payment_reversal", OccurredAt: at}); x != nil {
			return x
		}
		result = record
		return nil
	})
	if err != nil {
		return PaymentReversalRead{}, mapFinancialReadError(err)
	}
	return result, nil
}
func authorizePlatformFinancialRead(ctx context.Context, tx FinancialReadTx, actor AdjustmentActor, tenant domain.ID, at time.Time, capability string) error {
	facts, tie, e := tx.ResolveAdjustmentAuthority(ctx, actor, tenant, at)
	if e != nil {
		return e
	}
	allowed := (capability == "platform.invoice.read" && facts.InvoiceReader) || (capability == "platform.payment.read" && facts.PaymentReader)
	if tie || !allowed || facts.UserID != actor.UserID || facts.ContextGeneration != actor.ContextGeneration || (!facts.BillingAdministrator && !facts.PlatformOwner) || !assurance.Platform(facts.AssuranceMethod) || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) {
		return ErrFinancialReadNotAuthorized
	}
	return nil
}
func invalidPlatformFinancialRead(ctx context.Context, in PlatformFinancialReadInput) bool {
	return ctx == nil || in.Actor.UserID.IsZero() || in.Actor.SessionID.IsZero() || in.Actor.ContextGeneration < 1 || in.Actor.SupportSession || in.TargetID.IsZero() || in.CorrelationID.IsZero()
}
func mapFinancialReadError(e error) error {
	switch {
	case e == nil:
		return nil
	case errors.Is(e, ErrFinancialReadInvalid), errors.Is(e, ErrFinancialReadNotAuthorized), errors.Is(e, ErrFinancialReadNotFound):
		return e
	default:
		return ErrFinancialReadUnavailable
	}
}
