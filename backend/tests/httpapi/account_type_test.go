package httpapi_test

import (
	"bytes"
	"context"
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

type accountTypeHTTPFake struct {
	selectResult identityapp.AccountTypeSelectionResult
	selectErr    error
	selectInput  identityapp.SelectAccountTypeInput
	selectCalls  int
}

func (*accountTypeHTTPFake) Resolve(context.Context, identityapp.SessionPrincipal) (identityapp.PortalResolution, error) {
	return identityapp.PortalResolution{}, nil
}
func (f *accountTypeHTTPFake) Select(_ context.Context, input identityapp.SelectAccountTypeInput) (identityapp.AccountTypeSelectionResult, error) {
	f.selectCalls++
	f.selectInput = input
	return f.selectResult, f.selectErr
}

func accountTypeHTTPHarness(t *testing.T, service *accountTypeHTTPFake) (*fiber.App, string, string) {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 63})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	principal := identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: now}
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	middleware, err := httpapi.NewBrowserSessionMiddleware(&sessionValidatorFake{principal: principal}, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := httpapi.NewAccountTypeHandler(service, protection)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{BodyLimit: 4096})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.AccountTypeSelectionPath, middleware.Handle, handler.Select)
	return app, pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()
}

func accountTypeRequest(body, sessionRaw, csrfRaw string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, httpapi.AccountTypeSelectionPath, bytes.NewBufferString(body))
	request.Header.Set("Origin", "https://app.example.com")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-4000-8000-000000000009")
	request.Header.Set(session.CSRFHeader, csrfRaw)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionRaw})
	request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfRaw})
	return request
}

func TestAccountTypeSelectionHTTPBindsAuthenticatedCSRFAndIdempotency(t *testing.T) {
	service := &accountTypeHTTPFake{selectResult: identityapp.AccountTypeSelectionResult{Replay: true}}
	app, sessionRaw, csrfRaw := accountTypeHTTPHarness(t, service)
	response, err := app.Test(accountTypeRequest(`{"account_type":"TENDERPRENEUR"}`, sessionRaw, csrfRaw))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Idempotency-Replayed") != "true" || service.selectCalls != 1 || service.selectInput.AccountType != "TENDERPRENEUR" || service.selectInput.IdempotencyKey != "019c0000-0000-4000-8000-000000000009" || service.selectInput.Principal.UserID != httpIdentityID(2) || service.selectInput.CorrelationID.IsZero() {
		t.Fatal("account type HTTP composition drifted", response.StatusCode)
	}
}

func TestAccountTypeSelectionHTTPRejectsUnsafeRequestsAndMapsConflicts(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		mutate     func(*http.Request)
		err        error
		status     int
	}{
		{name: "missing field", body: `{}`, err: errors.New("account_type_selection_invalid"), status: 400},
		{name: "unknown field", body: `{"account_type":"TENDERPRENEUR","role":"admin"}`, status: 400},
		{name: "duplicate field", body: `{"account_type":"TENDERPRENEUR","account_type":"PROCUREMENT"}`, status: 400},
		{name: "platform forbidden", body: `{"account_type":"PLATFORM_ADMIN"}`, err: errors.New("account_type_selection_invalid"), status: 400},
		{name: "missing idempotency", body: `{"account_type":"TENDERPRENEUR"}`, mutate: func(r *http.Request) { r.Header.Del(httpapi.IdempotencyKeyHeader) }, status: 400},
		{name: "csrf", body: `{"account_type":"TENDERPRENEUR"}`, mutate: func(r *http.Request) { r.Header.Set(session.CSRFHeader, "wrong") }, status: 403},
		{name: "already selected", body: `{"account_type":"TENDERPRENEUR"}`, err: errors.New("account_type_already_selected"), status: 409},
		{name: "idempotency conflict", body: `{"account_type":"TENDERPRENEUR"}`, err: errors.New("account_type_selection_conflict"), status: 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &accountTypeHTTPFake{selectErr: tc.err}
			app, sessionRaw, csrfRaw := accountTypeHTTPHarness(t, service)
			request := accountTypeRequest(tc.body, sessionRaw, csrfRaw)
			if tc.mutate != nil {
				tc.mutate(request)
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatal("account type unsafe request mapping drifted", response.StatusCode)
			}
		})
	}
	if handler, err := httpapi.NewAccountTypeHandler(nil, nil); err == nil || handler != nil {
		t.Fatal("nil account type dependencies accepted")
	}
}
