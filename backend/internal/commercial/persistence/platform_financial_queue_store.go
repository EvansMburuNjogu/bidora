package persistence

import (
	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	commercialsql "bidora/backend/internal/commercial/persistence/sqlc"
	controlapp "bidora/backend/internal/controlplane/application"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformdomain "bidora/backend/internal/platformadmin/domain"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"time"
)

func (s *Store) WithinPlatformFinancialQueue(ctx context.Context, fn func(context.Context, commercialapp.PlatformFinancialQueueTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_platform_financial_queue_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := newTrialTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}
func (tx *trialTx) ResolveGlobalFinancialAuthority(ctx context.Context, actor commercialapp.AdjustmentActor, at time.Time) (commercialapp.AdjustmentAuthority, bool, error) {
	facts, tie, e := tx.financialAuthority.ResolveGlobal(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(actor.UserID), SessionID: platformapp.ID(actor.SessionID), ObservedAt: at})
	if errors.Is(e, platformapp.ErrAuthorityNotFound) {
		return commercialapp.AdjustmentAuthority{}, false, nil
	}
	if e != nil {
		return commercialapp.AdjustmentAuthority{}, false, e
	}
	billing, owner := false, false
	for _, r := range facts.Roles {
		billing = billing || r == platformdomain.BillingAdministrator
		owner = owner || r == platformdomain.PlatformOwner
	}
	return commercialapp.AdjustmentAuthority{UserID: domain.ID(facts.UserID), ContextGeneration: facts.ContextGeneration, AdministratorVersion: facts.AdministratorVersion, BillingAdministrator: billing, PlatformOwner: owner, InvoiceReader: facts.Grants("platform.invoice.read"), PaymentReader: facts.Grants("platform.payment.read"), AssuranceMethod: facts.AssuranceLevel + ":" + facts.AssuranceMethod, AssuredAt: facts.AssuredAt.UTC()}, tie, nil
}
func (tx *trialTx) ListPlatformAIUsageSettlements(ctx context.Context, limit int32, cursor *commercialapp.CommercialListCursor) ([]commercialapp.PlatformAIUsageItem, error) {
	at, id := listCursor(cursor)
	rows, err := tx.commercial.ListPlatformAIUsageSettlements(ctx, commercialsql.ListPlatformAIUsageSettlementsParams{CursorAt: at, CursorID: id, PageLimit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]commercialapp.PlatformAIUsageItem, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.TenantID.Valid || !domain.ValidAIUsageKind(domain.AIUsageKind(r.Kind)) || r.ActualCredits == nil || *r.ActualCredits < 1 || !r.SettledAt.Valid || r.SettledAt.Time.IsZero() {
			return nil, errors.New("commercial_platform_ai_usage_corrupt")
		}
		out = append(out, commercialapp.PlatformAIUsageItem{ID: domain.ID(r.ID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), Kind: domain.AIUsageKind(r.Kind), ActualCredits: *r.ActualCredits, SettledAt: r.SettledAt.Time.UTC()})
	}
	return out, nil
}
func (tx *trialTx) ListPlatformInvoices(ctx context.Context, limit int32, cursor *commercialapp.CommercialListCursor, state string) ([]commercialapp.PlatformInvoiceQueueItem, error) {
	at, id := listCursor(cursor)
	rows, e := tx.commercial.ListPlatformInvoiceQueue(ctx, commercialsql.ListPlatformInvoiceQueueParams{StateFilter: queueFilter(state), CursorAt: at, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]commercialapp.PlatformInvoiceQueueItem, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.TenantID.Valid || !r.SubscriptionID.Valid || r.InvoiceNumber == "" || r.State == "" || r.Version < 1 || r.Currency != domain.CurrencyKES || r.TotalMinorUnits < 1 || r.OutstandingMinorUnits < 0 || !r.IssuedAt.Valid || !r.DueAt.Valid {
			return nil, errors.New("commercial_platform_invoice_queue_corrupt")
		}
		out = append(out, commercialapp.PlatformInvoiceQueueItem{ID: domain.ID(r.ID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), SubscriptionID: domain.ID(r.SubscriptionID.Bytes), Number: r.InvoiceNumber, State: r.State, PurchaseKind: r.PurchaseKind, Currency: r.Currency, Version: r.Version, TotalMinorUnits: r.TotalMinorUnits, OutstandingMinorUnits: r.OutstandingMinorUnits, IssuedAt: r.IssuedAt.Time.UTC(), DueAt: r.DueAt.Time.UTC()})
	}
	return out, nil
}
func (tx *trialTx) ListPlatformPayments(ctx context.Context, limit int32, cursor *commercialapp.CommercialListCursor, state, reconciliation string) ([]commercialapp.PlatformPaymentQueueItem, error) {
	at, id := listCursor(cursor)
	rows, e := tx.commercial.ListPlatformPaymentQueue(ctx, commercialsql.ListPlatformPaymentQueueParams{StateFilter: queueFilter(state), ReconciliationFilter: queueFilter(reconciliation), CursorAt: at, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]commercialapp.PlatformPaymentQueueItem, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.TenantID.Valid || !r.InvoiceID.Valid || !validPaymentReadState(r.State) || !validReconciliationReadState(r.ReconciliationState) || r.Version < 1 || r.Currency != domain.CurrencyKES || r.AmountMinorUnits < 1 || !r.CreatedAt.Valid || !r.UpdatedAt.Valid {
			return nil, errors.New("commercial_platform_payment_queue_corrupt")
		}
		v := commercialapp.PlatformPaymentQueueItem{ID: domain.ID(r.ID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), State: r.State, Currency: r.Currency, ReconciliationState: r.ReconciliationState, Version: r.Version, AmountMinorUnits: r.AmountMinorUnits, ReconciliationPending: r.ReconciliationState != "COMPLETE" && r.ReconciliationState != "NOT_REQUIRED", CreatedAt: r.CreatedAt.Time.UTC(), UpdatedAt: r.UpdatedAt.Time.UTC()}
		if r.ReconciledAt.Valid {
			x := r.ReconciledAt.Time.UTC()
			v.ReconciledAt = &x
		}
		out = append(out, v)
	}
	return out, nil
}
func (tx *trialTx) ListPlatformAdjustments(ctx context.Context, limit int32, cursor *commercialapp.CommercialListCursor, state, action string) ([]commercialapp.PlatformAdjustmentQueueItem, error) {
	at, id := listCursor(cursor)
	rows, e := tx.commercial.ListPlatformAdjustmentQueue(ctx, commercialsql.ListPlatformAdjustmentQueueParams{StateFilter: queueFilter(state), ActionFilter: queueFilter(action), CursorAt: at, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]commercialapp.PlatformAdjustmentQueueItem, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.TenantID.Valid || !r.InvoiceID.Valid || !r.PaymentID.Valid || !r.MakerUserID.Valid || r.Version < 1 || r.AmountMinorUnits < 1 || r.NetMinorUnits < 0 || r.TaxMinorUnits < 0 || r.AmountMinorUnits != r.NetMinorUnits+r.TaxMinorUnits || !r.CreatedAt.Valid || !r.ExpiresAt.Valid || !validAdjustmentReadShape(r.State, r.CheckerUserID.Valid, r.DecidedAt.Valid, r.State == "APPROVED" || r.State == "REJECTED" || r.State == "REVIEW_REQUIRED") {
			return nil, errors.New("commercial_platform_adjustment_queue_corrupt")
		}
		v := commercialapp.PlatformAdjustmentQueueItem{ID: domain.ID(r.ID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), MakerUserID: domain.ID(r.MakerUserID.Bytes), CheckerUserID: domain.ID(r.CheckerUserID.Bytes), Action: r.Action, State: r.State, ReasonCode: r.ReasonCode, EntitlementDisposition: r.EntitlementDisposition, Version: r.Version, AmountMinorUnits: r.AmountMinorUnits, NetMinorUnits: r.NetMinorUnits, TaxMinorUnits: r.TaxMinorUnits, CreatedAt: r.CreatedAt.Time.UTC(), ExpiresAt: r.ExpiresAt.Time.UTC()}
		if r.DecidedAt.Valid {
			x := r.DecidedAt.Time.UTC()
			v.DecidedAt = &x
		}
		out = append(out, v)
	}
	return out, nil
}
func (tx *trialTx) ListPlatformReversals(ctx context.Context, limit int32, cursor *commercialapp.CommercialListCursor, state string) ([]commercialapp.PlatformReversalQueueItem, error) {
	at, id := listCursor(cursor)
	rows, e := tx.commercial.ListPlatformReversalQueue(ctx, commercialsql.ListPlatformReversalQueueParams{StateFilter: queueFilter(state), CursorAt: at, CursorID: id, PageLimit: limit})
	if e != nil {
		return nil, e
	}
	out := make([]commercialapp.PlatformReversalQueueItem, 0, len(rows))
	for _, r := range rows {
		if !r.ID.Valid || !r.PaymentID.Valid || !r.TenantID.Valid || !r.InvoiceID.Valid || !r.ActorUserID.Valid || r.ExpectedPaymentVersion < 1 || !validPaymentReadState(r.PaymentState) || !validReconciliationReadState(r.ReconciliationState) || !validQueueState(r.QueueState) || r.Currency != domain.CurrencyKES || r.AmountMinorUnits < 1 || !r.RequestedAt.Valid || (r.QueueState == "APPLIED" && (!r.AppliedAt.Valid || r.HoldState == nil || *r.HoldState != "ACTIVE")) {
			return nil, errors.New("commercial_platform_reversal_queue_corrupt")
		}
		v := commercialapp.PlatformReversalQueueItem{ID: domain.ID(r.ID.Bytes), PaymentID: domain.ID(r.PaymentID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), ActorUserID: domain.ID(r.ActorUserID.Bytes), ExpectedPaymentVersion: r.ExpectedPaymentVersion, AmountMinorUnits: r.AmountMinorUnits, PaymentState: r.PaymentState, ReconciliationState: r.ReconciliationState, QueueState: r.QueueState, Currency: r.Currency, RequestedAt: r.RequestedAt.Time.UTC()}
		if r.AppliedAt.Valid {
			x := r.AppliedAt.Time.UTC()
			v.AppliedAt = &x
		}
		if r.HoldState != nil {
			v.HoldState = *r.HoldState
		}
		out = append(out, v)
	}
	return out, nil
}
func (tx *trialTx) AppendPlatformQueueReadEvidence(ctx context.Context, e commercialapp.PlatformQueueReadEvidence) error {
	return tx.evidence.AppendCommercialPlatformQueueRead(ctx, controlapp.CommercialPlatformQueueReadEvidence{AuditID: controlapp.ID(e.AuditID), CorrelationID: controlapp.ID(e.CorrelationID), ActorID: controlapp.ID(e.ActorID), TargetID: controlapp.ID(e.TargetID), Queue: e.Queue, StateFilter: e.StateFilter, SecondaryFilter: e.SecondaryFilter, ResultCount: e.ResultCount, HasMore: e.HasMore, OccurredAt: e.OccurredAt})
}
func queueFilter(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
func validQueueState(v string) bool {
	switch v {
	case "PENDING", "CONFIRMED", "HELD", "APPLIED":
		return true
	}
	return false
}
