package identity_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/googleoidc"
)

type googleTransactionStoreFake struct {
	state                 string
	material              googleoidc.TransactionMaterial
	createErr, consumeErr error
	creates, consumes     int
}

func (s *googleTransactionStoreFake) Create(_ context.Context, state string, material googleoidc.TransactionMaterial) error {
	s.creates++
	s.state = state
	s.material = material
	return s.createErr
}
func (s *googleTransactionStoreFake) Consume(_ context.Context, state string) (googleoidc.TransactionMaterial, error) {
	s.consumes++
	if state != s.state {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	return s.material, s.consumeErr
}

type googleExchangeFake struct {
	result         googleoidc.ExchangeResult
	err            error
	code, verifier string
	calls          int
}

func (e *googleExchangeFake) Exchange(_ context.Context, code, verifier string) (googleoidc.ExchangeResult, error) {
	e.calls++
	e.code, e.verifier = code, verifier
	return e.result, e.err
}

type googleTokenVerifierFake struct {
	identity     googleoidc.Identity
	err          error
	token, nonce string
	calls        int
}

func (v *googleTokenVerifierFake) Verify(_ context.Context, token, nonce string) (googleoidc.Identity, error) {
	v.calls++
	v.token, v.nonce = token, nonce
	return v.identity, v.err
}

type googleVerifiedAuthenticatorFake struct {
	result identityapp.GoogleIdentityResult
	err    error
	input  identityapp.VerifiedGoogleAuthenticationInput
	calls  int
}

func (a *googleVerifiedAuthenticatorFake) AuthenticateVerified(_ context.Context, input identityapp.VerifiedGoogleAuthenticationInput) (identityapp.GoogleIdentityResult, error) {
	a.calls++
	a.input = input
	return a.result, a.err
}
func googleFlowIdentity(t *testing.T) googleoidc.Identity {
	t.Helper()
	address, err := email.Parse("User@example.test")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	return googleoidc.Identity{Subject: "google-subject", Email: address, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
}
func newGoogleFlow(t *testing.T, store *googleTransactionStoreFake, exchange *googleExchangeFake, verifier *googleTokenVerifierFake, authenticator *googleVerifiedAuthenticatorFake) (*identityapp.GoogleOIDCFlow, []byte) {
	t.Helper()
	entropy := make([]byte, googleoidc.TransactionSecretBytes*4)
	for index := range entropy {
		entropy[index] = byte(index + 1)
	}
	flow, err := identityapp.NewGoogleOIDCFlowWithRandom(googleoidc.AuthorizationConfig{ClientID: "client.apps.googleusercontent.com", RedirectURI: "https://app.example.test/v1/auth/google/callback"}, store, exchange, verifier, authenticator, bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	return flow, entropy
}
func TestGoogleOIDCFlowStartsBoundAuthorization(t *testing.T) {
	store := &googleTransactionStoreFake{}
	flow, _ := newGoogleFlow(t, store, &googleExchangeFake{}, &googleTokenVerifierFake{}, &googleVerifiedAuthenticatorFake{})
	start, err := flow.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(start.URLForRedirect())
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "accounts.google.com" || parsed.Path != "/o/oauth2/v2/auth" || query.Get("client_id") != "client.apps.googleusercontent.com" || query.Get("redirect_uri") != "https://app.example.test/v1/auth/google/callback" || query.Get("response_type") != "code" || query.Get("scope") != "openid email" || query.Get("state") != store.state || query.Get("nonce") != store.material.Nonce || query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") == "" || store.creates != 1 || start.BrowserBindingForCookie() == "" {
		t.Fatal("Google authorization request drifted", parsed.String())
	}
	matches, err := googleoidc.MatchesBrowserBinding(start.BrowserBindingForCookie(), store.material.BrowserBindingHash)
	if err != nil || !matches {
		t.Fatal("authorization browser binding missing")
	}
	formatted := fmt.Sprintf("%#v", start)
	if strings.Contains(formatted, store.state) || strings.Contains(formatted, start.BrowserBindingForCookie()) {
		t.Fatal("authorization start formatting leaked secrets")
	}
}
func TestGoogleOIDCFlowConsumesBeforeExchangeAndAuthenticates(t *testing.T) {
	store := &googleTransactionStoreFake{}
	exchange := &googleExchangeFake{result: googleoidc.ExchangeResult{IDToken: "signed.id.token"}}
	verifier := &googleTokenVerifierFake{identity: googleFlowIdentity(t)}
	authenticator := &googleVerifiedAuthenticatorFake{result: identityapp.GoogleIdentityResult{UserID: identityID(9), AuthEpoch: 1}}
	flow, _ := newGoogleFlow(t, store, exchange, verifier, authenticator)
	start, err := flow.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := flow.Callback(context.Background(), identityapp.GoogleCallbackInput{State: store.state, BrowserBinding: start.BrowserBindingForCookie(), Code: "authorization-code", CorrelationID: identityID(8)})
	if err != nil {
		t.Fatal(err)
	}
	if result.UserID != identityID(9) || store.consumes != 1 || exchange.calls != 1 || exchange.code != "authorization-code" || exchange.verifier != store.material.Verifier || verifier.calls != 1 || verifier.token != "signed.id.token" || verifier.nonce != store.material.Nonce || authenticator.calls != 1 || authenticator.input.CorrelationID != identityID(8) {
		t.Fatal("Google callback orchestration drifted", result)
	}
}
func TestGoogleOIDCFlowRejectsLinkPurposeBeforeCodeExchange(t *testing.T) {
	store := &googleTransactionStoreFake{}
	exchange := &googleExchangeFake{}
	flow, _ := newGoogleFlow(t, store, exchange, &googleTokenVerifierFake{}, &googleVerifiedAuthenticatorFake{})
	start, err := flow.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	transaction := googleTransaction(t)
	var userID, sessionID [16]byte
	userID[15], sessionID[15] = 1, 2
	store.material, err = transaction.MaterialForLink(userID, sessionID, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	result, err := flow.Callback(context.Background(), identityapp.GoogleCallbackInput{State: store.state, BrowserBinding: start.BrowserBindingForCookie(), Code: "code", CorrelationID: identityID(1)})
	if err == nil || err.Error() != "google_authentication_invalid" || result != (identityapp.GoogleIdentityResult{}) || exchange.calls != 0 {
		t.Fatal("link-purpose state crossed sign-in flow", result, err)
	}
}

func TestGoogleOIDCFlowRejectsBrowserSwapBeforeCodeExchange(t *testing.T) {
	store := &googleTransactionStoreFake{}
	exchange := &googleExchangeFake{}
	flow, _ := newGoogleFlow(t, store, exchange, &googleTokenVerifierFake{}, &googleVerifiedAuthenticatorFake{})
	_, err := flow.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := flow.Callback(context.Background(), identityapp.GoogleCallbackInput{State: store.state, BrowserBinding: store.state, Code: "code", CorrelationID: identityID(1)})
	if err == nil || err.Error() != "google_authentication_invalid" || result != (identityapp.GoogleIdentityResult{}) || store.consumes != 1 || exchange.calls != 0 {
		t.Fatal("browser-swapped callback reached exchange", result, err)
	}
}
func TestGoogleOIDCFlowMapsDependencyFailuresWithoutSecrets(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		storeErr, exchangeErr, verifyErr error
		want                             string
	}{{"state invalid", errors.New("google_oidc_transaction_invalid"), nil, nil, "google_authentication_invalid"}, {"state unavailable", errors.New("google_oidc_transaction_unavailable"), nil, nil, "google_authentication_unavailable"}, {"exchange unavailable", nil, errors.New("google_oidc_exchange_unavailable"), nil, "google_authentication_unavailable"}, {"token invalid", nil, nil, errors.New("google_oidc_token_invalid"), "google_authentication_invalid"}} {
		t.Run(tc.name, func(t *testing.T) {
			store := &googleTransactionStoreFake{consumeErr: tc.storeErr}
			exchange := &googleExchangeFake{result: googleoidc.ExchangeResult{IDToken: "token"}, err: tc.exchangeErr}
			verifier := &googleTokenVerifierFake{identity: googleFlowIdentity(t), err: tc.verifyErr}
			flow, _ := newGoogleFlow(t, store, exchange, verifier, &googleVerifiedAuthenticatorFake{})
			start, err := flow.Start(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			result, err := flow.Callback(context.Background(), identityapp.GoogleCallbackInput{State: store.state, BrowserBinding: start.BrowserBindingForCookie(), Code: "code", CorrelationID: identityID(1)})
			if err == nil || err.Error() != tc.want || result != (identityapp.GoogleIdentityResult{}) || strings.Contains(fmt.Sprintf("%#v", err), "token") {
				t.Fatal("unsafe callback failure", result, err)
			}
		})
	}
}
