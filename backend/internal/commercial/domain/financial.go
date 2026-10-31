package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const FinancialArithmeticProfile = "KES_COMMERCIAL_V1"
const TaxRoundingProfile = "KES_COMMERCIAL_V1_HALF_UP_PER_TAX_CODE"

var ErrFinancialInvalid = errors.New("commercial_financial_invalid")
var ErrFinancialOverflow = errors.New("commercial_financial_overflow")

var financialCode = regexp.MustCompile(`^[A-Z0-9_]+$`)

type TaxProfile struct {
	ID                       ID
	Version                  int64
	Country, Currency        string
	TaxCode, DisplayLabel    string
	RateNumerator            int64
	RateDenominator          int64
	RoundingProfile          string
	EffectiveFrom            time.Time
	EffectiveUntil           *time.Time
	ConfigurationHash        [32]byte
	LegalReviewReferenceHash [32]byte
}

type BillingSnapshot struct {
	Version                                                    int64
	SellerLegalName, SellerTaxIdentifier, SellerBillingAddress string
	CustomerLegalName, CustomerBillingAddress                  string
	CustomerTaxIdentifier                                      *string
	Hash                                                       [32]byte
}

type InvoiceEntitlementSnapshot struct {
	Kind                        string
	ExpectedSubscriptionVersion int64
	TargetTermID                ID
	PlanTerm                    *TermSnapshot
	AdditionalLicences          int64
}

type PurchaseQuote struct {
	OperationID                                 ID
	Kind                                        string
	PlanKey                                     PlanKey
	CatalogueVersion                            string
	Cadence                                     BillingCadence
	Quantity, UnitMinorUnits, NetMinorUnits     int64
	ProrationNumerator, ProrationDenominator    int64
	TermStart, TermEnd, CalculatedAt, ExpiresAt time.Time
	Entitlement                                 InvoiceEntitlementSnapshot
	Hash                                        [32]byte
}

type InvoiceLine struct {
	ID                                       ID
	Number                                   int32
	Kind, DescriptionCode, TaxCode           string
	Quantity, UnitMinorUnits, NetMinorUnits  int64
	ProrationNumerator, ProrationDenominator int64
	TaxMinorUnits, GrossMinorUnits           int64
	CalculationHash                          [32]byte
}

type CreditNote struct {
	ID, InvoiceID, ChangeID                                ID
	Number, State, ReasonCode, Currency                    string
	Version, NetMinorUnits, TaxMinorUnits, TotalMinorUnits int64
	IssuedAt                                               time.Time
	ContentHash                                            [32]byte
}

func (n CreditNote) Valid() bool {
	return !n.ID.IsZero() && !n.InvoiceID.IsZero() && !n.ChangeID.IsZero() && n.Number != "" && validCreditNoteState(n.State) && n.Version > 0 && n.Currency == CurrencyKES && n.NetMinorUnits >= 0 && n.TaxMinorUnits >= 0 && n.TotalMinorUnits > 0 && n.TotalMinorUnits == n.NetMinorUnits+n.TaxMinorUnits && n.IssuedAt.IsZero() == false && n.ContentHash != [32]byte{} && validAdjustmentReason(n.ReasonCode)
}

func validCreditNoteState(value string) bool {
	switch value {
	case "ISSUED", "REFUND_PENDING", "SETTLED", "REVIEW_REQUIRED", "VOID":
		return true
	}
	return false
}

type Invoice struct {
	ID, TenantID, SubscriptionID, PurchaseOperationID, TaxProfileID, IssuedBy  ID
	Number, State, PurchaseKind, CatalogueVersion, Currency, ArithmeticProfile string
	PlanKey                                                                    PlanKey
	Cadence                                                                    BillingCadence
	Version, Quantity                                                          int64
	TermStart, TermEnd, IssuedAt, DueAt                                        time.Time
	SubtotalMinorUnits, CreditMinorUnits, TaxableMinorUnits                    int64
	TaxMinorUnits, TotalMinorUnits, OutstandingMinorUnits                      int64
	TaxProfileHash, BillingSnapshotHash, QuoteHash                             [32]byte
	BillingSnapshotVersion                                                     int64
	Billing                                                                    BillingSnapshot
	Entitlement                                                                InvoiceEntitlementSnapshot
	Lines                                                                      []InvoiceLine
}

func (p TaxProfile) ActiveAt(at time.Time) bool {
	if p.ID.IsZero() || p.Version < 1 || p.Country != "KE" || p.Currency != CurrencyKES || !safeFinancialText(p.TaxCode, 1, 40) || !financialCode.MatchString(p.TaxCode) || !safeFinancialText(p.DisplayLabel, 1, 200) || p.RateNumerator < 0 || p.RateDenominator < 1 || p.RateNumerator > p.RateDenominator || p.RateDenominator > 1_000_000 || p.RoundingProfile != TaxRoundingProfile || p.EffectiveFrom.IsZero() || p.ConfigurationHash == [32]byte{} || p.LegalReviewReferenceHash == [32]byte{} || at.IsZero() {
		return false
	}
	at = at.UTC()
	if at.Before(p.EffectiveFrom.UTC()) {
		return false
	}
	return p.EffectiveUntil == nil || at.Before(p.EffectiveUntil.UTC())
}

func NewBillingSnapshot(version int64, sellerName, sellerTax, sellerAddress, customerName string, customerTax *string, customerAddress string) (BillingSnapshot, error) {
	value := BillingSnapshot{Version: version, SellerLegalName: sellerName, SellerTaxIdentifier: sellerTax, SellerBillingAddress: sellerAddress, CustomerLegalName: customerName, CustomerTaxIdentifier: cloneString(customerTax), CustomerBillingAddress: customerAddress}
	if version < 1 || !safeFinancialText(sellerName, 1, 300) || !safeFinancialText(sellerTax, 1, 200) || !safeFinancialText(sellerAddress, 1, 1000) || !safeFinancialText(customerName, 1, 300) || !optionalFinancialText(customerTax, 1, 200) || !safeFinancialText(customerAddress, 1, 1000) {
		return BillingSnapshot{}, ErrFinancialInvalid
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return BillingSnapshot{}, ErrFinancialInvalid
	}
	value.Hash = sha256.Sum256(append([]byte("commercial-billing-snapshot:v1:"), encoded...))
	return value, nil
}

func (b BillingSnapshot) Valid() bool {
	if b.Version < 1 || !safeFinancialText(b.SellerLegalName, 1, 300) || !safeFinancialText(b.SellerTaxIdentifier, 1, 200) || !safeFinancialText(b.SellerBillingAddress, 1, 1000) || !safeFinancialText(b.CustomerLegalName, 1, 300) || !optionalFinancialText(b.CustomerTaxIdentifier, 1, 200) || !safeFinancialText(b.CustomerBillingAddress, 1, 1000) {
		return false
	}
	original := b.Hash
	b.Hash = [32]byte{}
	encoded, err := json.Marshal(b)
	return err == nil && original != [32]byte{} && sha256.Sum256(append([]byte("commercial-billing-snapshot:v1:"), encoded...)) == original
}

func NewPlanInvoiceEntitlement(expectedVersion int64, term TermSnapshot) (InvoiceEntitlementSnapshot, error) {
	if expectedVersion < 1 || !validPaidTerm(term) {
		return InvoiceEntitlementSnapshot{}, ErrFinancialInvalid
	}
	copy := cloneTerm(term)
	return InvoiceEntitlementSnapshot{Kind: "PLAN_ACTIVATION", ExpectedSubscriptionVersion: expectedVersion, PlanTerm: &copy}, nil
}

func NewAdditionalLicenceEntitlement(expectedVersion int64, targetTermID ID, quantity int64) (InvoiceEntitlementSnapshot, error) {
	if expectedVersion < 1 || targetTermID.IsZero() || quantity < 1 || quantity > 10000 {
		return InvoiceEntitlementSnapshot{}, ErrFinancialInvalid
	}
	return InvoiceEntitlementSnapshot{Kind: "ADDITIONAL_LICENCES", ExpectedSubscriptionVersion: expectedVersion, TargetTermID: targetTermID, AdditionalLicences: quantity}, nil
}

func (e InvoiceEntitlementSnapshot) Hash() ([32]byte, error) {
	if e.ExpectedSubscriptionVersion < 1 || (e.Kind != "PLAN_ACTIVATION" && e.Kind != "ADDITIONAL_LICENCES") {
		return [32]byte{}, ErrFinancialInvalid
	}
	if e.PlanTerm != nil {
		term := cloneTerm(*e.PlanTerm)
		term.CommercialPeriodStart = term.CommercialPeriodStart.UTC().Truncate(time.Microsecond)
		term.CommercialPeriodEnd = term.CommercialPeriodEnd.UTC().Truncate(time.Microsecond)
		term.UsagePeriodStart = term.UsagePeriodStart.UTC().Truncate(time.Microsecond)
		term.UsagePeriodEnd = term.UsagePeriodEnd.UTC().Truncate(time.Microsecond)
		e.PlanTerm = &term
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		return [32]byte{}, ErrFinancialInvalid
	}
	return sha256.Sum256(append([]byte("commercial-invoice-entitlement:v1:"), encoded...)), nil
}

func NewPurchaseQuote(operationID ID, kind string, plan PlanKey, cadence BillingCadence, catalogue string, quantity, unitMinor, netMinor, numerator, denominator int64, termStart, termEnd, calculatedAt time.Time, entitlement InvoiceEntitlementSnapshot) (PurchaseQuote, error) {
	q := PurchaseQuote{OperationID: operationID, Kind: kind, PlanKey: plan, Cadence: cadence, CatalogueVersion: catalogue, Quantity: quantity, UnitMinorUnits: unitMinor, NetMinorUnits: netMinor, ProrationNumerator: numerator, ProrationDenominator: denominator, TermStart: termStart.UTC(), TermEnd: termEnd.UTC(), CalculatedAt: calculatedAt.UTC(), ExpiresAt: calculatedAt.UTC().Add(15 * time.Minute), Entitlement: entitlement}
	if operationID.IsZero() || (kind != "PLAN_BASE" && kind != "ADDITIONAL_LICENCE") || plan == Trial || catalogue == "" || quantity < 1 || unitMinor < 1 || netMinor < 1 || numerator < 1 || denominator < 1 || termStart.IsZero() || !termEnd.After(termStart) || calculatedAt.IsZero() || (cadence != MonthlyCadence && cadence != AnnualCadence && cadence != ContractCadence) || entitlement.ExpectedSubscriptionVersion < 1 {
		return PurchaseQuote{}, ErrFinancialInvalid
	}
	if kind == "PLAN_BASE" {
		if quantity != 1 || entitlement.Kind != "PLAN_ACTIVATION" || entitlement.PlanTerm == nil || !entitlement.TargetTermID.IsZero() || entitlement.AdditionalLicences != 0 || !validPaidTerm(*entitlement.PlanTerm) || entitlement.PlanTerm.PlanKey != plan || entitlement.PlanTerm.Cadence != cadence || entitlement.PlanTerm.CatalogueVersion != catalogue || entitlement.PlanTerm.Price.Currency != CurrencyKES || entitlement.PlanTerm.Price.MinorUnits != netMinor || !entitlement.PlanTerm.CommercialPeriodStart.Equal(q.TermStart) || !entitlement.PlanTerm.CommercialPeriodEnd.Equal(q.TermEnd) {
			return PurchaseQuote{}, ErrFinancialInvalid
		}
	} else if entitlement.Kind != "ADDITIONAL_LICENCES" || entitlement.PlanTerm != nil || entitlement.TargetTermID.IsZero() || entitlement.AdditionalLicences != quantity {
		return PurchaseQuote{}, ErrFinancialInvalid
	}
	expectedNumerator := new(big.Int).Mul(big.NewInt(quantity), big.NewInt(unitMinor))
	expectedNumerator.Mul(expectedNumerator, big.NewInt(numerator))
	expectedNumerator.Add(expectedNumerator, big.NewInt(denominator-1))
	expected := expectedNumerator.Quo(expectedNumerator, big.NewInt(denominator))
	if !expected.IsInt64() || expected.Int64() != netMinor {
		return PurchaseQuote{}, ErrFinancialInvalid
	}
	encoded, err := json.Marshal(q)
	if err != nil {
		return PurchaseQuote{}, ErrFinancialInvalid
	}
	q.Hash = sha256.Sum256(append([]byte("commercial-purchase-quote:v1:"), encoded...))
	return q, nil
}

func (q PurchaseQuote) Valid() bool {
	rebuilt, err := NewPurchaseQuote(q.OperationID, q.Kind, q.PlanKey, q.Cadence, q.CatalogueVersion, q.Quantity, q.UnitMinorUnits, q.NetMinorUnits, q.ProrationNumerator, q.ProrationDenominator, q.TermStart, q.TermEnd, q.CalculatedAt, q.Entitlement)
	return err == nil && rebuilt.ExpiresAt.Equal(q.ExpiresAt) && rebuilt.Hash == q.Hash
}

func CalculateTax(taxableMinor int64, profile TaxProfile, at time.Time) (int64, error) {
	if taxableMinor < 0 || !profile.ActiveAt(at) {
		return 0, ErrFinancialInvalid
	}
	numerator := new(big.Int).Mul(big.NewInt(taxableMinor), big.NewInt(profile.RateNumerator))
	numerator.Add(numerator, big.NewInt(profile.RateDenominator/2))
	value := numerator.Quo(numerator, big.NewInt(profile.RateDenominator))
	if !value.IsInt64() || value.Sign() < 0 {
		return 0, ErrFinancialOverflow
	}
	return value.Int64(), nil
}

func ProratedLicenceCharge(monthlyMinor, remainingNanoseconds, periodNanoseconds int64, futureFullPeriods int32) (int64, error) {
	if monthlyMinor < 1 || remainingNanoseconds < 1 || periodNanoseconds < remainingNanoseconds || futureFullPeriods < 0 || futureFullPeriods > 12 {
		return 0, ErrFinancialInvalid
	}
	partialNumerator := new(big.Int).Mul(big.NewInt(monthlyMinor), big.NewInt(remainingNanoseconds))
	partialNumerator.Add(partialNumerator, big.NewInt(periodNanoseconds-1))
	partial := partialNumerator.Quo(partialNumerator, big.NewInt(periodNanoseconds))
	full := new(big.Int).Mul(big.NewInt(monthlyMinor), big.NewInt(int64(futureFullPeriods)))
	partial.Add(partial, full)
	if !partial.IsInt64() || partial.Sign() <= 0 {
		return 0, ErrFinancialOverflow
	}
	return partial.Int64(), nil
}

func NewIssuedInvoice(id, tenantID, subscriptionID, issuedBy ID, number string, quote PurchaseQuote, tax TaxProfile, billing BillingSnapshot, issuedAt, dueAt time.Time, lineID, taxLineID ID) (Invoice, error) {
	if id.IsZero() || tenantID.IsZero() || subscriptionID.IsZero() || issuedBy.IsZero() || lineID.IsZero() || taxLineID.IsZero() || quote.OperationID.IsZero() || !quote.Valid() || !billing.Valid() || issuedAt.IsZero() || dueAt.Before(issuedAt) || issuedAt.UTC().After(quote.ExpiresAt) || !tax.ActiveAt(issuedAt) || !validInvoiceNumber(number) {
		return Invoice{}, ErrFinancialInvalid
	}
	taxMinor, err := CalculateTax(quote.NetMinorUnits, tax, issuedAt)
	if err != nil || quote.NetMinorUnits > math.MaxInt64-taxMinor {
		if err != nil {
			return Invoice{}, err
		}
		return Invoice{}, ErrFinancialOverflow
	}
	total := quote.NetMinorUnits + taxMinor
	baseHash := lineHash(quote.Kind, quote.Quantity, quote.UnitMinorUnits, quote.NetMinorUnits, 0, quote.NetMinorUnits, quote.ProrationNumerator, quote.ProrationDenominator, tax.TaxCode)
	taxHash := lineHash("VAT", 1, taxMinor, 0, taxMinor, taxMinor, 1, 1, tax.TaxCode)
	lines := []InvoiceLine{
		{ID: lineID, Number: 1, Kind: quote.Kind, DescriptionCode: quote.Kind, TaxCode: tax.TaxCode, Quantity: quote.Quantity, UnitMinorUnits: quote.UnitMinorUnits, NetMinorUnits: quote.NetMinorUnits, ProrationNumerator: quote.ProrationNumerator, ProrationDenominator: quote.ProrationDenominator, TaxMinorUnits: 0, GrossMinorUnits: quote.NetMinorUnits, CalculationHash: baseHash},
		{ID: taxLineID, Number: 2, Kind: "VAT", DescriptionCode: "VAT", TaxCode: tax.TaxCode, Quantity: 1, UnitMinorUnits: taxMinor, NetMinorUnits: 0, ProrationNumerator: 1, ProrationDenominator: 1, TaxMinorUnits: taxMinor, GrossMinorUnits: taxMinor, CalculationHash: taxHash},
	}
	return Invoice{ID: id, TenantID: tenantID, SubscriptionID: subscriptionID, PurchaseOperationID: quote.OperationID, TaxProfileID: tax.ID, IssuedBy: issuedBy, Number: number, State: "ISSUED", PurchaseKind: quote.Kind, CatalogueVersion: quote.CatalogueVersion, Currency: CurrencyKES, ArithmeticProfile: FinancialArithmeticProfile, PlanKey: quote.PlanKey, Cadence: quote.Cadence, Version: 1, Quantity: quote.Quantity, TermStart: quote.TermStart, TermEnd: quote.TermEnd, IssuedAt: issuedAt.UTC(), DueAt: dueAt.UTC(), SubtotalMinorUnits: quote.NetMinorUnits, TaxableMinorUnits: quote.NetMinorUnits, TaxMinorUnits: taxMinor, TotalMinorUnits: total, OutstandingMinorUnits: total, TaxProfileHash: tax.ConfigurationHash, BillingSnapshotHash: billing.Hash, QuoteHash: quote.Hash, BillingSnapshotVersion: billing.Version, Billing: billing, Entitlement: quote.Entitlement, Lines: lines}, nil
}

func FormatFinancialNumber(kind string, year int, sequence int64) (string, error) {
	prefix := map[string]string{"INVOICE": "INV", "CREDIT_NOTE": "CRN", "RECEIPT": "RCT"}[kind]
	if prefix == "" || year < 2020 || year > 9999 || sequence < 1 || sequence > 99_999_999 {
		return "", ErrFinancialInvalid
	}
	return fmt.Sprintf("BDR-%s-%04d-%08d", prefix, year, sequence), nil
}

func validInvoiceNumber(value string) bool {
	return regexp.MustCompile(`^BDR-INV-[0-9]{4}-[0-9]{8}$`).MatchString(value)
}
func lineHash(kind string, quantity, unit, net, tax, gross, numerator, denominator int64, taxCode string) [32]byte {
	encoded, _ := json.Marshal([]any{kind, quantity, unit, net, tax, gross, numerator, denominator, taxCode, FinancialArithmeticProfile})
	return sha256.Sum256(append([]byte("commercial-invoice-line:v1:"), encoded...))
}
func safeFinancialText(value string, min, max int) bool {
	return utf8.ValidString(value) && len(value) >= min && len(value) <= max && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
func optionalFinancialText(value *string, min, max int) bool {
	return value == nil || safeFinancialText(*value, min, max)
}
func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
