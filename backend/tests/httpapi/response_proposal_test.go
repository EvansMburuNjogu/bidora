package httpapi_test

import (
	"context"
	"crypto/sha256"
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

type responseHTTPFake struct {
	input opportunityapp.SaveTechnicalResponseInput
}

func (f *responseHTTPFake) Save(_ context.Context, in opportunityapp.SaveTechnicalResponseInput) (opportunityapp.TechnicalResponseResult, error) {
	f.input = in
	return opportunityapp.TechnicalResponseResult{Response: opportunityapp.TechnicalResponse{SectionID: captureHTTPID(40), BidID: in.BidID, RequirementID: in.RequirementID, SectionVersion: 2, ResponseVersionID: captureHTTPID(41), Revision: 2, Content: in.Content, ChangeNote: in.ChangeNote, EvidenceIDs: in.EvidenceIDs, CreatedBy: in.Subject.IdentityID, CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}, WorkspaceVersion: 7}, nil
}
func (*responseHTTPFake) Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) (opportunityapp.TechnicalResponse, error) {
	return opportunityapp.TechnicalResponse{}, nil
}
func (*responseHTTPFake) History(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.TechnicalResponse, error) {
	return nil, nil
}
func (*responseHTTPFake) GapReview(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.GapReview, error) {
	return opportunityapp.GapReview{}, nil
}

type proposalHTTPFake struct {
	input opportunityapp.SaveProposalInput
}

func (f *proposalHTTPFake) Save(_ context.Context, in opportunityapp.SaveProposalInput) (opportunityapp.ProposalSaveResult, error) {
	f.input = in
	return opportunityapp.ProposalSaveResult{Proposal: opportunityapp.ProposalDocument{ProposalID: captureHTTPID(42), ProposalVersionID: captureHTTPID(43), BidID: in.BidID, Title: in.Title, Recipient: in.Recipient, Introduction: in.Introduction, Closing: in.Closing, ChangeNote: in.ChangeNote, ProposalVersion: 2, Revision: 2, WorkspaceVersion: 8, CreatedBy: in.Subject.IdentityID, CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*proposalHTTPFake) Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ProposalDocument, error) {
	return opportunityapp.ProposalDocument{}, nil
}
func (*proposalHTTPFake) History(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.ProposalHistoryItem, error) {
	return nil, nil
}

type responseDraftHTTPFake struct {
	input opportunityapp.RequestResponseDraftInput
}

func (f *responseDraftHTTPFake) Request(_ context.Context, in opportunityapp.RequestResponseDraftInput) (opportunityapp.ResponseDraftRequestResult, error) {
	f.input = in
	return opportunityapp.ResponseDraftRequestResult{RunID: captureHTTPID(44), JobID: captureHTTPID(45), State: "PENDING_RESERVATION"}, nil
}
func (*responseDraftHTTPFake) List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ResponseDraftSuggestion, error) {
	return nil, nil
}

type proposalApplicationHTTPFake struct {
	input opportunityapp.ApplyProposalSuggestionInput
}

func (f *proposalApplicationHTTPFake) Apply(_ context.Context, in opportunityapp.ApplyProposalSuggestionInput) (opportunityapp.ProposalSaveResult, error) {
	f.input = in
	return opportunityapp.ProposalSaveResult{Proposal: opportunityapp.ProposalDocument{ProposalID: captureHTTPID(42), ProposalVersionID: captureHTTPID(46), BidID: in.BidID, Introduction: in.Introduction, Closing: in.Closing, ProposalVersion: 3, Revision: 3, WorkspaceVersion: 9, CreatedBy: in.Subject.IdentityID, CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*proposalApplicationHTTPFake) Readiness(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ProposalApprovalReadiness, error) {
	return opportunityapp.ProposalApprovalReadiness{}, nil
}

type exportHTTPFake struct{ format string }

func (f *exportHTTPFake) Export(_ context.Context, _ opportunityapp.WorkflowSubject, _ opportunityapp.ID, _ opportunityapp.ID, format string) (opportunityapp.ProposalExport, error) {
	f.format = format
	b := []byte("synthetic-pdf")
	return opportunityapp.ProposalExport{Bytes: b, SHA256: sha256.Sum256(b), ContentType: "application/pdf", Extension: "pdf", Revision: 3}, nil
}

func TestResponseProposalHTTPSuccessContractsBindPrivateInputs(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 73})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	userID := httpIdentityID(5)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(4), UserID: userID, CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour), AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: now}}
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	responses := &responseHTTPFake{}
	proposals := &proposalHTTPFake{}
	drafts := &responseDraftHTTPFake{}
	applications := &proposalApplicationHTTPFake{}
	exports := &exportHTTPFake{}
	responseHandler, _ := httpapi.NewTechnicalResponseHandler(responses)
	proposalHandler, _ := httpapi.NewProposalHandler(proposals)
	draftHandler, _ := httpapi.NewAIResponseDraftHandler(drafts, true)
	applicationHandler, _ := httpapi.NewProposalApplicationHandler(applications)
	exportHandler, _ := httpapi.NewProposalExportHandler(exports)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.TechnicalResponsePath, middleware.Handle, responseHandler.Save)
	app.Post(httpapi.ProposalDocumentPath, middleware.Handle, proposalHandler.Save)
	app.Post(httpapi.AIResponseDraftPath, middleware.Handle, draftHandler.Request)
	app.Post(httpapi.AIProposalApplyPath, middleware.Handle, applicationHandler.Apply)
	app.Get(httpapi.ProposalExportPath, middleware.Handle, exportHandler.Download)
	bid := "019c0000-0000-4000-8000-000000000050"
	reqID := "019c0000-0000-4000-8000-000000000051"
	evidence := "019c0000-0000-4000-8000-000000000052"
	suggestion := "019c0000-0000-4000-8000-000000000053"
	version := "019c0000-0000-4000-8000-000000000054"
	key := "019c0000-0000-4000-8000-000000000055"
	cases := []struct {
		name, method, path, body, fragment string
		status                             int
	}{
		{"response", http.MethodPost, "/v1/tenderpreneur/bids/" + bid + "/requirements/" + reqID + "/response", `{"content":"Grounded response.","change_note":"Human revision.","evidence_ids":["` + evidence + `"],"expected_section_version":1,"expected_workspace_version":6}`, `"workspace_version":7`, 200},
		{"proposal", http.MethodPost, "/v1/tenderpreneur/bids/" + bid + "/proposal", `{"title":"Technical Proposal","recipient":"Evaluation Committee","letterhead_lines":[],"introduction":"Introduction.","closing":"Closing.","change_note":"Human revision.","expected_proposal_version":1,"expected_workspace_version":7}`, `"proposal_version":2`, 200},
		{"response draft", http.MethodPost, "/v1/tenderpreneur/bids/" + bid + "/requirements/" + reqID + "/ai-response-drafts", `{"expected_workspace_version":7,"expected_section_version":2}`, `"state":"PENDING_RESERVATION"`, 202},
		{"apply suggestion", http.MethodPost, "/v1/tenderpreneur/bids/" + bid + "/proposal/ai-writing-suggestions/" + suggestion + "/apply", `{"introduction":"Reviewed introduction.","closing":"Reviewed closing.","review_note":"Accepted after review.","expected_suggestion_version":1,"expected_proposal_version":2,"expected_workspace_version":8}`, `"proposal_version":3`, 200},
		{"export", http.MethodGet, "/v1/tenderpreneur/bids/" + bid + "/proposal/versions/" + version + "/export/pdf", "", "synthetic-pdf", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.method != http.MethodGet {
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("Origin", "https://app.example.com")
				r.Header.Set(httpapi.IdempotencyKeyHeader, key)
				r.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
				r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			}
			r.Header.Set(httpapi.TenantIDHeader, "019c0000-0000-4000-8000-000000000056")
			r.Header.Set(httpapi.ContextGenerationHeader, "9")
			r.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
			resp, e := app.Test(r)
			if e != nil {
				t.Fatal(e)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != tc.status || resp.Header.Get("Cache-Control") != "private, no-store" || !strings.Contains(string(body), tc.fragment) {
				t.Fatalf("HTTP contract drifted: status=%d cache=%q body=%s", resp.StatusCode, resp.Header.Get("Cache-Control"), body)
			}
		})
	}
	if responses.input.ExpectedSectionVersion != 1 || len(responses.input.EvidenceIDs) != 1 || proposals.input.ExpectedProposalVersion != 1 || drafts.input.ExpectedSectionVersion != 2 || applications.input.ExpectedSuggestionVersion != 1 || exports.format != "pdf" {
		t.Fatal("response/proposal HTTP binding drifted")
	}
}
