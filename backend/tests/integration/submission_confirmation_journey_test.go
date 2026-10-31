package integration_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	opportunitypersistence "bidora/backend/internal/opportunity/persistence"
	"bidora/backend/internal/opportunity/proposalexport"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func TestLiveExternalSubmissionProofAndConfirmationJourney(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenant := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	tenantB := livePool(t, "BIDORA_TEST_TENANT_B_RUNTIME_DSN")
	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := insertWorkflowAuthority(t, control, livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN").Database)
	grant := func(user opportunityapp.ID, capability, effect string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
			_, err := db.Exec(ctx, `INSERT INTO tenancy.role_grants(id,membership_id,role_key,capability,effect,created_by,created_at) SELECT $1,m.id,'SUBMISSION_REVIEWER',$4,$5,$2,$6 FROM tenancy.memberships m WHERE m.user_id=$2 AND m.tenant_id=$3 AND m.state='ACTIVE'`, id, liveUUID(user), liveUUID(fixture.tenantID), capability, effect, at)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	for _, capability := range []string{authzapp.ReviewProposalCapability, authzapp.ApproveProposalCapability, authzapp.ReportSubmissionCapability, authzapp.VerifySubmissionCapability} {
		grant(fixture.actorID, capability, "ALLOW")
	}
	for _, capability := range []string{authzapp.ReviewProposalCapability, authzapp.ApproveProposalCapability, authzapp.VerifySubmissionCapability} {
		grant(fixture.ownerID, capability, "ALLOW")
	}
	fixture.subject.ContextGeneration += 4
	ownerSubject := opportunityapp.WorkflowSubject{IdentityID: fixture.ownerID, SelectedTenantID: fixture.tenantID, ContextGeneration: 5}
	store := attachWorkflowServices(t, &fixture, tenant, at.Add(time.Minute))
	captured, workspace := establishBidWorkspace(t, tenant, &fixture, at)
	seedV7OrganizationAndResponse(t, observer, fixture, captured.SourceVersionID, workspace.WorkspaceID, at)

	proposals, err := opportunityapp.NewProposalService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := proposals.Save(context.Background(), opportunityapp.SaveProposalInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, CorrelationID: liveID(), Title: "Approved submission candidate", Recipient: "Evaluation Committee", LetterheadLines: []string{"Nairobi, Kenya"}, Introduction: "Human-authored submission candidate.", Closing: "Human-authored closing.", ChangeNote: "Exact V8 candidate.", ExpectedProposalVersion: 0, ExpectedWorkspaceVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil || proposal.Proposal.WorkspaceVersion != 3 {
		t.Fatalf("proposal prerequisite: %+v %v", proposal, err)
	}
	decisions, err := opportunityapp.NewInternalReviewDecisionService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(3 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	review, err := decisions.Record(context.Background(), opportunityapp.RecordInternalReviewDecisionInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, ProposalVersionID: proposal.Proposal.ProposalVersionID, CorrelationID: liveID(), Decision: "REVIEW_COMPLETE", Rationale: "Independent exact-revision review complete.", ExpectedPreviousSequence: 0, IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	packages, err := opportunityapp.NewProposalPackageService(fixture.authorizer, store, store, proposalexport.New(), randomWorkflowIDs{}, workflowClock{at: at.Add(4 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := packages.Prepare(context.Background(), opportunityapp.PrepareProposalPackageInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, ProposalVersionID: proposal.Proposal.ProposalVersionID, CorrelationID: liveID(), ExpectedProposalVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: uuid.NewString()})
	if err != nil || !manifest.Manifest.ReadinessReady {
		t.Fatalf("package prerequisite: %+v %v", manifest, err)
	}
	approvals, err := opportunityapp.NewProposalApprovalService(fixture.authorizer, store, store, randomWorkflowIDs{}, workflowClock{at: at.Add(5 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	approved, err := approvals.Approve(context.Background(), opportunityapp.ApproveProposalInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, ManifestID: manifest.Manifest.ID, CorrelationID: liveID(), ExpectedProposalVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: uuid.NewString(), Rationale: "Independent package approval.", SessionAuthenticatedAt: at.Add(4 * time.Minute), AssuranceAt: at.Add(4 * time.Minute), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP"})
	if err != nil || approved.Approval.ReviewDecisionID != review.Decision.ID {
		t.Fatalf("approval prerequisite: %+v %v", approved, err)
	}

	reports, err := opportunityapp.NewExternalActionEvidenceService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(7 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	reportedText := at.Add(-2 * time.Hour).In(time.FixedZone("EAT", 3*60*60)).Format(time.RFC3339)
	reference := "portal-reference-synthetic"
	reportKey := uuid.NewString()
	reportInput := opportunityapp.ReportExternalActionEvidenceInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, PackageManifestID: manifest.Manifest.ID, CorrelationID: liveID(), ExternalSystemName: "Synthetic External Portal", ExternalReference: &reference, ReportedActionTimeText: reportedText, ReportNote: "Human reports an external action; Bidora did not transmit it.", ExpectedWorkspaceVersion: 3, IdempotencyKey: reportKey}
	initial, err := reports.Report(context.Background(), reportInput)
	if err != nil || initial.Evidence.VerificationStatus != "REPORTED_UNVERIFIED" || initial.Evidence.ReportedActionTimeText != reportedText || !initial.Evidence.ReportedActionAt.Before(initial.Evidence.RecordedAt) {
		t.Fatalf("external report: %+v %v", initial, err)
	}
	replayReport, err := reports.Report(context.Background(), reportInput)
	if err != nil || !replayReport.Replay || replayReport.Evidence.ID != initial.Evidence.ID {
		t.Fatalf("report replay: %+v %v", replayReport, err)
	}
	conflictReport := reportInput
	conflictReport.ReportNote = "Conflicting body."
	conflictReport.CorrelationID = liveID()
	if _, err = reports.Report(context.Background(), conflictReport); err == nil || err.Error() != "external_action_evidence_conflict" {
		t.Fatalf("conflicting report replay accepted: %v", err)
	}
	foreignCaptured, foreignWorkspace := establishBidWorkspace(t, tenant, &fixture, at)
	_ = foreignCaptured
	foreign := reportInput
	foreign.BidID = foreignWorkspace.WorkspaceID
	foreign.ExpectedWorkspaceVersion = 2
	foreign.IdempotencyKey = uuid.NewString()
	foreign.CorrelationID = liveID()
	if _, err = reports.Report(context.Background(), foreign); err == nil || err.Error() != "external_action_evidence_conflict" {
		t.Fatalf("foreign-bid package accepted: %v", err)
	}

	correction := reportInput
	correction.SupersedesEvidenceID = initial.Evidence.ID
	correction.ReportNote = "Corrected human report retaining the original immutable report."
	correction.IdempotencyKey, correction.CorrelationID = uuid.NewString(), liveID()
	corrected, err := reports.Report(context.Background(), correction)
	if err != nil || corrected.Evidence.SupersedesEvidenceID != initial.Evidence.ID {
		t.Fatalf("corrected report: %+v %v", corrected, err)
	}
	history, err := reports.List(context.Background(), opportunityapp.ExternalActionEvidenceHistoryInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, Limit: 10})
	if err != nil || len(history.Items) != 2 || history.HasMore {
		t.Fatalf("report history: %+v %v", history, err)
	}
	reconciliation, err := reports.Reconciliation(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || reconciliation.ActiveReportCount != 1 || len(reconciliation.ActiveReports) != 1 || reconciliation.ActiveReports[0].EvidenceID != corrected.Evidence.ID || reconciliation.CanMarkSubmitted || !reconciliation.ProofPolicyConfigured {
		t.Fatalf("evidence reconciliation: %+v %v", reconciliation, err)
	}

	storage := newSyntheticDocumentStorage()
	submissions, err := opportunityapp.NewSubmissionService(fixture.authorizer, store, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(8 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	proofBody := []byte("%PDF-1.7 synthetic external submission proof")
	proofHash := sha256.Sum256(proofBody)
	if _, err = submissions.Initiate(context.Background(), opportunityapp.InitiateSubmissionProofInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, CorrelationID: liveID(), SizeBytes: opportunityapp.MaxTenderDocumentBytes + 1, SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "submission_proof_invalid" {
		t.Fatalf("oversized proof accepted: %v", err)
	}
	if _, err = submissions.Initiate(context.Background(), opportunityapp.InitiateSubmissionProofInput{Subject: fixture.subject, BidID: foreignWorkspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, CorrelationID: liveID(), SizeBytes: int64(len(proofBody)), SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "submission_conflict" {
		t.Fatalf("foreign-bid evidence accepted for proof upload: %v", err)
	}
	initiated, err := submissions.Initiate(context.Background(), opportunityapp.InitiateSubmissionProofInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, CorrelationID: liveID(), SizeBytes: int64(len(proofBody)), SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()})
	if err != nil || initiated.Session.MediaType != opportunityapp.PDFMediaType || initiated.Session.ExpiresAt.Sub(at.Add(8*time.Minute)) != opportunityapp.UploadInstructionTTL {
		t.Fatalf("proof initiation: %+v %v", initiated, err)
	}
	storage.put(initiated.Session.ObjectKey, "", opportunityapp.PDFMediaType, proofBody, proofHash)
	if _, err = submissions.Complete(context.Background(), opportunityapp.CompleteSubmissionProofInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "submission_proof_mismatch" {
		t.Fatalf("missing object version accepted: %v", err)
	}
	wrongHash := sha256.Sum256([]byte("wrong proof"))
	storage.put(initiated.Session.ObjectKey, "proof-v1", opportunityapp.PDFMediaType, proofBody, wrongHash)
	if _, err = submissions.Complete(context.Background(), opportunityapp.CompleteSubmissionProofInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "submission_proof_mismatch" {
		t.Fatalf("proof checksum mismatch accepted: %v", err)
	}
	storage.put(initiated.Session.ObjectKey, "proof-v2", opportunityapp.PDFMediaType, proofBody, proofHash)
	completed, err := submissions.Complete(context.Background(), opportunityapp.CompleteSubmissionProofInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil || completed.ScanState != "QUARANTINED" {
		t.Fatalf("proof completion: %+v %v", completed, err)
	}
	unclean := opportunityapp.ConfirmSubmissionInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, ManifestID: manifest.Manifest.ID, ProofVersionID: completed.VersionID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 3, Rationale: "Proof is not yet CLEAN.", IdempotencyKey: uuid.NewString(), SessionAuthenticatedAt: at.Add(7 * time.Minute), AssuranceAt: at.Add(7 * time.Minute), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP"}
	if _, err = submissions.Confirm(context.Background(), unclean); err == nil || err.Error() != "submission_conflict" {
		t.Fatalf("quarantined proof authorized submission: %v", err)
	}
	if err = fixture.authorizer.AuthorizeDocumentProcessing(context.Background(), fixture.actorID, fixture.tenantID, 1, authzapp.ReportSubmissionCapability); err != nil {
		t.Fatalf("submission proof processing authorization unavailable: %v", err)
	}
	processingStore, err := opportunitypersistence.NewDocumentProcessingStore(tenant, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	processor, err := opportunityapp.NewDocumentProcessor(processingStore, storage, syntheticScanner{out: opportunityapp.ScanOutcome{Clean: true, ResultCode: "CLEAN", Engine: "SYNTHETIC_CLAMAV"}}, unusedExtractor{}, fixture.authorizer, randomWorkflowIDs{}, workflowClock{at: at.Add(9 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	clean := false
	for attempt := 0; attempt < 50; attempt++ {
		processed, scanErr := processor.ProcessNextScan(context.Background())
		if rowCount(t, observer, `SELECT count(*) FROM documents.document_versions WHERE id=$1 AND scan_state='CLEAN'`, liveUUID(completed.VersionID)) == 1 {
			if scanErr != nil {
				t.Fatalf("target scan commit returned error: %v", scanErr)
			}
			clean = true
			break
		}
		if !processed {
			t.Fatalf("scan queue exhausted before submission proof: %v", scanErr)
		}
		if scanErr != nil && rowCount(t, observer, `SELECT count(*) FROM jobs.jobs WHERE document_version_id=$1 AND state='RUNNING'`, liveUUID(completed.VersionID)) != 0 {
			t.Fatalf("submission proof scan commit failed: %v", errors.Unwrap(scanErr))
		}
		if rowCount(t, observer, `SELECT count(*) FROM jobs.jobs WHERE document_version_id=$1 AND state IN ('FAILED','CANCELLED')`, liveUUID(completed.VersionID)) != 0 {
			t.Fatalf("submission proof scan failed closed unexpectedly: %v", scanErr)
		}
	}
	if !clean {
		t.Fatal("submission proof scan was starved")
	}
	if rowCount(t, observer, `SELECT count(*) FROM documents.document_versions v JOIN documents.documents d ON d.id=v.document_id WHERE v.id=$1 AND v.scan_state='CLEAN' AND v.object_version='proof-v2' AND d.document_kind='SUBMISSION_PROOF' AND d.external_action_evidence_id=$2`, liveUUID(completed.VersionID), liveUUID(corrected.Evidence.ID)) != 1 {
		t.Fatal("exact CLEAN submission-proof version missing")
	}

	submissions, err = opportunityapp.NewSubmissionService(fixture.authorizer, store, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(10 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	confirmInput := func(subject opportunityapp.WorkflowSubject, key string) opportunityapp.ConfirmSubmissionInput {
		return opportunityapp.ConfirmSubmissionInput{Subject: subject, BidID: workspace.WorkspaceID, EvidenceID: corrected.Evidence.ID, ManifestID: manifest.Manifest.ID, ProofVersionID: completed.VersionID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 3, Rationale: "Independent human verified the exact CLEAN proof and package lineage.", IdempotencyKey: key, SessionAuthenticatedAt: at.Add(9 * time.Minute), AssuranceAt: at.Add(9 * time.Minute), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP"}
	}
	if _, err = submissions.Confirm(context.Background(), confirmInput(fixture.subject, uuid.NewString())); err == nil || err.Error() != "submission_conflict" {
		t.Fatalf("reporter self-verification accepted: %v", err)
	}
	stale := confirmInput(ownerSubject, uuid.NewString())
	stale.AssuranceAt = at.Add(-20 * time.Minute)
	if _, err = submissions.Confirm(context.Background(), stale); err == nil || err.Error() != "submission_fresh_auth_required" {
		t.Fatalf("stale assurance accepted: %v", err)
	}
	denyID := grant(fixture.ownerID, authzapp.VerifySubmissionCapability, "DENY")
	ownerSubject.ContextGeneration = 6
	if _, err = submissions.Confirm(context.Background(), confirmInput(ownerSubject, uuid.NewString())); err == nil || err.Error() != "submission_not_authorized" {
		t.Fatalf("submission verification DENY did not win: %v", err)
	}
	if err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE tenancy.role_grants SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, denyID, at.Add(9*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	ownerSubject.ContextGeneration = 7

	type confirmOutcome struct {
		result opportunityapp.SubmissionConfirmationResult
		err    error
		key    string
	}
	outcomes := make(chan confirmOutcome, 2)
	start := make(chan struct{})
	for range 2 {
		key := uuid.NewString()
		go func() {
			<-start
			result, callErr := submissions.Confirm(context.Background(), confirmInput(ownerSubject, key))
			outcomes <- confirmOutcome{result, callErr, key}
		}()
	}
	close(start)
	first, second := <-outcomes, <-outcomes
	if !((first.err == nil && second.err != nil && (second.err.Error() == "submission_conflict" || second.err.Error() == "submission_blocked")) || (second.err == nil && first.err != nil && (first.err.Error() == "submission_conflict" || first.err.Error() == "submission_blocked"))) {
		t.Fatalf("confirmation race did not preserve one winner: %+v/%v %+v/%v", first.result, first.err, second.result, second.err)
	}
	winner := first
	if winner.err != nil {
		winner = second
	}
	confirmation := winner.result.Confirmation
	if confirmation.SubmissionBasis != "HUMAN_CONFIRMED_EXTERNAL_ACTION" || confirmation.ReporterID != fixture.actorID || confirmation.VerifierID != fixture.ownerID || confirmation.ManifestID != manifest.Manifest.ID || confirmation.ProofVersionID != completed.VersionID || confirmation.WorkspaceVersion != 4 || !confirmation.ReportedActionAt.Before(confirmation.ReportRecordedAt) || !confirmation.ReportRecordedAt.Before(confirmation.ConfirmedAt) {
		t.Fatalf("confirmation lineage drifted: %+v", confirmation)
	}
	replayed, err := submissions.Confirm(context.Background(), confirmInput(ownerSubject, winner.key))
	if err != nil || !replayed.Replay || replayed.Confirmation.ID != confirmation.ID {
		t.Fatalf("lost-response replay failed after SUBMITTED transition: %+v %v", replayed, err)
	}
	conflicting := confirmInput(ownerSubject, winner.key)
	conflicting.Rationale = "Conflicting retry body."
	if _, err = submissions.Confirm(context.Background(), conflicting); err == nil || err.Error() != "submission_conflict" {
		t.Fatalf("conflicting confirmation retry accepted: %v", err)
	}
	read, err := submissions.Get(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || read.ID != confirmation.ID || read.Rationale != confirmation.Rationale {
		t.Fatalf("submission read: %+v %v", read, err)
	}
	if rowCount(t, observer, `SELECT count(*) FROM bid.workspaces WHERE id=$1 AND state='SUBMITTED' AND version=4`, liveUUID(workspace.WorkspaceID)) != 1 {
		t.Fatal("workspace was not atomically transitioned to SUBMITTED")
	}
	if rowCount(t, observer, `SELECT count(*) FROM bidora_eventing.outbox_events WHERE actor_id=$1 AND event_type='bid.external_submission.human_confirmed.v1' AND payload->>'bidora_transmitted'='false' AND payload->>'external_acceptance_confirmed'='false' AND payload->>'on_time_submission_confirmed'='false'`, liveUUID(fixture.ownerID)) != 1 {
		t.Fatal("submission outbox provenance or non-claim flags drifted")
	}

	storeB, err := opportunitypersistence.NewWorkflowStore(tenantB, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	authority := opportunityapp.WorkflowAuthority{ActorID: fixture.actorID, TenantID: fixture.tenantID, AssignmentGeneration: 1}
	if _, err = storeB.GetSubmissionConfirmationByBid(context.Background(), authority, workspace.WorkspaceID); !errors.Is(err, opportunityapp.ErrSubmissionNotFound) {
		t.Fatalf("tenant B exposed tenant A confirmation: %v", err)
	}
}
