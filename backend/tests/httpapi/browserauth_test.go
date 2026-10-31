package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type sessionValidatorFake struct {
	principal identityapp.SessionPrincipal
	err       error
	raw       string
	calls     int
}

func (v *sessionValidatorFake) Validate(_ context.Context, raw string) (identityapp.SessionPrincipal, error) {
	v.calls++
	v.raw = raw
	return v.principal, v.err
}

type sessionToucherFake struct {
	err       error
	sessionID identityapp.ID
	at        time.Time
	calls     int
}

func (t *sessionToucherFake) TouchSession(_ context.Context, id identityapp.ID, at time.Time) error {
	t.calls++
	t.sessionID = id
	t.at = at
	return t.err
}

type browserClockFake struct{ at time.Time }

func (c browserClockFake) Now() time.Time  { return c.at }
func httpIdentityID(n byte) identityapp.ID { var value identityapp.ID; value[15] = n; return value }
func browserHarness(t *testing.T, method string, validator *sessionValidatorFake, toucher *sessionToucherFake) (*fiber.App, string, string, time.Time) {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 9})
	if err != nil {
		t.Fatal(err)
	}
	rawSession, rawCSRF := pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()
	validator.principal = identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: time.Now().Add(time.Minute), AbsoluteExpiresAt: time.Now().Add(time.Hour)}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, toucher, protection, browserClockFake{at})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Add([]string{method}, "/protected", middleware.Handle, func(c fiber.Ctx) error {
		principal, ok := httpapi.SessionPrincipalFrom(c)
		if !ok {
			return errors.New("principal missing")
		}
		return c.JSON(map[string]string{"user": string(rune(principal.UserID[15]))})
	})
	return app, rawSession, rawCSRF, at
}

type repeatingReader struct{ value byte }

func (r *repeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.value
		r.value++
	}
	return len(p), nil
}
func requestWithBrowser(method, sessionRaw, csrfRaw string) *http.Request {
	req := httptest.NewRequest(method, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionRaw})
	if csrfRaw != "" {
		req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfRaw})
		req.Header.Set(session.CSRFHeader, csrfRaw)
	}
	req.Header.Set("Origin", "https://app.example.com")
	return req
}

func TestBrowserSessionMiddlewareValidatesPolicyBeforeTouchAndNext(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		t.Run(method, func(t *testing.T) {
			validator := &sessionValidatorFake{}
			toucher := &sessionToucherFake{}
			app, rawSession, rawCSRF, at := browserHarness(t, method, validator, toucher)
			res, err := app.Test(requestWithBrowser(method, rawSession, rawCSRF))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != 200 || validator.calls != 1 || validator.raw != rawSession || toucher.calls != 1 || toucher.sessionID != httpIdentityID(1) || !toucher.at.Equal(at) {
				t.Fatal("browser middleware ordering drifted", res.StatusCode)
			}
		})
	}
}
func TestBrowserSessionMiddlewareRejectsBeforeTouch(t *testing.T) {
	cases := []struct {
		name         string
		method       string
		validatorErr error
		mutate       func(*http.Request)
	}{{"missing session", "GET", errors.New("session_invalid"), nil}, {"session dependency", "GET", errors.New("session_unavailable"), nil}, {"origin", "POST", nil, func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }}, {"header", "POST", nil, func(r *http.Request) { r.Header.Del(session.CSRFHeader) }}, {"cookie", "POST", nil, func(r *http.Request) {
		r.Header.Del("Cookie")
		r.AddCookie(&http.Cookie{Name: session.CookieName, Value: "placeholder"})
	}}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator := &sessionValidatorFake{err: tc.validatorErr}
			toucher := &sessionToucherFake{}
			app, rawSession, rawCSRF, _ := browserHarness(t, tc.method, validator, toucher)
			if tc.validatorErr != nil {
				validator.err = tc.validatorErr
			}
			req := requestWithBrowser(tc.method, rawSession, rawCSRF)
			if tc.mutate != nil {
				tc.mutate(req)
			}
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode == 200 || toucher.calls != 0 {
				t.Fatal("rejected request touched session", res.StatusCode, toucher.calls)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err = json.NewDecoder(res.Body).Decode(&body); err != nil || body.Error.Code == "" {
				t.Fatal("unsafe error envelope", err)
			}
		})
	}
}
func TestBrowserSessionMiddlewareTouchFailureStopsHandler(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{{"expired race", identityapp.ErrSessionNotFound, 401}, {"database", errors.New("secret-canary database"), 503}} {
		t.Run(tc.name, func(t *testing.T) {
			validator := &sessionValidatorFake{}
			toucher := &sessionToucherFake{err: tc.err}
			app, rawSession, rawCSRF, _ := browserHarness(t, "POST", validator, toucher)
			res, err := app.Test(requestWithBrowser("POST", rawSession, rawCSRF))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status || toucher.calls != 1 {
				t.Fatal("touch failure crossed handler", res.StatusCode)
			}
		})
	}
}
func TestBrowserSessionMiddlewareConstructionAndPrincipalLookupFailClosed(t *testing.T) {
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	validator := &sessionValidatorFake{}
	toucher := &sessionToucherFake{}
	clock := browserClockFake{time.Now()}
	for _, tc := range []struct {
		name string
		v    httpapi.SessionValidator
		t    httpapi.SessionToucher
		p    *browser.Protection
		c    httpapi.BrowserClock
	}{{"validator", nil, toucher, protection, clock}, {"toucher", validator, nil, protection, clock}, {"protection", validator, toucher, nil, clock}, {"clock", validator, toucher, protection, nil}} {
		if middleware, err := httpapi.NewBrowserSessionMiddleware(tc.v, tc.t, tc.p, tc.c); err == nil || middleware != nil {
			t.Fatal("invalid middleware dependency accepted", tc.name)
		}
	}
	if principal, ok := httpapi.SessionPrincipalFrom(nil); ok || principal != (identityapp.SessionPrincipal{}) {
		t.Fatal("nil context exposed principal")
	}
}
