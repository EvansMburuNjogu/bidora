package persistence

import (
	"context"
	"errors"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	commercialsql "bidora/backend/internal/commercial/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Store) WithinFinancialList(ctx context.Context, fn func(context.Context, commercialapp.FinancialListTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_financial_list_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := newTrialTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}
func (tx *trialTx) ListInvoiceSummaries(ctx context.Context, tenant domain.ID, limit int32, cursor *commercialapp.CommercialListCursor) ([]commercialapp.InvoiceSummary, error) {
	at, id := listCursor(cursor)
	rows, e := tx.commercial.ListTenantInvoiceSummaries(ctx, commercialsql.ListTenantInvoiceSummariesParams{TenantID: commercialID(tenant), CursorAt: at, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]commercialapp.InvoiceSummary, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.SubscriptionID.Valid || r.InvoiceNumber == "" || r.Version < 1 || r.Quantity < 1 || r.Currency != domain.CurrencyKES || r.SubtotalMinorUnits < 0 || r.TaxMinorUnits < 0 || r.TotalMinorUnits < 0 || r.OutstandingMinorUnits < 0 || !r.IssuedAt.Valid || !r.DueAt.Valid {
			return nil, errors.New("commercial_invoice_summary_corrupt")
		}
		out = append(out, commercialapp.InvoiceSummary{ID: domain.ID(r.ID.Bytes), SubscriptionID: domain.ID(r.SubscriptionID.Bytes), Number: r.InvoiceNumber, State: r.State, PurchaseKind: r.PurchaseKind, CatalogueVersion: r.CatalogueVersion, Currency: r.Currency, PlanKey: domain.PlanKey(r.PlanKey), Cadence: domain.BillingCadence(r.Cadence), Version: r.Version, Quantity: r.Quantity, SubtotalMinorUnits: r.SubtotalMinorUnits, TaxMinorUnits: r.TaxMinorUnits, TotalMinorUnits: r.TotalMinorUnits, OutstandingMinorUnits: r.OutstandingMinorUnits, IssuedAt: r.IssuedAt.Time.UTC(), DueAt: r.DueAt.Time.UTC()})
	}
	return out, nil
}
func (tx *trialTx) ListPaymentSummaries(ctx context.Context, tenant domain.ID, limit int32, cursor *commercialapp.CommercialListCursor) ([]commercialapp.PaymentSummary, error) {
	at, id := listCursor(cursor)
	rows, e := tx.commercial.ListTenantPaymentSummaries(ctx, commercialsql.ListTenantPaymentSummariesParams{TenantID: commercialID(tenant), CursorAt: at, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]commercialapp.PaymentSummary, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.InvoiceID.Valid || !validPaymentReadState(r.State) || !validReconciliationReadState(r.ReconciliationState) || r.Version < 1 || r.Currency != domain.CurrencyKES || r.AmountMinorUnits < 1 || !r.CreatedAt.Valid || !r.UpdatedAt.Valid {
			return nil, errors.New("commercial_payment_summary_corrupt")
		}
		v := commercialapp.PaymentSummary{ID: domain.ID(r.ID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), State: r.State, Currency: r.Currency, ReconciliationPending: r.ReconciliationState != "COMPLETE" && r.ReconciliationState != "NOT_REQUIRED", Version: r.Version, AmountMinorUnits: r.AmountMinorUnits, CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()}
		if r.ReconciledAt.Valid {
			x := r.ReconciledAt.Time.UTC()
			v.ReconciledAt = &x
		}
		out = append(out, v)
	}
	return out, nil
}
func (tx *trialTx) ListFinancialHistory(ctx context.Context, tenant domain.ID, limit int32, cursor *commercialapp.CommercialListCursor) ([]commercialapp.FinancialHistoryItem, error) {
	at, id := listCursor(cursor)
	var kind *string
	if cursor != nil {
		kind = &cursor.Kind
	}
	rows, e := tx.commercial.ListTenantFinancialHistory(ctx, commercialsql.ListTenantFinancialHistoryParams{TenantID: commercialID(tenant), CursorAt: at, CursorKind: kind, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]commercialapp.FinancialHistoryItem, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.InvoiceID.Valid || !validHistoryReadKind(r.Kind) || r.State == "" || r.Currency != domain.CurrencyKES || r.AmountMinorUnits < 1 || !r.OccurredAt.Valid || ((r.Kind == "INVOICE" || r.Kind == "RECEIPT" || r.Kind == "CREDIT_NOTE") && r.DocumentNumber == "") {
			return nil, errors.New("commercial_financial_history_corrupt")
		}
		out = append(out, commercialapp.FinancialHistoryItem{ID: domain.ID(r.ID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), Kind: r.Kind, DocumentNumber: r.DocumentNumber, State: r.State, Currency: r.Currency, AmountMinorUnits: r.AmountMinorUnits, OccurredAt: r.OccurredAt.Time.UTC()})
	}
	return out, nil
}
func listCursor(c *commercialapp.CommercialListCursor) (pgtype.Timestamptz, pgtype.UUID) {
	if c == nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	return commercialTime(c.OccurredAt), commercialID(c.ID)
}
func validPaymentReadState(v string) bool {
	switch v {
	case "CREATED", "PROVIDER_CREATE_PENDING", "AWAITING_CUSTOMER", "PROVIDER_PENDING", "SETTLED", "FAILED", "CANCELLED", "REVERSED", "REFUND_PENDING", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED":
		return true
	}
	return false
}
func validReconciliationReadState(v string) bool {
	switch v {
	case "NOT_REQUIRED", "DUE", "LEASED", "RETRY_WAIT", "HELD", "COMPLETE":
		return true
	}
	return false
}
func validHistoryReadKind(v string) bool {
	switch v {
	case "INVOICE", "PAYMENT", "RECEIPT", "CREDIT_NOTE", "REFUND_INTENT", "PAYMENT_REVERSAL":
		return true
	}
	return false
}
