package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	app "bidora/backend/internal/platformsettings/application"
	"github.com/gofiber/fiber/v3"
)

type settingsHTTPFake struct {
	information app.Information
	saves       int
	saved       app.InformationSave
}

func (f *settingsHTTPFake) Read(context.Context, app.Subject) (app.Information, error) {
	return f.information, nil
}
func (f *settingsHTTPFake) Shared(context.Context) (app.Information, error) {
	return f.information, nil
}
func (f *settingsHTTPFake) Save(_ context.Context, in app.InformationSave) (app.Information, error) {
	f.saves++
	f.saved = in
	return f.information, nil
}

type policyHTTPFake struct{ saves int }

func (f *policyHTTPFake) Read(context.Context, app.Subject) (app.SessionPolicy, error) {
	return app.SessionPolicy{IdleSeconds: 300, AbsoluteSeconds: 3600, Version: 2}, nil
}
func (f *policyHTTPFake) Save(_ context.Context, in app.SessionPolicySave) (app.SessionPolicy, error) {
	f.saves++
	return in.Policy, nil
}
func TestPlatformSettingsHTTPBindsIdentityAndRejectsAmbiguousWrites(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 19})
	if err != nil {
		t.Fatal(err)
	}
	validator := &sessionValidatorFake{}
	validator.principal.SessionID = httpIdentityID(1)
	validator.principal.UserID = httpIdentityID(2)
	validator.principal.AuthEpoch = 1
	validator.principal.CSRFHash = pair.CSRF.Hash()
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	server := fiber.New()
	server.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	info := &settingsHTTPFake{information: app.Information{PlatformName: "Bidora", Version: 2}}
	handler, _ := httpapi.NewPlatformInformationHandler(info)
	if err = httpapi.RegisterPlatformInformationRoutes(server, middleware, handler); err != nil {
		t.Fatal(err)
	}
	policy := &policyHTTPFake{}
	policyHandler, _ := httpapi.NewPlatformSessionPolicyHandler(policy)
	if err = httpapi.RegisterPlatformSessionPolicyRoutes(server, middleware, policyHandler); err != nil {
		t.Fatal(err)
	}
	send := func(path, method, body string, csrf bool, context bool) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", "https://app.example.com")
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
		if context {
			req.Header.Set("X-Bidora-Platform-Context-Generation", "7")
		}
		if csrf {
			req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			req.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
		}
		req.Header.Set("If-Match", "\"2\"")
		req.Header.Set("Idempotency-Key", "019c0000-0000-4000-8000-000000000002")
		res, e := server.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if status := send("/v1/shared/platform/information", "GET", "", false, false); status != 200 {
		t.Fatal("shared identity required Platform context", status)
	}
	valid := `{"platform_name":"Bidora Pilot","contact_phone":"+254 700 000 001","support_email":"support@example.test","website":"https://example.test"}`
	if status := send("/v1/platform/information", "PUT", valid, true, true); status != 200 || info.saves != 1 || info.saved.PlatformName == nil || *info.saved.PlatformName != "Bidora Pilot" || info.saved.Subject.ContextGeneration != 7 {
		t.Fatal("identity update did not bind", status)
	}
	for _, body := range []string{`{"support_email":"a@example.test","website":"https://example.test","platform_name":null}`, `{"support_email":"a@example.test","website":"https://example.test","website":"https://other.test"}`, `{"support_email":"a@example.test","website":"https://example.test","api_key":"secret"}`} {
		if status := send("/v1/platform/information", "PUT", body, true, true); status != 400 || info.saves != 1 {
			t.Fatal("ambiguous identity write accepted", status)
		}
	}
	if status := send("/v1/platform/security/session-policy", "PUT", `{"idle_seconds":300,"absolute_seconds":3600}`, false, true); status != 403 || policy.saves != 0 {
		t.Fatal("policy bypassed CSRF", status)
	}
	if status := send("/v1/platform/security/session-policy", "PUT", `{"idle_seconds":300,"idle_seconds":600,"absolute_seconds":3600}`, true, true); status != 400 || policy.saves != 0 {
		t.Fatal("ambiguous policy accepted", status)
	}
	if status := send("/v1/platform/security/session-policy", "PUT", `{"idle_seconds":300,"absolute_seconds":3600}`, true, true); status != 200 || policy.saves != 1 {
		t.Fatal("valid policy not bound", status)
	}
}
