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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Store) WithinPayment(ctx context.Context, fn func(context.Context, commercialapp.PaymentTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_payment_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		tx, err := newTrialTx(db)
		if err != nil {
			return err
		}
		return fn(txCtx, tx)
	})
}

func (tx *trialTx) GetPaymentCommand(ctx context.Context, actor domain.ID, key [32]byte) (commercialapp.PaymentCommand, bool, error) {
	row, err := tx.commercial.GetFinancialCommand(ctx, commercialsql.GetFinancialCommandParams{ActorUserID: commercialID(actor), Operation: "INITIATE_PAYMENT", KeyHash: key[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.PaymentCommand{}, false, nil
	}
	if err != nil {
		return commercialapp.PaymentCommand{}, false, err
	}
	if len(row.RequestHash) != 32 {
		return commercialapp.PaymentCommand{}, false, errors.New("commercial_payment_command_corrupt")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	return commercialapp.PaymentCommand{RequestHash: hash, PaymentID: domain.ID(row.TargetID.Bytes), Version: row.ResponseVersion}, true, nil
}

func (tx *trialTx) GetPayment(ctx context.Context, tenantID, paymentID domain.ID) (domain.Payment, bool, error) {
	row, err := tx.commercial.GetCommercialPayment(ctx, commercialsql.GetCommercialPaymentParams{ID: commercialID(paymentID), TenantID: commercialID(tenantID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, false, nil
	}
	if err != nil {
		return domain.Payment{}, false, err
	}
	return paymentFrom(row.ID, row.InvoiceID, row.TenantID, row.MerchantOperationID, row.ProviderConfigurationVersionID, row.CreateAttemptID, row.CreatedBy, row.ProviderOrderReference, row.State, row.Currency, row.ReconciliationState, row.ReconciliationReason, row.Version, row.AmountMinorUnits, row.ReconciliationAttempts, row.CreatedAt, row.UpdatedAt, row.ReconciledAt, row.ReconciliationDueAt, row.LastQueriedAt), true, nil
}

func (tx *trialTx) LockPaymentInvoice(ctx context.Context, tenantID, invoiceID domain.ID) (commercialapp.PaymentInvoice, bool, error) {
	row, err := tx.commercial.LockPaymentInvoice(ctx, commercialsql.LockPaymentInvoiceParams{InvoiceID: commercialID(invoiceID), TenantID: commercialID(tenantID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.PaymentInvoice{}, false, nil
	}
	if err != nil {
		return commercialapp.PaymentInvoice{}, false, err
	}
	return commercialapp.PaymentInvoice{ID: domain.ID(row.ID.Bytes), TenantID: domain.ID(row.TenantID.Bytes), State: row.State, Currency: row.Currency, Version: row.Version, OutstandingMinorUnits: row.OutstandingMinorUnits}, true, nil
}

func (tx *trialTx) GetActivePayment(ctx context.Context, invoiceID domain.ID) (domain.Payment, bool, error) {
	row, err := tx.commercial.GetActivePaymentForInvoice(ctx, commercialID(invoiceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, false, nil
	}
	if err != nil {
		return domain.Payment{}, false, err
	}
	return paymentFrom(row.ID, row.InvoiceID, row.TenantID, row.MerchantOperationID, row.ProviderConfigurationVersionID, row.CreateAttemptID, row.CreatedBy, row.ProviderOrderReference, row.State, row.Currency, row.ReconciliationState, row.ReconciliationReason, row.Version, row.AmountMinorUnits, row.ReconciliationAttempts, row.CreatedAt, row.UpdatedAt, pgtype.Timestamptz{}, row.ReconciliationDueAt, pgtype.Timestamptz{}), true, nil
}

func (tx *trialTx) InsertPaymentIntent(ctx context.Context, payment domain.Payment, actorID, commandID domain.ID, keyHash, requestHash [32]byte, expectedInvoiceVersion int64, evidence commercialapp.PaymentEvidence) error {
	if !payment.Valid() || actorID.IsZero() || commandID.IsZero() || keyHash == [32]byte{} || requestHash == [32]byte{} {
		return errors.New("commercial_payment_intent_invalid")
	}
	if err := tx.commercial.InsertPaymentCreateIntent(ctx, commercialsql.InsertPaymentCreateIntentParams{ID: commercialID(payment.ID), InvoiceID: commercialID(payment.InvoiceID), TenantID: commercialID(payment.TenantID), MerchantOperationID: commercialID(payment.MerchantOperationID), AmountMinorUnits: payment.AmountMinorUnits, ProviderConfigurationVersionID: commercialID(payment.ProviderConfigurationVersionID), CreatedBy: commercialID(actorID), At: commercialTime(payment.CreatedAt), CreateAttemptID: commercialID(payment.CreateAttemptID), ReconciliationDueAt: commercialTime(*payment.ReconciliationDueAt)}); err != nil {
		return paymentInsertError(err)
	}
	rows, err := tx.commercial.MarkInvoicePaymentPending(ctx, commercialsql.MarkInvoicePaymentPendingParams{At: commercialTime(payment.CreatedAt), InvoiceID: commercialID(payment.InvoiceID), TenantID: commercialID(payment.TenantID), ExpectedVersion: expectedInvoiceVersion})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrPaymentConflict
	}
	if err = tx.commercial.InsertPaymentFinancialCommand(ctx, commercialsql.InsertPaymentFinancialCommandParams{ID: commercialID(commandID), ActorUserID: commercialID(actorID), TenantID: commercialID(payment.TenantID), KeyHash: keyHash[:], RequestHash: requestHash[:], PaymentID: commercialID(payment.ID), At: commercialTime(payment.CreatedAt)}); err != nil {
		return paymentInsertError(err)
	}
	return tx.appendPaymentEvidence(ctx, evidence)
}

func (tx *trialTx) ApplyPaymentCreateResult(ctx context.Context, current domain.Payment, result domain.PaymentCreateResult, observation *commercialapp.PaymentObservation, evidence commercialapp.PaymentEvidence) error {
	var rows int64
	var err error
	switch result.Disposition {
	case domain.PaymentOrderCreated:
		ref := result.ProviderOrderReference
		rows, err = tx.commercial.ConfirmPaymentOrderCreated(ctx, commercialsql.ConfirmPaymentOrderCreatedParams{ProviderOrderReference: &ref, At: commercialTime(evidence.OccurredAt), ID: commercialID(current.ID), TenantID: commercialID(current.TenantID), ExpectedVersion: current.Version, CreateAttemptID: commercialID(current.CreateAttemptID)})
	case domain.PaymentCreateNoEffect:
		rows, err = tx.commercial.ConfirmPaymentCreateNoEffect(ctx, commercialsql.ConfirmPaymentCreateNoEffectParams{At: commercialTime(evidence.OccurredAt), ID: commercialID(current.ID), TenantID: commercialID(current.TenantID), ExpectedVersion: current.Version, CreateAttemptID: commercialID(current.CreateAttemptID)})
	case domain.PaymentCreateAmbiguous:
		rows, err = tx.commercial.RecordPaymentCreateAmbiguous(ctx, commercialsql.RecordPaymentCreateAmbiguousParams{At: commercialTime(evidence.OccurredAt), ID: commercialID(current.ID), TenantID: commercialID(current.TenantID), ExpectedVersion: current.Version, CreateAttemptID: commercialID(current.CreateAttemptID)})
	default:
		return commercialapp.ErrPaymentConflict
	}
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrPaymentConflict
	}
	if observation != nil {
		if err = tx.insertPaymentObservation(ctx, *observation); err != nil {
			return err
		}
	}
	return tx.appendPaymentEvidence(ctx, evidence)
}

func (tx *trialTx) ClaimPaymentReconciliation(ctx context.Context, claimID, leaseID domain.ID) (commercialapp.PaymentClaim, bool, error) {
	row, err := tx.commercial.ClaimPaymentReconciliation(ctx, commercialsql.ClaimPaymentReconciliationParams{LeaseID: commercialID(leaseID), ClaimID: commercialID(claimID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return commercialapp.PaymentClaim{}, false, nil
	}
	if err != nil {
		return commercialapp.PaymentClaim{}, false, err
	}
	payment := paymentFrom(row.ID, row.InvoiceID, row.TenantID, row.MerchantOperationID, row.ProviderConfigurationVersionID, row.CreateAttemptID, row.CreatedBy, row.ProviderOrderReference, row.State, row.Currency, "LEASED", row.ReconciliationReason, row.Version, row.AmountMinorUnits, row.ReconciliationAttempts, row.CreatedAt, row.UpdatedAt, pgtype.Timestamptz{}, pgtype.Timestamptz{}, pgtype.Timestamptz{})
	if payment.ProviderOrderReference == nil {
		hint, e := tx.commercial.LatestPaymentCallbackHint(ctx, row.ID)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return commercialapp.PaymentClaim{}, false, e
		}
		if e == nil && hint.Valid {
			reference := uuid.UUID(hint.Bytes).String()
			payment.ProviderOrderReference = &reference
		}
	}
	return commercialapp.PaymentClaim{ClaimID: claimID, LeaseID: leaseID, Payment: payment, StartedAt: row.UpdatedAt.Time.UTC(), LeaseExpiresAt: row.ReconciliationLeaseExpiresAt.Time.UTC()}, true, nil
}

func (tx *trialTx) CompletePaymentReconciliation(ctx context.Context, claim commercialapp.PaymentClaim, outcome, reconciliationState string, reconciliationReason, providerReference *string, due *time.Time, observation *commercialapp.PaymentObservation, evidence commercialapp.PaymentEvidence) error {
	dueValue := pgtype.Timestamptz{}
	if due != nil {
		dueValue = commercialTime(*due)
	}
	rows, err := tx.commercial.CompletePaymentReconciliation(ctx, commercialsql.CompletePaymentReconciliationParams{ProviderOrderReference: providerReference, PaymentState: evidence.State, At: commercialTime(evidence.OccurredAt), ReconciliationState: reconciliationState, ReconciliationReason: reconciliationReason, ReconciliationDueAt: dueValue, ID: commercialID(claim.Payment.ID), ExpectedVersion: claim.Payment.Version, LeaseID: commercialID(claim.LeaseID)})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrPaymentConflict
	}
	var normalized, code *string
	var raw []byte
	var observationID pgtype.UUID
	if observation != nil {
		observationID = commercialID(observation.ID)
		normalized = &observation.NormalizedState
		code = &observation.ProviderStateCode
		raw = observation.RawEvidenceHash[:]
		if err = tx.insertPaymentObservation(ctx, *observation); err != nil {
			return err
		}
	}
	if err = tx.commercial.InsertPaymentReconciliationOutcome(ctx, commercialsql.InsertPaymentReconciliationOutcomeParams{ClaimID: commercialID(claim.ClaimID), Outcome: outcome, ObservationID: observationID, NormalizedState: normalized, ProviderStateCode: code, RawEvidenceHash: raw, CompletedAt: commercialTime(evidence.OccurredAt)}); err != nil {
		return paymentInsertError(err)
	}
	return tx.appendPaymentEvidence(ctx, evidence)
}

func (tx *trialTx) insertPaymentObservation(ctx context.Context, value commercialapp.PaymentObservation) error {
	if value.ID.IsZero() || value.PaymentID.IsZero() || value.RawEvidenceHash == [32]byte{} || value.RecordedAt.IsZero() {
		return errors.New("commercial_payment_observation_invalid")
	}
	observed := pgtype.Timestamptz{}
	if value.ProviderObservedAt != nil {
		observed = commercialTime(*value.ProviderObservedAt)
	}
	return tx.commercial.InsertPaymentObservation(ctx, commercialsql.InsertPaymentObservationParams{ID: commercialID(value.ID), PaymentID: commercialID(value.PaymentID), Source: value.Source, ProviderStateCode: value.ProviderStateCode, NormalizedState: value.NormalizedState, AmountMinorUnits: value.AmountMinorUnits, Currency: value.Currency, RawEvidenceHash: value.RawEvidenceHash[:], ProviderObservedAt: observed, RecordedAt: commercialTime(value.RecordedAt)})
}

func (tx *trialTx) appendPaymentEvidence(ctx context.Context, value commercialapp.PaymentEvidence) error {
	return tx.evidence.AppendCommercialPayment(ctx, controlapp.CommercialPaymentEvidence{AuditID: controlapp.ID(value.AuditID), OutboxID: controlapp.ID(value.OutboxID), CorrelationID: controlapp.ID(value.CorrelationID), ActorID: controlapp.ID(value.ActorID), TenantID: controlapp.ID(value.TenantID), InvoiceID: controlapp.ID(value.InvoiceID), PaymentID: controlapp.ID(value.PaymentID), EventType: value.EventType, State: value.State, ReconciliationState: value.ReconciliationState, Version: value.Version, AmountMinorUnits: value.AmountMinorUnits, OccurredAt: value.OccurredAt})
}

func paymentInsertError(err error) error {
	if err == nil {
		return nil
	}
	mapped := financialInsertError(err)
	if errors.Is(mapped, commercialapp.ErrInvoiceConflict) {
		return commercialapp.ErrPaymentConflict
	}
	return mapped
}
func paymentFrom(id, invoiceID, tenantID, merchantID, configurationID, attemptID, createdBy pgtype.UUID, reference *string, state, currency, reconciliationState string, reconciliationReason *string, version, amount int64, attempts int32, createdAt, updatedAt, reconciledAt, dueAt, lastQueriedAt pgtype.Timestamptz) domain.Payment {
	value := domain.Payment{ID: domain.ID(id.Bytes), InvoiceID: domain.ID(invoiceID.Bytes), TenantID: domain.ID(tenantID.Bytes), MerchantOperationID: domain.ID(merchantID.Bytes), ProviderConfigurationVersionID: domain.ID(configurationID.Bytes), CreateAttemptID: domain.ID(attemptID.Bytes), CreatedBy: domain.ID(createdBy.Bytes), ProviderOrderReference: reference, State: state, Currency: currency, ReconciliationState: reconciliationState, Version: version, AmountMinorUnits: amount, ReconciliationAttempts: attempts, CreatedAt: createdAt.Time.UTC(), UpdatedAt: updatedAt.Time.UTC()}
	if reconciliationReason != nil {
		value.ReconciliationReason = *reconciliationReason
	}
	if reconciledAt.Valid {
		v := reconciledAt.Time.UTC()
		value.ReconciledAt = &v
	}
	if dueAt.Valid {
		v := dueAt.Time.UTC()
		value.ReconciliationDueAt = &v
	}
	if lastQueriedAt.Valid {
		v := lastQueriedAt.Time.UTC()
		value.LastQueriedAt = &v
	}
	return value
}
