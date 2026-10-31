package identity_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/googleoidc"
)

type googleVerifiedLinkerFake struct {
	result      identityapp.LoginResult
	err         error
	proof       identityapp.GoogleLinkProof
	identity    googleoidc.Identity
	correlation identityapp.ID
	calls       int
}

func (l *googleVerifiedLinkerFake) LinkVerified(_ context.Context, proof identityapp.GoogleLinkProof, identity googleoidc.Identity, correlation identityapp.ID) (identityapp.LoginResult, error) {
	l.calls++
	l.proof, l.identity, l.correlation = proof, identity, correlation
	return l.result, l.err
}
func newGoogleLinkFlow(t *testing.T, store *googleTransactionStoreFake, exchange *googleExchangeFake, verifier *googleTokenVerifierFake, linker *googleVerifiedLinkerFake) *identityapp.GoogleOIDCLinkFlow {
	t.Helper()
	entropy := make([]byte, googleoidc.TransactionSecretBytes*4)
	for index := range entropy {
		entropy[index] = byte(index + 1)
	}
	flow, err := identityapp.NewGoogleOIDCLinkFlowWithRandom(googleoidc.AuthorizationConfig{ClientID: "client.apps.googleusercontent.com", RedirectURI: "https://app.example.test/v1/auth/google/callback"}, store, exchange, verifier, linker, bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	return flow
}
func TestGoogleLinkInitiationFreshlyAuthenticatesAndPurposeBinds(t *testing.T) {
	store := &googleTransactionStoreFake{}
	flow := newGoogleLinkFlow(t, store, &googleExchangeFake{}, &googleTokenVerifierFake{}, &googleVerifiedLinkerFake{})
	auth := &authServiceFake{result: identityapp.Authentication{UserID: identityID(9), AuthEpoch: 4, CredentialVersion: 3}}
	proofService, _ := identityapp.NewGoogleLinkReauthenticationService(auth, &rehasherFake{})
	initiation, err := identityapp.NewGoogleLinkInitiationService(proofService, flow)
	if err != nil {
		t.Fatal(err)
	}
	start, err := initiation.Start(context.Background(), identityapp.GoogleLinkReauthenticationInput{Principal: googleLinkPrincipal(), Credentials: loginInput("user@example.test", []byte("fresh password"))})
	if err != nil {
		t.Fatal(err)
	}
	if start.URLForRedirect() == "" || start.BrowserBindingForCookie() == "" || store.material.Purpose != googleoidc.TransactionPurposeLink || store.material.CredentialVersion != 3 || !store.material.MatchesLink([16]byte(identityID(9)), [16]byte(identityID(8)), 4) || auth.calls != 1 {
		t.Fatal("Google link initiation binding drifted")
	}
}
func TestGoogleLinkCallbackValidatesPrincipalBeforeExchangeAndCommit(t *testing.T) {
	store := &googleTransactionStoreFake{}
	exchange := &googleExchangeFake{result: googleoidc.ExchangeResult{IDToken: "signed-token"}}
	verifier := &googleTokenVerifierFake{identity: googleFlowIdentity(t)}
	linker := &googleVerifiedLinkerFake{result: identityapp.LoginResult{Session: sessionPair(t).Session, CSRF: sessionPair(t).CSRF}}
	flow := newGoogleLinkFlow(t, store, exchange, verifier, linker)
	proof := identityapp.GoogleLinkProof{UserID: identityID(9), SessionID: identityID(8), AuthEpoch: 4, CredentialVersion: 3}
	start, err := flow.Start(context.Background(), proof)
	if err != nil {
		t.Fatal(err)
	}
	principal := googleLinkPrincipal()
	result, err := flow.Callback(context.Background(), identityapp.GoogleLinkCallbackInput{Principal: principal, State: store.state, BrowserBinding: start.BrowserBindingForCookie(), Code: "code", CorrelationID: identityID(7)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.IsZero() || exchange.calls != 1 || verifier.calls != 1 || linker.calls != 1 || linker.proof != proof || linker.correlation != identityID(7) {
		t.Fatal("Google link callback orchestration drifted")
	}
}
func TestGoogleLinkCallbackRejectsPrincipalSwapBeforeExchange(t *testing.T) {
	store := &googleTransactionStoreFake{}
	exchange := &googleExchangeFake{}
	flow := newGoogleLinkFlow(t, store, exchange, &googleTokenVerifierFake{}, &googleVerifiedLinkerFake{})
	start, err := flow.Start(context.Background(), identityapp.GoogleLinkProof{UserID: identityID(9), SessionID: identityID(8), AuthEpoch: 4, CredentialVersion: 3})
	if err != nil {
		t.Fatal(err)
	}
	principal := googleLinkPrincipal()
	principal.SessionID = identityID(6)
	result, err := flow.Callback(context.Background(), identityapp.GoogleLinkCallbackInput{Principal: principal, State: store.state, BrowserBinding: start.BrowserBindingForCookie(), Code: "code", CorrelationID: identityID(7)})
	if err == nil || err.Error() != "google_link_invalid" || result != (identityapp.LoginResult{}) || exchange.calls != 0 {
		t.Fatal("swapped Google link principal reached exchange", result, err)
	}
}
func TestGoogleLinkFlowFailsClosed(t *testing.T) {
	store := &googleTransactionStoreFake{consumeErr: errors.New("google_oidc_transaction_unavailable")}
	flow := newGoogleLinkFlow(t, store, &googleExchangeFake{}, &googleTokenVerifierFake{}, &googleVerifiedLinkerFake{})
	start, err := flow.Start(context.Background(), identityapp.GoogleLinkProof{UserID: identityID(9), SessionID: identityID(8), AuthEpoch: 4, CredentialVersion: 3})
	if err != nil {
		t.Fatal(err)
	}
	result, err := flow.Callback(context.Background(), identityapp.GoogleLinkCallbackInput{Principal: googleLinkPrincipal(), State: store.state, BrowserBinding: start.BrowserBindingForCookie(), Code: "code", CorrelationID: identityID(7)})
	if err == nil || err.Error() != "google_link_unavailable" || result != (identityapp.LoginResult{}) {
		t.Fatal("Google link dependency failure drifted", result, err)
	}
	if service, err := identityapp.NewGoogleLinkInitiationService(nil, flow); err == nil || service != nil {
		t.Fatal("nil link proof issuer accepted")
	}
}
