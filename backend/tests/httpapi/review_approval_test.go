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
)

type reviewV7HTTPFake struct {
	create opportunityapp.CreateReviewThreadInput
}

func (f *reviewV7HTTPFake) Create(_ context.Context, in opportunityapp.CreateReviewThreadInput) (opportunityapp.ReviewThreadResult, error) {
	f.create = in
	return opportunityapp.ReviewThreadResult{Thread: opportunityapp.ReviewThread{ID: captureHTTPID(61), BidID: in.BidID, TargetID: in.TargetID, TargetType: in.TargetType, Subject: in.ThreadSubject, Status: "OPEN", Version: 1, CreatedBy: in.Subject.IdentityID, CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*reviewV7HTTPFake) Comment(context.Context, opportunityapp.AddReviewCommentInput) (opportunityapp.ReviewThreadResult, error) {
	return opportunityapp.ReviewThreadResult{}, nil
}
func (*reviewV7HTTPFake) Resolve(context.Context, opportunityapp.ResolveReviewThreadInput) (opportunityapp.ReviewThreadResult, error) {
	return opportunityapp.ReviewThreadResult{}, nil
}
func (*reviewV7HTTPFake) List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.ReviewThread, error) {
	return nil, nil
}
func (*reviewV7HTTPFake) Assign(context.Context, opportunityapp.AssignReviewThreadInput) (opportunityapp.ReviewThreadResult, error) {
	return opportunityapp.ReviewThreadResult{}, nil
}
func (*reviewV7HTTPFake) AssignmentHistory(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ReviewAssignment, error) {
	return nil, nil
}
func (*reviewV7HTTPFake) Dashboard(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ReviewDashboard, error) {
	return opportunityapp.ReviewDashboard{}, nil
}
func (*reviewV7HTTPFake) Workloads(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.TenantReviewWorkloads, error) {
	return opportunityapp.TenantReviewWorkloads{}, nil
}
func (*reviewV7HTTPFake) ReviewerWork(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.ReviewerOpenWork, error) {
	return nil, nil
}

type internalReviewV7HTTPFake struct {
	input opportunityapp.RecordInternalReviewDecisionInput
}

func (f *internalReviewV7HTTPFake) Record(_ context.Context, in opportunityapp.RecordInternalReviewDecisionInput) (opportunityapp.InternalReviewDecisionResult, error) {
	f.input = in
	return opportunityapp.InternalReviewDecisionResult{Decision: opportunityapp.InternalReviewDecision{ID: captureHTTPID(62), BidID: in.BidID, ProposalVersionID: in.ProposalVersionID, ReviewerID: in.Subject.IdentityID, ReviewerSequence: 1, Decision: in.Decision, Rationale: in.Rationale, DecidedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*internalReviewV7HTTPFake) History(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.InternalReviewDecision, error) {
	return nil, nil
}
func (*internalReviewV7HTTPFake) Preparation(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ApprovalPolicyPreparation, error) {
	return opportunityapp.ApprovalPolicyPreparation{}, nil
}

type packageV7HTTPFake struct {
	input opportunityapp.PrepareProposalPackageInput
}

func (f *packageV7HTTPFake) Prepare(_ context.Context, in opportunityapp.PrepareProposalPackageInput) (opportunityapp.ProposalPackageResult, error) {
	f.input = in
	return opportunityapp.ProposalPackageResult{Manifest: opportunityapp.ProposalPackageManifest{ID: captureHTTPID(63), BidID: in.BidID, ProposalID: captureHTTPID(64), ProposalVersionID: in.ProposalVersionID, ProposalRevision: 2, Status: "UNAPPROVED_PREPARATION_ONLY", PDFRenderProfile: "BIDORA_PDF_V1", DOCXRenderProfile: "BIDORA_DOCX_V1", PDFSize: 100, DOCXSize: 200, ReadinessObservedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC), ReadinessReady: true, ReadinessBlockers: []string{}, ManifestVersion: 2, CreatedBy: in.Subject.IdentityID, CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*packageV7HTTPFake) Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalPackageManifest, error) {
	return opportunityapp.ProposalPackageManifest{}, nil
}
func (*packageV7HTTPFake) List(context.Context, opportunityapp.ProposalPackageHistoryInput) (opportunityapp.ProposalPackageHistoryPage, error) {
	return opportunityapp.ProposalPackageHistoryPage{}, nil
}

type approvalV7HTTPFake struct {
	input opportunityapp.ApproveProposalInput
}

func (f *approvalV7HTTPFake) Approve(_ context.Context, in opportunityapp.ApproveProposalInput) (opportunityapp.ProposalApprovalResult, error) {
	f.input = in
	return opportunityapp.ProposalApprovalResult{Approval: opportunityapp.ProposalApproval{ID: captureHTTPID(65), BidID: in.BidID, ProposalID: captureHTTPID(64), ProposalVersionID: captureHTTPID(54), ManifestID: in.ManifestID, ReviewDecisionID: captureHTTPID(62), ApproverID: in.Subject.IdentityID, PolicyVersion: 1, Rationale: in.Rationale, ApprovedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*approvalV7HTTPFake) Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalApproval, error) {
	return opportunityapp.ProposalApproval{}, nil
}

func TestReviewApprovalHTTPSuccessContractsBindAAL2AndPrivateResponses(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 81})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	userID := httpIdentityID(8)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(7), UserID: userID, CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: now.Add(-time.Minute)}}
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	reviews := &reviewV7HTTPFake{}
	internal := &internalReviewV7HTTPFake{}
	packages := &packageV7HTTPFake{}
	approvals := &approvalV7HTTPFake{}
	reviewHandler, _ := httpapi.NewReviewThreadHandler(reviews)
	internalHandler, _ := httpapi.NewInternalReviewDecisionHandler(internal)
	packageHandler, _ := httpapi.NewProposalPackageHandler(packages)
	approvalHandler, _ := httpapi.NewProposalApprovalHandler(approvals)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.ReviewThreadListPath, middleware.Handle, reviewHandler.Create)
	app.Post(httpapi.InternalReviewDecisionPath, middleware.Handle, internalHandler.Record)
	app.Post(httpapi.ProposalPackagePreparationPath, middleware.Handle, packageHandler.Prepare)
	app.Post(httpapi.ProposalApprovalPath, middleware.Handle, approvalHandler.Approve)
	bid := "019c0000-0000-4000-8000-000000000050"
	version := "019c0000-0000-4000-8000-000000000054"
	manifest := "019c0000-0000-4000-8000-000000000063"
	key := "019c0000-0000-4000-8000-000000000066"
	cases := []struct {
		name, path, body, fragment string
		status                     int
	}{
		{"review", "/v1/tenderpreneur/bids/" + bid + "/review-threads", `{"target_type":"PROPOSAL_VERSION","target_id":"` + version + `","subject":"Exact revision review","comment":"Review this immutable revision."}`, `"status":"OPEN"`, 200},
		{"decision", "/v1/tenderpreneur/bids/" + bid + "/proposal/versions/" + version + "/internal-review-decisions", `{"decision":"REVIEW_COMPLETE","rationale":"Reviewed exact revision.","expected_previous_sequence":0}`, `"decision":"REVIEW_COMPLETE"`, 200},
		{"package", "/v1/tenderpreneur/bids/" + bid + "/proposal/package-preparations", `{"proposal_version_id":"` + version + `","expected_proposal_version":2,"expected_workspace_version":4}`, `"manifest_version":2`, 201},
		{"approval", "/v1/tenderpreneur/bids/" + bid + "/proposal/package-preparations/" + manifest + "/approval", `{"expected_proposal_version":2,"expected_workspace_version":4,"rationale":"Independent approval."}`, `"signed":false`, 201},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", "https://app.example.com")
			r.Header.Set(httpapi.IdempotencyKeyHeader, key)
			r.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
			r.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
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
	if reviews.create.TargetType != "PROPOSAL_VERSION" || internal.input.ExpectedPreviousSequence != 0 || packages.input.ExpectedProposalVersion != 2 || approvals.input.AssuranceLevel != "AAL2" || approvals.input.AssuranceMethod != "TOTP" || !approvals.input.AssuranceAt.Equal(now.Add(-time.Minute)) || !approvals.input.SessionAuthenticatedAt.Equal(now) {
		t.Fatal("review/approval HTTP binding drifted")
	}
}
