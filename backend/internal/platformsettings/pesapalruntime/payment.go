package pesapalruntime

import (
	commercial "bidora/backend/internal/commercial/application"
	money "bidora/backend/internal/commercial/domain"
	"bidora/backend/internal/platformsettings/domain"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The five-minute deadline is Bidora's local redirect display window, NOT a
// claim that PesaPal supplied an expiry or that an order/payment will expire.
const RedirectDisplayWindow = 5 * time.Minute

func (r *Runtime) CreateOrder(ctx context.Context, c commercial.PaymentProviderConfiguration, in commercial.PaymentCreateRequest) (money.PaymentCreateResult, error) {
	unavailable := money.PaymentCreateResult{Disposition: money.PaymentCreateAmbiguous}
	if !c.Valid() || c.ID != in.ConfigurationVersionID || !r.ValidatePayer(in.PayerEmail, in.PayerCountry) || in.Currency != "KES" || in.AmountMinorUnits < 1 {
		return unavailable, commercial.ErrPaymentRequestInvalid
	}
	current, e := r.Current(ctx, in.TenantID)
	if e != nil || current.ID != c.ID {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	v, e := r.versions.PesaPalVersion(ctx, domain.ID(c.ID))
	if e != nil || v.Configuration.PesaPal == nil || !r.allowed(v.Configuration.PesaPal) {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	cfg := *v.Configuration.PesaPal
	token, e := r.authenticate(ctx, cfg)
	if e != nil {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	body := map[string]any{"id": uuid.UUID(in.MerchantOperationID).String(), "currency": "KES", "amount": json.Number(fmt.Sprintf("%d.%02d", in.AmountMinorUnits/100, in.AmountMinorUnits%100)), "description": "Bidora invoice " + uuid.UUID(in.InvoiceID).String(), "callback_url": r.options.CallbackURL, "notification_id": uuid.UUID(cfg.NotificationID).String(), "billing_address": map[string]string{"email_address": in.PayerEmail, "country_code": in.PayerCountry}}
	payload, e := json.Marshal(body)
	if e != nil {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	defer wipe(payload)
	raw, e := r.send(ctx, cfg, "/Transactions/SubmitOrderRequest", token, payload)
	token = ""
	if e != nil {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	defer wipe(raw)
	var result struct {
		Tracking string `json:"order_tracking_id"`
		Merchant string `json:"merchant_reference"`
		Redirect string `json:"redirect_url"`
		Status   string `json:"status"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Status != "200" || result.Merchant != uuid.UUID(in.MerchantOperationID).String() {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	tracking, e := uuid.Parse(result.Tracking)
	if e != nil || tracking == uuid.Nil {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	until := time.Now().UTC().Add(RedirectDisplayWindow)
	out := money.PaymentCreateResult{Disposition: money.PaymentOrderCreated, ProviderOrderReference: tracking.String(), ProviderStateCode: "ORDER_CREATED", RedirectURL: result.Redirect, RedirectExpiresAt: &until, RawEvidenceHash: sha256.Sum256(raw)}
	if !out.Valid(c.AllowedRedirectOrigin) {
		return unavailable, commercial.ErrPaymentUnavailable
	}
	return out, nil
}

var decimalAmount = regexp.MustCompile(`^(0|[1-9][0-9]*)(?:\.([0-9]{1,2}))?$`)

func AmountMinor(raw string) (int64, bool) {
	parts := decimalAmount.FindStringSubmatch(raw)
	if parts == nil {
		return 0, false
	}
	whole, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil {
		return 0, false
	}
	fraction := parts[2]
	if len(fraction) == 1 {
		fraction += "0"
	}
	cents := int64(0)
	if fraction != "" {
		cents, _ = strconv.ParseInt(fraction, 10, 64)
	}
	if whole > (math.MaxInt64-cents)/100 {
		return 0, false
	}
	return whole*100 + cents, true
}
func (r *Runtime) QueryOrder(ctx context.Context, c commercial.PaymentProviderConfiguration, in commercial.PaymentQueryRequest) (money.PaymentQueryResult, error) {
	if !c.Valid() || c.ID != in.ConfigurationVersionID || in.ProviderOrderReference == nil {
		return money.PaymentQueryResult{}, commercial.ErrPaymentReconciliationRequired
	}
	tracking, e := uuid.Parse(*in.ProviderOrderReference)
	if e != nil || tracking == uuid.Nil {
		return money.PaymentQueryResult{}, commercial.ErrPaymentReconciliationRequired
	}
	v, e := r.versions.PesaPalVersion(ctx, domain.ID(c.ID))
	if e != nil || v.Configuration.PesaPal == nil || !r.allowed(v.Configuration.PesaPal) {
		return money.PaymentQueryResult{}, commercial.ErrPaymentUnavailable
	}
	cfg := *v.Configuration.PesaPal
	token, e := r.authenticate(ctx, cfg)
	if e != nil {
		return money.PaymentQueryResult{}, commercial.ErrPaymentUnavailable
	}
	raw, e := r.send(ctx, cfg, "/Transactions/GetTransactionStatus?orderTrackingId="+url.QueryEscape(tracking.String()), token, nil)
	token = ""
	if e != nil {
		return money.PaymentQueryResult{}, commercial.ErrPaymentUnavailable
	}
	defer wipe(raw)
	var result struct {
		Merchant    string      `json:"merchant_reference"`
		Amount      json.Number `json:"amount"`
		Currency    string      `json:"currency"`
		Code        json.Number `json:"status_code"`
		Description string      `json:"payment_status_description"`
		Status      string      `json:"status"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Status != "200" {
		return money.PaymentQueryResult{}, commercial.ErrPaymentUnavailable
	}
	out := money.PaymentQueryResult{Disposition: money.PaymentIntegrityConflict, ProviderStateCode: "UNVERIFIED_PROVIDER_RESPONSE", NormalizedState: "UNKNOWN", RawEvidenceHash: sha256.Sum256(raw)}
	amount, ok := AmountMinor(result.Amount.String())
	if !ok || result.Merchant != uuid.UUID(in.MerchantOperationID).String() || result.Currency != in.Currency || amount != in.AmountMinorUnits {
		return out, nil
	}
	out.ProviderOrderReference = tracking.String()
	out.AmountMinorUnits = &amount
	out.Currency = &result.Currency
	code, e := result.Code.Int64()
	if e != nil {
		return out, nil
	}
	switch {
	case code == 1 && strings.EqualFold(result.Description, "Completed"):
		out.Disposition = money.PaymentAccountingRequired
		out.NormalizedState = "SETTLED"
		out.ProviderStateCode = "COMPLETED"
	case code == 2 && strings.EqualFold(result.Description, "Failed"):
		out.Disposition = money.PaymentTerminalFailed
		out.NormalizedState = "FAILED"
		out.ProviderStateCode = "FAILED"
	case code == 3 && strings.EqualFold(result.Description, "Reversed"):
		out.Disposition = money.PaymentAccountingRequired
		out.NormalizedState = "REVERSED"
		out.ProviderStateCode = "REVERSED"
	case code == 0 && strings.EqualFold(result.Description, "Pending"):
		out.Disposition = money.PaymentProviderPending
		out.NormalizedState = "PENDING"
		out.ProviderStateCode = "PENDING"
	}
	return out, nil
}
