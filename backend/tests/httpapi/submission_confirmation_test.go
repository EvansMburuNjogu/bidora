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

type externalEvidenceV8HTTPFake struct {
	input opportunityapp.ReportExternalActionEvidenceInput
}

func (f *externalEvidenceV8HTTPFake) Report(_ context.Context, in opportunityapp.ReportExternalActionEvidenceInput) (opportunityapp.ExternalActionEvidenceResult, error) {
	f.input = in
	return opportunityapp.ExternalActionEvidenceResult{Evidence: opportunityapp.ExternalActionEvidence{ID: captureHTTPID(71), BidID: in.BidID, PackageManifestID: in.PackageManifestID, RecordedBy: in.Subject.IdentityID, ReportType: "SUBMISSION_REPORTED", VerificationStatus: "REPORTED_UNVERIFIED", ExternalSystemName: in.ExternalSystemName, ReportedActionTimeText: in.ReportedActionTimeText, ReportNote: in.ReportNote, ReportedActionAt: time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC), RecordedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*externalEvidenceV8HTTPFake) Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ExternalActionEvidence, error) {
	return opportunityapp.ExternalActionEvidence{}, nil
}
func (*externalEvidenceV8HTTPFake) List(context.Context, opportunityapp.ExternalActionEvidenceHistoryInput) (opportunityapp.ExternalActionEvidencePage, error) {
	return opportunityapp.ExternalActionEvidencePage{}, nil
}
func (*externalEvidenceV8HTTPFake) Reconciliation(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ExternalEvidenceReconciliation, error) {
	return opportunityapp.ExternalEvidenceReconciliation{}, nil
}

type submissionV8HTTPFake struct {
	input opportunityapp.ConfirmSubmissionInput
}

func (*submissionV8HTTPFake) Initiate(context.Context, opportunityapp.InitiateSubmissionProofInput) (opportunityapp.InitiateUploadResult, error) {
	return opportunityapp.InitiateUploadResult{}, nil
}
func (*submissionV8HTTPFake) Complete(context.Context, opportunityapp.CompleteSubmissionProofInput) (opportunityapp.SubmissionProofResult, error) {
	return opportunityapp.SubmissionProofResult{}, nil
}
func (f *submissionV8HTTPFake) Confirm(_ context.Context, in opportunityapp.ConfirmSubmissionInput) (opportunityapp.SubmissionConfirmationResult, error) {
	f.input = in
	return opportunityapp.SubmissionConfirmationResult{Confirmation: opportunityapp.SubmissionConfirmation{ID: captureHTTPID(72), BidID: in.BidID, EvidenceID: in.EvidenceID, ManifestID: in.ManifestID, ApprovalID: captureHTTPID(73), ProofVersionID: in.ProofVersionID, ReporterID: captureHTTPID(74), VerifierID: in.Subject.IdentityID, ReportedActionAt: time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC), ReportRecordedAt: time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC), ConfirmedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC), Rationale: in.Rationale, WorkspaceVersion: 5, SubmissionBasis: "HUMAN_CONFIRMED_EXTERNAL_ACTION"}}, nil
}
func (*submissionV8HTTPFake) Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.SubmissionConfirmation, error) {
	return opportunityapp.SubmissionConfirmation{}, nil
}

func TestExternalSubmissionHTTPSuccessPreservesHumanReportAndNonClaims(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 91})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(7), UserID: httpIdentityID(8), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: now.Add(-time.Minute)}}
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	reports := &externalEvidenceV8HTTPFake{}
	submissions := &submissionV8HTTPFake{}
	reportHandler, _ := httpapi.NewExternalActionEvidenceHandler(reports)
	submissionHandler, _ := httpapi.NewSubmissionHandler(submissions)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.ExternalActionEvidencePath, middleware.Handle, reportHandler.Report)
	app.Post(httpapi.SubmissionConfirmationPath, middleware.Handle, submissionHandler.Confirm)
	bid := "019c0000-0000-4000-8000-000000000050"
	evidence := "019c0000-0000-4000-8000-000000000071"
	manifest := "019c0000-0000-4000-8000-000000000063"
	proof := "019c0000-0000-4000-8000-000000000075"
	cases := []struct {
		name, path, body, fragment string
		status                     int
	}{{"report", "/v1/tenderpreneur/bids/" + bid + "/submission-evidence-reports", `{"package_manifest_id":"` + manifest + `","external_system_name":"Synthetic Portal","reported_action_time":"2026-10-23T12:00:00+03:00","report_note":"Human-reported external action.","expected_workspace_version":4}`, `"verification_status":"REPORTED_UNVERIFIED"`, 201}, {"confirm", "/v1/tenderpreneur/bids/" + bid + "/submission-confirmation", `{"evidence_id":"` + evidence + `","manifest_id":"` + manifest + `","proof_document_version_id":"` + proof + `","expected_workspace_version":4,"rationale":"Independent verification."}`, `"bidora_transmitted":false`, 201}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", "https://app.example.com")
			r.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-4000-8000-000000000076")
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
			if tc.name == "confirm" && (!strings.Contains(string(body), `"external_acceptance_confirmed":false`) || !strings.Contains(string(body), `"on_time_submission_confirmed":false`) || !strings.Contains(string(body), `"submitted":true`)) {
				t.Fatalf("submission non-claim contract drifted: %s", body)
			}
		})
	}
	if reports.input.ReportedActionTimeText != "2026-10-23T12:00:00+03:00" || submissions.input.AssuranceLevel != "AAL2" || submissions.input.AssuranceMethod != "TOTP" || !submissions.input.AssuranceAt.Equal(now.Add(-time.Minute)) {
		t.Fatal("external submission HTTP binding drifted")
	}
}
