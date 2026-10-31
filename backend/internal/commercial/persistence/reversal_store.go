package persistence

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	commercialsql "bidora/backend/internal/commercial/persistence/sqlc"
	controlapp "bidora/backend/internal/controlplane/application"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Store) WithinReversal(ctx context.Context, fn func(context.Context, commercialapp.ReversalTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("commercial_reversal_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		tx, e := newTrialTx(db)
		if e != nil {
			return e
		}
		return fn(c, tx)
	})
}
func (tx *trialTx) LockPaymentForReconciliationRequest(ctx context.Context, paymentID domain.ID) (domain.Payment, bool, error) {
	r, e := tx.commercial.LockPaymentForReconciliationRequest(ctx, commercialID(paymentID))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Payment{}, false, nil
	}
	if e != nil {
		return domain.Payment{}, false, e
	}
	return paymentFrom(r.ID, r.InvoiceID, r.TenantID, r.MerchantOperationID, r.ProviderConfigurationVersionID, r.CreateAttemptID, r.CreatedBy, r.ProviderOrderReference, r.State, r.Currency, r.ReconciliationState, r.ReconciliationReason, r.Version, r.AmountMinorUnits, r.ReconciliationAttempts, r.CreatedAt, r.UpdatedAt, r.ReconciledAt, r.ReconciliationDueAt, r.LastQueriedAt), true, nil
}
func (tx *trialTx) GetReconciliationRequestCommand(ctx context.Context, actor domain.ID, key [32]byte) (commercialapp.ReconciliationRequestRecord, bool, error) {
	r, e := tx.commercial.GetReconciliationRequestCommand(ctx, commercialsql.GetReconciliationRequestCommandParams{ActorUserID: commercialID(actor), KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return commercialapp.ReconciliationRequestRecord{}, false, nil
	}
	if e != nil {
		return commercialapp.ReconciliationRequestRecord{}, false, e
	}
	h, ok := hash32(r.RequestHash)
	if !ok || !r.TargetID.Valid {
		return commercialapp.ReconciliationRequestRecord{}, false, errors.New("commercial_reconciliation_request_corrupt")
	}
	return commercialapp.ReconciliationRequestRecord{RequestHash: h, PaymentID: domain.ID(r.TargetID.Bytes), State: r.ResponseState, Version: r.ResponseVersion}, true, nil
}
func (tx *trialTx) SchedulePaymentReconciliation(ctx context.Context, p domain.Payment, command domain.ID, reason string, key, request, authority [32]byte, e commercialapp.PaymentEvidence) error {
	rows, err := tx.commercial.ScheduleSettledPaymentReconciliation(ctx, commercialsql.ScheduleSettledPaymentReconciliationParams{At: commercialTime(e.OccurredAt), PaymentID: commercialID(p.ID), ExpectedVersion: p.Version})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrReversalConflict
	}
	if err = tx.commercial.InsertPaymentReconciliationRequest(ctx, commercialsql.InsertPaymentReconciliationRequestParams{ID: commercialID(command), PaymentID: commercialID(p.ID), ActorUserID: commercialID(e.ActorID), ExpectedPaymentVersion: p.Version, Reason: reason, AuthorityHash: authority[:], RequestHash: request[:], At: commercialTime(e.OccurredAt)}); err != nil {
		return paymentInsertError(err)
	}
	if err = tx.commercial.InsertReconciliationRequestCommand(ctx, commercialsql.InsertReconciliationRequestCommandParams{ID: commercialID(command), ActorUserID: commercialID(e.ActorID), TenantID: commercialID(p.TenantID), KeyHash: key[:], RequestHash: request[:], PaymentID: commercialID(p.ID), ResponseVersion: p.Version + 1, At: commercialTime(e.OccurredAt)}); err != nil {
		return paymentInsertError(err)
	}
	return tx.appendPaymentEvidence(ctx, e)
}
func (tx *trialTx) GetPaymentReversal(ctx context.Context, paymentID domain.ID) (commercialapp.ReversalStored, bool, error) {
	r, e := tx.commercial.GetPaymentReversalEffect(ctx, commercialID(paymentID))
	if errors.Is(e, pgx.ErrNoRows) {
		return commercialapp.ReversalStored{}, false, nil
	}
	if e != nil {
		return commercialapp.ReversalStored{}, false, e
	}
	settlement, ok1 := hash32(r.SettlementEffectHash)
	effect, ok2 := hash32(r.EffectHash)
	if !ok1 || !ok2 || !r.RecordedAt.Valid || !r.AppliedAt.Valid {
		return commercialapp.ReversalStored{}, false, errors.New("commercial_reversal_effect_corrupt")
	}
	var observed *time.Time
	if r.ProviderObservedAt.Valid {
		v := r.ProviderObservedAt.Time.UTC()
		observed = &v
	}
	decision := domain.ReversalDecision{PaymentID: domain.ID(r.PaymentID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), SubscriptionID: domain.ID(r.SubscriptionID.Bytes), SettlementObservationID: domain.ID(r.SettlementObservationID.Bytes), ReversalObservationID: domain.ID(r.ReversalObservationID.Bytes), PriorPaymentVersion: r.PriorPaymentVersion, ResultingPaymentVersion: r.ResultingPaymentVersion, PriorInvoiceVersion: r.PriorInvoiceVersion, ResultingInvoiceVersion: r.ResultingInvoiceVersion, PriorSubscriptionVersion: r.PriorSubscriptionVersion, ResultingSubscriptionVersion: r.ResultingSubscriptionVersion, AmountMinorUnits: r.AmountMinorUnits, Currency: r.Currency, AccessMode: "RETAINED_READ_ONLY", ProviderObservedAt: observed, RecordedAt: r.RecordedAt.Time.UTC(), AppliedAt: r.AppliedAt.Time.UTC(), SettlementEffectHash: settlement, EffectHash: effect}
	decision.AccessHoldHash = accessHoldHash(effect)
	return commercialapp.ReversalStored{Decision: decision, AccessHoldID: domain.ID(r.AccessHoldID.Bytes)}, true, nil
}
func (tx *trialTx) LockPaymentReversalCandidate(ctx context.Context, paymentID domain.ID) (domain.ReversalCandidate, bool, error) {
	r, e := tx.commercial.LockPaymentReversalCandidate(ctx, commercialID(paymentID))
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ReversalCandidate{}, false, nil
	}
	if e != nil {
		return domain.ReversalCandidate{}, false, e
	}
	settlement, ok1 := hash32(r.SettlementEffectHash)
	raw, ok2 := hash32(r.RawEvidenceHash)
	if !ok1 || !ok2 || !r.RecordedAt.Valid {
		return domain.ReversalCandidate{}, false, errors.New("commercial_reversal_candidate_corrupt")
	}
	reason := ""
	if r.ReconciliationReason != nil {
		reason = *r.ReconciliationReason
	}
	var observed *time.Time
	if r.ProviderObservedAt.Valid {
		v := r.ProviderObservedAt.Time.UTC()
		observed = &v
	}
	o := domain.ReversalObservation{ID: domain.ID(r.ReversalObservationID.Bytes), NormalizedState: r.NormalizedState, AmountMinorUnits: r.ObservedAmountMinorUnits, Currency: r.ObservedCurrency, RawEvidenceHash: raw, ProviderObservedAt: observed, RecordedAt: r.RecordedAt.Time.UTC()}
	return domain.ReversalCandidate{PaymentID: domain.ID(r.PaymentID.Bytes), InvoiceID: domain.ID(r.InvoiceID.Bytes), TenantID: domain.ID(r.TenantID.Bytes), SubscriptionID: domain.ID(r.SubscriptionID.Bytes), SettlementObservationID: domain.ID(r.SettlementObservationID.Bytes), PaymentVersion: r.PaymentVersion, InvoiceVersion: r.InvoiceVersion, SubscriptionVersion: r.SubscriptionVersion, PaymentState: r.PaymentState, InvoiceState: r.InvoiceState, SubscriptionState: r.SubscriptionState, Currency: r.Currency, AmountMinorUnits: r.AmountMinorUnits, ReconciliationState: r.ReconciliationState, ReconciliationReason: reason, EffectKind: r.EffectKind, SettlementEffectHash: settlement, Observation: o}, true, nil
}
func (tx *trialTx) CommitPaymentReversal(ctx context.Context, d domain.ReversalDecision, holdID domain.ID, e commercialapp.ReversalEvidence) error {
	rows, err := tx.commercial.MarkPaymentReversed(ctx, commercialsql.MarkPaymentReversedParams{At: commercialTime(d.AppliedAt), PaymentID: commercialID(d.PaymentID), ExpectedVersion: d.PriorPaymentVersion})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrReversalConflict
	}
	rows, err = tx.commercial.MarkReversedInvoiceReviewRequired(ctx, commercialsql.MarkReversedInvoiceReviewRequiredParams{At: commercialTime(d.AppliedAt), InvoiceID: commercialID(d.InvoiceID), TenantID: commercialID(d.TenantID), ExpectedVersion: d.PriorInvoiceVersion})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrReversalConflict
	}
	rows, err = tx.commercial.SuspendReversedSubscription(ctx, commercialsql.SuspendReversedSubscriptionParams{At: commercialTime(d.AppliedAt), SubscriptionID: commercialID(d.SubscriptionID), TenantID: commercialID(d.TenantID), ExpectedVersion: d.PriorSubscriptionVersion})
	if err != nil {
		return err
	}
	if rows != 1 {
		return commercialapp.ErrReversalConflict
	}
	if err = tx.commercial.InsertReversalAccessHold(ctx, commercialsql.InsertReversalAccessHoldParams{ID: commercialID(holdID), TenantID: commercialID(d.TenantID), SubscriptionID: commercialID(d.SubscriptionID), PaymentID: commercialID(d.PaymentID), At: commercialTime(d.AppliedAt), EffectHash: d.AccessHoldHash[:]}); err != nil {
		return paymentInsertError(err)
	}
	observed := pgtype.Timestamptz{}
	if d.ProviderObservedAt != nil {
		observed = commercialTime(*d.ProviderObservedAt)
	}
	if err = tx.commercial.InsertPaymentReversalEffect(ctx, commercialsql.InsertPaymentReversalEffectParams{PaymentID: commercialID(d.PaymentID), InvoiceID: commercialID(d.InvoiceID), SubscriptionID: commercialID(d.SubscriptionID), SettlementObservationID: commercialID(d.SettlementObservationID), ReversalObservationID: commercialID(d.ReversalObservationID), AccessHoldID: commercialID(holdID), PriorPaymentVersion: d.PriorPaymentVersion, ResultingPaymentVersion: d.ResultingPaymentVersion, PriorInvoiceVersion: d.PriorInvoiceVersion, ResultingInvoiceVersion: d.ResultingInvoiceVersion, PriorSubscriptionVersion: d.PriorSubscriptionVersion, ResultingSubscriptionVersion: d.ResultingSubscriptionVersion, AmountMinorUnits: d.AmountMinorUnits, ProviderObservedAt: observed, RecordedAt: commercialTime(d.RecordedAt), AppliedAt: commercialTime(d.AppliedAt), SettlementEffectHash: d.SettlementEffectHash[:], EffectHash: d.EffectHash[:]}); err != nil {
		return paymentInsertError(err)
	}
	return tx.appendReversalEvidence(ctx, e)
}
func (tx *trialTx) appendReversalEvidence(ctx context.Context, e commercialapp.ReversalEvidence) error {
	return tx.evidence.AppendCommercialReversal(ctx, controlapp.CommercialReversalEvidence{AuditID: controlapp.ID(e.AuditID), OutboxID: controlapp.ID(e.OutboxID), CorrelationID: controlapp.ID(e.CorrelationID), ActorID: controlapp.ID(e.ActorID), TenantID: controlapp.ID(e.TenantID), InvoiceID: controlapp.ID(e.InvoiceID), PaymentID: controlapp.ID(e.PaymentID), SubscriptionID: controlapp.ID(e.SubscriptionID), ObservationID: controlapp.ID(e.ObservationID), AccessHoldID: controlapp.ID(e.AccessHoldID), EventType: e.EventType, PaymentState: e.PaymentState, InvoiceState: e.InvoiceState, SubscriptionState: e.SubscriptionState, AccessMode: e.AccessMode, PaymentVersion: e.PaymentVersion, InvoiceVersion: e.InvoiceVersion, SubscriptionVersion: e.SubscriptionVersion, AmountMinorUnits: e.AmountMinorUnits, OccurredAt: e.OccurredAt})
}
func accessHoldHash(effect [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("commercial-reversal-access-hold:v1:"), effect[:]...))
}
