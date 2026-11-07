package platformsettings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
)

func TestSMTPSaveRetainsExactCredentialVersionWithoutExposingOrReplacingSecrets(t *testing.T) {
	at := time.Now().UTC()
	f := &smtpDirectFixture{authority: app.Authority{EffectiveOwner: true, Capabilities: map[string]bool{"platform.integration.smtp.manage": true, "platform.integration.smtp.activate": true}, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: at}}
	cfg, _, err := domain.NewConfiguration(domain.SMTP, &domain.SMTPConfiguration{Host: "smtp.example.test", Port: 587, CredentialSecretRef: "old-sealed-credential", TrustRootsSecretRef: "old-sealed-roots", SenderName: "Old sender", SenderAddress: "sender@example.test", ConnectTimeoutSeconds: 10, CommandTimeoutSeconds: 15, DeliveryTimeoutSeconds: 30, MaxConcurrency: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.active = domain.Version{ID: domain.ID{21}, Version: 4, State: domain.Active, Configuration: cfg}
	replacement := *cfg.SMTP
	replacement.SenderName = "Updated sender"
	replacement.CredentialSecretRef = "retained"
	replacement.TrustRootsSecretRef = "retained"
	next, _, _ := domain.NewConfiguration(domain.SMTP, &replacement, nil)
	ids := directSMTPIDs(0)
	service, _ := app.NewService(f, unusedSMTPTester{}, &ids, directSMTPClock{at})
	in := app.CreateInput{Subject: app.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, Configuration: next, ReuseSMTPSecrets: true, ExpectedSMTPVersionID: f.active.ID, ExpectedSMTPVersion: 4, Reason: "Update sender only", IdempotencyKey: "retain-smtp", CorrelationID: domain.ID{3}}
	v, err := service.Create(context.Background(), in)
	if err != nil || v.Configuration.SMTP.CredentialSecretRef != "old-sealed-credential" || v.Configuration.SMTP.TrustRootsSecretRef != "old-sealed-roots" || v.State != domain.Active {
		t.Fatal("credential retention failed", err)
	}
	if in.Configuration.SMTP.CredentialSecretRef != "retained" {
		t.Fatal("mutated caller configuration")
	}
	f.active.Version = 5
	if _, err = service.Create(context.Background(), in); !errors.Is(err, app.ErrConflict) {
		t.Fatal("stale version reused", err)
	}
	f.active.Version = 4
	in.ExpectedSMTPVersionID = domain.ID{22}
	if _, err = service.Create(context.Background(), in); !errors.Is(err, app.ErrConflict) {
		t.Fatal("wrong active version reused", err)
	}
	in.ExpectedSMTPVersionID = f.active.ID
	in.Configuration.SMTP.Host = "other.example.test"
	if _, err = service.Create(context.Background(), in); !errors.Is(err, app.ErrInvalid) {
		t.Fatal("stored secret redirected to another host", err)
	}
}

type smtpUsernameFixture struct {
	app.ReadTx
	authority     app.Authority
	usernameReads int
	version       domain.ID
}

func (f *smtpUsernameFixture) WithinSettingsRead(ctx context.Context, fn func(context.Context, app.ReadTx) error) error {
	return fn(ctx, f)
}
func (f *smtpUsernameFixture) Authorize(context.Context, app.Subject, time.Time) (app.Authority, error) {
	return f.authority, nil
}
func (f *smtpUsernameFixture) GetActiveVersion(context.Context, domain.Kind) (app.RedactedVersion, bool, error) {
	return app.RedactedVersion{ID: f.version, Kind: domain.SMTP, SMTP: &app.RedactedSMTP{Host: "smtp.example.test"}}, true, nil
}
func (f *smtpUsernameFixture) SMTPUsername(_ context.Context, id domain.ID) (string, error) {
	if id != f.version {
		panic("mismatched version")
	}
	f.usernameReads++
	return "saved-login", nil
}
func TestSMTPUsernameIsResolvedOnlyAfterCurrentPlatformAuthorization(t *testing.T) {
	at := time.Now().UTC()
	f := &smtpUsernameFixture{version: domain.ID{5}, authority: app.Authority{Capabilities: map[string]bool{"platform.integration.read": true}, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: at}}
	service, _ := app.NewReadService(f, directSMTPClock{at})
	service.EnableSMTPUsernames(f)
	subject := app.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}
	v, err := service.Active(context.Background(), subject, domain.SMTP)
	if err != nil || v.SMTP.Username == nil || *v.SMTP.Username != "saved-login" || f.usernameReads != 1 {
		t.Fatal("username not populated", err)
	}
	f.authority.Capabilities["platform.integration.read"] = false
	if _, err = service.Active(context.Background(), subject, domain.SMTP); !errors.Is(err, app.ErrForbidden) || f.usernameReads != 1 {
		t.Fatal("unauthorized secret lookup", err)
	}
	f.authority.Capabilities["platform.integration.read"] = true
	f.authority.AssuredAt = at.Add(-16 * time.Minute)
	if _, err = service.Active(context.Background(), subject, domain.SMTP); !errors.Is(err, app.ErrForbidden) || f.usernameReads != 1 {
		t.Fatal("stale assurance read username", err)
	}
}
