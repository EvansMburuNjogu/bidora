package httpapi_test

import (
	"context"
	"encoding/json"
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
	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type manualCaptureExecutorFake struct {
	result opportunityapp.CaptureResult
	err    error
	input  opportunityapp.ExecuteCaptureInput
	calls  int
}

func (e *manualCaptureExecutorFake) Execute(_ context.Context, input opportunityapp.ExecuteCaptureInput) (opportunityapp.CaptureResult, error) {
	e.calls++
	e.input = input
	return e.result, e.err
}
func captureHTTPID(n byte) opportunityapp.ID {
	var value opportunityapp.ID
	value[15] = n
	return value
}
func manualCaptureAppAuthenticated(t *testing.T, executor *manualCaptureExecutorFake) *fiber.App {
	t.Helper()
	handler, err := httpapi.NewManualCaptureHandler(executor)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 3})
	if err != nil {
		t.Fatal(err)
	}
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: time.Now().Add(time.Hour), AbsoluteExpiresAt: time.Now().Add(2 * time.Hour)}}
	toucher := &sessionToucherFake{}
	protection, err := browserProtectionForCapture()
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, toucher, protection, browserClockFake{time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{BodyLimit: 33 * 1024})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post("/v1/tenderpreneur/opportunities/manual", middleware.Handle, handler.Handle)
	return app
}
func browserProtectionForCapture() (*browser.Protection, error) {
	return browser.NewProtection([]string{"https://app.example.com"})
}
func captureRequest(body string) *http.Request {
	req := httptest.NewRequest("POST", "/v1/tenderpreneur/opportunities/manual", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpapi.TenantIDHeader, "019c0000-0000-7000-8000-000000000002")
	req.Header.Set(httpapi.ContextGenerationHeader, "4")
	req.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-7000-8000-000000000003")
	req.Header.Set("Origin", "https://app.example.com")
	req.AddCookie(&http.Cookie{Name: "__Host-bidora_session", Value: "session-for-fake"})
	return req
}
func validCaptureRequest(t *testing.T, body string) (*http.Request, string) {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 3})
	if err != nil {
		t.Fatal(err)
	}
	req := captureRequest(body)
	req.Header.Del("Cookie")
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
	req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
	req.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
	return req, pair.CSRF.RevealForBootstrap()
}

func TestManualCaptureHandlerBindsApprovedRequestAndResponse(t *testing.T) {
	created := time.Date(2026, 10, 18, 12, 0, 0, 0, time.FixedZone("source", 3600))
	executor := &manualCaptureExecutorFake{result: opportunityapp.CaptureResult{OpportunityID: captureHTTPID(9), SourceVersionID: captureHTTPID(10), Version: 1, CreatedAt: created}}
	app := manualCaptureAppAuthenticated(t, executor)
	req, _ := validCaptureRequest(t, `{"title":"Synthetic","buyer_name":null}`)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 || res.Header.Get("Cache-Control") != "private, no-store" || executor.calls != 1 || executor.input.Subject.IdentityID != captureHTTPID(2) || uuid.UUID(executor.input.Subject.SelectedTenantID).String() != "019c0000-0000-7000-8000-000000000002" || executor.input.Subject.ContextGeneration != 4 || executor.input.IdempotencyKey != "019c0000-0000-7000-8000-000000000003" {
		t.Fatal("capture HTTP binding drifted", res.StatusCode, executor.input)
	}
	var body struct {
		Data struct {
			ID                string `json:"id"`
			SourceVersionID   string `json:"source_version_id"`
			VerificationState string `json:"verification_state"`
			CreatedAt         string `json:"created_at"`
			Version           int64  `json:"version"`
		}
		Meta struct {
			TraceID string `json:"trace_id"`
		}
	}
	if err = json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Data.ID != "00000000-0000-0000-0000-000000000009" || body.Data.SourceVersionID != "00000000-0000-0000-0000-00000000000a" || body.Data.VerificationState != "UNVERIFIED" || body.Data.Version != 1 || body.Meta.TraceID != res.Header.Get("X-Request-ID") {
		t.Fatal("capture response drifted", body)
	}
}
func TestManualCaptureHandlerRejectsBindingBeforeUseCase(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*http.Request)
	}{{"query", func(r *http.Request) { r.URL.RawQuery = "private=secret" }}, {"media type", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }}, {"tenant", func(r *http.Request) { r.Header.Set(httpapi.TenantIDHeader, "bad") }}, {"generation", func(r *http.Request) { r.Header.Set(httpapi.ContextGenerationHeader, "0") }}, {"idempotency", func(r *http.Request) { r.Header.Set(httpapi.IdempotencyKeyHeader, "bad") }}, {"duplicate tenant", func(r *http.Request) { r.Header.Add(httpapi.TenantIDHeader, "019c0000-0000-7000-8000-000000000004") }}, {"unknown body", func(r *http.Request) { r.Body = http.NoBody; r.ContentLength = 0 }}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executor := &manualCaptureExecutorFake{}
			app := manualCaptureAppAuthenticated(t, executor)
			req, _ := validCaptureRequest(t, `{"title":"Synthetic","buyer_name":null}`)
			tc.mutate(req)
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode < 400 || executor.calls != 0 {
				t.Fatal("invalid HTTP input crossed use case", res.StatusCode, executor.calls)
			}
		})
	}
}
func TestManualCaptureHandlerMapsDomainValidationFieldSafely(t *testing.T) {
	executor := &manualCaptureExecutorFake{}
	app := manualCaptureAppAuthenticated(t, executor)
	req, _ := validCaptureRequest(t, `{"title":"","buyer_name":null}`)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct {
		Error struct {
			Code        string `json:"code"`
			FieldErrors []struct {
				Path string `json:"path"`
				Code string `json:"code"`
			} `json:"field_errors"`
		} `json:"error"`
	}
	if err = json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 422 || executor.calls != 0 || body.Error.Code != "VALIDATION_FAILED" || len(body.Error.FieldErrors) != 1 || body.Error.FieldErrors[0].Path != "title" {
		t.Fatal("domain validation mapping drifted", res.StatusCode, body)
	}
}

func TestManualCaptureHandlerMapsOnlyStableSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
	}{{"capture_not_authorized", 403}, {"capture_source_unavailable", 404}, {"capture_idempotency_conflict", 409}, {"capture_operation_in_progress", 409}, {"capture_persistence_failed", 503}, {"secret-canary sql detail", 503}} {
		t.Run(tc.code, func(t *testing.T) {
			executor := &manualCaptureExecutorFake{err: errors.New(tc.code)}
			app := manualCaptureAppAuthenticated(t, executor)
			req, _ := validCaptureRequest(t, `{"title":"Synthetic","buyer_name":null}`)
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status {
				t.Fatal("wrong safe status", res.StatusCode)
			}
			var body map[string]any
			if err = json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(body)
			if strings.Contains(string(encoded), "secret-canary") {
				t.Fatal("diagnostic leaked", string(encoded))
			}
		})
	}
}
func TestManualCaptureHandlerRejectsInconsistentUseCaseResult(t *testing.T) {
	executor := &manualCaptureExecutorFake{}
	app := manualCaptureAppAuthenticated(t, executor)
	req, _ := validCaptureRequest(t, `{"title":"Synthetic","buyer_name":null}`)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 503 || executor.calls != 1 {
		t.Fatal("inconsistent result exposed", res.StatusCode)
	}
}

func TestManualCaptureHandlerRequiresMiddlewareAndDependency(t *testing.T) {
	executor := &manualCaptureExecutorFake{}
	handler, _ := httpapi.NewManualCaptureHandler(executor)
	app := fiber.New()
	app.Post("/", handler.Handle)
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"title":"Synthetic","buyer_name":null}`))
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 || executor.calls != 0 {
		t.Fatal("capture handler bypassed middleware")
	}
	if handler, err := httpapi.NewManualCaptureHandler(nil); err == nil || handler != nil {
		t.Fatal("nil executor accepted")
	}
}
