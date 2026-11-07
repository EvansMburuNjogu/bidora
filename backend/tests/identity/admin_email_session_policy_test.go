package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/securitypolicy"
)

func TestEmailLoginSessionUsesAccountSpecificSavedPolicyForDatabaseAndCookies(t *testing.T) {
	for _, method := range []app.AuthenticationMethod{app.AuthenticationMethodPassword, app.AuthenticationMethodGoogle} {
		for _, tc := range []struct {
			name                     string
			admin                    bool
			adminIdle, adminAbsolute int32
		}{
			{"administrator_seven_days", true, 604800, 604800},
			{"administrator_configured_limits", true, 7200, 14400},
			{"ordinary_user", false, 604800, 604800},
		} {
			t.Run(string(method)+"/"+tc.name, func(t *testing.T) {
				base := &mfaStoreFake{platformAdministrator: tc.admin}
				mail := &loginMailFake{}
				service, now := emailPolicyService(t, base, &mfaAttemptsFake{threshold: 5}, mail)
				service.RequireEmailLogin() // Match both deployed password and Google composition paths.
				policy := securitypolicy.Default()
				policy.IdleSeconds = 300
				policy.AbsoluteSeconds = 3600
				policy.AdminIdleSeconds = tc.adminIdle
				policy.AdminAbsoluteSeconds = tc.adminAbsolute
				ctx, err := securitypolicy.With(context.Background(), policy)
				if err != nil {
					t.Fatal(err)
				}
				gate, err := service.BeginLogin(ctx, identityID(2), 1, method)
				if err != nil {
					t.Fatal(err)
				}
				result, err := service.CompleteLogin(ctx, gate.Token, mail.code, "EMAIL_OTP", identityID(9))
				if err != nil {
					t.Fatal(err)
				}
				idle, absolute := policy.IdleSeconds, policy.AbsoluteSeconds
				if tc.admin {
					idle, absolute = tc.adminIdle, tc.adminAbsolute
				}
				if !result.IdleExpiresAt.Equal(now.Add(time.Duration(idle)*time.Second)) || !result.AbsoluteExpiresAt.Equal(now.Add(time.Duration(absolute)*time.Second)) {
					t.Fatal("wrong account policy issued", result.IdleExpiresAt.Sub(now), result.AbsoluteExpiresAt.Sub(now))
				}
				if !base.loginCommit.IdleExpiresAt.Equal(result.IdleExpiresAt) || !base.loginCommit.AbsoluteExpiresAt.Equal(result.AbsoluteExpiresAt) {
					t.Fatal("database and response expiries differ")
				}
				cookie, err := browser.SessionCookie(result.Session.RevealForCookie(), result.AbsoluteExpiresAt)
				if err != nil {
					t.Fatal(err)
				}
				csrf, err := browser.CSRFCookie(result.CSRF.RevealForBootstrap(), result.AbsoluteExpiresAt)
				if err != nil {
					t.Fatal(err)
				}
				if !cookie.Expires.Equal(result.AbsoluteExpiresAt) || !csrf.Expires.Equal(result.AbsoluteExpiresAt) || !cookie.Secure || !cookie.HttpOnly {
					t.Fatal("cookie dropped policy or protection")
				}
				if tc.admin && !result.IdleExpiresAt.After(now.Add(31*time.Minute)) {
					t.Fatal("admin still capped by ordinary timeout")
				}
				if _, err = service.CompleteLogin(ctx, gate.Token, mail.code, "EMAIL_OTP", identityID(9)); err == nil {
					t.Fatal("longer session accepted code replay")
				}
			})
		}
	}
}
func TestEmailSessionClassificationFailureDoesNotConsumeChallengeOrIssueSession(t *testing.T) {
	base := &mfaStoreFake{classificationErr: errors.New("classification unavailable")}
	mail := &loginMailFake{}
	service, _ := emailPolicyService(t, base, &mfaAttemptsFake{threshold: 5}, mail)
	gate, err := service.BeginLogin(context.Background(), identityID(2), 1, app.AuthenticationMethodPassword)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CompleteLogin(context.Background(), gate.Token, mail.code, "EMAIL_OTP", identityID(9))
	if err == nil || !result.Session.IsZero() || base.loginConsumed || !base.loginCommit.SessionID.IsZero() {
		t.Fatal("failed classification created session")
	}
}
