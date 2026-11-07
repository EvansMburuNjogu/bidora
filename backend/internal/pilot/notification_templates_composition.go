package pilot

import (
	"context"
	"time"

	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/delivery"
	"bidora/backend/internal/notification/domain"
	persistence "bidora/backend/internal/notification/persistence"
	settingsapp "bidora/backend/internal/platformsettings/application"
	settingsdomain "bidora/backend/internal/platformsettings/domain"
	settingspersistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/smtpdelivery"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type configuredTemplateRenderer struct {
	templates   *app.TemplateService
	information *settingsapp.InformationService
	origin      string
}

func (r configuredTemplateRenderer) Render(ctx context.Context, key string, values map[string]string) (app.RenderedTemplate, error) {
	info, e := r.information.Shared(ctx)
	if e != nil {
		return app.RenderedTemplate{}, e
	}
	copyValues := make(map[string]string, len(values)+1)
	for k, v := range values {
		copyValues[k] = v
	}
	copyValues["platform_name"] = info.PlatformName
	copyValues["web_origin"] = r.origin
	if info.SupportEmail != nil {
		copyValues["support_email"] = *info.SupportEmail
	}
	rendered, e := r.templates.Render(ctx, key, copyValues)
	if e != nil {
		return rendered, e
	}
	if info.SupportEmail != nil && *info.SupportEmail != "" {
		rendered.Text += "\nSupport: " + *info.SupportEmail + "\n"
	}
	return rendered, nil
}

type configuredSMTPDelivery struct {
	base     smtpDelivery
	renderer app.TemplateRenderer
}

func (d *configuredSMTPDelivery) send(ctx context.Context, recipient email.Address, key string, values map[string]string) error {
	if recipient.IsZero() {
		return app.ErrTemplateInvalid
	}
	v, e := d.renderer.Render(ctx, key, values)
	if e != nil {
		return e
	}
	return d.base.DeliverRendered(ctx, recipient, v.Subject, v.Text, v.HTML)
}
func (d *configuredSMTPDelivery) DeliverEmailVerification(ctx context.Context, to email.Address, token verification.Token, expiry time.Time) error {
	if token.IsZero() || expiry.IsZero() {
		return app.ErrTemplateInvalid
	}
	return d.send(ctx, to, "identity.email_verification", map[string]string{"token": token.RevealForDelivery(), "expires_at": expiry.UTC().Format(time.RFC3339)})
}
func (d *configuredSMTPDelivery) DeliverPasswordRecovery(ctx context.Context, to email.Address, token verification.Token, expiry time.Time) error {
	if token.IsZero() || expiry.IsZero() {
		return app.ErrTemplateInvalid
	}
	return d.send(ctx, to, "identity.password_recovery", map[string]string{"token": token.RevealForDelivery(), "expires_at": expiry.UTC().Format(time.RFC3339)})
}
func (d *configuredSMTPDelivery) EmailLoginAvailable(ctx context.Context) bool {
	availability, ok := d.base.(interface{ EmailLoginAvailable(context.Context) bool })
	return ok && availability.EmailLoginAvailable(ctx)
}
func (d *configuredSMTPDelivery) DeliverLoginCode(ctx context.Context, to email.Address, code string, expiry time.Time) error {
	if len(code) != 6 || expiry.IsZero() {
		return app.ErrTemplateInvalid
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return app.ErrTemplateInvalid
		}
	}
	return d.send(ctx, to, "identity.login_code", map[string]string{"code": code, "expires_at": expiry.UTC().Format(time.RFC3339)})
}
func (d *configuredSMTPDelivery) DeliverTenantRoleInvitation(ctx context.Context, to email.Address, role, token string, expiry time.Time) error {
	if token == "" || expiry.IsZero() {
		return app.ErrTemplateInvalid
	}
	return d.send(ctx, to, "tenant.invitation", map[string]string{"token": token, "role": role, "expires_at": expiry.UTC().Format(time.RFC3339)})
}
func (d *configuredSMTPDelivery) DeliverRendered(ctx context.Context, to email.Address, subject, text, html string) error {
	return d.base.DeliverRendered(ctx, to, subject, text, html)
}

type templateSelfTestSender struct {
	recipients *settingspersistence.SMTPTestRecipientDirectory
	active     *smtpdelivery.Active
	recorder   *delivery.Recorder
}

func (s templateSelfTestSender) SendTemplateTest(ctx context.Context, subject app.TemplateSubject, t app.Template, r app.RenderedTemplate, key string) (string, error) {
	recipient, e := s.recipients.VerifiedEmail(ctx, settingsdomain.ID(subject.ActorID))
	if e != nil {
		return "UNAVAILABLE", e
	}
	effect := uuid.NewSHA1(uuid.NameSpaceOID, []byte(uuid.UUID(subject.ActorID).String()+":"+key))
	var sendErr error
	sendErr = s.active.WithActive(ctx, func(version settingsdomain.ID, smtp *deliverysmtp.Delivery) error {
		prepared, e := smtp.PrepareRendered(recipient, r.Subject, r.Text, r.HTML)
		if e != nil {
			return e
		}
		return s.recorder.Send(ctx, delivery.Request{ActorID: subject.ActorID, RecipientUserID: subject.ActorID, SourceID: domain.ID(effect), CorrelationID: subject.CorrelationID, SMTPVersionID: domain.ID(version), Category: "PLATFORM_TEMPLATE_TEST", TemplateKey: t.Key, TemplateVersion: t.Version, SourceType: "PLATFORM_TEMPLATE_TEST", IdempotencyKey: effect.String(), AttemptNumber: 1, Subject: prepared.RecordSubject(), Text: prepared.RecordText(), RecipientCanonical: recipient.Lookup(), RenderedContent: r.ContentForFingerprint()}, func(c context.Context) error { return smtp.DeliverPrepared(c, prepared) })
	})
	if sendErr != nil {
		return "UNAVAILABLE", sendErr
	}
	return "SMTP_ACCEPTED", nil
}
func buildNotificationTemplateService(pool *postgres.Pool, config Config) (*app.TemplateService, app.TemplateRenderer, error) {
	store, e := persistence.NewTemplateStore(pool)
	if e != nil {
		return nil, nil, e
	}
	var tester app.TemplateTestSender
	if len(config.NotificationFingerprintKey) == 32 && config.NotificationFingerprintKeyVersion > 0 && len(config.SMTPVaultKey) == 32 {
		vault, e := smtpsecret.NewVault(pool, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if e != nil {
			return nil, nil, e
		}
		activeStore, e := settingspersistence.NewActiveSMTPStore(pool)
		if e != nil {
			return nil, nil, e
		}
		active, e := smtpdelivery.New(activeStore, vault)
		if e != nil {
			return nil, nil, e
		}
		evidenceStore, e := persistence.NewStore(pool)
		if e != nil {
			return nil, nil, e
		}
		evidence, e := app.NewService(evidenceStore, notificationIDs{}, systemClock{})
		if e != nil {
			return nil, nil, e
		}
		recorder, e := delivery.NewRecorder(evidence, delivery.Config{FingerprintKey: config.NotificationFingerprintKey, FingerprintKeyVersion: config.NotificationFingerprintKeyVersion})
		if e != nil {
			return nil, nil, e
		}
		recipients, e := settingspersistence.NewSMTPTestRecipientDirectory(pool)
		if e != nil {
			return nil, nil, e
		}
		tester = templateSelfTestSender{recipients, active, recorder}
	}
	service, e := app.NewTemplateService(store, notificationIDs{}, systemClock{}, tester)
	if e != nil {
		return nil, nil, e
	}
	infoStore, e := settingspersistence.NewInformationStore(pool)
	if e != nil {
		return nil, nil, e
	}
	information, e := settingsapp.NewInformationService(infoStore, platformSettingIDs{}, systemClock{})
	if e != nil {
		return nil, nil, e
	}
	origin := ""
	if len(config.AllowedOrigins) == 1 {
		origin = config.AllowedOrigins[0]
	}
	return service, configuredTemplateRenderer{service, information, origin}, nil
}
