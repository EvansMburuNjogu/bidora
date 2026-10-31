package httpapi_test

import (
	"context"
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
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type passwordChangeHTTPFake struct {
	result identityapp.LoginResult
	calls  int
}

func (f *passwordChangeHTTPFake) Change(_ context.Context, _ identityapp.PasswordChangeInput) (identityapp.LoginResult, error) {
	f.calls++
	return f.result, nil
}
func passwordChangeHTTPHarness(t *testing.T) (*fiber.App, *passwordChangeHTTPFake, string, string) {
	t.Helper()
	pair, e := session.GeneratePairWithRandom(&repeatingReader{value: 31})
	if e != nil {
		t.Fatal(e)
	}
	replacement, e := session.GeneratePairWithRandom(&repeatingReader{value: 71})
	if e != nil {
		t.Fatal(e)
	}
	protection, e := browser.NewProtection([]string{"https://app.example.test"})
	if e != nil {
		t.Fatal(e)
	}
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: time.Now().Add(time.Minute), AbsoluteExpiresAt: time.Now().Add(time.Hour), AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: time.Now()}}
	middleware, e := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: time.Now()})
	if e != nil {
		t.Fatal(e)
	}
	use := &passwordChangeHTTPFake{result: identityapp.LoginResult{Session: replacement.Session, CSRF: replacement.CSRF, AbsoluteExpiresAt: time.Now().Add(time.Hour)}}
	var key attempt.Key
	key[0] = 1
	handler, e := httpapi.NewPasswordChangeHandler(use, &loginIPFake{address: netip.MustParseAddr("198.51.100.2")}, &loginKeyerFake{key: key})
	if e != nil {
		t.Fatal(e)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.PasswordChangePath, middleware.Handle, handler.Handle)
	return app, use, pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()
}
func TestPasswordChangeHTTPStrictBindingAndReplacementCookies(t *testing.T) {
	app, use, rawSession, rawCSRF := passwordChangeHTTPHarness(t)
	send := func(body string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, httpapi.PasswordChangePath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://app.example.test")
		req.Header.Set(session.CSRFHeader, rawCSRF)
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: rawSession})
		req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: rawCSRF})
		res, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		return res
	}
	for _, body := range []string{`{"email":"u@example.test","current_password":"current password","new_password":"replacement password","unknown":true}`, `{"email":"u@example.test","email":"v@example.test","current_password":"current password","new_password":"replacement password"}`, `{"email":"u@example.test","current_password":"current password","new_password":"replacement password"}{}`} {
		res := send(body)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatal("invalid password change accepted", res.StatusCode)
		}
	}
	if use.calls != 0 {
		t.Fatal("invalid password change crossed boundary")
	}
	res := send(`{"email":"u@example.test","current_password":"current password","new_password":"replacement password"}`)
	if res.StatusCode != http.StatusNoContent || len(res.Cookies()) != 2 {
		t.Fatal("password change replacement response drifted", res.StatusCode, len(res.Cookies()))
	}
}
