package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/mfa"
)

type resendGraceStore struct{ *emailCommitStore }

func (s *resendGraceStore) GetEmailLoginResendChallenge(ctx context.Context, token, binding [32]byte, at time.Time) (app.MFALoginChallenge, error) {
	v, err := s.GetMFALoginChallenge(ctx, token, binding, at.Add(-app.EmailLoginResendGrace))
	if err != nil || v.Purpose != "EMAIL_LOGIN" || v.Attempts >= 5 {
		return app.MFALoginChallenge{}, app.ErrMFAConflict
	}
	return v, nil
}

type resendClock struct{ at time.Time }

func (c *resendClock) Now() time.Time { return c.at }

type resendLimit struct {
	calls int
	err   error
}

func (l *resendLimit) Allow(context.Context, [32]byte, [32]byte) error { l.calls++; return l.err }

type resendKeys struct{}

func (resendKeys) EmailLoginResendIdentity(app.ID) ([32]byte, error) { return [32]byte{1}, nil }

func TestExpiredEmailCodeCanResendButCannotVerify(t *testing.T) {
	for _, mode := range []string{"expired", "outside grace", "consumed", "attempts exhausted", "wrong binding", "rate limited", "wrong purpose"} {
		t.Run(mode, func(t *testing.T) {
			base := &mfaStoreFake{}
			store := &resendGraceStore{&emailCommitStore{base}}
			clock := &resendClock{time.Now().UTC()}
			ring, _ := mfa.NewKeyRing(1, map[int][32]byte{1: {1}})
			service, err := app.NewMFAService(store, ring, &tokenGeneratorFake{pair: sessionPair(t)}, &loginIDsFake{id: identityID(7)}, clock, &mfaAttemptsFake{threshold: 5}, mfaScopesFake{})
			if err != nil {
				t.Fatal(err)
			}
			mail := &loginMailFake{}
			_ = service.EnableEmailLogin(mail, []byte(strings.Repeat("k", 32)))
			_ = service.EnablePlatformEmailLogin(platformEmailPolicyFake{required: true}, mail)
			limits := &resendLimit{}
			_ = service.EnableEmailLoginResend(limits, resendKeys{})
			gate, err := service.BeginLogin(context.Background(), identityID(2), 1, app.AuthenticationMethodPassword)
			if err != nil {
				t.Fatal(err)
			}
			oldCode := mail.code
			clock.at = clock.at.Add(6 * time.Minute)
			if _, err = service.CompleteLogin(context.Background(), gate.Token, oldCode, "EMAIL_OTP", identityID(9)); err == nil {
				t.Fatal("expired code verified")
			}
			token := gate.Token
			switch mode {
			case "outside grace":
				clock.at = clock.at.Add(10 * time.Minute)
			case "consumed":
				base.loginConsumed = true
			case "attempts exhausted":
				base.loginChallenge.Attempts = 5
			case "wrong binding":
				token = strings.Repeat("A", 43)
			case "rate limited":
				limits.err = errors.New("email_login_resend_denied")
			case "wrong purpose":
				base.loginChallenge.Purpose = "MFA_LOGIN"
			}
			result, err := service.ResendLogin(context.Background(), token, [32]byte{2})
			if mode != "expired" {
				if err == nil || mail.calls != 1 {
					t.Fatal("unsafe resend accepted")
				}
				return
			}
			if err != nil || result.Token == "" || result.Token == gate.Token || mail.calls != 2 || !result.ExpiresAt.Equal(clock.at.Add(5*time.Minute)) {
				t.Fatal("expired-code resend failed", err)
			}
			if _, err = service.CompleteLogin(context.Background(), gate.Token, oldCode, "EMAIL_OTP", identityID(9)); err == nil {
				t.Fatal("old challenge revived")
			}
			if _, err = service.CompleteLogin(context.Background(), result.Token, mail.code, "EMAIL_OTP", identityID(9)); err != nil {
				t.Fatal("new code rejected", err)
			}
		})
	}
}
