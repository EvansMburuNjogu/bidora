package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

type mfaReadinessFake struct {
	result identityapp.MFAReadiness
	err    error
	calls  int
}

func (f *mfaReadinessFake) Get(context.Context, identityapp.SessionPrincipal) (identityapp.MFAReadiness, error) {
	f.calls++
	return f.result, f.err
}

type recoveryPreparationFake struct {
	result identityapp.RecoveryPreparation
	err    error
	calls  int
}

func (f *recoveryPreparationFake) Get(context.Context, identityapp.SessionPrincipal) (identityapp.RecoveryPreparation, error) {
	f.calls++
	return f.result, f.err
}

func preparationHTTPHarness(t *testing.T, mfaService *mfaReadinessFake, recoveryService *recoveryPreparationFake) (*fiber.App, string, string) {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 31})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 3, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: now}}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	mfaHandler, err := httpapi.NewMFAReadinessHandler(mfaService, protection)
	if err != nil {
		t.Fatal(err)
	}
	recoveryHandler, err := httpapi.NewRecoveryPreparationHandler(recoveryService, protection)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Get(httpapi.MFAReadinessPath, middleware.Handle, mfaHandler.Get)
	app.Get(httpapi.RecoveryPreparationPath, middleware.Handle, recoveryHandler.Get)
	return app, pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()
}

func preparationRequest(path, sessionRaw, csrfRaw, origin string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Origin", origin)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionRaw})
	if csrfRaw != "" {
		request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfRaw})
	}
	return request
}

func TestPreparationEndpointsEnforceOriginAndSessionBoundCSRFCookie(t *testing.T) {
	for _, path := range []string{httpapi.MFAReadinessPath, httpapi.RecoveryPreparationPath} {
		for _, tc := range []struct {
			name, origin, csrf string
		}{
			{name: "origin", origin: "https://evil.example", csrf: "valid"},
			{name: "csrf_missing", origin: "https://app.example.com"},
			{name: "csrf_mismatch", origin: "https://app.example.com", csrf: "mismatch"},
		} {
			t.Run(path+"_"+tc.name, func(t *testing.T) {
				mfaService, recoveryService := &mfaReadinessFake{}, &recoveryPreparationFake{}
				app, sessionRaw, csrfRaw := preparationHTTPHarness(t, mfaService, recoveryService)
				csrf := tc.csrf
				if csrf == "valid" {
					csrf = csrfRaw
				}
				response, err := app.Test(preparationRequest(path, sessionRaw, csrf, tc.origin))
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != http.StatusForbidden || mfaService.calls != 0 || recoveryService.calls != 0 {
					t.Fatal("invalid browser binding crossed preparation boundary", response.StatusCode)
				}
			})
		}
	}
}

func TestPreparationResponsesExposeOnlyApprovedCurrentState(t *testing.T) {
	now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	mfaService := &mfaReadinessFake{result: identityapp.MFAReadiness{Facts: identityapp.RecoveryAccountFacts{EmailVerified: true, PasswordCredentialPresent: true, GoogleIdentityPresent: true}, MFAEnrolled: true, StepUpAvailable: true, StepUpPolicyConfigured: true, SessionAssuranceProvenanceAvailable: true, RecoveryCodesAvailable: true, SessionAuthenticatedAt: now.Add(-time.Hour), SessionAgeSeconds: 3600, Blockers: []string{}, ObservedAt: now}}
	recoveryService := &recoveryPreparationFake{result: identityapp.RecoveryPreparation{Facts: identityapp.RecoveryAccountFacts{EmailVerified: true, PasswordCredentialPresent: true, GoogleIdentityPresent: true}, RecoveryRequestAvailable: true, PasswordResetAvailable: true, TokenPolicyConfigured: true, DeliveryPolicyConfigured: true, SessionEffectPolicyConfigured: true, Blockers: []string{}, ObservedAt: now}}
	app, sessionRaw, csrfRaw := preparationHTTPHarness(t, mfaService, recoveryService)
	for _, path := range []string{httpapi.MFAReadinessPath, httpapi.RecoveryPreparationPath} {
		response, err := app.Test(preparationRequest(path, sessionRaw, csrfRaw, "https://app.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("preparation response failed", response.StatusCode, readErr)
		}
		for _, forbidden := range []string{`"user_id"`, `"session_id"`, `"email"`, `"factor_id"`, `"secret"`, `"recovery_codes"`, `"token"`} {
			if strings.Contains(string(body), forbidden) {
				t.Fatal("preparation response exposed sensitive field", forbidden, string(body))
			}
		}
		var envelope struct {
			Data map[string]json.RawMessage `json:"data"`
			Meta map[string]json.RawMessage `json:"meta"`
		}
		if err = json.Unmarshal(body, &envelope); err != nil || len(envelope.Data) == 0 || len(envelope.Meta) != 1 {
			t.Fatal("preparation response envelope drifted", err)
		}
	}
}

func TestPreparationFailuresDoNotLeakDiagnostics(t *testing.T) {
	mfaService := &mfaReadinessFake{err: errors.New("database secret diagnostic")}
	recoveryService := &recoveryPreparationFake{err: errors.New("provider credential diagnostic")}
	app, sessionRaw, csrfRaw := preparationHTTPHarness(t, mfaService, recoveryService)
	for _, path := range []string{httpapi.MFAReadinessPath, httpapi.RecoveryPreparationPath} {
		response, err := app.Test(preparationRequest(path, sessionRaw, csrfRaw, "https://app.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusServiceUnavailable || strings.Contains(string(body), "diagnostic") || strings.Contains(string(body), "credential") {
			t.Fatal("preparation failure leaked diagnostic", response.StatusCode, string(body), readErr)
		}
	}
}
