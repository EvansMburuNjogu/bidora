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

type outcomeV9HTTPFake struct {
	input opportunityapp.ReportOutcomeInput
}

func (*outcomeV9HTTPFake) InitiateProof(context.Context, opportunityapp.InitiateOutcomeProofInput) (opportunityapp.InitiateUploadResult, error) {
	return opportunityapp.InitiateUploadResult{}, nil
}
func (*outcomeV9HTTPFake) CompleteProof(context.Context, opportunityapp.CompleteOutcomeProofInput) (opportunityapp.OutcomeProofResult, error) {
	return opportunityapp.OutcomeProofResult{}, nil
}
func (f *outcomeV9HTTPFake) Report(_ context.Context, in opportunityapp.ReportOutcomeInput) (opportunityapp.ReportOutcomeResult, error) {
	f.input = in
	return opportunityapp.ReportOutcomeResult{Outcome: opportunityapp.ReportedOutcome{ID: captureHTTPID(81), BidID: in.BidID, ProofVersionID: in.ProofVersionID, SupersedesID: in.SupersedesID, ReportedBy: in.Subject.IdentityID, Sequence: in.ExpectedSequence + 1, Outcome: in.Outcome, VerificationStatus: "REPORTED_UNVERIFIED", Provenance: "HUMAN_REPORTED_EXTERNAL_OUTCOME", SourceClass: in.SourceClass, Reason: in.Reason, Lessons: in.Lessons, ReportedTimeText: in.ReportedTimeText, CorrectionReason: in.CorrectionReason, ReportedAt: time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC), RecordedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*outcomeV9HTTPFake) Current(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ReportedOutcome, error) {
	return opportunityapp.ReportedOutcome{}, nil
}
func (*outcomeV9HTTPFake) History(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.ReportedOutcome, error) {
	return []opportunityapp.ReportedOutcome{}, nil
}

type archivePreparationV9HTTPFake struct{}

func (*archivePreparationV9HTTPFake) Get(_ context.Context, _ opportunityapp.WorkflowSubject, bid opportunityapp.ID) (opportunityapp.BidArchivePreparation, error) {
	return opportunityapp.BidArchivePreparation{BidID: bid, WorkspaceState: "NO_BID", WorkspaceVersion: 3, ArchivePolicyConfigured: true, ReadOnlyClosureSupported: true, CanArchive: true, Blockers: []string{}, ObservedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}, nil
}

type archiveV9HTTPFake struct {
	input opportunityapp.ArchiveBidInput
}

func (f *archiveV9HTTPFake) Archive(_ context.Context, in opportunityapp.ArchiveBidInput) (opportunityapp.BidArchiveResult, error) {
	f.input = in
	return opportunityapp.BidArchiveResult{Archive: opportunityapp.BidArchive{ID: captureHTTPID(82), BidID: in.BidID, ArchivedBy: in.Subject.IdentityID, WorkspaceVersion: in.ExpectedWorkspaceVersion + 1, Reason: in.Reason, PolicyVersion: 1, ArchivedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}}, nil
}
func (*archiveV9HTTPFake) Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.BidArchive, error) {
	return opportunityapp.BidArchive{}, nil
}

func TestReportedOutcomeAndArchiveHTTPSuccessRemainUnverifiedAndNonDestructive(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 101})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(7), UserID: httpIdentityID(8), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(12 * time.Hour)}}
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	outcomes := &outcomeV9HTTPFake{}
	preparations := &archivePreparationV9HTTPFake{}
	archives := &archiveV9HTTPFake{}
	outcomeHandler, _ := httpapi.NewReportedOutcomeHandler(outcomes)
	preparationHandler, _ := httpapi.NewArchivePreparationHandler(preparations)
	archiveHandler, _ := httpapi.NewBidArchiveHandler(archives)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.ReportedOutcomesPath, middleware.Handle, outcomeHandler.Report)
	app.Get(httpapi.ArchivePreparationPath, middleware.Handle, preparationHandler.Get)
	app.Post(httpapi.BidArchivePath, middleware.Handle, archiveHandler.Archive)
	bid := "019c0000-0000-4000-8000-000000000050"
	proof := "019c0000-0000-4000-8000-000000000075"
	cases := []struct {
		name, method, path, body, fragment string
		status                             int
	}{{"outcome", http.MethodPost, "/v1/tenderpreneur/bids/" + bid + "/outcomes", `{"outcome":"LOST","source_class":"BUYER_NOTICE","reason":"Human-reported result.","lessons":"Retain evidence.","reported_outcome_at":"2026-10-23T12:00:00+03:00","proof_document_version_id":"` + proof + `","expected_sequence":0,"expected_workspace_version":4}`, `"authoritative_buyer_decision":false`, 201}, {"preparation", http.MethodGet, "/v1/tenderpreneur/bids/" + bid + "/archive-preparation", "", `"can_archive":true`, 200}, {"archive", http.MethodPost, "/v1/tenderpreneur/bids/" + bid + "/archive", `{"reason":"Close as read-only history.","expected_workspace_version":3}`, `"read_only":true`, 201}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}
			r.Header.Set("Origin", "https://app.example.com")
			r.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-4000-8000-000000000086")
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
			if tc.name == "outcome" && (!strings.Contains(string(body), `"contract_award_confirmed":false`) || !strings.Contains(string(body), `"withdrawal_executed":false`) || !strings.Contains(string(body), `"analytics_accepted":false`)) {
				t.Fatalf("outcome non-claim contract drifted: %s", body)
			}
			if tc.name == "archive" && (!strings.Contains(string(body), `"deleted":false`) || !strings.Contains(string(body), `"legal_hold_changed":false`) || !strings.Contains(string(body), `"restoration_supported":false`)) {
				t.Fatalf("archive non-destructive contract drifted: %s", body)
			}
		})
	}
	if outcomes.input.Outcome != "LOST" || outcomes.input.ReportedTimeText != "2026-10-23T12:00:00+03:00" || archives.input.ExpectedWorkspaceVersion != 3 {
		t.Fatal("outcome/archive HTTP binding drifted")
	}
}
