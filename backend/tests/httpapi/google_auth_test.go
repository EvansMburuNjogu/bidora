package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type googleHTTPStore struct {
	state    string
	material googleoidc.TransactionMaterial
}

func (s *googleHTTPStore) Create(_ context.Context, state string, material googleoidc.TransactionMaterial) error {
	s.state, s.material = state, material
	return nil
}
func (s *googleHTTPStore) Consume(context.Context, string) (googleoidc.TransactionMaterial, error) {
	return s.material, nil
}

type googleHTTPExchange struct{}

func (googleHTTPExchange) Exchange(context.Context, string, string) (googleoidc.ExchangeResult, error) {
	return googleoidc.ExchangeResult{}, errors.New("unused")
}

type googleHTTPVerifier struct{}

func (googleHTTPVerifier) Verify(context.Context, string, string) (googleoidc.Identity, error) {
	return googleoidc.Identity{}, errors.New("unused")
}

type googleHTTPAuthenticator struct{}

func (googleHTTPAuthenticator) AuthenticateVerified(context.Context, identityapp.VerifiedGoogleAuthenticationInput) (identityapp.GoogleIdentityResult, error) {
	return identityapp.GoogleIdentityResult{}, errors.New("unused")
}

type googleLoginHTTPFake struct {
	result identityapp.GoogleLoginResult
	err    error
	input  identityapp.GoogleCallbackInput
	calls  int
}

func (f *googleLoginHTTPFake) Login(_ context.Context, input identityapp.GoogleCallbackInput) (identityapp.GoogleLoginResult, error) {
	f.calls++
	f.input = input
	return f.result, f.err
}
func googleHandlersApp(t *testing.T, login *googleLoginHTTPFake) (*fiber.App, *googleHTTPStore) {
	t.Helper()
	entropy := make([]byte, googleoidc.TransactionSecretBytes*4)
	for index := range entropy {
		entropy[index] = byte(index + 1)
	}
	store := &googleHTTPStore{}
	flow, err := identityapp.NewGoogleOIDCFlowWithRandom(googleoidc.AuthorizationConfig{ClientID: "client.apps.googleusercontent.com", RedirectURI: "https://api.example.com/v1/auth/google/callback"}, store, googleHTTPExchange{}, googleHTTPVerifier{}, googleHTTPAuthenticator{}, bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	start, callback, err := httpapi.NewGoogleHTTPHandlers(flow, login, protection, httpapi.GoogleHTTPConfig{SuccessRedirect: "https://app.example.com/auth/complete", FailureRedirect: "https://app.example.com/login", TransactionTTL: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.GoogleStartPath, start.Handle)
	app.Get(httpapi.GoogleCallbackPath, callback.Handle)
	return app, store
}
func googleHTTPLoginResult(t *testing.T) identityapp.GoogleLoginResult {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 44})
	if err != nil {
		t.Fatal(err)
	}
	return identityapp.GoogleLoginResult{Identity: identityapp.GoogleIdentityResult{UserID: httpIdentityIDFromString(t, "019c0000-0000-4000-8000-000000000009"), AuthEpoch: 1}, Login: identityapp.LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: time.Now().Add(30 * time.Minute), AbsoluteExpiresAt: time.Now().Add(12 * time.Hour)}}
}
func TestGoogleStartHandlerSetsBoundCookieAndRedirects(t *testing.T) {
	login := &googleLoginHTTPFake{}
	app, store := googleHandlersApp(t, login)
	request := httptest.NewRequest(http.MethodPost, httpapi.GoogleStartPath, nil)
	request.Header.Set("Origin", "https://app.example.com")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || !strings.HasPrefix(response.Header.Get("Location"), "https://accounts.google.com/o/oauth2/v2/auth?") || store.state == "" || login.calls != 0 || response.Header.Get("Referrer-Policy") != "no-referrer" || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("Google start response drifted", response.StatusCode, response.Header)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatal("Google start cookie count drifted", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != httpapi.GoogleCorrelationCookie || cookie.Value == "" || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 300 || cookie.Domain != "" {
		t.Fatal("Google correlation cookie policy drifted", cookie)
	}
}
func TestGoogleStartHandlerRejectsCrossSiteAndNonEmptyRequests(t *testing.T) {
	for _, tc := range []struct {
		name, origin, target, body string
		status                     int
	}{{"origin", "https://evil.example", httpapi.GoogleStartPath, "", 403}, {"query", "https://app.example.com", httpapi.GoogleStartPath + "?x=1", "", 400}, {"body", "https://app.example.com", httpapi.GoogleStartPath, "x", 400}} {
		t.Run(tc.name, func(t *testing.T) {
			app, store := googleHandlersApp(t, &googleLoginHTTPFake{})
			request := httptest.NewRequest(http.MethodPost, tc.target, strings.NewReader(tc.body))
			request.Header.Set("Origin", tc.origin)
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status || store.state != "" || len(response.Cookies()) != 0 {
				t.Fatal("invalid Google start crossed use case", response.StatusCode)
			}
		})
	}
}
func TestGoogleCallbackSetsSessionCookiesAndClearsCorrelation(t *testing.T) {
	login := &googleLoginHTTPFake{result: googleHTTPLoginResult(t)}
	app, store := googleHandlersApp(t, login)
	start := httptest.NewRequest(http.MethodPost, httpapi.GoogleStartPath, nil)
	start.Header.Set("Origin", "https://app.example.com")
	started, err := app.Test(start)
	if err != nil {
		t.Fatal(err)
	}
	correlation := started.Cookies()[0]
	callback := httptest.NewRequest(http.MethodGet, httpapi.GoogleCallbackPath+"?state="+store.state+"&code=authorization-code", nil)
	callback.AddCookie(correlation)
	response, err := app.Test(callback)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "https://app.example.com/auth/complete" || login.calls != 1 || login.input.State != store.state || login.input.Code != "authorization-code" || login.input.BrowserBinding != correlation.Value || login.input.CorrelationID.IsZero() {
		t.Fatal("Google callback binding drifted", response.StatusCode, login.input)
	}
	cookies := map[string]*http.Cookie{}
	for _, cookie := range response.Cookies() {
		cookies[cookie.Name] = cookie
	}
	if len(cookies) != 3 || cookies[session.CookieName] == nil || cookies[session.CSRFCookieName] == nil || cookies[httpapi.GoogleCorrelationCookie] == nil || cookies[httpapi.GoogleCorrelationCookie].MaxAge != -1 || !cookies[session.CookieName].Secure || !cookies[session.CookieName].HttpOnly || cookies[session.CSRFCookieName].HttpOnly {
		t.Fatal("Google callback cookie policy drifted", response.Cookies())
	}
}
func TestGoogleCallbackFailuresAreGenericRedirectsAndClearCookie(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		cookie      bool
		loginErr    error
		wantCalls   int
	}{{"missing query", "", true, nil, 0}, {"provider error", "?error=access_denied&state=x", true, nil, 0}, {"duplicate", "?state=a&state=b&code=c", true, nil, 0}, {"missing cookie", "?state=a&code=c", false, nil, 0}, {"application", "?state=" + strings.Repeat("s", 43) + "&code=c", true, errors.New("secret-canary provider"), 1}} {
		t.Run(tc.name, func(t *testing.T) {
			login := &googleLoginHTTPFake{result: googleHTTPLoginResult(t), err: tc.loginErr}
			app, _ := googleHandlersApp(t, login)
			request := httptest.NewRequest(http.MethodGet, httpapi.GoogleCallbackPath+tc.query, nil)
			if tc.cookie {
				request.AddCookie(&http.Cookie{Name: httpapi.GoogleCorrelationCookie, Value: strings.Repeat("b", 43)})
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "https://app.example.com/login" || login.calls != tc.wantCalls || strings.Contains(response.Header.Get("Location"), "secret") || len(response.Cookies()) != 1 || response.Cookies()[0].Name != httpapi.GoogleCorrelationCookie || response.Cookies()[0].MaxAge != -1 {
				t.Fatal("Google callback failure leaked or drifted", response.StatusCode, response.Header, response.Cookies())
			}
		})
	}
}
