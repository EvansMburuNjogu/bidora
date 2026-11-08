package notification_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"
	"time"

	authz "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/authorization/invitation"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/delivery"
	"bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/tenantemail"
	settings "bidora/backend/internal/platformsettings/domain"
)

type templateSMTP struct{ adapter *deliverysmtp.Delivery }

func (a templateSMTP) WithActive(ctx context.Context, use func(settings.ID, *deliverysmtp.Delivery) error) error {
	return use(settings.ID{1}, a.adapter)
}

type tenantTemplateRenderer struct {
	calls  int
	values map[string]string
	err    error
}

func (r *tenantTemplateRenderer) Render(_ context.Context, key string, values map[string]string) (app.RenderedTemplate, error) {
	r.calls++
	r.values = values
	if key != "tenant.invitation" {
		panic("wrong template")
	}
	return app.RenderedTemplate{Subject: "Managed tenant invitation", Text: "Managed body", HTML: "<p>Managed body</p>", Version: 7, Category: "TENANT_INVITATION", Key: key}, r.err
}

type tenantTemplateEvidence struct {
	prepared app.PrepareInput
	calls    int
}

func (e *tenantTemplateEvidence) Prepare(_ context.Context, in app.PrepareInput) (domain.Notification, error) {
	e.prepared = in
	e.calls++
	return domain.Notification{ID: domain.ID{1}}, nil
}
func (*tenantTemplateEvidence) StartAttemptForDelivery(context.Context, app.StartAttemptInput) (domain.Attempt, bool, error) {
	return domain.Attempt{}, false, nil
}
func (*tenantTemplateEvidence) FinishAttempt(context.Context, app.FinishAttemptInput) (domain.Result, error) {
	panic("must not contact SMTP in template wiring test")
}
func TestTenantInvitationsConsumeManagedTemplateRevisionAndFailClosed(t *testing.T) {
	pool := x509.NewCertPool()
	pool.AddCert(&x509.Certificate{Raw: []byte{1}, RawSubject: []byte{1}})
	sender, _ := email.Parse("sender@example.test")
	smtp, err := deliverysmtp.New(deliverysmtp.Config{Host: "localhost", Port: 1, Username: "synthetic", Password: "synthetic", Sender: sender, TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: pool}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	evidence := &tenantTemplateEvidence{}
	recorder, _ := delivery.NewRecorder(evidence, delivery.Config{FingerprintKey: make([]byte, 32), FingerprintKeyVersion: 1})
	adapter, _ := tenantemail.New(templateSMTP{smtp}, recorder)
	renderer := &tenantTemplateRenderer{}
	if err = adapter.EnableTemplates(renderer); err != nil {
		t.Fatal(err)
	}
	token, _ := invitation.Generate()
	request := authz.TenantInvitationDeliveryRequest{InvitationID: authz.ID{1}, ActorID: authz.ID{2}, TenantID: authz.ID{3}, CorrelationID: authz.ID{4}, Recipient: "recipient@example.test", Role: authz.TenderpreneurRole("CONTRIBUTOR"), Token: token, ExpiresAt: time.Now().UTC().Add(time.Hour)}
	err = adapter.DeliverTenantRoleInvitationWithEvidence(context.Background(), request)
	if !errors.Is(err, app.ErrConflict) || renderer.calls != 1 || evidence.calls != 1 || evidence.prepared.TemplateVersion != 7 || renderer.values["token"] != token.RevealForDelivery() || renderer.values["role"] != "CONTRIBUTOR" {
		t.Fatal("tenant template settings bypassed", err)
	}
	renderer.err = app.ErrTemplateUnavailable
	if err = adapter.DeliverTenantRoleInvitationWithEvidence(context.Background(), request); !errors.Is(err, app.ErrTemplateUnavailable) || evidence.calls != 1 {
		t.Fatal("template resolver error fell back to hardcoded email", err)
	}
}
