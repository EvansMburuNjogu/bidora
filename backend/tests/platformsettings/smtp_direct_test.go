package platformsettings_test

import (
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"context"
	"testing"
	"time"
)

type smtpDirectFixture struct {
	app.Tx
	authority app.Authority
	published int
	active    domain.Version
	inserted  domain.Version
	command   app.Command
	evidence  app.Evidence
}

func (f *smtpDirectFixture) WithinSettings(ctx context.Context, fn func(context.Context, app.Tx) error) error {
	return fn(ctx, f)
}
func (f *smtpDirectFixture) Authorize(context.Context, app.Subject, time.Time) (app.Authority, error) {
	return f.authority, nil
}
func (f *smtpDirectFixture) GetCommand(context.Context, domain.ID, string, [32]byte) (app.Command, bool, error) {
	return app.Command{}, false, nil
}
func (f *smtpDirectFixture) LockHead(context.Context, domain.Kind) (app.Head, error) {
	return app.Head{Kind: domain.SMTP, NextSequence: 1, ActiveVersionID: f.active.ID}, nil
}
func (f *smtpDirectFixture) InsertVersion(_ context.Context, v domain.Version, _ string) error {
	f.inserted = v
	return nil
}
func (f *smtpDirectFixture) LockVersion(context.Context, domain.ID) (domain.Version, bool, error) {
	return f.active, !f.active.ID.IsZero(), nil
}
func (f *smtpDirectFixture) PublishSMTP(_ context.Context, v domain.Version, h app.Head, actor, history domain.ID, _ time.Time) error {
	if v.Kind != domain.SMTP || actor.IsZero() || history.IsZero() {
		panic("invalid publication")
	}
	f.published++
	return nil
}
func (f *smtpDirectFixture) CompleteCommand(_ context.Context, c app.Command, _ string, _ domain.ID, _ time.Time) error {
	f.command = c
	return nil
}
func (f *smtpDirectFixture) AppendEvidence(_ context.Context, e app.Evidence) error {
	f.evidence = e
	return nil
}

type directSMTPIDs byte

func (i *directSMTPIDs) NewID() (domain.ID, error) { *i++; return domain.ID{byte(*i)}, nil }

type directSMTPClock struct{ at time.Time }

func (c directSMTPClock) Now() time.Time { return c.at }

type unusedSMTPTester struct{}

func (unusedSMTPTester) Test(context.Context, domain.Configuration, domain.ID) (app.TestOutcome, error) {
	panic("saving must not send test email")
}
func TestSMTPSaveAppliesImmediatelyWithoutApprovalOrTestEmail(t *testing.T) {
	at := time.Now().UTC()
	f := &smtpDirectFixture{authority: app.Authority{EffectiveOwner: true, Capabilities: map[string]bool{"platform.integration.smtp.manage": true, "platform.integration.smtp.activate": true}, AssuranceLevel: "AAL2", AssuranceMethod: "EMAIL_OTP", AssuredAt: at}}
	ids := directSMTPIDs(0)
	service, _ := app.NewService(f, unusedSMTPTester{}, &ids, directSMTPClock{at})
	config, _, e := domain.NewConfiguration(domain.SMTP, &domain.SMTPConfiguration{Host: "smtp.example.test", Port: 587, CredentialSecretRef: "encrypted-credentials", TrustRootsSecretRef: "trusted-roots", SenderName: "Bidora", SenderAddress: "hello@example.test", ConnectTimeoutSeconds: 10, CommandTimeoutSeconds: 15, DeliveryTimeoutSeconds: 30, MaxConcurrency: 4}, nil)
	if e != nil {
		t.Fatal(e)
	}
	in := app.CreateInput{Subject: app.Subject{ActorID: domain.ID{10}, SessionID: domain.ID{11}, ContextGeneration: 1}, Configuration: config, Reason: "Set system default SMTP", IdempotencyKey: "smtp-save", CorrelationID: domain.ID{12}}
	result, e := service.Create(context.Background(), in)
	if e != nil || result.State != domain.Active || result.Version != 2 || f.published != 1 || f.command.ResponseState != "ACTIVE" || f.evidence.State != "ACTIVE" {
		t.Fatal("SMTP was not atomically activated", e, result.State)
	}
	f.authority.EffectiveOwner = false
	if _, e = service.Create(context.Background(), in); e != app.ErrForbidden || f.published != 1 {
		t.Fatal("non-Owner could activate SMTP", e)
	}
}
