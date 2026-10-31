package integration_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	opportunitypersistence "bidora/backend/internal/opportunity/persistence"
	"bidora/backend/internal/opportunity/proposalexport"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type generationCounter struct{}

func (generationCounter) Count(_ context.Context, r io.Reader, _ opportunityapp.MeasurementRequest, _ opportunityapp.MeasurementLimits) (int64, error) {
	body, err := io.ReadAll(io.LimitReader(r, opportunityapp.MaxResponseDraftInputBytes+1))
	if err != nil || len(body) == 0 {
		return 0, errors.New("synthetic_generation_count_failed")
	}
	return int64(utf8.RuneCount(body)), nil
}

type generationCredits struct{}

func (generationCredits) CalculateGenerationCredits(_ context.Context, input, output int64) (int64, error) {
	if input < 1 || output < 1 {
		return 0, errors.New("synthetic_generation_credits_invalid")
	}
	return (input+output+999)/1000 + 1, nil
}

type generationModel struct {
	mu            sync.Mutex
	response      opportunityapp.GeneratedResponseDraft
	proposal      opportunityapp.GeneratedProposalDraft
	responseErr   error
	proposalErr   error
	responseCalls int
	proposalCalls int
}

func (*generationModel) Model() string { return "synthetic-v6-model" }
func (*generationModel) PromptCharacters(requirement string) (int64, error) {
	return int64(utf8.RuneCountInString("fixed response prompt") + utf8.RuneCountInString(requirement)), nil
}
func (*generationModel) MaximumOutputCharacters() int64         { return 2000 }
func (*generationModel) ProposalPromptCharacters() int64        { return 40 }
func (*generationModel) ProposalMaximumOutputCharacters() int64 { return 2000 }
func (m *generationModel) Generate(_ context.Context, _ string, inputs []opportunityapp.DraftInput) (opportunityapp.GeneratedResponseDraft, error) {
	m.mu.Lock()
	m.responseCalls++
	m.mu.Unlock()
	for _, input := range inputs {
		if _, err := io.Copy(io.Discard, input.Reader); err != nil {
			return opportunityapp.GeneratedResponseDraft{}, err
		}
	}
	return m.response, m.responseErr
}
func (m *generationModel) GenerateProposal(_ context.Context, inputs []opportunityapp.DraftInput) (opportunityapp.GeneratedProposalDraft, error) {
	m.mu.Lock()
	m.proposalCalls++
	m.mu.Unlock()
	for _, input := range inputs {
		if _, err := io.Copy(io.Discard, input.Reader); err != nil {
			return opportunityapp.GeneratedProposalDraft{}, err
		}
	}
	return m.proposal, m.proposalErr
}
func (*generationModel) ActualOutputCharacters(v opportunityapp.GeneratedResponseDraft) (int64, error) {
	return int64(utf8.RuneCountInString(v.Content + v.Rationale + v.Citations[0].Location + v.Citations[0].Quote)), nil
}
func (*generationModel) ProposalActualOutputCharacters(v opportunityapp.GeneratedProposalDraft) (int64, error) {
	return int64(utf8.RuneCountInString(v.Introduction + v.Closing + v.Rationale + v.Citations[0].Location + v.Citations[0].Quote)), nil
}

type reservationVerifier struct{}

func (reservationVerifier) VerifyResponseDraftReservation(_ context.Context, c opportunityapp.ResponseDraftClaim) error {
	if c.ReservationID.IsZero() || c.CheckpointID.IsZero() || c.EstimatedCredits < 1 || c.UsageKind != "GENERATION_TEXT" {
		return errors.New("synthetic_reservation_invalid")
	}
	return nil
}
func (reservationVerifier) VerifyProposalDraftReservation(_ context.Context, c opportunityapp.ProposalDraftClaim) error {
	if c.ReservationID.IsZero() || c.CheckpointID.IsZero() || c.EstimatedCredits < 1 || c.UsageKind != "GENERATION_TEXT" {
		return errors.New("synthetic_reservation_invalid")
	}
	return nil
}

func TestLiveTechnicalResponseProposalAndAdvisoryAIJourney(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenant := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	tenantB := livePool(t, "BIDORA_TEST_TENANT_B_RUNTIME_DSN")
	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := insertWorkflowAuthority(t, control, livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN").Database)
	store := attachWorkflowServices(t, &fixture, tenant, at.Add(time.Minute))
	captured, workspace := establishBidWorkspace(t, tenant, &fixture, at)
	if err := observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, err := db.Exec(ctx, `INSERT INTO bidora_tenant.organization_profile(singleton_key,id,tenant_registry_id,legal_name,country,timezone,currency,policy_version,created_at,updated_at) VALUES(true,$1,$2,'Synthetic Bidora Limited','KE','Africa/Nairobi','KES',1,$3,$3) ON CONFLICT(singleton_key) DO UPDATE SET legal_name=EXCLUDED.legal_name,updated_at=EXCLUDED.updated_at`, uuid.New(), liveUUID(fixture.tenantID), at)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	storage := newSyntheticDocumentStorage()
	processingStore, err := opportunitypersistence.NewDocumentProcessingStore(tenant, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	uploads, err := opportunityapp.NewDocumentUploadService(fixture.authorizer, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	sourceBody := []byte("%PDF-1.7 exact synthetic tender source for V6")
	sourceSum := sha256.Sum256(sourceBody)
	sourceUpload, err := uploads.Initiate(context.Background(), opportunityapp.InitiateUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, CorrelationID: liveID(), Title: "V6 tender.pdf", MediaType: opportunityapp.PDFMediaType, SizeBytes: int64(len(sourceBody)), SHA256Hex: fmt.Sprintf("%x", sourceSum), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	storage.put(sourceUpload.Session.ObjectKey, "v6-source-v1", opportunityapp.PDFMediaType, sourceBody, sourceSum)
	source, err := uploads.Complete(context.Background(), opportunityapp.CompleteUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, UploadID: sourceUpload.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := opportunityapp.NewDocumentProcessor(processingStore, storage, syntheticScanner{out: opportunityapp.ScanOutcome{Clean: true, ResultCode: "CLEAN", Engine: "SYNTHETIC_CLAMAV"}}, unusedExtractor{}, fixture.authorizer, randomWorkflowIDs{}, workflowClock{at: at.Add(3 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	processUntilVersionState(t, scanner, observer, source.DocumentVersionID, "CLEAN")

	runID, suggestionID := uuid.New(), uuid.New()
	if err = observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, x := db.Exec(ctx, `INSERT INTO ai.extraction_runs(id,job_id,document_version_id,provider,model,schema_version,provider_response_id,state,started_at,completed_at) VALUES($1,$2,$3,'OPENAI','synthetic-v6-seed',1,'synthetic-v6-response','SUCCEEDED',$4,$4)`, runID, liveUUID(source.AnalysisJobID), liveUUID(source.DocumentVersionID), at.Add(4*time.Minute)); x != nil {
			return x
		}
		_, x := db.Exec(ctx, `INSERT INTO ai.requirement_suggestions(id,extraction_run_id,document_version_id,ordinal,requirement_text,category,mandatory,citation_page,citation_location,citation_quote,confidence,review_state,created_at) VALUES($1,$2,$3,1,'Submit the exact implementation methodology.','TECHNICAL',true,11,'Section 11','The bidder shall submit an implementation methodology.',0.9800,'PROPOSED_UNVERIFIED',$4)`, suggestionID, runID, liveUUID(source.DocumentVersionID), at.Add(4*time.Minute))
		return x
	}); err != nil {
		t.Fatal(err)
	}
	reviews, err := opportunityapp.NewAIRequirementReviewService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(5 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := reviews.Accept(context.Background(), opportunityapp.AcceptAISuggestionInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, DocumentVersionID: source.DocumentVersionID, SuggestionID: opportunityapp.ID(suggestionID), BidID: workspace.WorkspaceID, SourceVersionID: captured.SourceVersionID, OwnerID: fixture.ownerID, HasOwner: true, CorrelationID: liveID(), RequirementText: "Submit a verified implementation methodology.", Category: "TECHNICAL", ReviewNote: "Human checked section 11.", Mandatory: true, ExpectedSuggestionVersion: 1, ExpectedWorkspaceVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil || accepted.Requirement == nil || accepted.WorkspaceVersion != 3 {
		t.Fatalf("accept cited requirement: %+v %v", accepted, err)
	}
	requirement := *accepted.Requirement

	evidenceService, err := opportunityapp.NewRequirementEvidenceService(fixture.authorizer, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(6 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	evidenceBody := []byte("synthetic DOCX evidence supporting the technical response")
	evidenceSum := sha256.Sum256(evidenceBody)
	evidenceUpload, err := evidenceService.InitiateUpload(context.Background(), opportunityapp.InitiateEvidenceUploadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, CorrelationID: liveID(), Title: "Methodology evidence.docx", MediaType: opportunityapp.DOCXMediaType, SizeBytes: int64(len(evidenceBody)), SHA256Hex: fmt.Sprintf("%x", evidenceSum), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	storage.put(evidenceUpload.Session.ObjectKey, "v6-evidence-v1", opportunityapp.DOCXMediaType, evidenceBody, evidenceSum)
	evidenceVersion, err := evidenceService.CompleteUpload(context.Background(), opportunityapp.CompleteEvidenceUploadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, UploadID: evidenceUpload.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	processUntilVersionState(t, scanner, observer, evidenceVersion.DocumentVersionID, "CLEAN")
	satisfied, err := evidenceService.Satisfy(context.Background(), opportunityapp.SatisfyRequirementInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: requirement.ID, DocumentVersionID: evidenceVersion.DocumentVersionID, CorrelationID: liveID(), EvidenceLocation: "Methodology pages 1-2", Justification: "Exact clean evidence supports the response.", ExpectedRequirementVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: uuid.NewString()})
	if err != nil || satisfied.WorkspaceVersion != 4 {
		t.Fatalf("satisfy prerequisite: %+v %v", satisfied, err)
	}

	responses, err := opportunityapp.NewTechnicalResponseService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(7 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	initialResponse, err := responses.Save(context.Background(), opportunityapp.SaveTechnicalResponseInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: requirement.ID, CorrelationID: liveID(), Content: "We will deliver the cited implementation methodology in controlled phases.", ChangeNote: "Initial human response.", EvidenceIDs: []opportunityapp.ID{satisfied.Evidence.ID}, ExpectedSectionVersion: 0, ExpectedWorkspaceVersion: 4, IdempotencyKey: uuid.NewString()})
	if err != nil || initialResponse.Response.Revision != 1 || initialResponse.WorkspaceVersion != 5 || len(initialResponse.Response.EvidenceIDs) != 1 {
		t.Fatalf("initial technical response: %+v %v", initialResponse, err)
	}
	responseHistory, err := responses.History(context.Background(), fixture.subject, workspace.WorkspaceID, requirement.ID)
	if err != nil || len(responseHistory) != 1 || responseHistory[0].Content != initialResponse.Response.Content {
		t.Fatalf("immutable response history drifted: %+v %v", responseHistory, err)
	}

	model := &generationModel{response: opportunityapp.GeneratedResponseDraft{ProviderResponseID: "synthetic-response-draft", Content: "Advisory response grounded in section 11.", Rationale: "Uses only cited tender and evidence facts.", Citations: []opportunityapp.GeneratedCitation{{InputOrdinal: 1, Location: "Section 11", Quote: "The bidder shall submit an implementation methodology."}}, Gaps: []opportunityapp.GeneratedGap{{Code: "REVIEW_NEEDED", Description: "Human confirmation remains required."}}}, proposal: opportunityapp.GeneratedProposalDraft{ProviderResponseID: "synthetic-proposal-draft", Introduction: "We submit this proposal for human review.", Closing: "We remain available for authorized clarification.", Rationale: "Neutral wording grounded in the cited tender.", Citations: []opportunityapp.GeneratedCitation{{InputOrdinal: 1, Location: "Section 11", Quote: "The bidder shall submit an implementation methodology."}}}}
	limits := opportunityapp.MeasurementLimits{MaxPDFPages: 1000, MaxDOCXCharacters: 10_000_000, MaxArchiveEntries: 10_000, MaxExpandedBytes: opportunityapp.MaxResponseDraftInputBytes, MaxMemoryBytes: 256 * 1024 * 1024, MaxCPUMilliseconds: 60_000, MaxXMLDepth: 64, MaxArchiveDepth: 8, WallTimeout: time.Minute}
	responseDrafts, err := opportunityapp.NewResponseDraftService(fixture.authorizer, store, storage, generationCounter{}, generationCredits{}, limits, model, randomWorkflowIDs{}, workflowClock{at: at.Add(8 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	requestedDraft, err := responseDrafts.Request(context.Background(), opportunityapp.RequestResponseDraftInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: requirement.ID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 5, ExpectedSectionVersion: 1, IdempotencyKey: uuid.NewString()})
	if err != nil || requestedDraft.State != "PENDING_RESERVATION" {
		t.Fatalf("request response draft: %+v %v", requestedDraft, err)
	}
	acknowledgeGenerationReservation(t, tenant, observer, fixture, requestedDraft.RunID, at.Add(9*time.Minute))
	responseWorker, err := opportunityapp.NewResponseDraftWorker(store, storage, model, fixture.authorizer, reservationVerifier{}, generationCredits{}, fixture.tenantID, 1, randomWorkflowIDs{}, workflowClock{at: at.Add(10 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	processResponseDraftUntilState(t, responseWorker, observer, requestedDraft.RunID, "SUCCEEDED")
	drafts, err := responseDrafts.List(context.Background(), fixture.subject, workspace.WorkspaceID, requirement.ID)
	if err != nil || len(drafts) != 1 || drafts[0].ReviewState != "PROPOSED_UNVERIFIED" || len(drafts[0].Citations) != 1 || drafts[0].Citations[0].DocumentVersionID != source.DocumentVersionID {
		t.Fatalf("grounded response draft drifted: %+v %v", drafts, err)
	}

	concurrentInput := opportunityapp.SaveTechnicalResponseInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: requirement.ID, CorrelationID: liveID(), Content: "Revised human-authored implementation methodology response.", ChangeNote: "Concurrent human revision.", EvidenceIDs: []opportunityapp.ID{satisfied.Evidence.ID}, ExpectedSectionVersion: 1, ExpectedWorkspaceVersion: 5, IdempotencyKey: uuid.NewString()}
	type responseOutcome struct {
		result opportunityapp.TechnicalResponseResult
		err    error
	}
	responseOutcomes := make(chan responseOutcome, 2)
	start := make(chan struct{})
	for i := range 2 {
		in := concurrentInput
		in.Content = fmt.Sprintf("Concurrent human response revision %d.", i)
		in.CorrelationID, in.IdempotencyKey = liveID(), uuid.NewString()
		go func() {
			<-start
			result, saveErr := responses.Save(context.Background(), in)
			responseOutcomes <- responseOutcome{result, saveErr}
		}()
	}
	close(start)
	first, second := <-responseOutcomes, <-responseOutcomes
	if !((first.err == nil && second.err != nil && second.err.Error() == "technical_response_conflict") || (second.err == nil && first.err != nil && first.err.Error() == "technical_response_conflict")) {
		t.Fatalf("response race did not preserve one winner: first=%+v/%v second=%+v/%v", first.result, first.err, second.result, second.err)
	}
	winner := first.result
	if first.err != nil {
		winner = second.result
	}
	if winner.Response.SectionVersion != 2 || winner.WorkspaceVersion != 6 {
		t.Fatalf("response race winner drifted: %+v", winner)
	}

	staleDraft, err := responseDrafts.Request(context.Background(), opportunityapp.RequestResponseDraftInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: requirement.ID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 6, ExpectedSectionVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	acknowledgeGenerationReservation(t, tenant, observer, fixture, staleDraft.RunID, at.Add(11*time.Minute))
	thirdResponse, err := responses.Save(context.Background(), opportunityapp.SaveTechnicalResponseInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: requirement.ID, CorrelationID: liveID(), Content: "Final human response after advisory snapshot.", ChangeNote: "Invalidate stale advisory snapshot.", EvidenceIDs: []opportunityapp.ID{satisfied.Evidence.ID}, ExpectedSectionVersion: 2, ExpectedWorkspaceVersion: 6, IdempotencyKey: uuid.NewString()})
	if err != nil || thirdResponse.WorkspaceVersion != 7 {
		t.Fatalf("third response: %+v %v", thirdResponse, err)
	}
	callsBeforeStale := model.responseCalls
	processResponseDraftUntilState(t, responseWorker, observer, staleDraft.RunID, "FAILED")
	if model.responseCalls != callsBeforeStale || rowCount(t, observer, `SELECT count(*) FROM ai.response_draft_runs WHERE id=$1 AND failure_code='INPUT_EXPIRED'`, liveUUID(staleDraft.RunID)) != 1 {
		t.Fatal("stale response draft reached provider or lacked INPUT_EXPIRED evidence")
	}

	proposals, err := opportunityapp.NewProposalService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(12 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	letterheadID := uuid.New()
	letterheadHash := sha256.Sum256(append([]byte("v6 immutable synthetic letterhead:"), fixture.tenantID[:]...))
	if err := observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, err := db.Exec(ctx, `INSERT INTO organization.letterhead_versions(id,version_number,registered_name,phone,email,header_layout,footer_layout,page_number_style,primary_color,secondary_color,template_key,content_hash,created_by,created_at) SELECT $1,next_version_number,'Bidora Synthetic Ltd','+254700000000','synthetic@example.invalid','STANDARD','CONTACT','PAGE_X_OF_Y','#112233','#445566','STANDARD_PROPOSAL',$2,$3,$4 FROM organization.letterhead_configuration WHERE singleton_key=true`, letterheadID, letterheadHash[:], uuid.UUID(fixture.actorID), at.Add(12*time.Minute)); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `UPDATE organization.letterhead_configuration SET active_version_id=$1,next_version_number=next_version_number+1,version=version+1,updated_by=$2,updated_at=$3 WHERE singleton_key=true`, letterheadID, uuid.UUID(fixture.actorID), at.Add(12*time.Minute))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	proposal, err := proposals.Save(context.Background(), opportunityapp.SaveProposalInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, CorrelationID: liveID(), Title: "Implementation Methodology Proposal", Recipient: "Evaluation Committee", Introduction: "Human-authored introduction.", Closing: "Human-authored closing.", ChangeNote: "Initial immutable proposal.", LetterheadLines: []string{"Nairobi, Kenya", "contact@example.invalid"}, ExpectedProposalVersion: 0, ExpectedWorkspaceVersion: 7, IdempotencyKey: uuid.NewString()})
	if err != nil || proposal.Proposal.Revision != 1 || proposal.Proposal.WorkspaceVersion != 8 || len(proposal.Proposal.Responses) != 1 || proposal.Proposal.Responses[0].ResponseVersionID != thirdResponse.Response.ResponseVersionID {
		t.Fatalf("proposal snapshot: %+v %v", proposal, err)
	}
	proposalHistory, err := proposals.History(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || len(proposalHistory) != 1 || proposalHistory[0].ProposalVersionID != proposal.Proposal.ProposalVersionID {
		t.Fatalf("proposal history drifted: %+v %v", proposalHistory, err)
	}

	proposalDrafts, err := opportunityapp.NewProposalDraftService(fixture.authorizer, store, storage, generationCounter{}, generationCredits{}, limits, model, randomWorkflowIDs{}, workflowClock{at: at.Add(13 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	proposalDraft, err := proposalDrafts.Request(context.Background(), opportunityapp.RequestProposalDraftInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, ProposalVersionID: proposal.Proposal.ProposalVersionID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 8, ExpectedProposalVersion: 1, IdempotencyKey: uuid.NewString()})
	if err != nil || proposalDraft.State != "PENDING_RESERVATION" {
		t.Fatalf("request proposal draft: %+v %v", proposalDraft, err)
	}
	acknowledgeGenerationReservation(t, tenant, observer, fixture, proposalDraft.RunID, at.Add(14*time.Minute))
	proposalWorker, err := opportunityapp.NewProposalDraftWorker(store, storage, model, fixture.authorizer, reservationVerifier{}, generationCredits{}, fixture.tenantID, 1, randomWorkflowIDs{}, workflowClock{at: at.Add(15 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	processProposalDraftUntilState(t, proposalWorker, observer, proposalDraft.RunID, "SUCCEEDED")
	proposalSuggestions, err := proposalDrafts.List(context.Background(), fixture.subject, workspace.WorkspaceID, proposal.Proposal.ProposalVersionID)
	if err != nil || len(proposalSuggestions) != 1 || proposalSuggestions[0].ReviewState != "PROPOSED_UNVERIFIED" || len(proposalSuggestions[0].Citations) != 1 {
		t.Fatalf("proposal suggestion drifted: %+v %v", proposalSuggestions, err)
	}

	applications, err := opportunityapp.NewProposalApplicationService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(16 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := applications.Apply(context.Background(), opportunityapp.ApplyProposalSuggestionInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, SuggestionID: proposalSuggestions[0].ID, CorrelationID: liveID(), Introduction: "Human-edited introduction based on cited advisory text.", Closing: "Human-edited closing approved for continued review.", ReviewNote: "Edited and applied by an authorized human.", ExpectedSuggestionVersion: 1, ExpectedProposalVersion: 1, ExpectedWorkspaceVersion: 8, IdempotencyKey: uuid.NewString()})
	if err != nil || applied.Proposal.Revision != 2 || applied.Proposal.WorkspaceVersion != 9 || applied.Proposal.Introduction == proposalSuggestions[0].Introduction {
		t.Fatalf("human proposal application: %+v %v", applied, err)
	}
	if _, err = applications.Apply(context.Background(), opportunityapp.ApplyProposalSuggestionInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, SuggestionID: proposalSuggestions[0].ID, CorrelationID: liveID(), Introduction: "Stale overwrite", Closing: "Stale overwrite", ReviewNote: "Must conflict.", ExpectedSuggestionVersion: 1, ExpectedProposalVersion: 1, ExpectedWorkspaceVersion: 8, IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "proposal_suggestion_apply_conflict" {
		t.Fatalf("stale suggestion application accepted: %v", err)
	}
	readiness, err := applications.Readiness(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || !readiness.Ready || len(readiness.Blockers) != 0 {
		t.Fatalf("proposal readiness drifted: %+v %v", readiness, err)
	}

	exports, err := opportunityapp.NewProposalExportService(fixture.authorizer, store, proposalexport.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"pdf", "docx"} {
		one, exportErr := exports.Export(context.Background(), fixture.subject, workspace.WorkspaceID, applied.Proposal.ProposalVersionID, format)
		if exportErr != nil {
			t.Fatal(exportErr)
		}
		two, exportErr := exports.Export(context.Background(), fixture.subject, workspace.WorkspaceID, applied.Proposal.ProposalVersionID, format)
		if exportErr != nil || !bytes.Equal(one.Bytes, two.Bytes) || one.SHA256 != two.SHA256 || one.SHA256 != sha256.Sum256(one.Bytes) || len(one.Bytes) == 0 {
			t.Fatalf("non-deterministic %s export: %v", format, exportErr)
		}
	}

	failingDrafts, err := opportunityapp.NewResponseDraftService(fixture.authorizer, store, storage, generationCounter{}, generationCredits{}, limits, model, randomWorkflowIDs{}, workflowClock{at: at.Add(18 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	failedDraft, err := failingDrafts.Request(context.Background(), opportunityapp.RequestResponseDraftInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: requirement.ID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 9, ExpectedSectionVersion: 3, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	acknowledgeGenerationReservation(t, tenant, observer, fixture, failedDraft.RunID, at.Add(19*time.Minute))
	model.responseErr = opportunityapp.NewProviderExecutionError(opportunityapp.ProviderAmbiguousEffect, errors.New("synthetic_provider_failure"))
	failureWorker, err := opportunityapp.NewResponseDraftWorker(store, storage, model, fixture.authorizer, reservationVerifier{}, generationCredits{}, fixture.tenantID, 1, randomWorkflowIDs{}, workflowClock{at: at.Add(20 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	processResponseDraftUntilState(t, failureWorker, observer, failedDraft.RunID, "FAILED")
	if rowCount(t, observer, `SELECT count(*) FROM ai.response_draft_runs WHERE id=$1 AND failure_code='PROVIDER_OR_OUTPUT_FAILURE'`, liveUUID(failedDraft.RunID)) != 1 {
		t.Fatal("ambiguous provider failure was not retained as a failed advisory run")
	}

	storeB, err := opportunitypersistence.NewWorkflowStore(tenantB, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = storeB.GetProposalVersion(context.Background(), opportunityapp.WorkflowAuthority{ActorID: fixture.actorID, TenantID: fixture.tenantID, AssignmentGeneration: 1}, workspace.WorkspaceID, applied.Proposal.ProposalVersionID); !errors.Is(err, opportunityapp.ErrProposalNotFound) {
		t.Fatalf("tenant B exposed tenant A proposal: %v", err)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM bidora_eventing.outbox_events WHERE actor_id=$1 AND event_type IN ('bid.technical_response.revised.v1','ai.response_draft.requested.v1','ai.response_draft.proposed.v1','bid.proposal.revised.v1','ai.proposal_draft.requested.v1','ai.proposal_draft.proposed.v1','ai.proposal_draft.applied.v1')`, liveUUID(fixture.actorID)); count < 7 {
		t.Fatalf("V6 actor-provenance outbox lineage incomplete: %d", count)
	}
}

func acknowledgeGenerationReservation(t *testing.T, tenant, observer *postgres.Pool, fixture workflowFixture, targetID opportunityapp.ID, at time.Time) {
	t.Helper()
	var checkpoint uuid.UUID
	if err := observer.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT id FROM bidora_entitlement.ai_usage_checkpoints WHERE source_operation_id=$1 AND target_type IN ('RESPONSE_DRAFT','PROPOSAL_DRAFT')`, liveUUID(targetID)).Scan(&checkpoint)
	}); err != nil {
		t.Fatal(err)
	}
	uow, err := opportunitypersistence.NewDocumentEntitlementAcknowledgementStore(tenant)
	if err != nil {
		t.Fatal(err)
	}
	service, err := opportunityapp.NewDocumentEntitlementAcknowledgementService(uow, randomWorkflowIDs{}, workflowClock{at: at}, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	reservationID := liveID()
	result, err := service.Apply(context.Background(), entitlementapp.Acknowledgement{CheckpointID: entitlementapp.ID(checkpoint), TenantID: entitlementapp.ID(fixture.tenantID), EventID: entitlementapp.ID(liveID()), AggregateID: entitlementapp.ID(reservationID), ReservationID: entitlementapp.ID(reservationID), CorrelationID: entitlementapp.ID(liveID()), Disposition: entitlementapp.DispositionReserved, EventType: "commercial.ai_usage.reserved.v1", AggregateVersion: 1, AssignmentGeneration: 1, EventOccurredAt: at})
	if err != nil || result.Checkpoint.State != entitlementapp.Reserved {
		t.Fatalf("acknowledge generation reservation: %+v %v", result, err)
	}
}

func processResponseDraftUntilState(t *testing.T, worker *opportunityapp.ResponseDraftWorker, observer *postgres.Pool, runID opportunityapp.ID, state string) {
	t.Helper()
	for attempt := 0; attempt < 30; attempt++ {
		processed, err := worker.ProcessNext(context.Background())
		if !processed {
			t.Fatalf("response draft queue exhausted: %v", err)
		}
		if rowCount(t, observer, `SELECT count(*) FROM ai.response_draft_runs WHERE id=$1 AND state=$2`, liveUUID(runID), state) == 1 {
			return
		}
		if attempt == 29 {
			t.Fatalf("response draft target did not reach %s: %v", state, err)
		}
	}
}

func processProposalDraftUntilState(t *testing.T, worker *opportunityapp.ProposalDraftWorker, observer *postgres.Pool, runID opportunityapp.ID, state string) {
	t.Helper()
	for attempt := 0; attempt < 30; attempt++ {
		processed, err := worker.ProcessNext(context.Background())
		if !processed {
			t.Fatalf("proposal draft queue exhausted: %v", err)
		}
		if rowCount(t, observer, `SELECT count(*) FROM ai.proposal_draft_runs WHERE id=$1 AND state=$2`, liveUUID(runID), state) == 1 {
			return
		}
		if attempt == 29 {
			t.Fatalf("proposal draft target did not reach %s: %v", state, err)
		}
	}
}
