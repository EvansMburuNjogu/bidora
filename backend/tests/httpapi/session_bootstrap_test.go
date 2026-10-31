package httpapi_test

import (
	"encoding/json"
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

func bootstrapHarness(t *testing.T) (*fiber.App, string, string, *sessionToucherFake) {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 21})
	if err != nil {
		t.Fatal(err)
	}
	rawSession, rawCSRF := pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: time.Now().Add(time.Minute), AbsoluteExpiresAt: time.Now().Add(time.Hour)}}
	toucher := &sessionToucherFake{}
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, toucher, protection, browserClockFake{at: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := httpapi.NewSessionBootstrapHandler(protection)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Get(httpapi.SessionBootstrapPath, middleware.Bootstrap, handler.Handle)
	return app, rawSession, rawCSRF, toucher
}
func bootstrapRequest(sessionRaw, csrfRaw, origin, target string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Origin", origin)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionRaw})
	if csrfRaw != "" {
		request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfRaw})
	}
	return request
}
func TestSessionBootstrapReturnsOnlyValidatedCSRFValue(t *testing.T) {
	app, rawSession, rawCSRF, toucher := bootstrapHarness(t)
	response, err := app.Test(bootstrapRequest(rawSession, rawCSRF, "https://app.example.com", httpapi.SessionBootstrapPath))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || toucher.calls != 1 {
		t.Fatal("bootstrap failed", response.StatusCode, toucher.calls)
	}
	var body struct {
		Data struct {
			CSRFToken string `json:"csrf_token"`
		} `json:"data"`
		Meta struct {
			TraceID string `json:"trace_id"`
		} `json:"meta"`
	}
	if err = json.NewDecoder(response.Body).Decode(&body); err != nil || body.Data.CSRFToken != rawCSRF || body.Meta.TraceID == "" {
		t.Fatal("bootstrap response drifted", err)
	}
}
func TestSessionBootstrapRejectsOriginCookieAndAmbiguousRequestBeforeTouch(t *testing.T) {
	for _, tc := range []struct{ name, csrf, origin, target string }{{"origin", "valid", "https://evil.example", httpapi.SessionBootstrapPath}, {"cookie", "", "https://app.example.com", httpapi.SessionBootstrapPath}, {"query", "valid", "https://app.example.com", httpapi.SessionBootstrapPath + "?x=1"}} {
		t.Run(tc.name, func(t *testing.T) {
			app, rawSession, rawCSRF, toucher := bootstrapHarness(t)
			csrf := rawCSRF
			if tc.csrf == "" {
				csrf = ""
			}
			response, err := app.Test(bootstrapRequest(rawSession, csrf, tc.origin, tc.target))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode == 200 || toucher.calls != 0 {
				t.Fatal("unsafe bootstrap accepted or touched session", response.StatusCode, toucher.calls)
			}
		})
	}
	if handler, err := httpapi.NewSessionBootstrapHandler(nil); err == nil || handler != nil {
		t.Fatal("nil browser protection accepted")
	}
}
