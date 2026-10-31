package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
)

type googleCallbackFake struct {
	result identityapp.GoogleIdentityResult
	err    error
	input  identityapp.GoogleCallbackInput
	calls  int
}

func (f *googleCallbackFake) Callback(_ context.Context, input identityapp.GoogleCallbackInput) (identityapp.GoogleIdentityResult, error) {
	f.calls++
	f.input = input
	return f.result, f.err
}
func TestGoogleLoginCommitsApprovedSessionAfterIdentity(t *testing.T) {
	tokens := &tokenGeneratorFake{pair: sessionPair(t)}
	ids := &loginIDsFake{id: identityID(7)}
	committer := &committerFake{}
	issuer, err := identityapp.NewSessionIssuer(tokens, ids, loginClockFake{at: time.Now().UTC()}, committer)
	if err != nil {
		t.Fatal(err)
	}
	callback := &googleCallbackFake{result: identityapp.GoogleIdentityResult{UserID: identityID(9), AuthEpoch: 3, Created: true}}
	service, err := identityapp.NewGoogleLoginService(callback, issuer)
	if err != nil {
		t.Fatal(err)
	}
	input := identityapp.GoogleCallbackInput{State: "private-state", BrowserBinding: "private-binding", Code: "private-code", CorrelationID: identityID(8)}
	result, err := service.Login(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if callback.calls != 1 || callback.input != input || committer.calls != 1 || committer.commit.UserID != identityID(9) || committer.commit.AuthEpoch != 3 || committer.commit.AuthenticationMethod != identityapp.AuthenticationMethodGoogle || result.Identity.UserID != identityID(9) || result.Login.Session.IsZero() || result.Login.CSRF.IsZero() {
		t.Fatal("Google session issuance drifted", result)
	}
	formatted := fmt.Sprintf("%#v", result)
	for _, secret := range []string{result.Login.Session.RevealForCookie(), result.Login.CSRF.RevealForBootstrap(), input.Code} {
		if strings.Contains(formatted, secret) {
			t.Fatal("Google login result leaked secret")
		}
	}
}
func TestGoogleLoginDefersSessionWhenMFAIsRequired(t *testing.T) {
	tokens := &tokenGeneratorFake{pair: sessionPair(t)}
	committer := &committerFake{}
	issuer, _ := identityapp.NewSessionIssuer(tokens, &loginIDsFake{id: identityID(7)}, loginClockFake{at: time.Now().UTC()}, committer)
	callback := &googleCallbackFake{result: identityapp.GoogleIdentityResult{UserID: identityID(9), AuthEpoch: 3}}
	service, _ := identityapp.NewGoogleLoginService(callback, issuer)
	gate := &mfaGateFake{result: identityapp.MFALoginGateResult{Required: true, Token: "synthetic-mfa-challenge", ExpiresAt: time.Now().Add(5 * time.Minute)}}
	if err := service.EnableMFA(gate); err != nil {
		t.Fatal(err)
	}
	result, err := service.Login(context.Background(), identityapp.GoogleCallbackInput{CorrelationID: identityID(8)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Login.MFARequired || result.Login.MFAToken == "" || !result.Login.Session.IsZero() || !result.Login.CSRF.IsZero() || gate.calls != 1 || tokens.calls != 0 || committer.calls != 0 {
		t.Fatal("Google MFA deferral issued a primary session", result)
	}
}

func TestGoogleLoginReturnsNoSecretsOnIdentityOrCommitFailure(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		callbackErr, commitErr error
	}{{"identity", errors.New("google_authentication_invalid"), nil}, {"commit", nil, errors.New("secret-canary commit")}} {
		t.Run(tc.name, func(t *testing.T) {
			tokens := &tokenGeneratorFake{pair: sessionPair(t)}
			issuer, _ := identityapp.NewSessionIssuer(tokens, &loginIDsFake{id: identityID(7)}, loginClockFake{at: time.Now().UTC()}, &committerFake{err: tc.commitErr})
			callback := &googleCallbackFake{result: identityapp.GoogleIdentityResult{UserID: identityID(9), AuthEpoch: 1}, err: tc.callbackErr}
			service, _ := identityapp.NewGoogleLoginService(callback, issuer)
			result, err := service.Login(context.Background(), identityapp.GoogleCallbackInput{CorrelationID: identityID(8)})
			if err == nil || result != (identityapp.GoogleLoginResult{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("failed Google login exposed result", result, err)
			}
			if tc.name == "identity" && tokens.calls != 0 {
				t.Fatal("identity failure generated session secrets")
			}
		})
	}
	if service, err := identityapp.NewGoogleLoginService(nil, &identityapp.SessionIssuer{}); err == nil || service != nil {
		t.Fatal("nil Google callback accepted")
	}
	var zero *identityapp.GoogleLoginService
	if _, err := zero.Login(context.Background(), identityapp.GoogleCallbackInput{CorrelationID: identityID(1)}); err == nil {
		t.Fatal("zero Google login accepted")
	}
}
