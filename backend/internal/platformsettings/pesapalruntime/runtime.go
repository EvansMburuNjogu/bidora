// Package pesapalruntime adapts managed immutable configurations to the existing
// commercial payment/reconciliation ports. It creates no financial authority.
package pesapalruntime

import (
	commercial "bidora/backend/internal/commercial/application"
	money "bidora/backend/internal/commercial/domain"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

type Versions interface {
	ActivePesaPal(context.Context) (domain.Version, error)
	PesaPalVersion(context.Context, domain.ID) (domain.Version, error)
}
type Policies interface {
	CurrentPolicy(context.Context) (app.OperationalPolicy, error)
}
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}
type Options struct {
	CallbackURL string
	AllowLive   bool
}

func (o Options) Valid() bool {
	if o.CallbackURL == "" {
		return true
	}
	u, e := url.Parse(o.CallbackURL)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "/payments/pesapal/callback" && u.RawQuery == "" && u.Fragment == ""
}

type Runtime struct {
	versions Versions
	secrets  smtpsecret.Resolver
	policies Policies
	options  Options
	client   HTTPDoer
}

func New(v Versions, s smtpsecret.Resolver, p Policies, o Options, client HTTPDoer) (*Runtime, error) {
	if v == nil || p == nil || !o.Valid() {
		return nil, errors.New("pesapal_runtime_invalid")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 15 * time.Second, MaxIdleConnsPerHost: 2, MaxConnsPerHost: 10, IdleConnTimeout: 30 * time.Second}}
	}
	return &Runtime{v, s, p, o, client}, nil
}
func (r *Runtime) allowed(p *domain.PesaPalConfiguration) bool {
	return r != nil && p != nil && r.secrets != nil && r.options.CallbackURL != "" && (p.Environment != "LIVE" || r.options.AllowLive)
}
func (r *Runtime) Current(ctx context.Context, tenant money.ID) (commercial.PaymentProviderConfiguration, error) {
	if tenant.IsZero() {
		return commercial.PaymentProviderConfiguration{}, commercial.ErrPaymentUnavailable
	}
	p, e := r.policies.CurrentPolicy(ctx)
	if e != nil || p.UnderMaintenance(time.Now().UTC()) {
		return commercial.PaymentProviderConfiguration{}, commercial.ErrPaymentUnavailable
	}
	v, e := r.versions.ActivePesaPal(ctx)
	if e != nil || v.Configuration.PesaPal == nil || !v.Configuration.PesaPal.Enabled || !r.allowed(v.Configuration.PesaPal) {
		return commercial.PaymentProviderConfiguration{}, commercial.ErrPaymentUnavailable
	}
	return configuration(v), nil
}

// Pending reconciliation keeps using the exact original immutable revision,
// including after current configuration is disabled or replaced.
func (r *Runtime) Version(ctx context.Context, id money.ID) (commercial.PaymentProviderConfiguration, error) {
	v, e := r.versions.PesaPalVersion(ctx, domain.ID(id))
	if e != nil || v.Configuration.PesaPal == nil || !r.allowed(v.Configuration.PesaPal) {
		return commercial.PaymentProviderConfiguration{}, commercial.ErrPaymentUnavailable
	}
	return configuration(v), nil
}
func configuration(v domain.Version) commercial.PaymentProviderConfiguration {
	origin := "https://cybqa.pesapal.com"
	if v.Configuration.PesaPal.Environment == "LIVE" {
		origin = "https://pay.pesapal.com"
	}
	return commercial.PaymentProviderConfiguration{ID: money.ID(v.ID), Provider: "PESAPAL", AllowedRedirectOrigin: origin, CreateUncertaintyDelay: 2 * time.Minute, CreateTimeout: 45 * time.Second, QueryTimeout: 45 * time.Second, MaxQueryAttempts: 3, QueryRetryDelays: []time.Duration{time.Minute, 5 * time.Minute}}
}
func (r *Runtime) ValidatePayer(email, country string) bool {
	a, e := mail.ParseAddress(email)
	if e != nil || a.Address != email || a.Name != "" || len(email) > 254 || strings.ContainsAny(email, "\r\n\x00") {
		return false
	}
	for _, ref := range domain.CountryReferences() {
		if ref.Code == country {
			return true
		}
	}
	return false
}
func (r *Runtime) Test(ctx context.Context, c domain.Configuration, _ domain.ID) (app.TestOutcome, error) {
	if !c.Valid() || c.Kind != domain.PesaPal {
		return app.TestOutcome{ResultCode: "INVALID_RESPONSE"}, nil
	}
	if !c.PesaPal.Enabled {
		return app.TestOutcome{Passed: true, ResultCode: "PESAPAL_DISABLED_CONFIG_VERIFIED"}, nil
	}
	if !r.allowed(c.PesaPal) {
		return app.TestOutcome{ResultCode: "PROVIDER_REJECTED"}, nil
	}
	_, e := r.authenticate(ctx, *c.PesaPal)
	if e != nil {
		return app.TestOutcome{ResultCode: "AUTHENTICATION_FAILED"}, nil
	}
	return app.TestOutcome{Passed: true, ResultCode: "PESAPAL_AUTHENTICATION_VERIFIED"}, nil
}
