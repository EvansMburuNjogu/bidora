package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrSettlementInvalid = errors.New("commercial_settlement_invalid")
var ErrSettlementConflict = errors.New("commercial_settlement_conflict")

type SettlementObservation struct {
	ID                 ID
	NormalizedState    string
	AmountMinorUnits   *int64
	Currency           *string
	RawEvidenceHash    [32]byte
	ProviderObservedAt *time.Time
	RecordedAt         time.Time
}

type SettlementSubscription struct {
	ID, TenantID, CurrentTermID      ID
	State                            SubscriptionState
	Version                          int64
	CurrentPlan                      PlanKey
	CurrentCadence                   BillingCadence
	CurrentTermStart, CurrentTermEnd time.Time
}

type SettlementCandidate struct {
	Payment      Payment
	Invoice      Invoice
	Observation  SettlementObservation
	Entitlement  InvoiceEntitlementSnapshot
	Subscription SettlementSubscription
}

type SettlementDecision struct {
	EffectKind                    string
	PriorSubscriptionVersion      int64
	ResultingSubscriptionVersion  int64
	Term                          *TermSnapshot
	AdditionalLicenceQuantity     int64
	EffectiveFrom, EffectiveUntil time.Time
	SettledAt                     time.Time
}

type Receipt struct {
	ID, PaymentID, InvoiceID ID
	Number, Currency         string
	AmountMinorUnits         int64
	IssuedAt                 time.Time
	ContentHash              [32]byte
}

func DecideSettlement(candidate SettlementCandidate, at time.Time) (SettlementDecision, error) {
	p, i, o, e, s := candidate.Payment, candidate.Invoice, candidate.Observation, candidate.Entitlement, candidate.Subscription
	if at.IsZero() || !p.Valid() || p.InvoiceID != i.ID || p.TenantID != i.TenantID || p.State != "REVIEW_REQUIRED" || p.ReconciliationState != "HELD" || p.ReconciliationReason != "ACCOUNTING_COMPLETION_REQUIRED" || p.Currency != CurrencyKES || p.AmountMinorUnits < 1 || i.State != "PAYMENT_PENDING" || i.Currency != CurrencyKES || i.OutstandingMinorUnits != p.AmountMinorUnits || i.TotalMinorUnits < p.AmountMinorUnits || o.ID.IsZero() || o.NormalizedState != "SETTLED" || o.AmountMinorUnits == nil || *o.AmountMinorUnits != p.AmountMinorUnits || o.Currency == nil || *o.Currency != p.Currency || o.RawEvidenceHash == [32]byte{} || o.RecordedAt.IsZero() || s.ID != i.SubscriptionID || s.TenantID != i.TenantID || s.Version != e.ExpectedSubscriptionVersion || e.ExpectedSubscriptionVersion < 1 {
		return SettlementDecision{}, ErrSettlementConflict
	}
	settledAt := o.RecordedAt.UTC()
	if o.ProviderObservedAt != nil {
		settledAt = o.ProviderObservedAt.UTC()
	}
	if settledAt.After(at.UTC()) {
		return SettlementDecision{}, ErrSettlementConflict
	}
	decision := SettlementDecision{EffectKind: e.Kind, PriorSubscriptionVersion: s.Version, ResultingSubscriptionVersion: s.Version + 1, SettledAt: settledAt}
	switch e.Kind {
	case "PLAN_ACTIVATION":
		if i.PurchaseKind != "PLAN_BASE" || e.PlanTerm == nil || !validPaidTerm(*e.PlanTerm) || (s.CurrentPlan != "" && s.CurrentPlan != Trial && s.CurrentPlan != e.PlanTerm.PlanKey) || !e.TargetTermID.IsZero() || e.AdditionalLicences != 0 || e.PlanTerm.PlanKey != i.PlanKey || e.PlanTerm.Cadence != i.Cadence || e.PlanTerm.CatalogueVersion != i.CatalogueVersion || e.PlanTerm.Price.Currency != CurrencyKES || e.PlanTerm.Price.MinorUnits != i.SubtotalMinorUnits || settledAt.Before(e.PlanTerm.CommercialPeriodStart) || !settledAt.Before(e.PlanTerm.CommercialPeriodEnd) {
			return SettlementDecision{}, ErrSettlementConflict
		}
		term := cloneTerm(*e.PlanTerm)
		decision.Term = &term
		decision.EffectiveFrom = term.CommercialPeriodStart
		decision.EffectiveUntil = term.CommercialPeriodEnd
	case "ADDITIONAL_LICENCES":
		if i.PurchaseKind != "ADDITIONAL_LICENCE" || e.PlanTerm != nil || e.TargetTermID.IsZero() || e.TargetTermID != s.CurrentTermID || e.AdditionalLicences != i.Quantity || e.AdditionalLicences < 1 || (s.State != Active && s.State != PastDue) || s.CurrentPlan != i.PlanKey || s.CurrentCadence != i.Cadence || !i.TermEnd.Equal(s.CurrentTermEnd) || settledAt.Before(i.TermStart) || !settledAt.Before(i.TermEnd) {
			return SettlementDecision{}, ErrSettlementConflict
		}
		decision.AdditionalLicenceQuantity = e.AdditionalLicences
		decision.EffectiveFrom = i.TermStart
		decision.EffectiveUntil = i.TermEnd
	default:
		return SettlementDecision{}, ErrSettlementInvalid
	}
	return decision, nil
}

func NewReceipt(id, paymentID, invoiceID ID, number string, amount int64, issuedAt time.Time) (Receipt, error) {
	if id.IsZero() || paymentID.IsZero() || invoiceID.IsZero() || !regexpReceiptNumber(number) || amount < 1 || issuedAt.IsZero() {
		return Receipt{}, ErrSettlementInvalid
	}
	value := Receipt{ID: id, PaymentID: paymentID, InvoiceID: invoiceID, Number: number, Currency: CurrencyKES, AmountMinorUnits: amount, IssuedAt: issuedAt.UTC()}
	encoded, err := json.Marshal(value)
	if err != nil {
		return Receipt{}, ErrSettlementInvalid
	}
	value.ContentHash = sha256.Sum256(append([]byte("commercial-receipt:v1:"), encoded...))
	return value, nil
}

func SettlementEffectHash(candidate SettlementCandidate, decision SettlementDecision, receipt Receipt, resultingTermID, grantID ID) ([32]byte, error) {
	if receipt.ContentHash == [32]byte{} || decision.ResultingSubscriptionVersion != decision.PriorSubscriptionVersion+1 {
		return [32]byte{}, ErrSettlementInvalid
	}
	encoded, err := json.Marshal(struct {
		PaymentID, InvoiceID, ObservationID, SubscriptionID, ResultingTermID, GrantID ID
		EffectKind                                                                    string
		PriorVersion, ResultingVersion, Amount                                        int64
		SettledAt                                                                     time.Time
		ReceiptHash                                                                   [32]byte
	}{candidate.Payment.ID, candidate.Invoice.ID, candidate.Observation.ID, candidate.Subscription.ID, resultingTermID, grantID, decision.EffectKind, decision.PriorSubscriptionVersion, decision.ResultingSubscriptionVersion, candidate.Payment.AmountMinorUnits, decision.SettledAt, receipt.ContentHash})
	if err != nil {
		return [32]byte{}, ErrSettlementInvalid
	}
	return sha256.Sum256(append([]byte("commercial-settlement-effect:v1:"), encoded...)), nil
}

func regexpReceiptNumber(value string) bool { return validFinancialNumber(value, "BDR-RCT-") }
func validFinancialNumber(value, prefix string) bool {
	if len(value) != len(prefix)+4+1+8 || value[:len(prefix)] != prefix || value[len(prefix)+4] != '-' {
		return false
	}
	for index, r := range value[len(prefix):] {
		if index == 4 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
