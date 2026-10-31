package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"time"
)

var ErrAdjustmentInvalid = errors.New("commercial_adjustment_invalid")

const AdjustmentApprovalWindow = 30 * time.Minute
const RefundEligibilityWindow = 90 * 24 * time.Hour

type AdjustmentCandidate struct {
	InvoiceID, TenantID, PaymentID                          ID
	InvoiceVersion, PaymentVersion                          int64
	InvoiceTaxableMinor, InvoiceTaxMinor, InvoiceTotalMinor int64
	PriorCreditMinor, PaymentAmountMinor                    int64
	PaymentProvider, EntitlementKind                        string
	SettledAt                                               time.Time
	TaxProfile                                              TaxProfile
	TaxProfileHash                                          [32]byte
}

type AdjustmentProposal struct {
	ID, TenantID, InvoiceID, PaymentID, MakerUserID            ID
	Action, ReasonCode, EntitlementDisposition, ProposalReason string
	AmountMinorUnits, NetMinorUnits, TaxMinorUnits             int64
	ExpectedInvoiceVersion, ExpectedPaymentVersion             int64
	TaxProfileID                                               ID
	TaxProfileHash, MakerAuthorityHash, RequestHash            [32]byte
	Version                                                    int64
	CreatedAt, ExpiresAt                                       time.Time
}

func NewAdjustmentProposal(id, maker ID, action, reason, disposition, explanation string, amount int64, candidate AdjustmentCandidate, authorityHash, requestHash [32]byte, refundWindow time.Duration, at time.Time) (AdjustmentProposal, error) {
	if id.IsZero() || maker.IsZero() || at.IsZero() || authorityHash == [32]byte{} || requestHash == [32]byte{} || !validAdjustmentText(explanation, 1, 2000) || !validAdjustmentReason(reason) || !validDisposition(disposition) || (action != "ISSUE_CREDIT" && action != "REQUEST_REFUND") || amount < 1 || candidate.InvoiceID.IsZero() || candidate.TenantID.IsZero() || candidate.PaymentID.IsZero() || candidate.InvoiceVersion < 1 || candidate.PaymentVersion < 1 || candidate.PaymentProvider != "PESAPAL" || candidate.PaymentAmountMinor != candidate.InvoiceTotalMinor || candidate.PriorCreditMinor < 0 || candidate.PriorCreditMinor > candidate.InvoiceTotalMinor || amount > candidate.InvoiceTotalMinor-candidate.PriorCreditMinor || candidate.TaxProfile.ID.IsZero() || !candidate.TaxProfile.ActiveAt(candidate.TaxProfile.EffectiveFrom) || candidate.TaxProfileHash != candidate.TaxProfile.ConfigurationHash {
		return AdjustmentProposal{}, ErrAdjustmentInvalid
	}
	if action == "ISSUE_CREDIT" && disposition != "NO_ENTITLEMENT_CHANGE" {
		return AdjustmentProposal{}, ErrAdjustmentInvalid
	}
	if action == "REQUEST_REFUND" {
		if refundWindow <= 0 || refundWindow > RefundEligibilityWindow || disposition == "NO_ENTITLEMENT_CHANGE" || candidate.SettledAt.IsZero() || at.Before(candidate.SettledAt) || at.After(candidate.SettledAt.Add(refundWindow)) {
			return AdjustmentProposal{}, ErrAdjustmentInvalid
		}
		switch candidate.EntitlementKind {
		case "PLAN_ACTIVATION":
			if disposition == "REDUCE_ADDITIONAL_LICENCES_AT_RENEWAL" || (disposition == "END_PAID_ACCESS_ON_REFUND_SETTLEMENT" && (candidate.PriorCreditMinor != 0 || amount != candidate.InvoiceTotalMinor)) {
				return AdjustmentProposal{}, ErrAdjustmentInvalid
			}
		case "ADDITIONAL_LICENCES":
			if disposition == "END_PAID_ACCESS_ON_REFUND_SETTLEMENT" {
				return AdjustmentProposal{}, ErrAdjustmentInvalid
			}
		default:
			return AdjustmentProposal{}, ErrAdjustmentInvalid
		}
	}
	net, tax, err := allocateCredit(amount, candidate)
	if err != nil {
		return AdjustmentProposal{}, err
	}
	return AdjustmentProposal{ID: id, TenantID: candidate.TenantID, InvoiceID: candidate.InvoiceID, PaymentID: candidate.PaymentID, MakerUserID: maker, Action: action, ReasonCode: reason, EntitlementDisposition: disposition, ProposalReason: explanation, AmountMinorUnits: amount, NetMinorUnits: net, TaxMinorUnits: tax, ExpectedInvoiceVersion: candidate.InvoiceVersion, ExpectedPaymentVersion: candidate.PaymentVersion, TaxProfileID: candidate.TaxProfile.ID, TaxProfileHash: candidate.TaxProfileHash, MakerAuthorityHash: authorityHash, RequestHash: requestHash, Version: 1, CreatedAt: at.UTC(), ExpiresAt: at.UTC().Add(AdjustmentApprovalWindow)}, nil
}

func allocateCredit(total int64, c AdjustmentCandidate) (int64, int64, error) {
	if total == c.InvoiceTotalMinor {
		return c.InvoiceTaxableMinor, c.InvoiceTaxMinor, nil
	}
	lo, hi := int64(0), total
	for lo <= hi {
		mid := lo + (hi-lo)/2
		tax, err := CalculateTax(mid, c.TaxProfile, c.TaxProfile.EffectiveFrom)
		if err != nil {
			return 0, 0, ErrAdjustmentInvalid
		}
		if mid > math.MaxInt64-tax {
			return 0, 0, ErrAdjustmentInvalid
		}
		gross := mid + tax
		if gross == total {
			return mid, tax, nil
		}
		if gross < total {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return 0, 0, ErrAdjustmentInvalid
}

func AdjustmentAuthorityHash(user ID, contextGeneration, administratorVersion int64, role, capability, assurance string, assuredAt time.Time) ([32]byte, error) {
	if user.IsZero() || contextGeneration < 1 || administratorVersion < 1 || role == "" || capability == "" || assurance != "AAL2:TOTP" || assuredAt.IsZero() {
		return [32]byte{}, ErrAdjustmentInvalid
	}
	b, err := json.Marshal(struct {
		User       ID        `json:"user"`
		Context    int64     `json:"context"`
		Version    int64     `json:"version"`
		Role       string    `json:"role"`
		Capability string    `json:"capability"`
		Assurance  string    `json:"assurance"`
		AssuredAt  time.Time `json:"assured_at"`
	}{user, contextGeneration, administratorVersion, role, capability, assurance, assuredAt.UTC().Truncate(time.Microsecond)})
	if err != nil {
		return [32]byte{}, ErrAdjustmentInvalid
	}
	return sha256.Sum256(append([]byte("commercial-adjustment-authority:v1:"), b...)), nil
}

func CreditNoteHash(p AdjustmentProposal, number, state string, checker ID, at time.Time) ([32]byte, error) {
	if p.ID.IsZero() || number == "" || (state != "ISSUED" && state != "REFUND_PENDING") || checker.IsZero() || at.IsZero() {
		return [32]byte{}, ErrAdjustmentInvalid
	}
	b, err := json.Marshal(struct {
		Change, Invoice, Payment, TaxProfile ID
		Number, State, Action, Reason        string
		Net, Tax, Total                      int64
		TaxProfileHash                       [32]byte
		Checker                              ID
		At                                   time.Time
	}{p.ID, p.InvoiceID, p.PaymentID, p.TaxProfileID, number, state, p.Action, p.ReasonCode, p.NetMinorUnits, p.TaxMinorUnits, p.AmountMinorUnits, p.TaxProfileHash, checker, at.UTC().Truncate(time.Microsecond)})
	if err != nil {
		return [32]byte{}, ErrAdjustmentInvalid
	}
	return sha256.Sum256(append([]byte("commercial-credit-note:v1:"), b...)), nil
}

func validAdjustmentReason(v string) bool {
	switch v {
	case "SERVICE_CREDIT", "DUPLICATE_PAYMENT_REFUND", "SERVICE_FAILURE_REFUND", "PAYMENT_REVERSAL_CORRECTION", "OTHER_CONTRACTUAL_REFUND":
		return true
	}
	return false
}
func validDisposition(v string) bool {
	switch v {
	case "NO_ENTITLEMENT_CHANGE", "REDUCE_ADDITIONAL_LICENCES_AT_RENEWAL", "END_PAID_ACCESS_ON_REFUND_SETTLEMENT", "REVIEW_REQUIRED":
		return true
	}
	return false
}
func validAdjustmentText(v string, min, max int) bool { return safeFinancialText(v, min, max) }
