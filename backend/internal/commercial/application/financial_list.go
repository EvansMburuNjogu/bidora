package application

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrFinancialListInvalid = errors.New("commercial_financial_list_invalid")
var ErrFinancialListNotAuthorized = errors.New("commercial_financial_list_not_authorized")
var ErrFinancialListUnavailable = errors.New("commercial_financial_list_unavailable")

type CommercialListCursor struct {
	OccurredAt time.Time
	Kind       string
	ID         domain.ID
}
type FinancialListRequest struct {
	Access SubscriptionAccess
	Limit  int32
	Cursor *CommercialListCursor
}
type InvoiceSummary struct {
	ID, SubscriptionID                                                                           domain.ID
	Number, State, PurchaseKind, CatalogueVersion, Currency                                      string
	PlanKey                                                                                      domain.PlanKey
	Cadence                                                                                      domain.BillingCadence
	Version, Quantity, SubtotalMinorUnits, TaxMinorUnits, TotalMinorUnits, OutstandingMinorUnits int64
	IssuedAt, DueAt                                                                              time.Time
}
type PaymentSummary struct {
	ID, InvoiceID             domain.ID
	State, Currency           string
	ReconciliationPending     bool
	Version, AmountMinorUnits int64
	CreatedAt, UpdatedAt      time.Time
	ReconciledAt              *time.Time
}
type FinancialHistoryItem struct {
	ID, InvoiceID, PaymentID              domain.ID
	Kind, DocumentNumber, State, Currency string
	AmountMinorUnits                      int64
	OccurredAt                            time.Time
}
type InvoiceSummaryPage struct {
	Items      []InvoiceSummary
	NextCursor *CommercialListCursor
}
type PaymentSummaryPage struct {
	Items      []PaymentSummary
	NextCursor *CommercialListCursor
}
type FinancialHistoryPage struct {
	Items      []FinancialHistoryItem
	NextCursor *CommercialListCursor
}

type FinancialListTx interface {
	Authorize(context.Context, SubscriptionAccess, string) (bool, error)
	ListInvoiceSummaries(context.Context, domain.ID, int32, *CommercialListCursor) ([]InvoiceSummary, error)
	ListPaymentSummaries(context.Context, domain.ID, int32, *CommercialListCursor) ([]PaymentSummary, error)
	ListFinancialHistory(context.Context, domain.ID, int32, *CommercialListCursor) ([]FinancialHistoryItem, error)
}
type FinancialListUnitOfWork interface {
	WithinFinancialList(context.Context, func(context.Context, FinancialListTx) error) error
}
type FinancialListService struct{ uow FinancialListUnitOfWork }

func NewFinancialListService(uow FinancialListUnitOfWork) (*FinancialListService, error) {
	if uow == nil {
		return nil, errors.New("commercial_financial_list_service_invalid")
	}
	return &FinancialListService{uow: uow}, nil
}
func (s *FinancialListService) ListInvoices(ctx context.Context, in FinancialListRequest) (result InvoiceSummaryPage, err error) {
	if invalidFinancialListRequest(ctx, in, false) {
		return result, ErrFinancialListInvalid
	}
	err = s.uow.WithinFinancialList(ctx, func(c context.Context, tx FinancialListTx) error {
		ok, e := tx.Authorize(c, in.Access, CapabilitySubscriptionRead)
		if e != nil {
			return e
		}
		if !ok {
			return ErrFinancialListNotAuthorized
		}
		items, e := tx.ListInvoiceSummaries(c, in.Access.TenantID, in.Limit+1, in.Cursor)
		if e != nil {
			return e
		}
		result.Items, result.NextCursor = invoicePage(items, in.Limit)
		return nil
	})
	if err != nil {
		return InvoiceSummaryPage{}, mapFinancialListError(err)
	}
	return result, nil
}
func (s *FinancialListService) ListPayments(ctx context.Context, in FinancialListRequest) (result PaymentSummaryPage, err error) {
	if invalidFinancialListRequest(ctx, in, false) {
		return result, ErrFinancialListInvalid
	}
	err = s.uow.WithinFinancialList(ctx, func(c context.Context, tx FinancialListTx) error {
		ok, e := tx.Authorize(c, in.Access, CapabilitySubscriptionRead)
		if e != nil {
			return e
		}
		if !ok {
			return ErrFinancialListNotAuthorized
		}
		items, e := tx.ListPaymentSummaries(c, in.Access.TenantID, in.Limit+1, in.Cursor)
		if e != nil {
			return e
		}
		result.Items, result.NextCursor = paymentPage(items, in.Limit)
		return nil
	})
	if err != nil {
		return PaymentSummaryPage{}, mapFinancialListError(err)
	}
	return result, nil
}
func (s *FinancialListService) ListHistory(ctx context.Context, in FinancialListRequest) (result FinancialHistoryPage, err error) {
	if invalidFinancialListRequest(ctx, in, true) {
		return result, ErrFinancialListInvalid
	}
	err = s.uow.WithinFinancialList(ctx, func(c context.Context, tx FinancialListTx) error {
		ok, e := tx.Authorize(c, in.Access, CapabilitySubscriptionRead)
		if e != nil {
			return e
		}
		if !ok {
			return ErrFinancialListNotAuthorized
		}
		items, e := tx.ListFinancialHistory(c, in.Access.TenantID, in.Limit+1, in.Cursor)
		if e != nil {
			return e
		}
		result.Items, result.NextCursor = historyPage(items, in.Limit)
		return nil
	})
	if err != nil {
		return FinancialHistoryPage{}, mapFinancialListError(err)
	}
	return result, nil
}
func invalidFinancialListRequest(ctx context.Context, in FinancialListRequest, history bool) bool {
	if invalidSubscriptionAccess(ctx, in.Access) || in.Limit < 1 || in.Limit > 100 {
		return true
	}
	if in.Cursor == nil {
		return false
	}
	if in.Cursor.OccurredAt.IsZero() || in.Cursor.ID.IsZero() {
		return true
	}
	if history {
		return !validHistoryKind(in.Cursor.Kind)
	}
	return in.Cursor.Kind != ""
}
func validHistoryKind(v string) bool {
	switch v {
	case "INVOICE", "PAYMENT", "RECEIPT", "CREDIT_NOTE", "REFUND_INTENT", "PAYMENT_REVERSAL":
		return true
	}
	return false
}
func invoicePage(values []InvoiceSummary, limit int32) ([]InvoiceSummary, *CommercialListCursor) {
	more := len(values) > int(limit)
	if more {
		values = values[:limit]
	}
	if !more || len(values) == 0 {
		return values, nil
	}
	last := values[len(values)-1]
	return values, &CommercialListCursor{OccurredAt: last.IssuedAt, ID: last.ID}
}
func paymentPage(values []PaymentSummary, limit int32) ([]PaymentSummary, *CommercialListCursor) {
	more := len(values) > int(limit)
	if more {
		values = values[:limit]
	}
	if !more || len(values) == 0 {
		return values, nil
	}
	last := values[len(values)-1]
	return values, &CommercialListCursor{OccurredAt: last.CreatedAt, ID: last.ID}
}
func historyPage(values []FinancialHistoryItem, limit int32) ([]FinancialHistoryItem, *CommercialListCursor) {
	more := len(values) > int(limit)
	if more {
		values = values[:limit]
	}
	if !more || len(values) == 0 {
		return values, nil
	}
	last := values[len(values)-1]
	return values, &CommercialListCursor{OccurredAt: last.OccurredAt, Kind: last.Kind, ID: last.ID}
}
func mapFinancialListError(e error) error {
	switch {
	case e == nil:
		return nil
	case errors.Is(e, ErrFinancialListInvalid), errors.Is(e, ErrFinancialListNotAuthorized):
		return e
	default:
		return ErrFinancialListUnavailable
	}
}
