package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

var ErrPaymentInvalid = errors.New("commercial_payment_invalid")
var ErrPaymentTransition = errors.New("commercial_payment_transition_invalid")

type Payment struct {
	ID, InvoiceID, TenantID, MerchantOperationID, ProviderConfigurationVersionID, CreateAttemptID, CreatedBy ID
	ProviderOrderReference                                                                                   *string
	State, Currency, ReconciliationState, ReconciliationReason                                               string
	Version, AmountMinorUnits                                                                                int64
	ReconciliationAttempts                                                                                   int32
	CreatedAt, UpdatedAt                                                                                     time.Time
	ReconciledAt, ReconciliationDueAt, LastQueriedAt                                                         *time.Time
}

type PaymentCreateDisposition string

const (
	PaymentOrderCreated    PaymentCreateDisposition = "ORDER_CREATED"
	PaymentCreateNoEffect  PaymentCreateDisposition = "CONFIRMED_NO_EFFECT"
	PaymentCreateAmbiguous PaymentCreateDisposition = "AMBIGUOUS"
)

type PaymentCreateResult struct {
	Disposition                               PaymentCreateDisposition
	ProviderOrderReference, ProviderStateCode string
	RedirectURL                               string
	RedirectExpiresAt                         *time.Time
	RawEvidenceHash                           [32]byte
	ProviderObservedAt                        *time.Time
}

type PaymentQueryDisposition string

const (
	PaymentOrderAbsent        PaymentQueryDisposition = "ORDER_ABSENT"
	PaymentProviderPending    PaymentQueryDisposition = "PROVIDER_PENDING"
	PaymentTerminalFailed     PaymentQueryDisposition = "TERMINAL_FAILED"
	PaymentTerminalCancelled  PaymentQueryDisposition = "TERMINAL_CANCELLED"
	PaymentAccountingRequired PaymentQueryDisposition = "ACCOUNTING_REQUIRED"
	PaymentIntegrityConflict  PaymentQueryDisposition = "INTEGRITY_CONFLICT"
)

type PaymentQueryResult struct {
	Disposition                                                PaymentQueryDisposition
	ProviderOrderReference, ProviderStateCode, NormalizedState string
	AmountMinorUnits                                           *int64
	Currency                                                   *string
	RawEvidenceHash                                            [32]byte
	ProviderObservedAt                                         *time.Time
}

func NewPaymentIntent(id, invoiceID, tenantID, merchantID, configurationID, attemptID ID, actor ID, amount int64, at, reconcileAt time.Time) (Payment, error) {
	if id.IsZero() || invoiceID.IsZero() || tenantID.IsZero() || merchantID.IsZero() || configurationID.IsZero() || attemptID.IsZero() || actor.IsZero() || amount < 1 || at.IsZero() || reconcileAt.Before(at) {
		return Payment{}, ErrPaymentInvalid
	}
	return Payment{ID: id, InvoiceID: invoiceID, TenantID: tenantID, MerchantOperationID: merchantID, ProviderConfigurationVersionID: configurationID, CreateAttemptID: attemptID, CreatedBy: actor, State: "PROVIDER_CREATE_PENDING", Currency: CurrencyKES, Version: 1, AmountMinorUnits: amount, ReconciliationState: "DUE", ReconciliationReason: "CREATE_AMBIGUOUS", CreatedAt: at.UTC(), UpdatedAt: at.UTC(), ReconciliationDueAt: paymentTimePointer(reconcileAt.UTC())}, nil
}

func (p Payment) Valid() bool {
	if p.ID.IsZero() || p.InvoiceID.IsZero() || p.TenantID.IsZero() || p.MerchantOperationID.IsZero() || p.ProviderConfigurationVersionID.IsZero() || p.CreateAttemptID.IsZero() || p.CreatedBy.IsZero() || p.Version < 1 || p.AmountMinorUnits < 1 || p.Currency != CurrencyKES || p.CreatedAt.IsZero() || p.UpdatedAt.Before(p.CreatedAt) || p.ReconciliationAttempts < 0 || p.ReconciliationAttempts > 9 {
		return false
	}
	return paymentState(p.State) && paymentReconciliationState(p.ReconciliationState)
}

func (r PaymentCreateResult) Valid(allowedRedirectOrigin string) bool {
	switch r.Disposition {
	case PaymentOrderCreated:
		if !safeProviderReference(r.ProviderOrderReference) || !safeProviderCode(r.ProviderStateCode) || r.RawEvidenceHash == [32]byte{} || r.RedirectURL == "" || r.RedirectExpiresAt == nil || r.RedirectExpiresAt.IsZero() {
			return false
		}
		parsed, err := url.Parse(r.RedirectURL)
		origin, originErr := url.Parse(allowedRedirectOrigin)
		return err == nil && originErr == nil && parsed.Scheme == "https" && parsed.Host == origin.Host && origin.Scheme == "https" && parsed.User == nil && parsed.Fragment == ""
	case PaymentCreateNoEffect:
		return r.ProviderOrderReference == "" && safeProviderCode(r.ProviderStateCode) && r.RawEvidenceHash != [32]byte{} && r.RedirectURL == "" && r.RedirectExpiresAt == nil
	case PaymentCreateAmbiguous:
		return r.ProviderOrderReference == "" && r.RedirectURL == "" && r.RedirectExpiresAt == nil
	default:
		return false
	}
}

func (r PaymentQueryResult) Valid(expected Payment) bool {
	if !expected.Valid() || !safeProviderCode(r.ProviderStateCode) || r.RawEvidenceHash == [32]byte{} {
		return false
	}
	if r.ProviderOrderReference != "" && !safeProviderReference(r.ProviderOrderReference) {
		return false
	}
	if expected.ProviderOrderReference != nil && r.ProviderOrderReference != *expected.ProviderOrderReference && r.Disposition != PaymentIntegrityConflict {
		return false
	}
	switch r.Disposition {
	case PaymentOrderAbsent:
		return expected.State == "PROVIDER_CREATE_PENDING" && expected.ProviderOrderReference == nil && r.ProviderOrderReference == "" && r.NormalizedState == "UNKNOWN" && r.AmountMinorUnits == nil && r.Currency == nil
	case PaymentProviderPending:
		return (expected.State == "PROVIDER_CREATE_PENDING" || expected.State == "PROVIDER_PENDING" || expected.State == "AWAITING_CUSTOMER") && r.ProviderOrderReference != "" && r.NormalizedState == "PENDING" && matchesPaymentAmount(r, expected)
	case PaymentTerminalFailed:
		return r.NormalizedState == "FAILED" && matchesPaymentAmount(r, expected)
	case PaymentTerminalCancelled:
		return r.NormalizedState == "CANCELLED" && matchesPaymentAmount(r, expected)
	case PaymentAccountingRequired:
		if r.NormalizedState == "REVERSED" {
			return expected.State == "SETTLED" && r.ProviderOrderReference != "" && matchesPaymentAmount(r, expected)
		}
		return r.NormalizedState == "SETTLED" && r.ProviderOrderReference != "" && matchesPaymentAmount(r, expected)
	case PaymentIntegrityConflict:
		return r.NormalizedState == "UNKNOWN" || !matchesPaymentAmount(r, expected)
	default:
		return false
	}
}

func matchesPaymentAmount(result PaymentQueryResult, expected Payment) bool {
	return result.AmountMinorUnits != nil && result.Currency != nil && *result.AmountMinorUnits == expected.AmountMinorUnits && *result.Currency == expected.Currency
}
func safeProviderReference(value string) bool {
	return len(value) >= 1 && len(value) <= 300 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
func safeProviderCode(value string) bool {
	return len(value) >= 1 && len(value) <= 100 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
func paymentState(value string) bool {
	switch value {
	case "CREATED", "PROVIDER_CREATE_PENDING", "AWAITING_CUSTOMER", "PROVIDER_PENDING", "SETTLED", "FAILED", "CANCELLED", "REVERSED", "REFUND_PENDING", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED":
		return true
	}
	return false
}
func paymentReconciliationState(value string) bool {
	switch value {
	case "NOT_REQUIRED", "DUE", "LEASED", "RETRY_WAIT", "HELD", "COMPLETE":
		return true
	}
	return false
}
func paymentTimePointer(value time.Time) *time.Time { return &value }
