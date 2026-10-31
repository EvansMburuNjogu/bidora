package platformsettings_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
)

type googleFixture struct {
	stored                                app.GoogleStored
	authority                             app.Authority
	receipts                              map[string]app.GoogleReceipt
	fallback, failEvidence, revokeAtFence bool
	writes, evidence, tests               int
}

func (f *googleFixture) WithinGoogle(ctx context.Context, _ bool, fn func(context.Context, app.GoogleTx) error) error {
	old := f.stored
	w, e, tests := f.writes, f.evidence, f.tests
	receipts := map[string]app.GoogleReceipt{}
	for k, v := range f.receipts {
		receipts[k] = v
	}
	if err := fn(ctx, f); err != nil {
		f.stored = old
		f.writes, f.evidence, f.tests = w, e, tests
		f.receipts = receipts
		return err
	}
	return nil
}
func (f *googleFixture) AuthorizeInformation(context.Context, app.Subject, time.Time) (app.Authority, error) {
	return f.authority, nil
}
func (f *googleFixture) LockGoogle(context.Context) error {
	if f.revokeAtFence {
		f.authority.EffectiveOwner = false
	}
	return nil
}
func (f *googleFixture) PasswordFallbackAvailable(context.Context) (bool, error) {
	return f.fallback, nil
}
func (f *googleFixture) ReadGoogle(context.Context) (app.GoogleStored, error) { return f.stored, nil }
func (f *googleFixture) GoogleHistory(context.Context) ([]app.GoogleStored, error) {
	return []app.GoogleStored{f.stored}, nil
}
func (f *googleFixture) FindGoogleCommand(_ context.Context, actor domain.ID, op string, key [32]byte) (app.GoogleReceipt, bool, error) {
	v, ok := f.receipts[fmt.Sprintf("%s:%s:%x", actor, op, key)]
	return v, ok, nil
}
func (f *googleFixture) WriteGoogle(_ context.Context, v app.GoogleStored, _ domain.ID, _ time.Time) error {
	f.stored = v
	f.writes++
	return nil
}
func (f *googleFixture) RecordGoogleCommand(_ context.Context, actor domain.ID, op string, key [32]byte, r app.GoogleReceipt, _ time.Time) error {
	f.receipts[fmt.Sprintf("%s:%s:%x", actor, op, key)] = r
	if op == "TEST" {
		f.tests++
	}
	return nil
}
func (f *googleFixture) FinishGoogleTest(_ context.Context, actor domain.ID, key [32]byte, result string) error {
	k := fmt.Sprintf("%s:TEST:%x", actor, key)
	r := f.receipts[k]
	r.State = "COMPLETED"
	r.Result = result
	f.receipts[k] = r
	f.stored.Configuration.ConnectionStatus = result
	return nil
}
func (f *googleFixture) GoogleTestCount(context.Context, domain.ID, time.Time) (int64, error) {
	return int64(f.tests), nil
}
func (f *googleFixture) AppendGoogleEvidence(_ context.Context, _, _, _, _ domain.ID, _ string, _ app.GoogleStored, _ string, _ time.Time) error {
	if f.failEvidence {
		return errors.New("synthetic evidence failure")
	}
	f.evidence++
	return nil
}

type googleProbe struct {
	calls int
	err   error
}

func (p *googleProbe) TestGoogle(context.Context) error { p.calls++; return p.err }
func googleUnit(t *testing.T) (*app.GoogleService, *googleFixture, *googleProbe, app.GoogleSave) {
	t.Helper()
	at := time.Now().UTC()
	f := &googleFixture{stored: app.GoogleStored{Configuration: app.GoogleConfiguration{ID: domain.ID{39}, Version: 1, CreatedAt: at}}, receipts: map[string]app.GoogleReceipt{}, fallback: true, authority: app.Authority{EffectiveOwner: true, Capabilities: map[string]bool{"platform.integration.read": true}, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: at}}
	vault, err := smtpsecret.NewVault(&postgres.Pool{}, make([]byte, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	probe := &googleProbe{}
	service, err := app.NewGoogleService(f, vault, probe, &infoIDs{next: 40}, infoClock{at}, true)
	if err != nil {
		t.Fatal(err)
	}
	secret := "synthetic-google-secret"
	return service, f, probe, app.GoogleSave{Subject: app.Subject{ActorID: domain.ID{1}, SessionID: domain.ID{2}, ContextGeneration: 1}, CorrelationID: domain.ID{3}, Configuration: app.GoogleConfiguration{Version: 1, Enabled: true, AllowSignup: true, ClientID: "123-managed.apps.googleusercontent.com", AllowedDomains: []string{"example.test"}}, ClientSecret: &secret, IdempotencyKey: "google-save-1"}
}
func TestGoogleConfigurationDomainAndSignupValidation(t *testing.T) {
	valid := app.GoogleConfiguration{ClientID: "123-managed.apps.googleusercontent.com", Enabled: true, AllowSignup: true, AllowedDomains: []string{"example.test", "xn--example-9db.test"}}
	if !app.ValidGoogleConfiguration(valid) {
		t.Fatal("valid managed policy rejected")
	}
	for _, domains := range [][]string{{"*.example.test"}, {"EXAMPLE.test"}, {"https://example.test"}, {"example.test", "example.test"}, {"example.test.evil/"}, {strings.Repeat("a", 64) + ".test"}, {"localhost"}} {
		bad := valid
		bad.AllowedDomains = domains
		if app.ValidGoogleConfiguration(bad) {
			t.Fatal("unsafe domains accepted", domains)
		}
	}
	valid.Enabled = false
	if app.ValidGoogleConfiguration(valid) {
		t.Fatal("signup without login accepted")
	}
}
func TestGoogleCurrentPolicyMatchesExactCanonicalEmailDomainAndRevision(t *testing.T) {
	c := app.GoogleConfiguration{Version: 2, Enabled: true, ClientID: "123-managed.apps.googleusercontent.com", AllowedDomains: []string{"example.test"}}
	if !c.AllowsVerifiedEmail(2, "person@example.test") {
		t.Fatal("allowed verified domain denied")
	}
	for _, email := range []string{"person@sub.example.test", "person@example.test.evil", "person@other.test", "person@EXAMPLE.TEST", "@example.test", "person@example.test@evil.test"} {
		if c.AllowsVerifiedEmail(2, email) {
			t.Fatal("email restriction bypassed", email)
		}
	}
	if c.AllowsVerifiedEmail(1, "person@example.test") {
		t.Fatal("in-flight old revision accepted")
	}
	c.Enabled = false
	if c.AllowsVerifiedEmail(2, "person@example.test") {
		t.Fatal("disabled provider accepted")
	}
}

func TestGoogleOwnerSaveEncryptedReplayAndAtomicEvidence(t *testing.T) {
	s, f, _, in := googleUnit(t)
	out, err := s.Save(context.Background(), in)
	if err != nil || out.Version != 2 || !out.SecretConfigured || !out.RuntimeConfigured || f.writes != 1 || f.evidence != 1 {
		t.Fatal("configuration did not persist", err, out)
	}
	if strings.Contains(string(f.stored.Secret.Ciphertext), *in.ClientSecret) {
		t.Fatal("secret persisted in plaintext")
	}
	plain, err := s.Secret(f.stored)
	if err != nil || string(plain) != *in.ClientSecret {
		t.Fatal("active credential unavailable", err)
	}
	if _, err = s.Save(context.Background(), in); err != nil || f.writes != 1 {
		t.Fatal("replay repeated settings write", err)
	}
	changed := in
	changed.Configuration.AllowSignup = false
	if _, err = s.Save(context.Background(), changed); !errors.Is(err, app.ErrInformationConflict) {
		t.Fatal("changed replay accepted", err)
	}
	in.Configuration.Version = 2
	in.IdempotencyKey = "google-save-2"
	in.ClientSecret = nil
	f.failEvidence = true
	if _, err = s.Save(context.Background(), in); !errors.Is(err, app.ErrInformationUnavailable) || f.stored.Configuration.Version != 2 || f.writes != 1 || f.evidence != 1 {
		t.Fatal("unit rollback left configuration effects", err)
	}
	for _, v := range []any{in, f.stored, f.stored.Secret} {
		if strings.Contains(fmt.Sprintf("%#v", v), "synthetic-google-secret") {
			t.Fatal("sensitive formatting exposed secret")
		}
	}
}
func TestGoogleRecoveryOwnerAndRecentAuthorityRequired(t *testing.T) {
	for _, scenario := range []string{"no_fallback", "non_owner", "stale_totp", "capability_missing", "revoked_at_fence", "no_secret"} {
		t.Run(scenario, func(t *testing.T) {
			s, f, _, in := googleUnit(t)
			want := app.ErrInformationForbidden
			switch scenario {
			case "no_fallback":
				f.fallback = false
				want = app.ErrInformationConflict
			case "non_owner":
				f.authority.EffectiveOwner = false
			case "stale_totp":
				f.authority.AssuredAt = f.authority.AssuredAt.Add(-16 * time.Minute)
			case "capability_missing":
				f.authority.Capabilities = map[string]bool{}
			case "revoked_at_fence":
				f.revokeAtFence = true
			case "no_secret":
				in.ClientSecret = nil
				want = app.ErrInformationInvalid
			}
			if _, err := s.Save(context.Background(), in); !errors.Is(err, want) || f.writes != 0 {
				t.Fatal("unsafe setting accepted", err)
			}
		})
	}
}
func TestGoogleClientChangeRequiresReplacementAndPublicStatusFailsClosed(t *testing.T) {
	s, f, _, in := googleUnit(t)
	if _, err := s.Save(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if !s.PublicStatus(f.stored)["enabled"] {
		t.Fatal("configured login hidden")
	}
	in.ClientSecret = nil
	in.Configuration.ClientID = "456-other.apps.googleusercontent.com"
	in.Configuration.Version = 2
	in.IdempotencyKey = "changed-client"
	if _, err := s.Save(context.Background(), in); !errors.Is(err, app.ErrInformationInvalid) {
		t.Fatal("credential reused across clients", err)
	}
	f.stored.Secret.Ciphertext[0] ^= 1
	if s.PublicStatus(f.stored)["enabled"] || s.PublicStatus(f.stored)["allow_signup"] {
		t.Fatal("corrupt credentials advertised as available")
	}
}
func TestGoogleProbeTruthfulResultReplayAndBound(t *testing.T) {
	s, f, p, in := googleUnit(t)
	if _, err := s.Save(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	result, err := s.Test(context.Background(), in.Subject, 2, "probe-1", in.CorrelationID)
	if err != nil || result != "JWKS_REACHABLE_CREDENTIALS_UNVERIFIED" || p.calls != 1 {
		t.Fatal("probe falsely reported credentials", result, err)
	}
	if _, err = s.Test(context.Background(), in.Subject, 2, "probe-1", in.CorrelationID); err != nil || p.calls != 1 {
		t.Fatal("probe replay repeated network", err)
	}
	p.err = errors.New("synthetic provider failure")
	if result, err = s.Test(context.Background(), in.Subject, 2, "probe-2", in.CorrelationID); err != nil || result != "UNAVAILABLE" {
		t.Fatal("provider failure misreported", result, err)
	}
	f.tests = 3
	if _, err = s.Test(context.Background(), in.Subject, 2, "probe-4", in.CorrelationID); !errors.Is(err, app.ErrInformationConflict) || p.calls != 2 {
		t.Fatal("probe rate bound bypassed", err)
	}
}
func TestGoogleSecretAADNonceIntegrityAndVersion(t *testing.T) {
	vault, err := smtpsecret.NewVault(&postgres.Pool{}, make([]byte, 32), 1)
	if err != nil {
		t.Fatal(err)
	}
	id := domain.ID{7}
	a, err := vault.SealGoogleClientSecret(id, []byte("synthetic-google-secret"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := vault.SealGoogleClientSecret(id, []byte("synthetic-google-secret"))
	if err != nil || string(a.Nonce) == string(b.Nonce) {
		t.Fatal("nonce reused", err)
	}
	for _, mutate := range []func(*app.GoogleCipher){func(v *app.GoogleCipher) { v.ID = domain.ID{8} }, func(v *app.GoogleCipher) { v.KeyVersion++ }, func(v *app.GoogleCipher) { v.Ciphertext = append([]byte{}, v.Ciphertext...); v.Ciphertext[0] ^= 1 }} {
		bad := a
		mutate(&bad)
		if _, err = vault.OpenGoogleClientSecret(bad); err == nil {
			t.Fatal("tampered credential accepted")
		}
	}
}
