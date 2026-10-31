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
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) WithinInvoice(ctx context.Context, fn func(context.Context, commercialapp.InvoiceTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_invoice_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newTrialTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (tx *trialTx) GetInvoiceCommand(ctx context.Context, actor domain.ID, key [32]byte) (commercialapp.InvoiceCommand, bool, error) {
	row, err := tx.commercial.GetFinancialCommand(ctx, commercialsql.GetFinancialCommandParams{ActorUserID: commercialID(actor), Operation: "ISSUE_INVOICE", KeyHash: key[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.InvoiceCommand{}, false, nil
	}
	if err != nil {
		return commercialapp.InvoiceCommand{}, false, err
	}
	if len(row.RequestHash) != 32 {
		return commercialapp.InvoiceCommand{}, false, errors.New("commercial_invoice_command_corrupt")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	return commercialapp.InvoiceCommand{RequestHash: hash, InvoiceID: domain.ID(row.TargetID.Bytes), Version: row.ResponseVersion}, true, nil
}

func (tx *trialTx) LockInvoiceSubscription(ctx context.Context, tenantID, subscriptionID domain.ID) (commercialapp.InvoiceSubscription, bool, error) {
	row, err := tx.commercial.LockInvoiceSubscription(ctx, commercialsql.LockInvoiceSubscriptionParams{SubscriptionID: commercialID(subscriptionID), TenantID: commercialID(tenantID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.InvoiceSubscription{}, false, nil
	}
	if err != nil {
		return commercialapp.InvoiceSubscription{}, false, err
	}
	return commercialapp.InvoiceSubscription{ID: domain.ID(row.ID.Bytes), TenantID: domain.ID(row.TenantID.Bytes), Version: row.Version}, true, nil
}

func (tx *trialTx) LockActiveTaxProfile(ctx context.Context, at time.Time) (domain.TaxProfile, bool, error) {
	row, err := tx.commercial.LockActiveTaxProfile(ctx, commercialTime(at))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TaxProfile{}, false, nil
	}
	if err != nil {
		return domain.TaxProfile{}, false, err
	}
	var configurationHash, legalHash [32]byte
	if len(row.ConfigurationHash) != 32 || len(row.LegalReviewReferenceHash) != 32 {
		return domain.TaxProfile{}, false, errors.New("commercial_tax_profile_corrupt")
	}
	copy(configurationHash[:], row.ConfigurationHash)
	copy(legalHash[:], row.LegalReviewReferenceHash)
	var until *time.Time
	if row.EffectiveUntil.Valid {
		value := row.EffectiveUntil.Time.UTC()
		until = &value
	}
	return domain.TaxProfile{ID: domain.ID(row.ID.Bytes), Version: row.Version, Country: row.Country, Currency: row.Currency, TaxCode: row.TaxCode, DisplayLabel: row.DisplayLabel, RateNumerator: row.RateNumerator, RateDenominator: row.RateDenominator, RoundingProfile: row.RoundingProfile, EffectiveFrom: row.EffectiveFrom.Time.UTC(), EffectiveUntil: until, ConfigurationHash: configurationHash, LegalReviewReferenceHash: legalHash}, true, nil
}

func (tx *trialTx) LockInvoiceNumber(ctx context.Context, year int, at time.Time) (int64, error) {
	if err := tx.commercial.EnsureFinancialNumberHead(ctx, commercialsql.EnsureFinancialNumberHeadParams{DocumentKind: "INVOICE", CalendarYear: int32(year), At: commercialTime(at)}); err != nil {
		return 0, err
	}
	head, err := tx.commercial.LockFinancialNumberHead(ctx, commercialsql.LockFinancialNumberHeadParams{DocumentKind: "INVOICE", CalendarYear: int32(year)})
	if err != nil {
		return 0, err
	}
	rows, err := tx.commercial.AdvanceFinancialNumberHead(ctx, commercialsql.AdvanceFinancialNumberHeadParams{At: commercialTime(at), DocumentKind: "INVOICE", CalendarYear: int32(year), ExpectedSequence: head.NextSequence, ExpectedVersion: head.Version})
	if err != nil {
		return 0, err
	}
	if rows != 1 {
		return 0, commercialapp.ErrInvoiceConflict
	}
	return head.NextSequence, nil
}

func (tx *trialTx) InsertInvoice(ctx context.Context, invoice domain.Invoice, commandID domain.ID, keyHash, requestHash [32]byte) error {
	if len(invoice.Lines) != 2 || commandID.IsZero() || keyHash == [32]byte{} || requestHash == [32]byte{} {
		return errors.New("commercial_invoice_commit_invalid")
	}
	if err := tx.commercial.InsertCommercialInvoice(ctx, commercialsql.InsertCommercialInvoiceParams{
		ID: commercialID(invoice.ID), TenantID: commercialID(invoice.TenantID), SubscriptionID: commercialID(invoice.SubscriptionID), PurchaseOperationID: commercialID(invoice.PurchaseOperationID), InvoiceNumber: invoice.Number,
		PurchaseKind: invoice.PurchaseKind, Quantity: invoice.Quantity, PlanKey: string(invoice.PlanKey), CatalogueVersion: invoice.CatalogueVersion, Cadence: string(invoice.Cadence), TermStart: commercialTime(invoice.TermStart), TermEnd: commercialTime(invoice.TermEnd),
		TaxProfileID: commercialID(invoice.TaxProfileID), TaxProfileHash: invoice.TaxProfileHash[:], SubtotalMinorUnits: invoice.SubtotalMinorUnits, CreditMinorUnits: invoice.CreditMinorUnits, TaxableMinorUnits: invoice.TaxableMinorUnits, TaxMinorUnits: invoice.TaxMinorUnits, TotalMinorUnits: invoice.TotalMinorUnits, OutstandingMinorUnits: invoice.OutstandingMinorUnits,
		SellerLegalName: invoice.Billing.SellerLegalName, SellerTaxIdentifier: invoice.Billing.SellerTaxIdentifier, SellerBillingAddress: invoice.Billing.SellerBillingAddress, CustomerLegalName: invoice.Billing.CustomerLegalName, CustomerTaxIdentifier: invoice.Billing.CustomerTaxIdentifier, CustomerBillingAddress: invoice.Billing.CustomerBillingAddress,
		BillingSnapshotVersion: invoice.BillingSnapshotVersion, BillingSnapshotHash: invoice.BillingSnapshotHash[:], QuoteHash: invoice.QuoteHash[:], IssuedBy: commercialID(invoice.IssuedBy), IssuedAt: commercialTime(invoice.IssuedAt), DueAt: commercialTime(invoice.DueAt),
	}); err != nil {
		return financialInsertError(err)
	}
	for _, line := range invoice.Lines {
		if err := tx.commercial.InsertCommercialInvoiceLine(ctx, commercialsql.InsertCommercialInvoiceLineParams{ID: commercialID(line.ID), InvoiceID: commercialID(invoice.ID), LineNumber: line.Number, LineKind: line.Kind, DescriptionCode: line.DescriptionCode, Quantity: line.Quantity, UnitMinorUnits: line.UnitMinorUnits, ProrationNumerator: &line.ProrationNumerator, ProrationDenominator: &line.ProrationDenominator, NetMinorUnits: line.NetMinorUnits, TaxCode: &line.TaxCode, TaxMinorUnits: line.TaxMinorUnits, GrossMinorUnits: line.GrossMinorUnits, CalculationHash: line.CalculationHash[:]}); err != nil {
			return financialInsertError(err)
		}
	}
	if err := tx.insertInvoiceEntitlement(ctx, invoice); err != nil {
		return err
	}
	return financialInsertError(tx.commercial.InsertInvoiceFinancialCommand(ctx, commercialsql.InsertInvoiceFinancialCommandParams{ID: commercialID(commandID), ActorUserID: commercialID(invoice.IssuedBy), TenantID: commercialID(invoice.TenantID), KeyHash: keyHash[:], RequestHash: requestHash[:], InvoiceID: commercialID(invoice.ID), At: commercialTime(invoice.IssuedAt)}))
}

func (tx *trialTx) insertInvoiceEntitlement(ctx context.Context, invoice domain.Invoice) error {
	e := invoice.Entitlement
	hash, err := e.Hash()
	if err != nil {
		return errors.New("commercial_invoice_entitlement_invalid")
	}
	params := commercialsql.InsertInvoiceEntitlementSnapshotParams{InvoiceID: commercialID(invoice.ID), SubscriptionID: commercialID(invoice.SubscriptionID), EffectKind: e.Kind, ExpectedSubscriptionVersion: e.ExpectedSubscriptionVersion, PlanKey: string(invoice.PlanKey), CatalogueVersion: invoice.CatalogueVersion, Cadence: string(invoice.Cadence), CommercialPeriodStart: commercialTime(invoice.TermStart), CommercialPeriodEnd: commercialTime(invoice.TermEnd), SnapshotHash: hash[:], CreatedAt: commercialTime(invoice.IssuedAt)}
	if e.Kind == "PLAN_ACTIVATION" && e.PlanTerm != nil {
		term := e.PlanTerm
		params.PriceMinorUnits = &term.Price.MinorUnits
		if term.AdditionalLicenceMonthly != nil {
			value := term.AdditionalLicenceMonthly.MinorUnits
			params.AdditionalLicenceMonthlyMinorUnits = &value
		}
		params.BidWorkspaceLimit = &term.Limits.BidWorkspaces
		params.IncludedLicenceLimit = &term.Limits.IncludedLicences
		params.StorageLimitBytes = &term.Limits.StorageBytes
		params.DocumentVersionLimit = &term.Limits.DocumentVersions
		params.AiCreditLimit = &term.Limits.AICredits
		params.ConcurrentAiJobLimit = &term.Limits.ConcurrentAIJobs
		params.UsagePeriodStart = commercialTime(term.UsagePeriodStart)
		params.UsagePeriodEnd = commercialTime(term.UsagePeriodEnd)
	} else if e.Kind == "ADDITIONAL_LICENCES" {
		params.TargetTermID = commercialID(e.TargetTermID)
		params.AdditionalLicenceQuantity = &e.AdditionalLicences
	} else {
		return errors.New("commercial_invoice_entitlement_invalid")
	}
	return financialInsertError(tx.commercial.InsertInvoiceEntitlementSnapshot(ctx, params))
}

func financialInsertError(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return commercialapp.ErrInvoiceConflict
	}
	return err
}

func (tx *trialTx) GetInvoice(ctx context.Context, tenantID, invoiceID domain.ID) (domain.Invoice, bool, error) {
	row, err := tx.commercial.GetCommercialInvoice(ctx, commercialsql.GetCommercialInvoiceParams{ID: commercialID(invoiceID), TenantID: commercialID(tenantID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Invoice{}, false, nil
	}
	if err != nil {
		return domain.Invoice{}, false, err
	}
	lines, err := tx.commercial.ListCommercialInvoiceLines(ctx, commercialID(invoiceID))
	if err != nil {
		return domain.Invoice{}, false, err
	}
	invoice, err := mapFinancialInvoice(row, lines)
	return invoice, err == nil, err
}

func (tx *trialTx) AppendInvoiceEvidence(ctx context.Context, evidence commercialapp.InvoiceEvidence) error {
	return tx.evidence.AppendCommercialInvoiceIssued(ctx, controlapp.CommercialInvoiceEvidence{AuditID: controlapp.ID(evidence.AuditID), OutboxID: controlapp.ID(evidence.OutboxID), CorrelationID: controlapp.ID(evidence.CorrelationID), ActorID: controlapp.ID(evidence.ActorID), TenantID: controlapp.ID(evidence.TenantID), InvoiceID: controlapp.ID(evidence.InvoiceID), InvoiceNumber: evidence.InvoiceNumber, State: evidence.State, PurchaseKind: evidence.PurchaseKind, Version: evidence.Version, TotalMinorUnits: evidence.TotalMinorUnits, OccurredAt: evidence.OccurredAt})
}

func mapFinancialInvoice(row commercialsql.GetCommercialInvoiceRow, rows []commercialsql.ListCommercialInvoiceLinesRow) (domain.Invoice, error) {
	var taxHash, billingHash, quoteHash [32]byte
	if len(row.TaxProfileHash) != 32 || len(row.BillingSnapshotHash) != 32 || len(row.QuoteHash) != 32 {
		return domain.Invoice{}, errors.New("commercial_invoice_corrupt")
	}
	copy(taxHash[:], row.TaxProfileHash)
	copy(billingHash[:], row.BillingSnapshotHash)
	copy(quoteHash[:], row.QuoteHash)
	invoice := domain.Invoice{ID: domain.ID(row.ID.Bytes), TenantID: domain.ID(row.TenantID.Bytes), SubscriptionID: domain.ID(row.SubscriptionID.Bytes), PurchaseOperationID: domain.ID(row.PurchaseOperationID.Bytes), TaxProfileID: domain.ID(row.TaxProfileID.Bytes), IssuedBy: domain.ID(row.IssuedBy.Bytes), Number: row.InvoiceNumber, State: row.State, PurchaseKind: row.PurchaseKind, CatalogueVersion: row.CatalogueVersion, Currency: row.Currency, ArithmeticProfile: row.ArithmeticProfile, PlanKey: domain.PlanKey(row.PlanKey), Cadence: domain.BillingCadence(row.Cadence), Version: row.Version, Quantity: row.Quantity, TermStart: row.TermStart.Time.UTC(), TermEnd: row.TermEnd.Time.UTC(), IssuedAt: row.IssuedAt.Time.UTC(), DueAt: row.DueAt.Time.UTC(), SubtotalMinorUnits: row.SubtotalMinorUnits, CreditMinorUnits: row.CreditMinorUnits, TaxableMinorUnits: row.TaxableMinorUnits, TaxMinorUnits: row.TaxMinorUnits, TotalMinorUnits: row.TotalMinorUnits, OutstandingMinorUnits: row.OutstandingMinorUnits, TaxProfileHash: taxHash, BillingSnapshotHash: billingHash, QuoteHash: quoteHash, BillingSnapshotVersion: row.BillingSnapshotVersion, Billing: domain.BillingSnapshot{Version: row.BillingSnapshotVersion, SellerLegalName: row.SellerLegalName, SellerTaxIdentifier: row.SellerTaxIdentifier, SellerBillingAddress: row.SellerBillingAddress, CustomerLegalName: row.CustomerLegalName, CustomerTaxIdentifier: row.CustomerTaxIdentifier, CustomerBillingAddress: row.CustomerBillingAddress, Hash: billingHash}}
	for _, line := range rows {
		var hash [32]byte
		if len(line.CalculationHash) != 32 {
			return domain.Invoice{}, errors.New("commercial_invoice_line_corrupt")
		}
		copy(hash[:], line.CalculationHash)
		var numerator, denominator int64
		if line.ProrationNumerator != nil {
			numerator = *line.ProrationNumerator
		}
		if line.ProrationDenominator != nil {
			denominator = *line.ProrationDenominator
		}
		taxCode := ""
		if line.TaxCode != nil {
			taxCode = *line.TaxCode
		}
		invoice.Lines = append(invoice.Lines, domain.InvoiceLine{ID: domain.ID(line.ID.Bytes), Number: line.LineNumber, Kind: line.LineKind, DescriptionCode: line.DescriptionCode, TaxCode: taxCode, Quantity: line.Quantity, UnitMinorUnits: line.UnitMinorUnits, ProrationNumerator: numerator, ProrationDenominator: denominator, NetMinorUnits: line.NetMinorUnits, TaxMinorUnits: line.TaxMinorUnits, GrossMinorUnits: line.GrossMinorUnits, CalculationHash: hash})
	}
	return invoice, nil
}
