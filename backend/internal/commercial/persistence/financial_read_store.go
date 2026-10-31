package persistence

import (
	"context"
	"errors"
	"time"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	commercialsql "bidora/backend/internal/commercial/persistence/sqlc"
	controlapp "bidora/backend/internal/controlplane/application"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

func (s *Store) WithinFinancialRead(ctx context.Context, fn func(context.Context, commercialapp.FinancialReadTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_financial_read_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := newTrialTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}
func (tx *trialTx) GetTenantPaymentView(ctx context.Context, tenant, payment domain.ID) (commercialapp.TenantPaymentRead, bool, error) {
	r, e := tx.commercial.GetTenantCommercialPaymentView(ctx, commercialsql.GetTenantCommercialPaymentViewParams{PaymentID: commercialID(payment), TenantID: commercialID(tenant)})
	if errors.Is(e, pgx.ErrNoRows) {
		return commercialapp.TenantPaymentRead{}, false, nil
	}
	if e != nil {
		return commercialapp.TenantPaymentRead{}, false, e
	}
	settled := r.EffectKind != nil
	if !r.ID.Valid || !r.InvoiceID.Valid || !validPaymentReadState(r.State) || !validReconciliationReadState(r.ReconciliationState) || r.Version < 1 || r.Currency != domain.CurrencyKES || r.AmountMinorUnits < 1 || !r.UpdatedAt.Valid || settled != r.ReceiptID.Valid || (settled && (!settlementDerivedPaymentState(r.State) || (*r.EffectKind != "PLAN_ACTIVATION" && *r.EffectKind != "ADDITIONAL_LICENCES"))) {
		return commercialapp.TenantPaymentRead{}, false, errors.New("commercial_payment_read_corrupt")
	}
	v := commercialapp.TenantPaymentRead{ID: domain.ID(r.ID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), ReceiptID: domain.ID(r.ReceiptID.Bytes), State: r.State, Currency: r.Currency, ReconciliationState: r.ReconciliationState, Version: r.Version, AmountMinorUnits: r.AmountMinorUnits, UpdatedAt: r.UpdatedAt.Time.UTC(), Paid: settled && r.State != "REVERSED" && r.State != "REFUNDED", EntitlementsActivated: settled && r.State != "REVERSED" && r.State != "REFUNDED"}
	if r.EffectKind != nil {
		v.EntitlementEffect = *r.EffectKind
	}
	return v, true, nil
}

func (tx *trialTx) GetTenantReceipt(ctx context.Context, tenant, id domain.ID) (domain.Receipt, bool, error) {
	r, e := tx.commercial.GetTenantCommercialReceipt(ctx, commercialsql.GetTenantCommercialReceiptParams{ReceiptID: commercialID(id), TenantID: commercialID(tenant)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Receipt{}, false, nil
	}
	if e != nil {
		return domain.Receipt{}, false, e
	}
	h, ok := hash32(r.ContentHash)
	if !ok || !r.ID.Valid || !r.PaymentID.Valid || !r.InvoiceID.Valid || r.ReceiptNumber == "" || r.Currency != domain.CurrencyKES || r.AmountMinorUnits < 1 || !r.IssuedAt.Valid {
		return domain.Receipt{}, false, errors.New("commercial_receipt_corrupt")
	}
	return domain.Receipt{ID: domain.ID(r.ID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), Number: r.ReceiptNumber, Currency: r.Currency, AmountMinorUnits: r.AmountMinorUnits, IssuedAt: r.IssuedAt.Time.UTC(), ContentHash: h}, true, nil
}
func (tx *trialTx) GetTenantCreditNote(ctx context.Context, tenant, id domain.ID) (domain.CreditNote, bool, error) {
	r, e := tx.commercial.GetTenantCommercialCreditNote(ctx, commercialsql.GetTenantCommercialCreditNoteParams{CreditNoteID: commercialID(id), TenantID: commercialID(tenant)})
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.CreditNote{}, false, nil
	}
	if e != nil {
		return domain.CreditNote{}, false, e
	}
	h, ok := hash32(r.ContentHash)
	if !ok {
		return domain.CreditNote{}, false, errors.New("commercial_credit_note_corrupt")
	}
	n := domain.CreditNote{ID: domain.ID(r.ID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), ChangeID: domain.ID(r.ChangeID.Bytes), Number: r.CreditNumber, State: r.State, ReasonCode: r.ReasonCode, Currency: r.Currency, Version: r.Version, NetMinorUnits: r.NetMinorUnits, TaxMinorUnits: r.TaxMinorUnits, TotalMinorUnits: r.TotalMinorUnits, IssuedAt: r.IssuedAt.Time.UTC(), ContentHash: h}
	if !n.Valid() {
		return domain.CreditNote{}, false, errors.New("commercial_credit_note_corrupt")
	}
	return n, true, nil
}
func (tx *trialTx) GetPlatformAdjustment(ctx context.Context, id domain.ID) (commercialapp.FinancialAdjustmentRead, bool, error) {
	r, e := tx.commercial.GetPlatformFinancialAdjustment(ctx, commercialID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return commercialapp.FinancialAdjustmentRead{}, false, nil
	}
	if e != nil {
		return commercialapp.FinancialAdjustmentRead{}, false, e
	}
	if !r.ID.Valid || !r.TenantID.Valid || !r.InvoiceID.Valid || !r.PaymentID.Valid || !r.MakerUserID.Valid || r.Version < 1 || r.AmountMinorUnits < 1 || r.NetMinorUnits < 0 || r.TaxMinorUnits < 0 || r.AmountMinorUnits != r.NetMinorUnits+r.TaxMinorUnits || !r.CreatedAt.Valid || !r.ExpiresAt.Valid || !validAdjustmentReadShape(r.State, r.CheckerUserID.Valid, r.DecidedAt.Valid, r.DecisionReason != nil) {
		return commercialapp.FinancialAdjustmentRead{}, false, errors.New("commercial_adjustment_read_corrupt")
	}
	v := commercialapp.FinancialAdjustmentRead{ID: domain.ID(r.ID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), MakerUserID: domain.ID(r.MakerUserID.Bytes), CheckerUserID: domain.ID(r.CheckerUserID.Bytes), CreditNoteID: domain.ID(r.CreditNoteID.Bytes), RefundID: domain.ID(r.RefundID.Bytes), Action: r.Action, State: r.State, ReasonCode: r.ReasonCode, EntitlementDisposition: r.EntitlementDisposition, ProposalReason: r.ProposalReason, Version: r.Version, AmountMinorUnits: r.AmountMinorUnits, NetMinorUnits: r.NetMinorUnits, TaxMinorUnits: r.TaxMinorUnits, CreatedAt: r.CreatedAt.Time.UTC(), ExpiresAt: r.ExpiresAt.Time.UTC()}
	if r.DecisionReason != nil {
		v.DecisionReason = *r.DecisionReason
	}
	if r.DecidedAt.Valid {
		x := r.DecidedAt.Time.UTC()
		v.DecidedAt = &x
	}
	if r.CreditNumber != nil {
		v.CreditNumber = *r.CreditNumber
	}
	if r.CreditNoteState != nil {
		v.CreditNoteState = *r.CreditNoteState
	}
	if r.RefundState != nil {
		v.RefundState = *r.RefundState
	}
	return v, true, nil
}
func (tx *trialTx) GetPlatformReversal(ctx context.Context, id domain.ID) (commercialapp.PaymentReversalRead, bool, error) {
	r, e := tx.commercial.GetPlatformPaymentReversal(ctx, commercialID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return commercialapp.PaymentReversalRead{}, false, nil
	}
	if e != nil {
		return commercialapp.PaymentReversalRead{}, false, e
	}
	if !r.PaymentID.Valid || !r.TenantID.Valid || !r.InvoiceID.Valid || !r.SubscriptionID.Valid || !r.SettlementObservationID.Valid || !r.ReversalObservationID.Valid || !r.AccessHoldID.Valid || r.Currency != domain.CurrencyKES || r.AmountMinorUnits < 1 || !r.RecordedAt.Valid || !r.AppliedAt.Valid || r.PaymentState != "REVERSED" || r.InvoiceState != "REVIEW_REQUIRED" || r.SubscriptionState != "SUSPENDED" || r.AccessMode != "RETAINED_READ_ONLY" || r.HoldState != "ACTIVE" {
		return commercialapp.PaymentReversalRead{}, false, errors.New("commercial_reversal_read_corrupt")
	}
	var observed *time.Time
	if r.ProviderObservedAt.Valid {
		x := r.ProviderObservedAt.Time.UTC()
		observed = &x
	}
	return commercialapp.PaymentReversalRead{PaymentID: domain.ID(r.PaymentID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), SubscriptionID: domain.ID(r.SubscriptionID.Bytes), SettlementObservationID: domain.ID(r.SettlementObservationID.Bytes), ReversalObservationID: domain.ID(r.ReversalObservationID.Bytes), AccessHoldID: domain.ID(r.AccessHoldID.Bytes), PaymentState: r.PaymentState, InvoiceState: r.InvoiceState, SubscriptionState: r.SubscriptionState, AccessMode: r.AccessMode, HoldState: r.HoldState, Currency: r.Currency, PriorPaymentVersion: r.PriorPaymentVersion, ResultingPaymentVersion: r.ResultingPaymentVersion, PriorInvoiceVersion: r.PriorInvoiceVersion, ResultingInvoiceVersion: r.ResultingInvoiceVersion, PriorSubscriptionVersion: r.PriorSubscriptionVersion, ResultingSubscriptionVersion: r.ResultingSubscriptionVersion, AmountMinorUnits: r.AmountMinorUnits, ProviderObservedAt: observed, RecordedAt: r.RecordedAt.Time.UTC(), AppliedAt: r.AppliedAt.Time.UTC()}, true, nil
}
func settlementDerivedPaymentState(state string) bool {
	switch state {
	case "SETTLED", "REVERSED", "REFUND_PENDING", "PARTIALLY_REFUNDED", "REFUNDED":
		return true
	}
	return false
}

func validAdjustmentReadShape(state string, checker, decided, reason bool) bool {
	switch state {
	case "PENDING_APPROVAL":
		return !checker && !decided && !reason
	case "APPROVED", "REJECTED", "REVIEW_REQUIRED":
		return checker && decided && reason
	case "EXPIRED":
		return !checker && decided && !reason
	}
	return false
}

func (tx *trialTx) AppendFinancialReadEvidence(ctx context.Context, e commercialapp.FinancialReadEvidence) error {
	return tx.evidence.AppendCommercialFinancialRead(ctx, controlapp.CommercialFinancialReadEvidence{AuditID: controlapp.ID(e.AuditID), CorrelationID: controlapp.ID(e.CorrelationID), ActorID: controlapp.ID(e.ActorID), TenantID: controlapp.ID(e.TenantID), TargetID: controlapp.ID(e.TargetID), Action: e.Action, TargetType: e.TargetType, OccurredAt: e.OccurredAt})
}
