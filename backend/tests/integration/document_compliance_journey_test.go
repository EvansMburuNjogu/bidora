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

	opportunityapp "bidora/backend/internal/opportunity/application"
	opportunitypersistence "bidora/backend/internal/opportunity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type syntheticDocumentObject struct {
	key, version, mediaType string
	body                    []byte
	sum                     [32]byte
}

type syntheticDocumentStorage struct {
	mu      sync.Mutex
	objects map[string]syntheticDocumentObject
}

func newSyntheticDocumentStorage() *syntheticDocumentStorage {
	return &syntheticDocumentStorage{objects: make(map[string]syntheticDocumentObject)}
}
func (s *syntheticDocumentStorage) PresignPut(_ context.Context, key, mediaType string, size int64, sum [32]byte, ttl time.Duration) (opportunityapp.UploadInstruction, error) {
	if key == "" || mediaType == "" || size < 1 || sum == ([32]byte{}) || ttl <= 0 || ttl > opportunityapp.UploadInstructionTTL {
		return opportunityapp.UploadInstruction{}, errors.New("synthetic_presign_invalid")
	}
	return opportunityapp.UploadInstruction{Method: "PUT", URL: "https://storage.example.invalid/upload", Fields: map[string]string{"checksum": opportunityapp.SHA256Base64(sum)}}, nil
}
func (s *syntheticDocumentStorage) Inspect(_ context.Context, key string) (opportunityapp.StoredObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, ok := s.objects[key]
	if !ok {
		return opportunityapp.StoredObject{}, errors.New("synthetic_object_missing")
	}
	return opportunityapp.StoredObject{ObjectVersion: object.version, MediaType: object.mediaType, SizeBytes: int64(len(object.body)), SHA256: object.sum}, nil
}
func (s *syntheticDocumentStorage) OpenExact(_ context.Context, key, version string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, ok := s.objects[key]
	if !ok || object.version != version {
		return nil, errors.New("synthetic_exact_object_missing")
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), object.body...))), nil
}
func (s *syntheticDocumentStorage) put(key, version, mediaType string, body []byte, declaredSum [32]byte) {
	s.mu.Lock()
	s.objects[key] = syntheticDocumentObject{key: key, version: version, mediaType: mediaType, body: append([]byte(nil), body...), sum: declaredSum}
	s.mu.Unlock()
}

type syntheticScanner struct {
	out opportunityapp.ScanOutcome
	err error
}

func (s syntheticScanner) Scan(_ context.Context, reader io.Reader, expected int64) (opportunityapp.ScanOutcome, error) {
	body, err := io.ReadAll(io.LimitReader(reader, opportunityapp.MaxTenderDocumentBytes+1))
	if err != nil || int64(len(body)) != expected {
		return opportunityapp.ScanOutcome{}, errors.New("synthetic_scan_size_mismatch")
	}
	return s.out, s.err
}

type unusedExtractor struct{}

func (unusedExtractor) Model() string { return "synthetic-not-invoked" }
func (unusedExtractor) Extract(context.Context, io.Reader, int64, string) (opportunityapp.ExtractionOutput, error) {
	return opportunityapp.ExtractionOutput{}, errors.New("synthetic_extractor_must_not_run")
}

func establishBidWorkspace(t *testing.T, tenant *postgres.Pool, fixture *workflowFixture, at time.Time) (opportunityapp.CaptureResult, opportunityapp.WorkspaceResult) {
	t.Helper()
	captured := captureForWorkflow(t, tenant, fixture.tenantID, fixture.actorID, "Synthetic V5 opportunity "+uuid.NewString(), at)
	verified, err := fixture.service.Verify(context.Background(), opportunityapp.VerifyOpportunityInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, SourceVersionID: captured.SourceVersionID, CorrelationID: liveID(), ExpectedVersion: 1, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := fixture.service.CreateWorkspace(context.Background(), opportunityapp.CreateWorkspaceInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, SourceVersionID: verified.SourceVersionID, OwnerID: fixture.ownerID, CorrelationID: liveID(), ExpectedVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := fixture.service.RecordDecision(context.Background(), opportunityapp.RecordDecisionInput{Subject: fixture.subject, WorkspaceID: workspace.WorkspaceID, CorrelationID: liveID(), ExpectedVersion: 1, Decision: "BID", Rationale: "Proceed with synthetic compliance work.", IdempotencyKey: uuid.NewString()})
	if err != nil || decision.Version != 2 {
		t.Fatalf("establish BID state: %+v %v", decision, err)
	}
	workspace.Version = decision.Version
	captured.SourceVersionID = verified.SourceVersionID
	return captured, workspace
}

func TestLiveDocumentComplianceJourneyQuarantineReadinessAndIsolation(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenantA := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	tenantAObserver := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	tenantB := livePool(t, "BIDORA_TEST_TENANT_B_RUNTIME_DSN")
	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := insertWorkflowAuthority(t, control, livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN").Database)
	store := attachWorkflowServices(t, &fixture, tenantA, at.Add(time.Minute))
	captured, workspace := establishBidWorkspace(t, tenantA, &fixture, at)
	storage := newSyntheticDocumentStorage()
	processingStore, err := opportunitypersistence.NewDocumentProcessingStore(tenantA, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	uploads, err := opportunityapp.NewDocumentUploadService(fixture.authorizer, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(90 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	invalidUpload := opportunityapp.InitiateUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, CorrelationID: liveID(), Title: "Tender.pdf", MediaType: "application/octet-stream", SizeBytes: 10, SHA256Hex: fmt.Sprintf("%x", sha256.Sum256([]byte("0123456789"))), IdempotencyKey: uuid.NewString()}
	if _, err = uploads.Initiate(context.Background(), invalidUpload); err == nil || err.Error() != "document_upload_invalid" {
		t.Fatalf("unsupported media type accepted: %v", err)
	}
	invalidUpload.MediaType = opportunityapp.PDFMediaType
	invalidUpload.SizeBytes = opportunityapp.MaxTenderDocumentBytes + 1
	invalidUpload.IdempotencyKey = uuid.NewString()
	if _, err = uploads.Initiate(context.Background(), invalidUpload); err == nil || err.Error() != "document_upload_invalid" {
		t.Fatalf("oversized document accepted: %v", err)
	}
	pdfBody := []byte("%PDF-1.7 synthetic quarantined tender")
	pdfSum := sha256.Sum256(pdfBody)
	tenderUpload, err := uploads.Initiate(context.Background(), opportunityapp.InitiateUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, CorrelationID: liveID(), Title: "Tender.pdf", MediaType: opportunityapp.PDFMediaType, SizeBytes: int64(len(pdfBody)), SHA256Hex: fmt.Sprintf("%x", pdfSum), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	storage.put(tenderUpload.Session.ObjectKey, "tender-v1", opportunityapp.PDFMediaType, pdfBody, pdfSum)
	tenderCompleted, err := uploads.Complete(context.Background(), opportunityapp.CompleteUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, UploadID: tenderUpload.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil || tenderCompleted.ScanState != "QUARANTINED" || tenderCompleted.AnalysisState != "WAITING_FOR_CLEAN" {
		t.Fatalf("tender quarantine: %+v %v", tenderCompleted, err)
	}
	rejectProcessor, err := opportunityapp.NewDocumentProcessor(processingStore, storage, syntheticScanner{out: opportunityapp.ScanOutcome{Clean: false, ResultCode: "MALWARE_DETECTED", Engine: "SYNTHETIC_CLAMAV"}}, unusedExtractor{}, fixture.authorizer, randomWorkflowIDs{}, workflowClock{at: at.Add(100 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	processUntilVersionState(t, rejectProcessor, tenantAObserver, tenderCompleted.DocumentVersionID, "REJECTED")
	if count := rowCount(t, tenantAObserver, `SELECT count(*) FROM documents.document_versions v JOIN jobs.jobs a ON a.document_version_id=v.id AND a.kind='TENDER_ANALYSIS' WHERE v.id=$1 AND v.scan_state='REJECTED' AND a.state='CANCELLED'`, liveUUID(tenderCompleted.DocumentVersionID)); count != 1 {
		t.Fatalf("rejected document was not quarantined from analysis: %d", count)
	}

	requirements, err := opportunityapp.NewRequirementService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	created, err := requirements.Create(context.Background(), opportunityapp.CreateRequirementInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, SourceVersionID: captured.SourceVersionID, CorrelationID: liveID(), SourceLocation: "Tender section 4.2", Text: "Provide an unexpired tax compliance certificate.", Category: "COMPLIANCE", Mandatory: true, DueAt: ptrTime(at.Add(-time.Hour)), ExpectedWorkspaceVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil || created.Requirement.Version != 1 || created.WorkspaceVersion != 3 {
		t.Fatalf("create mandatory requirement: %+v %v", created, err)
	}
	readiness, err := requirements.Readiness(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || readiness.Ready || readiness.OpenMandatoryCount != 1 || readiness.UnassignedMandatoryCount != 1 || readiness.OverdueCount != 1 {
		t.Fatalf("mandatory blocker not represented safely: %+v %v", readiness, err)
	}
	assigned, err := requirements.Assign(context.Background(), opportunityapp.AssignRequirementInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: created.Requirement.ID, OwnerID: fixture.ownerID, CorrelationID: liveID(), DueAt: ptrTime(at.Add(time.Hour)), ExpectedRequirementVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: uuid.NewString()})
	if err != nil || assigned.Requirement.Version != 2 || assigned.WorkspaceVersion != 4 {
		t.Fatalf("assign requirement: %+v %v", assigned, err)
	}
	responses, err := opportunityapp.NewTechnicalResponseService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(150 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	response, err := responses.Save(context.Background(), opportunityapp.SaveTechnicalResponseInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: created.Requirement.ID, CorrelationID: liveID(), Content: "The current certificate is attached and validated.", ChangeNote: "Initial human response.", ExpectedSectionVersion: 0, ExpectedWorkspaceVersion: 4, IdempotencyKey: uuid.NewString()})
	if err != nil || response.Response.Revision != 1 || response.WorkspaceVersion != 5 {
		t.Fatalf("save technical response prerequisite: %+v %v", response, err)
	}

	corrections, err := opportunityapp.NewRequirementCorrectionService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(3 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	correctionInput := opportunityapp.CorrectRequirementInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: created.Requirement.ID, CorrelationID: liveID(), SourceLocation: "Tender section 4.2(a)", Text: "Provide a valid and unexpired tax compliance certificate.", Category: "COMPLIANCE", CorrectionReason: "Human corrected the cited source wording.", Mandatory: true, ExpectedRequirementVersion: 2, ExpectedWorkspaceVersion: 5, IdempotencyKey: uuid.NewString()}
	corrected, err := corrections.Correct(context.Background(), correctionInput)
	if err != nil || corrected.Requirement.Version != 3 || corrected.ContentRevision.Revision != 2 || corrected.WorkspaceVersion != 6 {
		t.Fatalf("correct requirement: %+v %v", corrected, err)
	}
	concurrentBase := correctionInput
	concurrentBase.ExpectedRequirementVersion = 3
	concurrentBase.ExpectedWorkspaceVersion = 6
	type correctionOutcome struct {
		result opportunityapp.RequirementCorrectionResult
		err    error
	}
	outcomes := make(chan correctionOutcome, 2)
	start := make(chan struct{})
	for i := range 2 {
		input := concurrentBase
		input.CorrelationID = liveID()
		input.IdempotencyKey = uuid.NewString()
		input.Text = fmt.Sprintf("Concurrent human correction %d must not overwrite its peer.", i)
		go func() {
			<-start
			result, correctionErr := corrections.Correct(context.Background(), input)
			outcomes <- correctionOutcome{result: result, err: correctionErr}
		}()
	}
	close(start)
	first, second := <-outcomes, <-outcomes
	var concurrentWinner opportunityapp.RequirementCorrectionResult
	if first.err == nil && second.err != nil && second.err.Error() == "requirement_correction_conflict" {
		concurrentWinner = first.result
	} else if second.err == nil && first.err != nil && first.err.Error() == "requirement_correction_conflict" {
		concurrentWinner = second.result
	} else {
		t.Fatalf("concurrent corrections did not produce one commit and one conflict: first=%+v/%v second=%+v/%v", first.result, first.err, second.result, second.err)
	}
	if concurrentWinner.Requirement.Version != 4 || concurrentWinner.ContentRevision.Revision != 3 || concurrentWinner.WorkspaceVersion != 7 {
		t.Fatalf("concurrent correction winner drifted: %+v", concurrentWinner)
	}
	staleCorrection := correctionInput
	staleCorrection.IdempotencyKey = uuid.NewString()
	if _, err = corrections.Correct(context.Background(), staleCorrection); err == nil || err.Error() != "requirement_correction_conflict" {
		t.Fatalf("stale correction accepted: %v", err)
	}
	history, err := corrections.History(context.Background(), fixture.subject, workspace.WorkspaceID, created.Requirement.ID)
	if err != nil || len(history) != 3 || history[0].Revision != 3 || history[1].Revision != 2 || history[2].Revision != 1 {
		t.Fatalf("immutable correction history drifted: %+v %v", history, err)
	}
	gap, err := responses.GapReview(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || len(gap.Requirements) != 1 || !containsString(gap.Requirements[0].Blockers, "TECHNICAL_RESPONSE_OUTDATED") {
		t.Fatalf("material correction did not invalidate response applicability: %+v %v", gap, err)
	}

	evidence, err := opportunityapp.NewRequirementEvidenceService(fixture.authorizer, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(4 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("synthetic DOCX bytes for immutable exact-version evidence")
	sum := sha256.Sum256(body)
	expires := at.Add(time.Hour).Format(time.RFC3339)
	initiated, err := evidence.InitiateUpload(context.Background(), opportunityapp.InitiateEvidenceUploadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, CorrelationID: liveID(), Title: "Tax certificate.docx", MediaType: opportunityapp.DOCXMediaType, SizeBytes: int64(len(body)), SHA256Hex: fmt.Sprintf("%x", sum), EvidenceExpiresAt: &expires, IdempotencyKey: uuid.NewString()})
	if err != nil || initiated.Session.State != "INITIATED" || initiated.Instruction.Method != "PUT" {
		t.Fatalf("initiate evidence: %+v %v", initiated, err)
	}
	storage.put(initiated.Session.ObjectKey, "", opportunityapp.DOCXMediaType, body, sum)
	if _, err = evidence.CompleteUpload(context.Background(), opportunityapp.CompleteEvidenceUploadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "evidence_upload_mismatch" {
		t.Fatalf("missing immutable object version accepted: %v", err)
	}
	storage.put(initiated.Session.ObjectKey, "v0", opportunityapp.DOCXMediaType, append(append([]byte(nil), body...), 'x'), sum)
	if _, err = evidence.CompleteUpload(context.Background(), opportunityapp.CompleteEvidenceUploadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "evidence_upload_mismatch" {
		t.Fatalf("size mismatch accepted: %v", err)
	}
	wrongSum := sha256.Sum256([]byte("wrong object"))
	storage.put(initiated.Session.ObjectKey, "v1", opportunityapp.DOCXMediaType, body, wrongSum)
	if _, err = evidence.CompleteUpload(context.Background(), opportunityapp.CompleteEvidenceUploadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "evidence_upload_mismatch" {
		t.Fatalf("checksum mismatch accepted: %v", err)
	}
	storage.put(initiated.Session.ObjectKey, "v2", opportunityapp.DOCXMediaType, body, sum)
	completed, err := evidence.CompleteUpload(context.Background(), opportunityapp.CompleteEvidenceUploadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil || completed.ScanState != "QUARANTINED" || completed.DocumentVersionID.IsZero() {
		t.Fatalf("complete evidence: %+v %v", completed, err)
	}
	processor, err := opportunityapp.NewDocumentProcessor(processingStore, storage, syntheticScanner{out: opportunityapp.ScanOutcome{Clean: true, ResultCode: "CLEAN", Engine: "SYNTHETIC_CLAMAV"}}, unusedExtractor{}, fixture.authorizer, randomWorkflowIDs{}, workflowClock{at: at.Add(5 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	processUntilVersionState(t, processor, tenantAObserver, completed.DocumentVersionID, "CLEAN")
	if count := rowCount(t, tenantAObserver, `SELECT count(*) FROM documents.document_versions WHERE id=$1 AND scan_state='CLEAN' AND object_version='v2' AND sha256=$2`, liveUUID(completed.DocumentVersionID), sum[:]); count != 1 {
		t.Fatalf("exact immutable clean version missing: %d", count)
	}
	foreignCaptured, foreignWorkspace := establishBidWorkspace(t, tenantA, &fixture, at)
	foreignRequirements, err := opportunityapp.NewRequirementService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(12 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	foreignRequirement, err := foreignRequirements.Create(context.Background(), opportunityapp.CreateRequirementInput{Subject: fixture.subject, BidID: foreignWorkspace.WorkspaceID, SourceVersionID: foreignCaptured.SourceVersionID, CorrelationID: liveID(), SourceLocation: "Foreign bid section 1", Text: "Foreign workspace requirement.", Category: "COMPLIANCE", Mandatory: true, ExpectedWorkspaceVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = evidence.Satisfy(context.Background(), opportunityapp.SatisfyRequirementInput{Subject: fixture.subject, BidID: foreignWorkspace.WorkspaceID, RequirementID: foreignRequirement.Requirement.ID, DocumentVersionID: completed.DocumentVersionID, CorrelationID: liveID(), EvidenceLocation: "Must not cross bids", Justification: "Negative cross-bid linkage probe.", ExpectedRequirementVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "evidence_conflict" {
		t.Fatalf("cross-bid evidence linkage accepted: %v", err)
	}

	satisfied, err := evidence.Satisfy(context.Background(), opportunityapp.SatisfyRequirementInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: created.Requirement.ID, DocumentVersionID: completed.DocumentVersionID, CorrelationID: liveID(), EvidenceLocation: "Certificate page 1", Justification: "The cited clean version contains the valid certificate.", ExpectedRequirementVersion: 4, ExpectedWorkspaceVersion: 7, IdempotencyKey: uuid.NewString()})
	if err != nil || satisfied.Requirement.Status != "SATISFIED" || satisfied.WorkspaceVersion != 8 {
		t.Fatalf("satisfy requirement: %+v %v", satisfied, err)
	}
	if _, err = evidence.Satisfy(context.Background(), opportunityapp.SatisfyRequirementInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, RequirementID: created.Requirement.ID, DocumentVersionID: completed.DocumentVersionID, CorrelationID: liveID(), EvidenceLocation: "Unsupported second transition", Justification: "A bare SATISFIED transition must not exist.", ExpectedRequirementVersion: 5, ExpectedWorkspaceVersion: 8, IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "evidence_conflict" {
		t.Fatalf("unsupported second SATISFIED transition accepted: %v", err)
	}
	ready, err := requirements.Readiness(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || !ready.Ready || ready.OpenMandatoryCount != 0 {
		t.Fatalf("clean unexpired evidence did not clear blocker: %+v %v", ready, err)
	}
	boundaryRequirements, err := opportunityapp.NewRequirementService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	boundary, err := boundaryRequirements.Readiness(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || boundary.Ready || boundary.ExpiredEvidenceCount != 1 {
		t.Fatalf("evidence remained valid at its exclusive expiry boundary: %+v %v", boundary, err)
	}
	lateRequirements, err := opportunityapp.NewRequirementService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	expired, err := lateRequirements.Readiness(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || expired.Ready || expired.OpenMandatoryCount != 1 || expired.ExpiredEvidenceCount != 1 {
		t.Fatalf("expired evidence did not restore blocker: %+v %v", expired, err)
	}

	storeB, err := opportunitypersistence.NewWorkflowStore(tenantB, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := storeB.ListRequirementEvidence(context.Background(), opportunityapp.WorkflowAuthority{ActorID: fixture.actorID, TenantID: fixture.tenantID, AssignmentGeneration: 1}, workspace.WorkspaceID, created.Requirement.ID); err != nil || len(rows) != 0 {
		t.Fatalf("tenant B exposed tenant A exact IDs: rows=%+v err=%v", rows, err)
	}
	failedBody := []byte("%PDF synthetic scanner failure")
	failedSum := sha256.Sum256(failedBody)
	failedUpload, err := uploads.Initiate(context.Background(), opportunityapp.InitiateUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, CorrelationID: liveID(), Title: "Scanner failure.pdf", MediaType: opportunityapp.PDFMediaType, SizeBytes: int64(len(failedBody)), SHA256Hex: fmt.Sprintf("%x", failedSum), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	storage.put(failedUpload.Session.ObjectKey, "failed-v1", opportunityapp.PDFMediaType, failedBody, failedSum)
	failedCompleted, err := uploads.Complete(context.Background(), opportunityapp.CompleteUploadInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, UploadID: failedUpload.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	failureProcessor, err := opportunityapp.NewDocumentProcessor(processingStore, storage, syntheticScanner{err: errors.New("synthetic_clamav_unavailable")}, unusedExtractor{}, fixture.authorizer, randomWorkflowIDs{}, workflowClock{at: at.Add(6 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	for attempts := 0; attempts < 20; attempts++ {
		processed, scanErr := failureProcessor.ProcessNextScan(context.Background())
		if !processed || scanErr == nil {
			t.Fatalf("scanner failure did not fail closed: processed=%t err=%v", processed, scanErr)
		}
		if rowCount(t, tenantAObserver, `SELECT count(*) FROM jobs.jobs WHERE document_version_id=$1 AND kind='MALWARE_SCAN' AND state='QUEUED' AND attempts>0`, liveUUID(failedCompleted.DocumentVersionID)) == 1 {
			if scanErr.Error() != "document_scan_unavailable" {
				t.Fatalf("unexpected scanner failure: %v", scanErr)
			}
			break
		}
		if attempts == 19 {
			t.Fatal("target scanner failure job was starved")
		}
	}
	if count := rowCount(t, tenantAObserver, `SELECT count(*) FROM documents.document_versions v JOIN jobs.jobs j ON j.document_version_id=v.id AND j.kind='MALWARE_SCAN' WHERE v.id=$1 AND v.scan_state='QUARANTINED' AND j.state='QUEUED'`, liveUUID(failedCompleted.DocumentVersionID)); count != 1 {
		t.Fatalf("scanner failure escaped quarantine or lost retry: %d", count)
	}
	if count := rowCount(t, tenantAObserver, `SELECT count(*) FROM bidora_eventing.outbox_events WHERE actor_id=$1 AND event_type IN ('bid.requirement.created.v1','bid.requirement.assigned.v1','bid.requirement.corrected.v1','bid.evidence.quarantined.v1','document.version.clean.v1','bid.requirement.satisfied.v1')`, liveUUID(fixture.actorID)); count < 6 {
		t.Fatalf("actor-provenance outbox lineage incomplete: %d", count)
	}
}

func processUntilVersionState(t *testing.T, processor *opportunityapp.DocumentProcessor, observer *postgres.Pool, versionID opportunityapp.ID, state string) {
	t.Helper()
	for attempts := 0; attempts < 20; attempts++ {
		processed, err := processor.ProcessNextScan(context.Background())
		if !processed {
			t.Fatalf("scan queue exhausted before target version: %v", err)
		}
		if rowCount(t, observer, `SELECT count(*) FROM documents.document_versions WHERE id=$1 AND scan_state=$2`, liveUUID(versionID), state) == 1 {
			if err != nil {
				t.Fatalf("target scan commit returned error: %v", err)
			}
			return
		}
		if err == nil || err.Error() != "document_scan_not_authorized" {
			t.Fatalf("unexpected earlier scan result: %v", err)
		}
	}
	t.Fatal("target scan job was starved")
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func ptrTime(value time.Time) *time.Time { return &value }
