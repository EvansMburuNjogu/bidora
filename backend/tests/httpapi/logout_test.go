package httpapi_test

import (
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
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type logoutUseCaseFake struct {
	err         error
	principal   identityapp.SessionPrincipal
	correlation identityapp.ID
	calls       int
}

func (l *logoutUseCaseFake) Logout(_ context.Context, principal identityapp.SessionPrincipal, correlation identityapp.ID) error {
	l.calls++
	l.principal = principal
	l.correlation = correlation
	return l.err
}
func logoutApp(t *testing.T, usecase *logoutUseCaseFake) (*fiber.App, session.Pair) {
	t.Helper()
	pair := loginPair(t)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: time.Now().Add(time.Hour), AbsoluteExpiresAt: time.Now().Add(2 * time.Hour)}}
	toucher := &sessionToucherFake{}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, toucher, protection, browserClockFake{time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := httpapi.NewLogoutHandler(usecase)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post("/v1/auth/logout", middleware.Handle, handler.Handle)
	return app, pair
}
func logoutRequest(pair session.Pair) *http.Request {
	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
	req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
	return req
}
func TestLogoutHandlerRevokesCurrentSessionThenClearsCookies(t *testing.T) {
	usecase := &logoutUseCaseFake{}
	app, pair := logoutApp(t, usecase)
	res, err := app.Test(logoutRequest(pair))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 204 || usecase.calls != 1 || usecase.principal.SessionID != httpIdentityID(1) || usecase.principal.UserID != httpIdentityID(2) || usecase.correlation != httpIdentityIDFromString(t, "019c0000-0000-4000-8000-000000000001") {
		t.Fatal("logout binding drifted", res.StatusCode)
	}
	cookies := res.Cookies()
	if len(cookies) != 2 {
		t.Fatal("logout clear cookie count drifted", len(cookies))
	}
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
	}
	if byName[session.CookieName] == nil || byName[session.CSRFCookieName] == nil || byName[session.CookieName].MaxAge != -1 || byName[session.CSRFCookieName].MaxAge != -1 || !byName[session.CookieName].HttpOnly || byName[session.CSRFCookieName].HttpOnly {
		t.Fatal("logout cookie clearing drifted", cookies)
	}
}
func TestLogoutHandlerDoesNotClearCookiesOnFailure(t *testing.T) {
	usecase := &logoutUseCaseFake{err: errors.New("secret-canary database")}
	app, pair := logoutApp(t, usecase)
	res, err := app.Test(logoutRequest(pair))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 503 || len(res.Cookies()) != 0 {
		t.Fatal("failed logout cleared cookies", res.StatusCode, res.Cookies())
	}
}
func TestLogoutHandlerRejectsBodyQueryAndMissingMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*http.Request)
	}{{"body", func(r *http.Request) { replaceLoginBody(r, "secret") }}, {"query", func(r *http.Request) { r.URL.RawQuery = "secret=value" }}} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &logoutUseCaseFake{}
			app, pair := logoutApp(t, usecase)
			req := logoutRequest(pair)
			tc.mutate(req)
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != 400 || usecase.calls != 0 {
				t.Fatal("invalid logout crossed use case", res.StatusCode)
			}
		})
	}
	usecase := &logoutUseCaseFake{}
	handler, _ := httpapi.NewLogoutHandler(usecase)
	app := fiber.New()
	app.Post("/", handler.Handle)
	res, err := app.Test(httptest.NewRequest("POST", "/", strings.NewReader("")))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 || usecase.calls != 0 {
		t.Fatal("logout bypassed authentication")
	}
	if handler, err := httpapi.NewLogoutHandler(nil); err == nil || handler != nil {
		t.Fatal("nil logout use case accepted")
	}
}
