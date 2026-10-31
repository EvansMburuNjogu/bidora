package application

import (
	"bidora/backend/internal/commercial/domain"
	"context"
	"errors"
	"time"
)

var ErrPlatformFinancialQueueInvalid = errors.New("commercial_platform_financial_queue_invalid")
var ErrPlatformFinancialQueueNotAuthorized = errors.New("commercial_platform_financial_queue_not_authorized")
var ErrPlatformFinancialQueueUnavailable = errors.New("commercial_platform_financial_queue_unavailable")

type PlatformQueueRequest struct {
	Actor                        AdjustmentActor
	CorrelationID                domain.ID
	Limit                        int32
	Cursor                       *CommercialListCursor
	StateFilter, SecondaryFilter string
}
type PlatformAIUsageItem struct {
	ID, TenantID  domain.ID
	Kind          domain.AIUsageKind
	ActualCredits int64
	SettledAt     time.Time
}
type PlatformAIUsagePage struct {
	Items      []PlatformAIUsageItem
	NextCursor *CommercialListCursor
}
type PlatformInvoiceQueueItem struct {
	ID, TenantID, SubscriptionID                    domain.ID
	Number, State, PurchaseKind, Currency           string
	Version, TotalMinorUnits, OutstandingMinorUnits int64
	IssuedAt, DueAt                                 time.Time
}
type PlatformPaymentQueueItem struct {
	ID, TenantID, InvoiceID              domain.ID
	State, Currency, ReconciliationState string
	Version, AmountMinorUnits            int64
	ReconciliationPending                bool
	CreatedAt, UpdatedAt                 time.Time
	ReconciledAt                         *time.Time
}
type PlatformAdjustmentQueueItem struct {
	ID, TenantID, InvoiceID, PaymentID, MakerUserID, CheckerUserID domain.ID
	Action, State, ReasonCode, EntitlementDisposition              string
	Version, AmountMinorUnits, NetMinorUnits, TaxMinorUnits        int64
	CreatedAt, ExpiresAt                                           time.Time
	DecidedAt                                                      *time.Time
}
type PlatformReversalQueueItem struct {
	ID, PaymentID, TenantID, InvoiceID, ActorUserID                    domain.ID
	ExpectedPaymentVersion, AmountMinorUnits                           int64
	PaymentState, ReconciliationState, QueueState, Currency, HoldState string
	RequestedAt                                                        time.Time
	AppliedAt                                                          *time.Time
}
type PlatformInvoiceQueuePage struct {
	Items      []PlatformInvoiceQueueItem
	NextCursor *CommercialListCursor
}
type PlatformPaymentQueuePage struct {
	Items      []PlatformPaymentQueueItem
	NextCursor *CommercialListCursor
}
type PlatformAdjustmentQueuePage struct {
	Items      []PlatformAdjustmentQueueItem
	NextCursor *CommercialListCursor
}
type PlatformReversalQueuePage struct {
	Items      []PlatformReversalQueueItem
	NextCursor *CommercialListCursor
}
type PlatformQueueReadEvidence struct {
	AuditID, CorrelationID, ActorID, TargetID domain.ID
	Queue, StateFilter, SecondaryFilter       string
	ResultCount                               int
	HasMore                                   bool
	OccurredAt                                time.Time
}
type PlatformFinancialQueueTx interface {
	ResolveGlobalFinancialAuthority(context.Context, AdjustmentActor, time.Time) (AdjustmentAuthority, bool, error)
	ListPlatformAIUsageSettlements(context.Context, int32, *CommercialListCursor) ([]PlatformAIUsageItem, error)
	ListPlatformInvoices(context.Context, int32, *CommercialListCursor, string) ([]PlatformInvoiceQueueItem, error)
	ListPlatformPayments(context.Context, int32, *CommercialListCursor, string, string) ([]PlatformPaymentQueueItem, error)
	ListPlatformAdjustments(context.Context, int32, *CommercialListCursor, string, string) ([]PlatformAdjustmentQueueItem, error)
	ListPlatformReversals(context.Context, int32, *CommercialListCursor, string) ([]PlatformReversalQueueItem, error)
	AppendPlatformQueueReadEvidence(context.Context, PlatformQueueReadEvidence) error
}
type PlatformFinancialQueueUnitOfWork interface {
	WithinPlatformFinancialQueue(context.Context, func(context.Context, PlatformFinancialQueueTx) error) error
}
type PlatformFinancialQueueService struct {
	uow   PlatformFinancialQueueUnitOfWork
	ids   InvoiceIDGenerator
	clock InvoiceClock
}

func NewPlatformFinancialQueueService(uow PlatformFinancialQueueUnitOfWork, ids InvoiceIDGenerator, clock InvoiceClock) (*PlatformFinancialQueueService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_platform_financial_queue_service_invalid")
	}
	return &PlatformFinancialQueueService{uow: uow, ids: ids, clock: clock}, nil
}

// ListAIUsage exposes only measured, settled credits. A current Billing Administrator
// with fresh TOTP is required; Platform Owner and invoice-read alone are insufficient.
func (s *PlatformFinancialQueueService) ListAIUsage(ctx context.Context, in PlatformQueueRequest) (out PlatformAIUsagePage, err error) {
	if invalidPlatformQueueRequest(ctx, in) || in.StateFilter != "" || in.SecondaryFilter != "" {
		return out, ErrPlatformFinancialQueueInvalid
	}
	at := s.clock.Now().UTC()
	audit, e := s.ids.NewID()
	if at.IsZero() || e != nil || audit.IsZero() {
		return out, ErrPlatformFinancialQueueUnavailable
	}
	err = s.uow.WithinPlatformFinancialQueue(ctx, func(c context.Context, tx PlatformFinancialQueueTx) error {
		facts, tie, x := tx.ResolveGlobalFinancialAuthority(c, in.Actor, at)
		if x != nil {
			return x
		}
		if tie || !facts.BillingAdministrator || facts.UserID != in.Actor.UserID || facts.ContextGeneration != in.Actor.ContextGeneration || facts.AssuranceMethod != "AAL2:TOTP" || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) || in.Actor.SupportSession {
			return ErrPlatformFinancialQueueNotAuthorized
		}
		items, x := tx.ListPlatformAIUsageSettlements(c, in.Limit+1, in.Cursor)
		if x != nil {
			return x
		}
		if len(items) > int(in.Limit) {
			items = items[:in.Limit]
			last := items[len(items)-1]
			out.NextCursor = &CommercialListCursor{OccurredAt: last.SettledAt, ID: last.ID}
		}
		out.Items = items
		return tx.AppendPlatformQueueReadEvidence(c, PlatformQueueReadEvidence{AuditID: audit, CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TargetID: audit, Queue: "AI_USAGE", ResultCount: len(items), HasMore: out.NextCursor != nil, OccurredAt: at})
	})
	if err != nil {
		return PlatformAIUsagePage{}, mapPlatformQueueError(err)
	}
	return out, nil
}
func (s *PlatformFinancialQueueService) ListInvoices(ctx context.Context, in PlatformQueueRequest) (out PlatformInvoiceQueuePage, err error) {
	if invalidPlatformQueueRequest(ctx, in) || !validOptionalInvoiceState(in.StateFilter) || in.SecondaryFilter != "" {
		return out, ErrPlatformFinancialQueueInvalid
	}
	err = s.run(ctx, in, "INVOICE", "platform.invoice.read", func(c context.Context, tx PlatformFinancialQueueTx) (int, bool, error) {
		v, e := tx.ListPlatformInvoices(c, in.Limit+1, in.Cursor, in.StateFilter)
		if e != nil {
			return 0, false, e
		}
		out.Items, out.NextCursor = platformInvoicePage(v, in.Limit)
		return len(out.Items), out.NextCursor != nil, nil
	})
	if err != nil {
		return PlatformInvoiceQueuePage{}, err
	}
	return out, nil
}
func (s *PlatformFinancialQueueService) ListPayments(ctx context.Context, in PlatformQueueRequest) (out PlatformPaymentQueuePage, err error) {
	if invalidPlatformQueueRequest(ctx, in) || !validOptionalPaymentState(in.StateFilter) || !validOptionalReconciliationState(in.SecondaryFilter) {
		return out, ErrPlatformFinancialQueueInvalid
	}
	err = s.run(ctx, in, "PAYMENT", "platform.payment.read", func(c context.Context, tx PlatformFinancialQueueTx) (int, bool, error) {
		v, e := tx.ListPlatformPayments(c, in.Limit+1, in.Cursor, in.StateFilter, in.SecondaryFilter)
		if e != nil {
			return 0, false, e
		}
		out.Items, out.NextCursor = platformPaymentPage(v, in.Limit)
		return len(out.Items), out.NextCursor != nil, nil
	})
	if err != nil {
		return PlatformPaymentQueuePage{}, err
	}
	return out, nil
}
func (s *PlatformFinancialQueueService) ListAdjustments(ctx context.Context, in PlatformQueueRequest) (out PlatformAdjustmentQueuePage, err error) {
	if invalidPlatformQueueRequest(ctx, in) || !validOptionalAdjustmentState(in.StateFilter) || !validOptionalAdjustmentAction(in.SecondaryFilter) {
		return out, ErrPlatformFinancialQueueInvalid
	}
	err = s.run(ctx, in, "ADJUSTMENT", "platform.invoice.read", func(c context.Context, tx PlatformFinancialQueueTx) (int, bool, error) {
		v, e := tx.ListPlatformAdjustments(c, in.Limit+1, in.Cursor, in.StateFilter, in.SecondaryFilter)
		if e != nil {
			return 0, false, e
		}
		out.Items, out.NextCursor = platformAdjustmentPage(v, in.Limit)
		return len(out.Items), out.NextCursor != nil, nil
	})
	if err != nil {
		return PlatformAdjustmentQueuePage{}, err
	}
	return out, nil
}
func (s *PlatformFinancialQueueService) ListReversals(ctx context.Context, in PlatformQueueRequest) (out PlatformReversalQueuePage, err error) {
	if invalidPlatformQueueRequest(ctx, in) || !validOptionalReversalQueueState(in.StateFilter) || in.SecondaryFilter != "" {
		return out, ErrPlatformFinancialQueueInvalid
	}
	err = s.run(ctx, in, "REVERSAL", "platform.payment.read", func(c context.Context, tx PlatformFinancialQueueTx) (int, bool, error) {
		v, e := tx.ListPlatformReversals(c, in.Limit+1, in.Cursor, in.StateFilter)
		if e != nil {
			return 0, false, e
		}
		out.Items, out.NextCursor = platformReversalPage(v, in.Limit)
		return len(out.Items), out.NextCursor != nil, nil
	})
	if err != nil {
		return PlatformReversalQueuePage{}, err
	}
	return out, nil
}
func (s *PlatformFinancialQueueService) run(ctx context.Context, in PlatformQueueRequest, queue, capability string, load func(context.Context, PlatformFinancialQueueTx) (int, bool, error)) (err error) {
	at := s.clock.Now().UTC()
	audit, e := s.ids.NewID()
	if at.IsZero() || e != nil || audit.IsZero() {
		return ErrPlatformFinancialQueueUnavailable
	}
	err = s.uow.WithinPlatformFinancialQueue(ctx, func(c context.Context, tx PlatformFinancialQueueTx) error {
		facts, tie, x := tx.ResolveGlobalFinancialAuthority(c, in.Actor, at)
		if x != nil {
			return x
		}
		allowed := (capability == "platform.invoice.read" && facts.InvoiceReader) || (capability == "platform.payment.read" && facts.PaymentReader)
		if tie || !allowed || (!facts.BillingAdministrator && !facts.PlatformOwner) || facts.UserID != in.Actor.UserID || facts.ContextGeneration != in.Actor.ContextGeneration || facts.AssuranceMethod != "AAL2:TOTP" || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) || in.Actor.SupportSession {
			return ErrPlatformFinancialQueueNotAuthorized
		}
		count, more, x := load(c, tx)
		if x != nil {
			return x
		}
		return tx.AppendPlatformQueueReadEvidence(c, PlatformQueueReadEvidence{AuditID: audit, CorrelationID: in.CorrelationID, ActorID: in.Actor.UserID, TargetID: audit, Queue: queue, StateFilter: in.StateFilter, SecondaryFilter: in.SecondaryFilter, ResultCount: count, HasMore: more, OccurredAt: at})
	})
	return mapPlatformQueueError(err)
}
func invalidPlatformQueueRequest(ctx context.Context, in PlatformQueueRequest) bool {
	return ctx == nil || in.Actor.UserID.IsZero() || in.Actor.SessionID.IsZero() || in.Actor.ContextGeneration < 1 || in.Actor.SupportSession || in.CorrelationID.IsZero() || in.Limit < 1 || in.Limit > 100 || in.Cursor != nil && (in.Cursor.OccurredAt.IsZero() || in.Cursor.ID.IsZero() || in.Cursor.Kind != "")
}
func validOptionalInvoiceState(v string) bool {
	if v == "" {
		return true
	}
	switch v {
	case "ISSUED", "PAYMENT_PENDING", "PAID", "PARTIALLY_CREDITED", "CREDITED", "VOID", "REVIEW_REQUIRED":
		return true
	}
	return false
}
func validOptionalPaymentState(v string) bool {
	if v == "" {
		return true
	}
	return validPlatformPaymentState(v)
}
func validOptionalReconciliationState(v string) bool {
	if v == "" {
		return true
	}
	return validPlatformReconciliationState(v)
}
func validPlatformPaymentState(v string) bool {
	switch v {
	case "CREATED", "PROVIDER_CREATE_PENDING", "AWAITING_CUSTOMER", "PROVIDER_PENDING", "SETTLED", "FAILED", "CANCELLED", "REVERSED", "REFUND_PENDING", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED":
		return true
	}
	return false
}
func validPlatformReconciliationState(v string) bool {
	switch v {
	case "NOT_REQUIRED", "DUE", "LEASED", "RETRY_WAIT", "HELD", "COMPLETE":
		return true
	}
	return false
}
func validOptionalAdjustmentState(v string) bool {
	if v == "" {
		return true
	}
	switch v {
	case "PENDING_APPROVAL", "APPROVED", "REJECTED", "EXPIRED", "REVIEW_REQUIRED":
		return true
	}
	return false
}
func validOptionalAdjustmentAction(v string) bool {
	if v == "" {
		return true
	}
	switch v {
	case "ISSUE_CREDIT", "REQUEST_REFUND":
		return true
	}
	return false
}
func validOptionalReversalQueueState(v string) bool {
	if v == "" {
		return true
	}
	switch v {
	case "PENDING", "CONFIRMED", "HELD", "APPLIED":
		return true
	}
	return false
}
func platformInvoicePage(v []PlatformInvoiceQueueItem, n int32) ([]PlatformInvoiceQueueItem, *CommercialListCursor) {
	if len(v) <= int(n) {
		return v, nil
	}
	v = v[:n]
	x := v[len(v)-1]
	return v, &CommercialListCursor{OccurredAt: x.IssuedAt, ID: x.ID}
}
func platformPaymentPage(v []PlatformPaymentQueueItem, n int32) ([]PlatformPaymentQueueItem, *CommercialListCursor) {
	if len(v) <= int(n) {
		return v, nil
	}
	v = v[:n]
	x := v[len(v)-1]
	return v, &CommercialListCursor{OccurredAt: x.CreatedAt, ID: x.ID}
}
func platformAdjustmentPage(v []PlatformAdjustmentQueueItem, n int32) ([]PlatformAdjustmentQueueItem, *CommercialListCursor) {
	if len(v) <= int(n) {
		return v, nil
	}
	v = v[:n]
	x := v[len(v)-1]
	return v, &CommercialListCursor{OccurredAt: x.CreatedAt, ID: x.ID}
}
func platformReversalPage(v []PlatformReversalQueueItem, n int32) ([]PlatformReversalQueueItem, *CommercialListCursor) {
	if len(v) <= int(n) {
		return v, nil
	}
	v = v[:n]
	x := v[len(v)-1]
	return v, &CommercialListCursor{OccurredAt: x.RequestedAt, ID: x.ID}
}
func mapPlatformQueueError(e error) error {
	switch {
	case e == nil:
		return nil
	case errors.Is(e, ErrPlatformFinancialQueueInvalid), errors.Is(e, ErrPlatformFinancialQueueNotAuthorized):
		return e
	default:
		return ErrPlatformFinancialQueueUnavailable
	}
}
