package platformsettings_test

import (
	commercial "bidora/backend/internal/commercial/application"
	money "bidora/backend/internal/commercial/domain"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/pesapalruntime"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strings"
	"testing"
)

type pspVersions struct {
	active     domain.Version
	historical map[domain.ID]domain.Version
}

func (v *pspVersions) ActivePesaPal(context.Context) (domain.Version, error) { return v.active, nil }
func (v *pspVersions) PesaPalVersion(_ context.Context, id domain.ID) (domain.Version, error) {
	return v.historical[id], nil
}

type pspPolicies struct{ p app.OperationalPolicy }

func (p *pspPolicies) CurrentPolicy(context.Context) (app.OperationalPolicy, error) { return p.p, nil }

type pspSecrets struct{}

func (pspSecrets) WithSecret(_ context.Context, _ string, use func([]byte) error) error {
	return use([]byte(`{"consumer_key":"synthetic-consumer-key","consumer_secret":"synthetic-consumer-secret"}`))
}

type pspHTTP struct {
	calls     int
	payloads  []string
	responses []string
}

func (h *pspHTTP) Do(r *http.Request) (*http.Response, error) {
	h.calls++
	if r.URL.Scheme != "https" || r.URL.Host != "cybqa.pesapal.com" {
		return nil, fmt.Errorf("unapproved provider host")
	}
	if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		h.payloads = append(h.payloads, string(raw))
	}
	body := `{"status":"200","token":"synthetic-bearer-token-for-test"}`
	if strings.Contains(r.URL.Path, "Transactions/") {
		if len(h.responses) == 0 {
			return nil, fmt.Errorf("unexpected transaction request")
		}
		body = h.responses[0]
		h.responses = h.responses[1:]
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}
func pspVersion(t *testing.T, id byte, env string, enabled bool) domain.Version {
	t.Helper()
	c, h, e := domain.NewPesaPalConfiguration(domain.PesaPalConfiguration{Enabled: enabled, Environment: env, MerchantName: "Synthetic test merchant", CredentialsSecretRef: "synthetic-secret-ref", NotificationID: domain.ID{4}})
	if e != nil {
		t.Fatal(e)
	}
	return domain.Version{ID: domain.ID{id}, Kind: domain.PesaPal, Version: 1, State: domain.Active, Configuration: c, ConfigurationHash: h}
}
func TestPesaPalConfigurationAndSecretsAreTypedRedactedAndReplayStable(t *testing.T) {
	v := pspVersion(t, 1, "SANDBOX", true)
	if !v.Configuration.Valid() || v.Configuration.PesaPal.API() != domain.PesaPalSandboxAPI {
		t.Fatal("sandbox configuration invalid")
	}
	v.Configuration.PesaPal.Environment = "http://untrusted"
	if v.Configuration.Valid() {
		t.Fatal("caller endpoint accepted")
	}
	vault, e := smtpsecret.NewVault(&postgres.Pool{}, make([]byte, 32), 1)
	if e != nil {
		t.Fatal(e)
	}
	key, secret := "synthetic-consumer-key", "synthetic-consumer-secret"
	ref, a, e := vault.PreparePesaPalSecrets(domain.ID{1}, "request-one", &key, &secret)
	if e != nil || len(a) != 1 || a[0].Kind != "PESAPAL_CREDENTIALS" || !strings.HasPrefix(ref, "dbpesapal:v1:") {
		t.Fatal("typed encryption failed", e)
	}
	again, b, e := vault.PreparePesaPalSecrets(domain.ID{1}, "request-one", &key, &secret)
	if e != nil || ref != again || a[0].ID != b[0].ID || string(a[0].Nonce) == string(b[0].Nonce) || strings.Contains(string(a[0].Ciphertext), secret) {
		t.Fatal("sealed replay reference/nonce invalid")
	}
	if _, _, e = vault.PreparePesaPalSecrets(domain.ID{1}, "request-one", &key, nil); e == nil {
		t.Fatal("partial replacement accepted")
	}
	credentials, e := smtpsecret.ParsePesaPalCredentials([]byte(`{"consumer_key":"synthetic-consumer-key","consumer_secret":"synthetic-consumer-secret"}`))
	if e != nil || strings.Contains(fmt.Sprintf("%+v", credentials), secret) {
		t.Fatal("credentials printed")
	}
}
func TestPesaPalRuntimeSeparatesCurrentPauseHistoricalReconciliationAndAuthentication(t *testing.T) {
	ctx := context.Background()
	old := pspVersion(t, 1, "SANDBOX", true)
	versions := &pspVersions{active: old, historical: map[domain.ID]domain.Version{old.ID: old}}
	policies := &pspPolicies{validPolicy()}
	network := &pspHTTP{}
	r, e := pesapalruntime.New(versions, pspSecrets{}, policies, pesapalruntime.Options{CallbackURL: "https://bidora.example/payments/pesapal/callback"}, network)
	if e != nil {
		t.Fatal(e)
	}
	if !r.ValidatePayer("payer@example.test", "KE") || r.ValidatePayer("Name <payer@example.test>", "KE") || r.ValidatePayer("payer@example.test", "ZZ") {
		t.Fatal("payer validation invalid")
	}
	configuration, e := r.Current(ctx, money.ID{7})
	if e != nil || configuration.ID != money.ID(old.ID) {
		t.Fatal("active version unavailable", e)
	}
	out, e := r.Test(ctx, old.Configuration, domain.ID{8})
	if e != nil || !out.Passed || out.ResultCode != "PESAPAL_AUTHENTICATION_VERIFIED" || network.calls != 1 {
		t.Fatal("authentication test invalid", e)
	}
	disabled := pspVersion(t, 2, "SANDBOX", false)
	versions.active = disabled
	versions.historical[disabled.ID] = disabled
	if _, e = r.Current(ctx, money.ID{7}); e == nil {
		t.Fatal("disabled new-order configuration usable")
	}
	if historical, e := r.Version(ctx, money.ID(old.ID)); e != nil || historical.ID != configuration.ID {
		t.Fatal("pause broke historical reconciliation", e)
	}
	out, e = r.Test(ctx, disabled.Configuration, domain.ID{8})
	if e != nil || !out.Passed || network.calls != 1 {
		t.Fatal("disabled test contacted provider")
	}
	live := pspVersion(t, 3, "LIVE", true)
	out, e = r.Test(ctx, live.Configuration, domain.ID{8})
	if e != nil || out.Passed || network.calls != 1 {
		t.Fatal("live gate bypassed")
	}
	policies.p.MaintenanceEnabled = true
	policies.p.MaintenanceMessage = "Maintenance"
	versions.active = old
	if _, e = r.Current(ctx, money.ID{7}); e == nil {
		t.Fatal("maintenance allowed new payment")
	}
}
func TestPesaPalPaymentEvidenceNeverTrustsRedirectsCallbacksOrMismatchedAmounts(t *testing.T) {
	ctx := context.Background()
	v := pspVersion(t, 1, "SANDBOX", true)
	versions := &pspVersions{v, map[domain.ID]domain.Version{v.ID: v}}
	network := &pspHTTP{}
	r, e := pesapalruntime.New(versions, pspSecrets{}, &pspPolicies{validPolicy()}, pesapalruntime.Options{CallbackURL: "https://bidora.example/payments/pesapal/callback"}, network)
	if e != nil {
		t.Fatal(e)
	}
	configuration, e := r.Current(ctx, money.ID{7})
	if e != nil {
		t.Fatal(e)
	}
	merchant := money.ID{2}
	tracking := uuid.New().String()
	network.responses = []string{fmt.Sprintf(`{"status":"200","order_tracking_id":%q,"merchant_reference":%q,"redirect_url":"https://evil.example/steal"}`, tracking, uuid.UUID(merchant).String())}
	created, e := r.CreateOrder(ctx, configuration, commercial.PaymentCreateRequest{ConfigurationVersionID: money.ID(v.ID), TenantID: money.ID{7}, InvoiceID: money.ID{9}, MerchantOperationID: merchant, Currency: "KES", AmountMinorUnits: 12345, PayerEmail: "payer@example.test", PayerCountry: "KE"})
	if e == nil || created.Disposition != money.PaymentCreateAmbiguous || created.RedirectURL != "" {
		t.Fatal("unapproved redirect trusted")
	}
	if !strings.Contains(network.payloads[1], `"amount":123.45`) || strings.Contains(network.payloads[1], "consumer_secret") {
		t.Fatal("order money or credential boundary invalid")
	}
	network.responses = []string{fmt.Sprintf(`{"status":"200","merchant_reference":%q,"amount":123.44,"currency":"KES","status_code":1,"payment_status_description":"Completed"}`, uuid.UUID(merchant).String())}
	query := commercial.PaymentQueryRequest{ConfigurationVersionID: money.ID(v.ID), MerchantOperationID: merchant, ProviderOrderReference: &tracking, Currency: "KES", AmountMinorUnits: 12345}
	observed, e := r.QueryOrder(ctx, configuration, query)
	if e != nil || observed.Disposition != money.PaymentIntegrityConflict || observed.NormalizedState != "UNKNOWN" {
		t.Fatal("mismatched amount settled")
	}
	network.responses = []string{fmt.Sprintf(`{"status":"200","merchant_reference":%q,"amount":123.45,"currency":"KES","status_code":1,"payment_status_description":"Completed"}`, uuid.UUID(merchant).String())}
	observed, e = r.QueryOrder(ctx, configuration, query)
	if e != nil || observed.Disposition != money.PaymentAccountingRequired || observed.NormalizedState != "SETTLED" {
		t.Fatal("verified provider completion did not require existing accounting authority")
	}
	query.ProviderOrderReference = nil
	before := network.calls
	if _, e = r.QueryOrder(ctx, configuration, query); e == nil || network.calls != before {
		t.Fatal("unknown create was guessed or resent")
	}
	network.responses = []string{`{"status":"200","status":"200"}`}
	query.ProviderOrderReference = &tracking
	if _, e = r.QueryOrder(ctx, configuration, query); e == nil {
		t.Fatal("duplicate financial keys accepted")
	}
	raw, _ := json.Marshal(observed)
	if strings.Contains(string(raw), "bearer") {
		t.Fatal("token returned as evidence")
	}
}
func TestPesaPalDecimalMoneyDoesNotUseFloatingPoint(t *testing.T) {
	for raw, want := range map[string]int64{"0": 0, "0.01": 1, "123.45": 12345, "10.5": 1050, "92233720368547758.07": 9223372036854775807} {
		got, ok := pesapalruntime.AmountMinor(raw)
		if !ok || got != want {
			t.Fatal("decimal mismatch", raw, got)
		}
	}
	for _, raw := range []string{"-1", "1e2", "1.001", "NaN", "92233720368547758.08", "01.00"} {
		if _, ok := pesapalruntime.AmountMinor(raw); ok {
			t.Fatal("unsafe decimal accepted", raw)
		}
	}
}
