package integration_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	opportunitypersistence "bidora/backend/internal/opportunity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func TestLiveHumanAcceptsCitedAISuggestionWithoutMutatingProposal(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenant := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := insertWorkflowAuthority(t, control, livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN").Database)
	store := attachWorkflowServices(t, &fixture, tenant, at.Add(time.Minute))
	captured, workspace := establishBidWorkspace(t, tenant, &fixture, at)
	storage := newSyntheticDocumentStorage()
	uploads, err := opportunityapp.NewDocumentUploadService(fixture.authorizer, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("%PDF-1.7 synthetic cited tender source")
	sum := sha256.Sum256(body)
	initiated, err := uploads.Initiate(context.Background(), opportunityapp.InitiateUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, CorrelationID: liveID(), Title: "Cited tender.pdf", MediaType: opportunityapp.PDFMediaType, SizeBytes: int64(len(body)), SHA256Hex: fmt.Sprintf("%x", sum), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	storage.put(initiated.Session.ObjectKey, "cited-v1", opportunityapp.PDFMediaType, body, sum)
	completed, err := uploads.Complete(context.Background(), opportunityapp.CompleteUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	processingStore, err := opportunitypersistence.NewDocumentProcessingStore(tenant, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	processor, err := opportunityapp.NewDocumentProcessor(processingStore, storage, syntheticScanner{out: opportunityapp.ScanOutcome{Clean: true, ResultCode: "CLEAN", Engine: "SYNTHETIC_CLAMAV"}}, unusedExtractor{}, fixture.authorizer, randomWorkflowIDs{}, workflowClock{at: at.Add(3 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	processUntilVersionState(t, processor, observer, completed.DocumentVersionID, "CLEAN")

	runID, suggestionID := uuid.New(), uuid.New()
	// Seed only the provider-produced immutable proposal; acceptance still runs through production policy and persistence.
	if err = observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, execErr := db.Exec(ctx, `INSERT INTO ai.extraction_runs(id,job_id,document_version_id,provider,model,schema_version,provider_response_id,state,started_at,completed_at) VALUES($1,$2,$3,'OPENAI','synthetic-seed',1,'synthetic-response','SUCCEEDED',$4,$4)`, runID, liveUUID(completed.AnalysisJobID), liveUUID(completed.DocumentVersionID), at.Add(4*time.Minute)); execErr != nil {
			return execErr
		}
		_, execErr := db.Exec(ctx, `INSERT INTO ai.requirement_suggestions(id,extraction_run_id,document_version_id,ordinal,requirement_text,category,mandatory,citation_page,citation_location,citation_quote,confidence,review_state,created_at) VALUES($1,$2,$3,1,$4,'ELIGIBILITY',true,7,'Section 7.1','The bidder shall provide a current registration certificate.',0.9700,'PROPOSED_UNVERIFIED',$5)`, suggestionID, runID, liveUUID(completed.DocumentVersionID), "AI proposed original wording", at.Add(4*time.Minute))
		return execErr
	}); err != nil {
		t.Fatal(err)
	}

	reviews, err := opportunityapp.NewAIRequirementReviewService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(5 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := reviews.List(context.Background(), fixture.subject, captured.OpportunityID, completed.DocumentVersionID)
	if err != nil {
		_, directErr := store.ListAISuggestions(context.Background(), opportunityapp.WorkflowAuthority{ActorID: fixture.actorID, TenantID: fixture.tenantID, AssignmentGeneration: 1}, captured.OpportunityID, completed.DocumentVersionID)
		t.Fatalf("cited proposal read failed: service=%v store=%v", err, directErr)
	}
	if len(listed) != 1 || listed[0].CitationPage != 7 || listed[0].CitationLocation != "Section 7.1" || listed[0].CitationQuote == "" || listed[0].ReviewState != "PROPOSED_UNVERIFIED" {
		t.Fatalf("cited proposal read drifted: %+v %v", listed, err)
	}
	accepted, err := reviews.Accept(context.Background(), opportunityapp.AcceptAISuggestionInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, DocumentVersionID: completed.DocumentVersionID, SuggestionID: opportunityapp.ID(suggestionID), BidID: workspace.WorkspaceID, SourceVersionID: captured.SourceVersionID, CorrelationID: liveID(), RequirementText: "Human-verified registration certificate requirement.", Category: "ELIGIBILITY", ReviewNote: "Verified against the exact cited page.", Mandatory: true, ExpectedSuggestionVersion: 1, ExpectedWorkspaceVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil || accepted.Requirement == nil || accepted.Requirement.Status != "OPEN" || accepted.Suggestion.ReviewState != "ACCEPTED" || accepted.WorkspaceVersion != 3 {
		t.Fatalf("human acceptance failed: %+v %v", accepted, err)
	}
	if _, err = reviews.Accept(context.Background(), opportunityapp.AcceptAISuggestionInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, DocumentVersionID: completed.DocumentVersionID, SuggestionID: opportunityapp.ID(suggestionID), BidID: workspace.WorkspaceID, SourceVersionID: captured.SourceVersionID, CorrelationID: liveID(), RequirementText: "Stale overwrite", Category: "ELIGIBILITY", Mandatory: true, ExpectedSuggestionVersion: 1, ExpectedWorkspaceVersion: 2, IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "ai_requirement_review_conflict" {
		t.Fatalf("stale AI review accepted: %v", err)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM ai.requirement_suggestions s JOIN bid.requirements r ON r.id=s.accepted_requirement_id WHERE s.id=$1 AND s.requirement_text='AI proposed original wording' AND s.citation_page=7 AND s.citation_location='Section 7.1' AND s.review_state='ACCEPTED' AND r.source_document_version_id=$2 AND r.text='Human-verified registration certificate requirement.' AND r.status='OPEN'`, suggestionID, liveUUID(completed.DocumentVersionID)); count != 1 {
		t.Fatalf("immutable proposal/citation or authoritative requirement linkage drifted: %d", count)
	}
}
