package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrReversalInvalid = errors.New("commercial_reversal_invalid")

type ReversalObservation struct {
	ID                 ID
	NormalizedState    string
	AmountMinorUnits   *int64
	Currency           *string
	RawEvidenceHash    [32]byte
	ProviderObservedAt *time.Time
	RecordedAt         time.Time
}

type ReversalCandidate struct {
	PaymentID, InvoiceID, TenantID, SubscriptionID, SettlementObservationID ID
	PaymentVersion, InvoiceVersion, SubscriptionVersion                     int64
	PaymentState, InvoiceState, SubscriptionState                           string
	Currency                                                                string
	AmountMinorUnits                                                        int64
	ReconciliationState, ReconciliationReason, EffectKind                   string
	SettlementEffectHash                                                    [32]byte
	Observation                                                             ReversalObservation
}

type ReversalDecision struct {
	PaymentID, InvoiceID, TenantID, SubscriptionID, SettlementObservationID, ReversalObservationID ID
	PriorPaymentVersion, ResultingPaymentVersion                                                   int64
	PriorInvoiceVersion, ResultingInvoiceVersion                                                   int64
	PriorSubscriptionVersion, ResultingSubscriptionVersion                                         int64
	AmountMinorUnits                                                                               int64
	Currency, AccessMode                                                                           string
	ProviderObservedAt                                                                             *time.Time
	RecordedAt, AppliedAt                                                                          time.Time
	SettlementEffectHash, EffectHash, AccessHoldHash                                               [32]byte
}

func DecidePaymentReversal(candidate ReversalCandidate, at time.Time) (ReversalDecision, error) {
	o := candidate.Observation
	if at.IsZero() || candidate.PaymentID.IsZero() || candidate.InvoiceID.IsZero() || candidate.TenantID.IsZero() || candidate.SubscriptionID.IsZero() || candidate.SettlementObservationID.IsZero() || candidate.PaymentVersion < 1 || candidate.InvoiceVersion < 1 || candidate.SubscriptionVersion < 1 || candidate.PaymentState != "REVIEW_REQUIRED" || candidate.InvoiceState != "PAID" || (candidate.SubscriptionState != "ACTIVE" && candidate.SubscriptionState != "PAST_DUE" && candidate.SubscriptionState != "REVIEW_REQUIRED") || candidate.Currency != CurrencyKES || candidate.AmountMinorUnits < 1 || candidate.ReconciliationState != "HELD" || candidate.ReconciliationReason != "ACCOUNTING_COMPLETION_REQUIRED" || (candidate.EffectKind != "PLAN_ACTIVATION" && candidate.EffectKind != "ADDITIONAL_LICENCES") || candidate.SettlementEffectHash == [32]byte{} || o.ID.IsZero() || o.NormalizedState != "REVERSED" || o.AmountMinorUnits == nil || *o.AmountMinorUnits != candidate.AmountMinorUnits || o.Currency == nil || *o.Currency != candidate.Currency || o.RawEvidenceHash == [32]byte{} || o.RecordedAt.IsZero() || o.RecordedAt.After(at) || (o.ProviderObservedAt != nil && o.ProviderObservedAt.After(at)) {
		return ReversalDecision{}, ErrReversalInvalid
	}
	d := ReversalDecision{PaymentID: candidate.PaymentID, InvoiceID: candidate.InvoiceID, TenantID: candidate.TenantID, SubscriptionID: candidate.SubscriptionID, SettlementObservationID: candidate.SettlementObservationID, ReversalObservationID: o.ID, PriorPaymentVersion: candidate.PaymentVersion, ResultingPaymentVersion: candidate.PaymentVersion + 1, PriorInvoiceVersion: candidate.InvoiceVersion, ResultingInvoiceVersion: candidate.InvoiceVersion + 1, PriorSubscriptionVersion: candidate.SubscriptionVersion, ResultingSubscriptionVersion: candidate.SubscriptionVersion + 1, AmountMinorUnits: candidate.AmountMinorUnits, Currency: candidate.Currency, AccessMode: "RETAINED_READ_ONLY", ProviderObservedAt: cloneTime(o.ProviderObservedAt), RecordedAt: o.RecordedAt.UTC(), AppliedAt: at.UTC(), SettlementEffectHash: candidate.SettlementEffectHash}
	body, err := json.Marshal(struct {
		PaymentID, InvoiceID, TenantID, SubscriptionID, SettlementObservationID, ReversalObservationID ID
		PaymentVersion, InvoiceVersion, SubscriptionVersion, AmountMinorUnits                          int64
		Currency, AccessMode                                                                           string
		ProviderObservedAt                                                                             *time.Time
		RecordedAt, AppliedAt                                                                          time.Time
		SettlementEffectHash                                                                           [32]byte
	}{d.PaymentID, d.InvoiceID, d.TenantID, d.SubscriptionID, d.SettlementObservationID, d.ReversalObservationID, d.ResultingPaymentVersion, d.ResultingInvoiceVersion, d.ResultingSubscriptionVersion, d.AmountMinorUnits, d.Currency, d.AccessMode, d.ProviderObservedAt, d.RecordedAt, d.AppliedAt, d.SettlementEffectHash})
	if err != nil {
		return ReversalDecision{}, ErrReversalInvalid
	}
	d.EffectHash = sha256.Sum256(append([]byte("commercial-payment-reversal:v1:"), body...))
	d.AccessHoldHash = sha256.Sum256(append([]byte("commercial-reversal-access-hold:v1:"), d.EffectHash[:]...))
	return d, nil
}

func ValidReconciliationReason(value string) bool { return validAdjustmentText(value, 1, 2000) }

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	v := value.UTC()
	return &v
}
