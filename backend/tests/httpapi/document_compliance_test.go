package httpapi_test

import (
	"context"
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
	"github.com/google/uuid"
)

type documentHTTPFake struct {
	initiateInput opportunityapp.InitiateUploadInput
	result        opportunityapp.InitiateUploadResult
}

func (f *documentHTTPFake) Initiate(_ context.Context, in opportunityapp.InitiateUploadInput) (opportunityapp.InitiateUploadResult, error) {
	f.initiateInput = in
	return f.result, nil
}
func (*documentHTTPFake) Complete(context.Context, opportunityapp.CompleteUploadInput) (opportunityapp.UploadResult, error) {
	return opportunityapp.UploadResult{}, nil
}
func (*documentHTTPFake) Job(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.DocumentJob, error) {
	return opportunityapp.DocumentJob{}, nil
}

type requirementHTTPFake struct {
	createInput    opportunityapp.CreateRequirementInput
	readinessBidID opportunityapp.ID
	requirement    opportunityapp.Requirement
}

func (f *requirementHTTPFake) Create(_ context.Context, in opportunityapp.CreateRequirementInput) (opportunityapp.RequirementResult, error) {
	f.createInput = in
	return opportunityapp.RequirementResult{Requirement: f.requirement, WorkspaceVersion: 3}, nil
}
func (*requirementHTTPFake) Assign(context.Context, opportunityapp.AssignRequirementInput) (opportunityapp.RequirementResult, error) {
	return opportunityapp.RequirementResult{}, nil
}
func (*requirementHTTPFake) List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.Requirement, error) {
	return nil, nil
}
func (f *requirementHTTPFake) Readiness(_ context.Context, _ opportunityapp.WorkflowSubject, bidID opportunityapp.ID) (opportunityapp.Readiness, error) {
	f.readinessBidID = bidID
	return opportunityapp.Readiness{BidID: bidID, WorkspaceState: "BID", WorkspaceVersion: 3, OpenMandatoryCount: 1, Blockers: []string{"MANDATORY_REQUIREMENTS_OPEN"}}, nil
}

type evidenceHTTPFake struct {
	satisfyInput opportunityapp.SatisfyRequirementInput
	result       opportunityapp.SatisfyRequirementResult
}

func (*evidenceHTTPFake) InitiateUpload(context.Context, opportunityapp.InitiateEvidenceUploadInput) (opportunityapp.InitiateUploadResult, error) {
	return opportunityapp.InitiateUploadResult{}, nil
}
func (*evidenceHTTPFake) CompleteUpload(context.Context, opportunityapp.CompleteEvidenceUploadInput) (opportunityapp.EvidenceUploadResult, error) {
	return opportunityapp.EvidenceUploadResult{}, nil
}
func (f *evidenceHTTPFake) Satisfy(_ context.Context, in opportunityapp.SatisfyRequirementInput) (opportunityapp.SatisfyRequirementResult, error) {
	f.satisfyInput = in
	return f.result, nil
}
func (*evidenceHTTPFake) List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.RequirementEvidence, error) {
	return nil, nil
}

func TestDocumentComplianceHTTPSuccessContractsBindPrivateInputs(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 61})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
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
	bidID := "019c0000-0000-4000-8000-000000000020"
	requirementID := "019c0000-0000-4000-8000-000000000021"
	sourceID := "019c0000-0000-4000-8000-000000000022"
	documentVersionID := "019c0000-0000-4000-8000-000000000023"
	opportunityID := "019c0000-0000-4000-8000-000000000024"
	tenantID := "019c0000-0000-4000-8000-000000000025"
	key := "019c0000-0000-4000-8000-000000000026"
	requirement := opportunityapp.Requirement{ID: captureHTTPID(21), BidID: captureHTTPID(20), SourceVersionID: captureHTTPID(22), SourceLocation: "Section 4.2", Text: "Provide evidence.", Category: "COMPLIANCE", Status: "OPEN", Mandatory: true, Version: 1, CreatedAt: now, UpdatedAt: now}
	docFake := &documentHTTPFake{result: opportunityapp.InitiateUploadResult{Session: opportunityapp.UploadSession{ID: captureHTTPID(30), DocumentID: captureHTTPID(31), State: "INITIATED", ExpiresAt: now.Add(10 * time.Minute)}, Instruction: opportunityapp.UploadInstruction{Method: "POST", URL: "https://storage.example.invalid/upload", Fields: map[string]string{"checksum": "synthetic"}}}}
	reqFake := &requirementHTTPFake{requirement: requirement}
	evidenceFake := &evidenceHTTPFake{result: opportunityapp.SatisfyRequirementResult{Requirement: requirement, Evidence: opportunityapp.RequirementEvidence{ID: captureHTTPID(32), RequirementID: captureHTTPID(21), BidID: captureHTTPID(20), DocumentVersionID: captureHTTPID(23), AddedBy: opportunityapp.ID(userID), EvidenceLocation: "Page 1", Justification: "Exact clean evidence.", AddedAt: now}, WorkspaceVersion: 4}}
	docHandler, _ := httpapi.NewDocumentUploadHandler(docFake)
	reqHandler, _ := httpapi.NewRequirementHandler(reqFake)
	evidenceHandler, _ := httpapi.NewRequirementEvidenceHandler(evidenceFake)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.DocumentUploadInitiatePath, middleware.Handle, docHandler.Initiate)
	app.Post(httpapi.RequirementListPath, middleware.Handle, reqHandler.Create)
	app.Get(httpapi.ReadinessPath, middleware.Handle, reqHandler.Readiness)
	app.Post(httpapi.RequirementSatisfyPath, middleware.Handle, evidenceHandler.Satisfy)

	cases := []struct {
		name, method, path, body, fragment string
		status                             int
		assert                             func(*testing.T)
	}{
		{"document upload", http.MethodPost, "/v1/tenderpreneur/opportunities/" + opportunityID + "/documents/uploads", `{"title":"Tender.pdf","media_type":"application/pdf","size_bytes":12,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, `"state":"INITIATED"`, 201, func(t *testing.T) {
			if docFake.initiateInput.Subject.IdentityID != opportunityapp.ID(userID) || docFake.initiateInput.SizeBytes != 12 || docFake.initiateInput.MediaType != opportunityapp.PDFMediaType || docFake.initiateInput.IdempotencyKey != key {
				t.Fatal("document HTTP binding drifted")
			}
		}},
		{"requirement create", http.MethodPost, "/v1/tenderpreneur/bids/" + bidID + "/requirements", `{"source_version_id":"` + sourceID + `","source_location":"Section 4.2","text":"Provide evidence.","category":"COMPLIANCE","mandatory":true,"expected_workspace_version":2}`, `"workspace_version":3`, 201, func(t *testing.T) {
			if reqFake.createInput.ExpectedWorkspaceVersion != 2 || reqFake.createInput.SourceVersionID != opportunityapp.ID(uuid.MustParse(sourceID)) || !reqFake.createInput.Mandatory {
				t.Fatal("requirement HTTP binding drifted")
			}
		}},
		{"readiness", http.MethodGet, "/v1/tenderpreneur/bids/" + bidID + "/readiness", "", `"ready":false`, 200, func(t *testing.T) {
			if reqFake.readinessBidID != opportunityapp.ID(uuid.MustParse(bidID)) {
				t.Fatal("readiness HTTP binding drifted")
			}
		}},
		{"satisfy", http.MethodPost, "/v1/tenderpreneur/bids/" + bidID + "/requirements/" + requirementID + "/satisfy", `{"document_version_id":"` + documentVersionID + `","evidence_location":"Page 1","justification":"Exact clean evidence.","expected_requirement_version":1,"expected_workspace_version":3}`, `"workspace_version":4`, 200, func(t *testing.T) {
			if evidenceFake.satisfyInput.DocumentVersionID != opportunityapp.ID(uuid.MustParse(documentVersionID)) || evidenceFake.satisfyInput.ExpectedRequirementVersion != 1 || evidenceFake.satisfyInput.ExpectedWorkspaceVersion != 3 {
				t.Fatal("evidence HTTP binding drifted")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.method != http.MethodGet {
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Origin", "https://app.example.com")
				request.Header.Set(httpapi.IdempotencyKeyHeader, key)
				request.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
				request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			}
			request.Header.Set(httpapi.TenantIDHeader, tenantID)
			request.Header.Set(httpapi.ContextGenerationHeader, "9")
			request.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
			response, testErr := app.Test(request)
			if testErr != nil {
				t.Fatal(testErr)
			}
			payload, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != tc.status || response.Header.Get("Cache-Control") != "private, no-store" || !strings.Contains(string(payload), tc.fragment) {
				t.Fatalf("HTTP contract drifted: status=%d cache=%q body=%s", response.StatusCode, response.Header.Get("Cache-Control"), payload)
			}
			tc.assert(t)
		})
	}
}
