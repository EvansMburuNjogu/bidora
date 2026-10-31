package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type googleLinkProofHTTPFake struct {
	proof identityapp.GoogleLinkProof
	err   error
	input identityapp.GoogleLinkReauthenticationInput
	calls int
}

func (f *googleLinkProofHTTPFake) Reauthenticate(_ context.Context, input identityapp.GoogleLinkReauthenticationInput) (identityapp.GoogleLinkProof, error) {
	f.calls++
	f.input = input
	f.input.Credentials.Password = append([]byte(nil), input.Credentials.Password...)
	return f.proof, f.err
}

type googleLinkerHTTPFake struct{}

func (googleLinkerHTTPFake) LinkVerified(context.Context, identityapp.GoogleLinkProof, googleoidc.Identity, identityapp.ID) (identityapp.LoginResult, error) {
	return identityapp.LoginResult{}, errors.New("unused")
}

type googleLinkCallbackHTTPFake struct {
	result identityapp.LoginResult
	err    error
	input  identityapp.GoogleLinkCallbackInput
	calls  int
}

func (f *googleLinkCallbackHTTPFake) Callback(_ context.Context, input identityapp.GoogleLinkCallbackInput) (identityapp.LoginResult, error) {
	f.calls++
	f.input = input
	return f.result, f.err
}
func googleLinkHandlersApp(t *testing.T, proof *googleLinkProofHTTPFake, callback *googleLinkCallbackHTTPFake) (*fiber.App, *googleHTTPStore, string, string) {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 33})
	if err != nil {
		t.Fatal(err)
	}
	principal := identityapp.SessionPrincipal{SessionID: httpIdentityID(8), UserID: httpIdentityID(9), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 4, IdleExpiresAt: time.Now().Add(time.Hour), AbsoluteExpiresAt: time.Now().Add(2 * time.Hour)}
	validator := &sessionValidatorFake{principal: principal}
	toucher := &sessionToucherFake{}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, toucher, protection, browserClockFake{at: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	entropy := make([]byte, googleoidc.TransactionSecretBytes*4)
	for index := range entropy {
		entropy[index] = byte(index + 1)
	}
	store := &googleHTTPStore{}
	flow, err := identityapp.NewGoogleOIDCLinkFlowWithRandom(googleoidc.AuthorizationConfig{ClientID: "client.apps.googleusercontent.com", RedirectURI: "https://api.example.com/v1/auth/google/link/callback"}, store, googleHTTPExchange{}, googleHTTPVerifier{}, googleLinkerHTTPFake{}, bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	initiation, err := identityapp.NewGoogleLinkInitiationService(proof, flow)
	if err != nil {
		t.Fatal(err)
	}
	ips := &loginIPFake{address: netip.MustParseAddr("198.51.100.9")}
	var networkKey attempt.Key
	networkKey[31] = 1
	keys := &loginKeyerFake{key: networkKey}
	startHandler, callbackHandler, err := httpapi.NewGoogleLinkHTTPHandlers(initiation, callback, ips, keys, httpapi.GoogleHTTPConfig{SuccessRedirect: "https://app.example.com/settings/security?linked=no", FailureRedirect: "https://app.example.com/settings/security", TransactionTTL: 5 * time.Minute})
	if err == nil {
		t.Fatal("query-bearing success redirect unexpectedly accepted")
	}
	startHandler, callbackHandler, err = httpapi.NewGoogleLinkHTTPHandlers(initiation, callback, ips, keys, httpapi.GoogleHTTPConfig{SuccessRedirect: "https://app.example.com/settings/security/linked", FailureRedirect: "https://app.example.com/settings/security", TransactionTTL: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.GoogleLinkStartPath, middleware.Handle, startHandler.Handle)
	app.Get(httpapi.GoogleLinkCallbackPath, middleware.Handle, callbackHandler.Handle)
	return app, store, pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()
}
func TestGoogleLinkStartRequiresSessionCSRFFreshPasswordAndNetwork(t *testing.T) {
	proof := &googleLinkProofHTTPFake{proof: identityapp.GoogleLinkProof{UserID: httpIdentityID(9), SessionID: httpIdentityID(8), AuthEpoch: 4, CredentialVersion: 2}}
	callback := &googleLinkCallbackHTTPFake{}
	app, store, rawSession, rawCSRF := googleLinkHandlersApp(t, proof, callback)
	request := httptest.NewRequest(http.MethodPost, httpapi.GoogleLinkStartPath, strings.NewReader(`{"email":"User@example.test","password":"fresh password proof"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set(session.CSRFHeader, rawCSRF)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: rawSession})
	request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: rawCSRF})
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSeeOther || !strings.HasPrefix(response.Header.Get("Location"), "https://accounts.google.com/") || proof.calls != 1 || proof.input.Principal.UserID != httpIdentityID(9) || proof.input.Credentials.Email != "User@example.test" || string(proof.input.Credentials.Password) != "fresh password proof" || proof.input.Credentials.NetworkScope.IsZero() || store.material.Purpose != googleoidc.TransactionPurposeLink || len(response.Cookies()) != 1 {
		t.Fatal("Google link start binding drifted", response.StatusCode, proof.input)
	}
}
func TestGoogleLinkStartRejectsMissingCSRFFirst(t *testing.T) {
	proof := &googleLinkProofHTTPFake{}
	app, store, rawSession, _ := googleLinkHandlersApp(t, proof, &googleLinkCallbackHTTPFake{})
	request := httptest.NewRequest(http.MethodPost, httpapi.GoogleLinkStartPath, strings.NewReader(`{"email":"user@example.test","password":"password value"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.com")
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: rawSession})
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden || proof.calls != 0 || store.state != "" {
		t.Fatal("missing CSRF crossed Google link proof", response.StatusCode)
	}
}
func TestGoogleLinkCallbackUsesCurrentPrincipalAndReplacesCookies(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 55})
	if err != nil {
		t.Fatal(err)
	}
	callback := &googleLinkCallbackHTTPFake{result: identityapp.LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: time.Now().Add(30 * time.Minute), AbsoluteExpiresAt: time.Now().Add(12 * time.Hour)}}
	app, _, rawSession, _ := googleLinkHandlersApp(t, &googleLinkProofHTTPFake{}, callback)
	state := strings.Repeat("s", 43)
	binding := strings.Repeat("b", 43)
	request := httptest.NewRequest(http.MethodGet, httpapi.GoogleLinkCallbackPath+"?state="+state+"&code=code", nil)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: rawSession})
	request.AddCookie(&http.Cookie{Name: httpapi.GoogleCorrelationCookie, Value: binding})
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "https://app.example.com/settings/security/linked" || callback.calls != 1 || callback.input.Principal.UserID != httpIdentityID(9) || callback.input.State != state || callback.input.BrowserBinding != binding {
		t.Fatal("Google link callback drifted", response.StatusCode, callback.input)
	}
	cookies := map[string]*http.Cookie{}
	for _, cookie := range response.Cookies() {
		cookies[cookie.Name] = cookie
	}
	if len(cookies) != 3 || cookies[session.CookieName] == nil || cookies[session.CSRFCookieName] == nil || cookies[httpapi.GoogleCorrelationCookie].MaxAge != -1 {
		t.Fatal("Google link replacement cookies drifted", response.Cookies())
	}
}
