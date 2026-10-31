package persistence

import (
	"context"
	"errors"
	"strconv"
	"time"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	commercialsql "bidora/backend/internal/commercial/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

func (s *Store) WithinSettlement(ctx context.Context, fn func(context.Context, commercialapp.SettlementTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_settlement_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newTrialTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (tx *trialTx) GetLicenceEntitlement(ctx context.Context, tenantID domain.ID, at time.Time) (commercialapp.LicenceEntitlement, bool, error) {
	row, err := tx.commercial.GetCurrentLicenceEntitlement(ctx, commercialsql.GetCurrentLicenceEntitlementParams{At: commercialTime(at), TenantID: commercialID(tenantID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.LicenceEntitlement{}, false, nil
	}
	if err != nil {
		return commercialapp.LicenceEntitlement{}, false, err
	}
	effective, parseErr := strconv.ParseInt(row.EffectiveLicenceLimit, 10, 64)
	if parseErr != nil || effective < row.IncludedLicenceLimit {
		return commercialapp.LicenceEntitlement{}, false, errors.New("commercial_licence_entitlement_corrupt")
	}
	return commercialapp.LicenceEntitlement{SubscriptionID: domain.ID(row.SubscriptionID.Bytes), TermID: domain.ID(row.TermID.Bytes), SubscriptionVersion: row.SubscriptionVersion, IncludedLicences: row.IncludedLicenceLimit, EffectiveLicences: effective}, true, nil
}

func (tx *trialTx) GetSettlement(ctx context.Context, paymentID domain.ID) (commercialapp.SettlementStored, bool, error) {
	row, err := tx.commercial.GetPaymentSettlementEffect(ctx, commercialID(paymentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.SettlementStored{}, false, nil
	}
	if err != nil {
		return commercialapp.SettlementStored{}, false, err
	}
	if len(row.EffectHash) != 32 || len(row.ContentHash) != 32 {
		return commercialapp.SettlementStored{}, false, errors.New("commercial_settlement_effect_corrupt")
	}
	var effectHash, contentHash [32]byte
	copy(effectHash[:], row.EffectHash)
	copy(contentHash[:], row.ContentHash)
	return commercialapp.SettlementStored{PaymentID: domain.ID(row.PaymentID.Bytes), InvoiceID: domain.ID(row.InvoiceID.Bytes), ObservationID: domain.ID(row.ObservationID.Bytes), SubscriptionID: domain.ID(row.SubscriptionID.Bytes), ResultingTermID: domain.ID(row.ResultingTermID.Bytes), GrantID: domain.ID(row.AdditionalLicenceGrantID.Bytes), Receipt: domain.Receipt{ID: domain.ID(row.ReceiptID.Bytes), PaymentID: domain.ID(row.PaymentID.Bytes), InvoiceID: domain.ID(row.InvoiceID.Bytes), Number: row.ReceiptNumber, Currency: row.Currency, AmountMinorUnits: row.AmountMinorUnits, IssuedAt: row.IssuedAt.Time.UTC(), ContentHash: contentHash}, EffectKind: row.EffectKind, PriorSubscriptionVersion: row.PriorSubscriptionVersion, ResultingSubscriptionVersion: row.ResultingSubscriptionVersion, PaymentVersion: row.PaymentVersion, InvoiceVersion: row.InvoiceVersion, SettledAt: row.SettledAt.Time.UTC(), EffectHash: effectHash}, true, nil
}

func (tx *trialTx) LockSettlementCandidate(ctx context.Context, paymentID domain.ID) (domain.SettlementCandidate, bool, error) {
	p, err := tx.commercial.LockSettlementPayment(ctx, commercialID(paymentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SettlementCandidate{}, false, nil
	}
	if err != nil {
		return domain.SettlementCandidate{}, false, err
	}
	payment := paymentFrom(p.ID, p.InvoiceID, p.TenantID, p.MerchantOperationID, p.ProviderConfigurationVersionID, p.CreateAttemptID, p.CreatedBy, p.ProviderOrderReference, p.State, p.Currency, p.ReconciliationState, p.ReconciliationReason, p.Version, p.AmountMinorUnits, p.ReconciliationAttempts, p.CreatedAt, p.UpdatedAt, p.ReconciledAt, p.ReconciliationDueAt, p.LastQueriedAt)
	i, err := tx.commercial.LockSettlementInvoice(ctx, commercialsql.LockSettlementInvoiceParams{InvoiceID: p.InvoiceID, TenantID: p.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SettlementCandidate{}, false, nil
	}
	if err != nil {
		return domain.SettlementCandidate{}, false, err
	}
	invoice := domain.Invoice{ID: domain.ID(i.ID.Bytes), TenantID: domain.ID(i.TenantID.Bytes), SubscriptionID: domain.ID(i.SubscriptionID.Bytes), PurchaseOperationID: domain.ID(i.PurchaseOperationID.Bytes), Number: i.InvoiceNumber, State: i.State, Version: i.Version, PurchaseKind: i.PurchaseKind, Quantity: i.Quantity, PlanKey: domain.PlanKey(i.PlanKey), CatalogueVersion: i.CatalogueVersion, Cadence: domain.BillingCadence(i.Cadence), TermStart: i.TermStart.Time.UTC(), TermEnd: i.TermEnd.Time.UTC(), Currency: i.Currency, SubtotalMinorUnits: i.SubtotalMinorUnits, TaxMinorUnits: i.TaxMinorUnits, TotalMinorUnits: i.TotalMinorUnits, OutstandingMinorUnits: i.OutstandingMinorUnits}
	o, err := tx.commercial.GetAuthoritativeSettlementObservation(ctx, p.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SettlementCandidate{}, false, nil
	}
	if err != nil {
		return domain.SettlementCandidate{}, false, err
	}
	if len(o.RawEvidenceHash) != 32 {
		return domain.SettlementCandidate{}, false, errors.New("commercial_settlement_observation_corrupt")
	}
	var raw [32]byte
	copy(raw[:], o.RawEvidenceHash)
	var observed *time.Time
	if o.ProviderObservedAt.Valid {
		value := o.ProviderObservedAt.Time.UTC()
		observed = &value
	}
	observation := domain.SettlementObservation{ID: domain.ID(o.ID.Bytes), NormalizedState: o.NormalizedState, AmountMinorUnits: o.AmountMinorUnits, Currency: o.Currency, RawEvidenceHash: raw, ProviderObservedAt: observed, RecordedAt: o.RecordedAt.Time.UTC()}
	e, err := tx.commercial.GetInvoiceEntitlementSnapshot(ctx, i.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SettlementCandidate{}, false, nil
	}
	if err != nil {
		return domain.SettlementCandidate{}, false, err
	}
	if domain.ID(e.InvoiceID.Bytes) != invoice.ID || domain.ID(e.SubscriptionID.Bytes) != invoice.SubscriptionID {
		return domain.SettlementCandidate{}, false, errors.New("commercial_invoice_entitlement_corrupt")
	}
	entitlement, err := mapInvoiceEntitlement(e)
	if err != nil {
		return domain.SettlementCandidate{}, false, err
	}
	s, err := tx.commercial.LockSettlementSubscription(ctx, commercialsql.LockSettlementSubscriptionParams{SubscriptionID: i.SubscriptionID, TenantID: i.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SettlementCandidate{}, false, nil
	}
	if err != nil {
		return domain.SettlementCandidate{}, false, err
	}
	subscription := domain.SettlementSubscription{ID: domain.ID(s.ID.Bytes), TenantID: domain.ID(s.TenantID.Bytes), CurrentTermID: domain.ID(s.CurrentTermID.Bytes), State: domain.SubscriptionState(s.State), Version: s.Version}
	if s.PlanKey != nil {
		subscription.CurrentPlan = domain.PlanKey(*s.PlanKey)
	}
	if s.Cadence != nil {
		subscription.CurrentCadence = domain.BillingCadence(*s.Cadence)
	}
	if s.CommercialPeriodStart.Valid {
		subscription.CurrentTermStart = s.CommercialPeriodStart.Time.UTC()
	}
	if s.CommercialPeriodEnd.Valid {
		subscription.CurrentTermEnd = s.CommercialPeriodEnd.Time.UTC()
	}
	return domain.SettlementCandidate{Payment: payment, Invoice: invoice, Observation: observation, Entitlement: entitlement, Subscription: subscription}, true, nil
}

func mapInvoiceEntitlement(row commercialsql.GetInvoiceEntitlementSnapshotRow) (domain.InvoiceEntitlementSnapshot, error) {
	value := domain.InvoiceEntitlementSnapshot{Kind: row.EffectKind, ExpectedSubscriptionVersion: row.ExpectedSubscriptionVersion, TargetTermID: domain.ID(row.TargetTermID.Bytes)}
	if row.EffectKind == "PLAN_ACTIVATION" {
		if row.PriceMinorUnits == nil || row.BidWorkspaceLimit == nil || row.IncludedLicenceLimit == nil || row.StorageLimitBytes == nil || row.DocumentVersionLimit == nil || row.AiCreditLimit == nil || row.ConcurrentAiJobLimit == nil || !row.UsagePeriodStart.Valid || !row.UsagePeriodEnd.Valid {
			return value, errors.New("commercial_invoice_entitlement_corrupt")
		}
		term := domain.TermSnapshot{PlanKey: domain.PlanKey(row.PlanKey), CatalogueVersion: row.CatalogueVersion, Cadence: domain.BillingCadence(row.Cadence), Price: domain.Money{Currency: domain.CurrencyKES, MinorUnits: *row.PriceMinorUnits}, PriceKind: "FIXED", Limits: domain.LimitSnapshot{BidWorkspaces: *row.BidWorkspaceLimit, IncludedLicences: *row.IncludedLicenceLimit, StorageBytes: *row.StorageLimitBytes, DocumentVersions: *row.DocumentVersionLimit, AICredits: *row.AiCreditLimit, ConcurrentAIJobs: *row.ConcurrentAiJobLimit}, CommercialPeriodStart: row.CommercialPeriodStart.Time.UTC(), CommercialPeriodEnd: row.CommercialPeriodEnd.Time.UTC(), UsagePeriodStart: row.UsagePeriodStart.Time.UTC(), UsagePeriodEnd: row.UsagePeriodEnd.Time.UTC()}
		if row.AdditionalLicenceMonthlyMinorUnits != nil {
			term.AdditionalLicenceMonthly = &domain.Money{Currency: domain.CurrencyKES, MinorUnits: *row.AdditionalLicenceMonthlyMinorUnits}
		}
		value.PlanTerm = &term
	} else if row.EffectKind == "ADDITIONAL_LICENCES" && row.AdditionalLicenceQuantity != nil {
		value.AdditionalLicences = *row.AdditionalLicenceQuantity
	} else {
		return value, errors.New("commercial_invoice_entitlement_corrupt")
	}
	hash, err := value.Hash()
	if err != nil || len(row.SnapshotHash) != 32 {
		return value, errors.New("commercial_invoice_entitlement_corrupt")
	}
	var stored [32]byte
	copy(stored[:], row.SnapshotHash)
	if hash != stored {
		return value, errors.New("commercial_invoice_entitlement_corrupt")
	}
	return value, nil
}

func (tx *trialTx) LockReceiptNumber(ctx context.Context, year int, at time.Time) (int64, error) {
	if err := tx.commercial.EnsureFinancialNumberHead(ctx, commercialsql.EnsureFinancialNumberHeadParams{DocumentKind: "RECEIPT", CalendarYear: int32(year), At: commercialTime(at)}); err != nil {
		return 0, err
	}
	head, err := tx.commercial.LockFinancialNumberHead(ctx, commercialsql.LockFinancialNumberHeadParams{DocumentKind: "RECEIPT", CalendarYear: int32(year)})
	if err != nil {
		return 0, err
	}
	rows, err := tx.commercial.AdvanceFinancialNumberHead(ctx, commercialsql.AdvanceFinancialNumberHeadParams{At: commercialTime(at), DocumentKind: "RECEIPT", CalendarYear: int32(year), ExpectedSequence: head.NextSequence, ExpectedVersion: head.Version})
	if err != nil {
		return 0, err
	}
	if rows != 1 {
		return 0, commercialapp.ErrSettlementConflict
	}
	return head.NextSequence, nil
}
func (tx *trialTx) NextTermVersion(ctx context.Context, subscriptionID domain.ID) (int64, error) {
	value, err := tx.commercial.NextSettlementTermVersion(ctx, commercialID(subscriptionID))
	if err != nil {
		return 0, err
	}
	next, parseErr := strconv.ParseInt(value, 10, 64)
	if parseErr != nil || next < 1 {
		return 0, errors.New("commercial_term_version_corrupt")
	}
	return next, nil
}

func (tx *trialTx) CommitSettlement(ctx context.Context, c commercialapp.SettlementCommit) error {
	candidate, decision := c.Candidate, c.Decision
	if decision.EffectKind == "PLAN_ACTIVATION" {
		if decision.Term == nil {
			return errors.New("commercial_settlement_term_missing")
		}
		term := decision.Term
		if c.ExecutionID.IsZero() {
			return errors.New("commercial_settlement_execution_invalid")
		}
		if err := tx.commercial.EnsurePaidLifetimeMarker(ctx, commercialsql.EnsurePaidLifetimeMarkerParams{TenantID: commercialID(candidate.Subscription.TenantID), SubscriptionID: commercialID(candidate.Subscription.ID), InitiatingUserID: commercialID(candidate.Payment.CreatedBy), ExecutionID: commercialID(c.ExecutionID), MarkedAt: commercialTime(c.OccurredAt)}); err != nil {
			return err
		}
		matches, err := tx.commercial.PaidLifetimeMarkerMatches(ctx, commercialsql.PaidLifetimeMarkerMatchesParams{TenantID: commercialID(candidate.Subscription.TenantID), SubscriptionID: commercialID(candidate.Subscription.ID)})
		if err != nil {
			return err
		}
		if !matches {
			return commercialapp.ErrSettlementConflict
		}
		var monthly *int64
		if term.AdditionalLicenceMonthly != nil {
			value := term.AdditionalLicenceMonthly.MinorUnits
			monthly = &value
		}
		limits := term.Limits
		if err := tx.commercial.InsertSettledSubscriptionTerm(ctx, commercialsql.InsertSettledSubscriptionTermParams{ID: commercialID(c.TermID), SubscriptionID: commercialID(candidate.Subscription.ID), TermVersion: c.TermVersion, PlanKey: string(term.PlanKey), CatalogueVersion: term.CatalogueVersion, Cadence: string(term.Cadence), PriceMinorUnits: term.Price.MinorUnits, AdditionalLicenceMonthlyMinorUnits: monthly, BidWorkspaceLimit: limits.BidWorkspaces, IncludedLicenceLimit: limits.IncludedLicences, StorageLimitBytes: limits.StorageBytes, DocumentVersionLimit: limits.DocumentVersions, AiCreditLimit: limits.AICredits, ConcurrentAiJobLimit: limits.ConcurrentAIJobs, CommercialPeriodStart: commercialTime(term.CommercialPeriodStart), CommercialPeriodEnd: commercialTime(term.CommercialPeriodEnd), UsagePeriodStart: commercialTime(term.UsagePeriodStart), UsagePeriodEnd: commercialTime(term.UsagePeriodEnd), CreatedAt: commercialTime(c.OccurredAt), PaymentID: commercialID(candidate.Payment.ID)}); err != nil {
			return settlementInsertError(err)
		}
		if err := tx.commercial.InsertSettledUsagePeriod(ctx, commercialsql.InsertSettledUsagePeriodParams{ID: commercialID(c.UsagePeriodID), SubscriptionID: commercialID(candidate.Subscription.ID), TermID: commercialID(c.TermID), PeriodStart: commercialTime(term.UsagePeriodStart), PeriodEnd: commercialTime(term.UsagePeriodEnd), CreatedAt: commercialTime(c.OccurredAt)}); err != nil {
			return settlementInsertError(err)
		}
		rows, err := tx.commercial.ActivateSettledSubscription(ctx, commercialsql.ActivateSettledSubscriptionParams{TermID: commercialID(c.TermID), SettledAt: commercialTime(decision.SettledAt), At: commercialTime(c.OccurredAt), ID: commercialID(candidate.Subscription.ID), TenantID: commercialID(candidate.Subscription.TenantID), ExpectedVersion: decision.PriorSubscriptionVersion})
		if err != nil {
			return err
		}
		if rows != 1 {
			return commercialapp.ErrSettlementConflict
		}
	} else if decision.EffectKind == "ADDITIONAL_LICENCES" {
		if err := tx.commercial.InsertAdditionalLicenceGrant(ctx, commercialsql.InsertAdditionalLicenceGrantParams{ID: commercialID(c.GrantID), SubscriptionID: commercialID(candidate.Subscription.ID), TermID: commercialID(candidate.Subscription.CurrentTermID), InvoiceID: commercialID(candidate.Invoice.ID), PaymentID: commercialID(candidate.Payment.ID), Quantity: decision.AdditionalLicenceQuantity, EffectiveFrom: commercialTime(decision.EffectiveFrom), EffectiveUntil: commercialTime(decision.EffectiveUntil), CreatedAt: commercialTime(c.OccurredAt)}); err != nil {
			return settlementInsertError(err)
		}
		rows, err := tx.commercial.AdvanceSubscriptionForLicenceGrant(ctx, commercialsql.AdvanceSubscriptionForLicenceGrantParams{At: commercialTime(c.OccurredAt), ID: commercialID(candidate.Subscription.ID), TenantID: commercialID(candidate.Subscription.TenantID), ExpectedVersion: decision.PriorSubscriptionVersion, TermID: commercialID(candidate.Subscription.CurrentTermID)})
		if err != nil {
			return err
		}
		if rows != 1 {
			return commercialapp.ErrSettlementConflict
		}
	} else {
		return commercialapp.ErrSettlementConflict
	}
	rows, err := tx.commercial.MarkPaymentSettled(ctx, commercialsql.MarkPaymentSettledParams{SettledAt: commercialTime(decision.SettledAt), At: commercialTime(c.OccurredAt), ID: commercialID(candidate.Payment.ID), ExpectedVersion: candidate.Payment.Version})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrSettlementConflict
	}
	rows, err = tx.commercial.MarkInvoicePaid(ctx, commercialsql.MarkInvoicePaidParams{At: commercialTime(c.OccurredAt), ID: commercialID(candidate.Invoice.ID), TenantID: commercialID(candidate.Invoice.TenantID), ExpectedVersion: candidate.Invoice.Version, ExpectedOutstanding: candidate.Payment.AmountMinorUnits})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrSettlementConflict
	}
	if err = tx.commercial.InsertCommercialReceipt(ctx, commercialsql.InsertCommercialReceiptParams{ID: commercialID(c.Receipt.ID), PaymentID: commercialID(candidate.Payment.ID), InvoiceID: commercialID(candidate.Invoice.ID), ReceiptNumber: c.Receipt.Number, AmountMinorUnits: c.Receipt.AmountMinorUnits, IssuedAt: commercialTime(c.Receipt.IssuedAt), ContentHash: c.Receipt.ContentHash[:]}); err != nil {
		return settlementInsertError(err)
	}
	return settlementInsertError(tx.commercial.InsertPaymentSettlementEffect(ctx, commercialsql.InsertPaymentSettlementEffectParams{PaymentID: commercialID(candidate.Payment.ID), InvoiceID: commercialID(candidate.Invoice.ID), ObservationID: commercialID(candidate.Observation.ID), ReceiptID: commercialID(c.Receipt.ID), SubscriptionID: commercialID(candidate.Subscription.ID), EffectKind: decision.EffectKind, ResultingTermID: commercialID(c.TermID), AdditionalLicenceGrantID: commercialID(c.GrantID), PriorSubscriptionVersion: decision.PriorSubscriptionVersion, ResultingSubscriptionVersion: decision.ResultingSubscriptionVersion, AmountMinorUnits: candidate.Payment.AmountMinorUnits, SettledAt: commercialTime(decision.SettledAt), EffectHash: c.EffectHash[:]}))
}
func (tx *trialTx) AppendSettlementEvidence(ctx context.Context, e commercialapp.PaymentEvidence) error {
	return tx.appendPaymentEvidence(ctx, e)
}
func settlementInsertError(err error) error {
	if err == nil {
		return nil
	}
	mapped := paymentInsertError(err)
	if errors.Is(mapped, commercialapp.ErrPaymentConflict) {
		return commercialapp.ErrSettlementConflict
	}
	return mapped
}
