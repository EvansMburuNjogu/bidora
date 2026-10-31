package httpapi_test

import (
	"context"
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
	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
)

type workflowHTTPFake struct {
	err             error
	calls           int
	verifyResult    opportunityapp.VerificationResult
	workspaceResult opportunityapp.WorkspaceResult
	decisionResult  opportunityapp.DecisionResult
	verifyInput     opportunityapp.VerifyOpportunityInput
	workspaceInput  opportunityapp.CreateWorkspaceInput
	decisionInput   opportunityapp.RecordDecisionInput
}

func (f *workflowHTTPFake) Verify(_ context.Context, input opportunityapp.VerifyOpportunityInput) (opportunityapp.VerificationResult, error) {
	f.calls++
	f.verifyInput = input
	return f.verifyResult, f.err
}
func (f *workflowHTTPFake) CreateWorkspace(_ context.Context, input opportunityapp.CreateWorkspaceInput) (opportunityapp.WorkspaceResult, error) {
	f.calls++
	f.workspaceInput = input
	return f.workspaceResult, f.err
}
func (f *workflowHTTPFake) RecordDecision(_ context.Context, input opportunityapp.RecordDecisionInput) (opportunityapp.DecisionResult, error) {
	f.calls++
	f.decisionInput = input
	return f.decisionResult, f.err
}

func TestOpportunityWorkflowSuccessContractsBindAuthoritativeInputs(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 52})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	userID := httpIdentityID(2)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: userID, CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour), AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: now}}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	opportunityID := "019c0000-0000-4000-8000-000000000010"
	sourceID := "019c0000-0000-4000-8000-000000000011"
	ownerID := "019c0000-0000-4000-8000-000000000012"
	workspaceID := "019c0000-0000-4000-8000-000000000013"
	tenantID := "019c0000-0000-4000-8000-000000000015"
	key := "019c0000-0000-4000-8000-000000000016"
	workflow := &workflowHTTPFake{
		verifyResult:    opportunityapp.VerificationResult{OpportunityID: captureHTTPID(10), SourceVersionID: captureHTTPID(11), Version: 2},
		workspaceResult: opportunityapp.WorkspaceResult{WorkspaceID: captureHTTPID(13), Version: 1},
		decisionResult:  opportunityapp.DecisionResult{DecisionID: captureHTTPID(14), WorkspaceID: captureHTTPID(13), Version: 2, Decision: "BID"},
	}
	handler, err := httpapi.NewOpportunityWorkflowHandler(workflow)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.OpportunityVerificationPath, middleware.Handle, handler.Verify)
	app.Post(httpapi.WorkspaceCreationPath, middleware.Handle, handler.CreateWorkspace)
	app.Post(httpapi.BidDecisionPath, middleware.Handle, handler.RecordDecision)

	cases := []struct {
		name, path, body, responseFragment string
		assertInput                        func()
	}{
		{"verify", "/v1/tenderpreneur/opportunities/" + opportunityID + "/verify-facts", `{"source_version_id":"` + sourceID + `","expected_version":1}`, `"version":2`, func() {
			if workflow.verifyInput.Subject.IdentityID != opportunityapp.ID(userID) || workflow.verifyInput.Subject.ContextGeneration != 6 || workflow.verifyInput.IdempotencyKey != key || workflow.verifyInput.ExpectedVersion != 1 {
				t.Fatal("verification HTTP input drifted")
			}
		}},
		{"workspace", "/v1/tenderpreneur/opportunities/" + opportunityID + "/create-workspace", `{"source_version_id":"` + sourceID + `","owner_id":"` + ownerID + `","expected_version":2}`, `"version":1`, func() {
			if workflow.workspaceInput.Subject.IdentityID != opportunityapp.ID(userID) || workflow.workspaceInput.Subject.ContextGeneration != 6 || workflow.workspaceInput.IdempotencyKey != key || workflow.workspaceInput.ExpectedVersion != 2 {
				t.Fatal("workspace HTTP input drifted")
			}
		}},
		{"decision", "/v1/tenderpreneur/bids/" + workspaceID + "/decision", `{"decision":"BID","rationale":"Human synthetic rationale.","expected_version":1}`, `"decision":"BID"`, func() {
			if workflow.decisionInput.Subject.IdentityID != opportunityapp.ID(userID) || workflow.decisionInput.Subject.ContextGeneration != 6 || workflow.decisionInput.IdempotencyKey != key || workflow.decisionInput.Rationale != "Human synthetic rationale." {
				t.Fatal("decision HTTP input drifted")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://app.example.com")
			request.Header.Set(httpapi.TenantIDHeader, tenantID)
			request.Header.Set(httpapi.ContextGenerationHeader, "6")
			request.Header.Set(httpapi.IdempotencyKeyHeader, key)
			request.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
			request.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
			request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			response, testErr := app.Test(request)
			if testErr != nil {
				t.Fatal(testErr)
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != http.StatusCreated || response.Header.Get("Cache-Control") != "private, no-store" || !strings.Contains(string(body), tc.responseFragment) {
				t.Fatal("workflow success response drifted", response.StatusCode, string(body))
			}
			tc.assertInput()
		})
	}
}

func TestOpportunityWorkflowStaleVersionAndDiagnosticsUseSafeErrors(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 51})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour), AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: now}}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, cause, code string
		status            int
	}{{"stale version", "opportunity_verification_conflict", "WORKFLOW_CONFLICT", 409}, {"database diagnostic", "secret-canary sql detail", "WORKFLOW_UNAVAILABLE", 503}} {
		t.Run(tc.name, func(t *testing.T) {
			workflow := &workflowHTTPFake{err: errors.New(tc.cause)}
			handler, err := httpapi.NewOpportunityWorkflowHandler(workflow)
			if err != nil {
				t.Fatal(err)
			}
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
				return c.Next()
			})
			app.Post(httpapi.OpportunityVerificationPath, middleware.Handle, handler.Verify)
			request := httptest.NewRequest(http.MethodPost, "/v1/tenderpreneur/opportunities/019c0000-0000-4000-8000-000000000010/verify-facts", strings.NewReader(`{"source_version_id":"019c0000-0000-4000-8000-000000000011","expected_version":1}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://app.example.com")
			request.Header.Set(httpapi.TenantIDHeader, "019c0000-0000-4000-8000-000000000012")
			request.Header.Set(httpapi.ContextGenerationHeader, "1")
			request.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-4000-8000-000000000013")
			request.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
			request.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
			request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != tc.status || workflow.calls != 1 || !strings.Contains(string(body), tc.code) || strings.Contains(string(body), "secret-canary") {
				t.Fatal("workflow error mapping was unsafe", response.StatusCode, workflow.calls, string(body))
			}
		})
	}
}
