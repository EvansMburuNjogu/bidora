package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/mfa"
	policy "bidora/backend/internal/platformadmin/domain"
)

type platformEmailPolicyFake struct {
	required bool
	err      error
}

func (p platformEmailPolicyFake) PlatformEmailLoginRequired(context.Context, app.ID, int64) (bool, error) {
	return p.required, p.err
}

type loginMailFake struct {
	code  string
	calls int
	err   error
}

func (d *loginMailFake) DeliverLoginCode(_ context.Context, _ email.Address, code string, _ time.Time) error {
	d.calls++
	d.code = code
	return d.err
}

type emailCommitStore struct{ *mfaStoreFake }

func (s *emailCommitStore) CommitMFALogin(_ context.Context, c app.MFALoginCommit) error {
	if s.loginConsumed || c.Method != "EMAIL_OTP" || c.UserID != s.loginChallenge.UserID || c.AuthEpoch != s.loginChallenge.AuthEpoch {
		return app.ErrMFAConflict
	}
	s.loginConsumed = true
	s.loginCommit = c
	return nil
}
func emailPolicyService(t *testing.T, base *mfaStoreFake, attempts *mfaAttemptsFake, mail *loginMailFake) (*app.MFAService, time.Time) {
	t.Helper()
	now := time.Now().UTC()
	var key [32]byte
	key[0] = 1
	ring, err := mfa.NewKeyRing(1, map[int][32]byte{1: key})
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewMFAService(&emailCommitStore{base}, ring, &tokenGeneratorFake{pair: sessionPair(t)}, &loginIDsFake{id: identityID(7)}, loginClockFake{at: now}, attempts, mfaScopesFake{})
	if err != nil {
		t.Fatal(err)
	}
	_ = service.EnableEmailLogin(mail, []byte(strings.Repeat("k", 32)))
	_ = service.EnablePlatformEmailLogin(platformEmailPolicyFake{required: true}, mail)
	return service, now
}
func TestPlatformEmailLoginVerificationRejectsWrongBindingExpiryReplayAndExcessAttempts(t *testing.T) {
	for _, mode := range []string{"replay", "binding", "expiry", "attempts"} {
		t.Run(mode, func(t *testing.T) {
			base, mail := &mfaStoreFake{}, &loginMailFake{}
			service, now := emailPolicyService(t, base, &mfaAttemptsFake{threshold: 5}, mail)
			gate, err := service.BeginLogin(context.Background(), identityID(2), 1, app.AuthenticationMethodPassword)
			if err != nil {
				t.Fatal(err)
			}
			token := gate.Token
			if mode == "binding" {
				token = strings.Repeat("A", 43)
			}
			if mode == "expiry" {
				base.loginChallenge.ExpiresAt = now.Add(-time.Second)
			}
			if mode == "attempts" {
				wrong := "000000"
				if mail.code == wrong {
					wrong = "111111"
				}
				for i := 0; i < 5; i++ {
					_, _ = service.CompleteLogin(context.Background(), token, wrong, "EMAIL_OTP", identityID(9))
				}
			}
			result, err := service.CompleteLogin(context.Background(), token, mail.code, "EMAIL_OTP", identityID(9))
			if mode == "replay" {
				if err != nil || result.Session.IsZero() {
					t.Fatal("valid mailbox challenge did not establish session")
				}
				if _, err = service.CompleteLogin(context.Background(), token, mail.code, "EMAIL_OTP", identityID(9)); err == nil {
					t.Fatal("mailbox code replay accepted")
				}
			} else if err == nil || !result.Session.IsZero() {
				t.Fatal("unsafe mailbox verification issued session")
			}
		})
	}
}
func TestPlatformEmailLoginUsesRealChallengeAndRetainsReplayAndBindingChecks(t *testing.T) {
	now := time.Now().UTC()
	base := &mfaStoreFake{}
	service, _ := mfaServiceFixture(t, now, base, &mfaAttemptsFake{threshold: 5})
	ordinary, admin := &loginMailFake{}, &loginMailFake{}
	if err := service.EnableEmailLogin(ordinary, []byte(strings.Repeat("k", 32))); err != nil {
		t.Fatal(err)
	}
	if err := service.EnablePlatformEmailLogin(platformEmailPolicyFake{required: true}, admin); err != nil {
		t.Fatal(err)
	}
	gate, err := service.BeginLogin(context.Background(), identityID(2), 1, app.AuthenticationMethodPassword)
	if err != nil || !gate.Required || gate.Token == "" || admin.calls != 1 || ordinary.calls != 0 || len(admin.code) != 6 {
		t.Fatal("administrator did not require the dedicated mailbox challenge")
	}
	// Full email completion/replay coverage is exercised below with a store that
	// consumes email rather than the existing TOTP-only fixture committer.
}
func TestPlatformEmailLoginFailsClosedOnDeliveryAndPolicyErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		p       platformEmailPolicyFake
		mailErr error
	}{{"delivery", platformEmailPolicyFake{required: true}, errors.New("provider unavailable")}, {"policy", platformEmailPolicyFake{err: errors.New("database unavailable")}, nil}} {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := mfaServiceFixture(t, time.Now().UTC(), &mfaStoreFake{}, &mfaAttemptsFake{threshold: 5})
			mail := &loginMailFake{err: tc.mailErr}
			_ = service.EnableEmailLogin(mail, []byte(strings.Repeat("k", 32)))
			_ = service.EnablePlatformEmailLogin(tc.p, mail)
			gate, err := service.BeginLogin(context.Background(), identityID(2), 1, app.AuthenticationMethodPassword)
			if err == nil || gate.Token != "" {
				t.Fatal("failed administrator verification fell back to a session")
			}
		})
	}
}
func TestPlatformEmailPolicyPreservesEnrolledGoogleTOTPLogin(t *testing.T) {
	base := &mfaStoreFake{active: &app.MFAFactorMaterial{FactorID: identityID(3), UserID: identityID(2)}}
	service, _ := mfaServiceFixture(t, time.Now().UTC(), base, &mfaAttemptsFake{threshold: 5})
	mail := &loginMailFake{}
	_ = service.EnableEmailLogin(mail, []byte(strings.Repeat("k", 32)))
	_ = service.EnablePlatformEmailLogin(platformEmailPolicyFake{required: true}, mail)
	gate, err := service.BeginLogin(context.Background(), identityID(2), 1, app.AuthenticationMethodGoogle)
	if err != nil || !gate.Required || mail.calls != 0 || base.loginChallenge.Purpose == "EMAIL_LOGIN" {
		t.Fatal("existing enrolled Google/TOTP login was changed")
	}
}
func TestPlatformAssuranceDoesNotAcceptOrdinaryEmailOrPasswordSessions(t *testing.T) {
	for _, tc := range []struct {
		level, method string
		want          bool
	}{{"AAL1", "EMAIL_OTP", false}, {"AAL1", "PRIMARY_ONLY", false}, {"AAL2", "PRIMARY_ONLY", false}, {"AAL2", "EMAIL_OTP", true}, {"AAL2", "TOTP", true}, {"AAL2", "RECOVERY_CODE", false}} {
		if policy.AdministratorAssurance(tc.level, tc.method) != tc.want {
			t.Fatal("administrator assurance policy drifted")
		}
	}
}
