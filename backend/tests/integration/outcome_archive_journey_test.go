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
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func TestLiveReportedOutcomeCorrectionAndArchiveJourney(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenant := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	tenantB := livePool(t, "BIDORA_TEST_TENANT_B_RUNTIME_DSN")
	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := insertWorkflowAuthority(t, control, livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN").Database)
	grant := func(capability, effect string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
			_, err := db.Exec(ctx, `INSERT INTO tenancy.role_grants(id,membership_id,role_key,capability,effect,created_by,created_at) VALUES($1,$2,'OUTCOME_MANAGER',$3,$4,$5,$6)`, id, fixture.membershipID, capability, effect, liveUUID(fixture.actorID), at)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	grant(authzapp.ReportOutcomeCapability, "ALLOW")
	grant(authzapp.ArchiveBidCapability, "ALLOW")
	fixture.subject.ContextGeneration += 2
	store := attachWorkflowServices(t, &fixture, tenant, at.Add(time.Minute))
	_, submitted := establishBidWorkspace(t, tenant, &fixture, at)
	if err := observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		command, err := db.Exec(ctx, `UPDATE bid.workspaces SET state='SUBMITTED',version=3,updated_at=$2 WHERE id=$1 AND state='BID' AND version=2`, liveUUID(submitted.WorkspaceID), at.Add(2*time.Minute))
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return fmt.Errorf("submitted prerequisite not updated")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	storage := newSyntheticDocumentStorage()
	outcomes, err := opportunityapp.NewReportedOutcomeService(fixture.authorizer, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(10 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	base := opportunityapp.ReportOutcomeInput{Subject: fixture.subject, BidID: submitted.WorkspaceID, CorrelationID: liveID(), Outcome: "PENDING", SourceClass: "INTERNAL_OBSERVATION", Reason: "Awaiting an external buyer notice.", Lessons: "No conclusion is inferred from silence.", ReportedTimeText: at.Add(-time.Hour).In(time.FixedZone("EAT", 3*60*60)).Format(time.RFC3339), ExpectedSequence: 0, ExpectedWorkspaceVersion: 3, IdempotencyKey: uuid.NewString()}
	pending, err := outcomes.Report(context.Background(), base)
	if err != nil || pending.Outcome.Sequence != 1 || !pending.Outcome.ProofVersionID.IsZero() || pending.Outcome.VerificationStatus != "REPORTED_UNVERIFIED" || pending.Outcome.Provenance != "HUMAN_REPORTED_EXTERNAL_OUTCOME" {
		t.Fatalf("pending outcome: %+v %v", pending, err)
	}
	pendingReplay, err := outcomes.Report(context.Background(), base)
	if err != nil || !pendingReplay.Replay || pendingReplay.Outcome.ID != pending.Outcome.ID {
		t.Fatalf("pending replay: %+v %v", pendingReplay, err)
	}
	bodyMismatch := base
	bodyMismatch.Reason = "Different retry body."
	bodyMismatch.CorrelationID = liveID()
	if _, err = outcomes.Report(context.Background(), bodyMismatch); err == nil || err.Error() != "outcome_conflict" {
		t.Fatalf("outcome body mismatch accepted: %v", err)
	}
	unknownInput := base
	unknownInput.Outcome = "UNKNOWN"
	unknownInput.Reason = "The external result remains unknown."
	unknownInput.Lessons = "Retain evidence without guessing."
	unknownInput.SupersedesID = pending.Outcome.ID
	unknownInput.CorrectionReason = "Updated after the expected notice window elapsed."
	unknownInput.ExpectedSequence = 1
	unknownInput.IdempotencyKey = uuid.NewString()
	unknownInput.CorrelationID = liveID()
	unknown, err := outcomes.Report(context.Background(), unknownInput)
	if err != nil || unknown.Outcome.Sequence != 2 || unknown.Outcome.SupersedesID != pending.Outcome.ID || !unknown.Outcome.ProofVersionID.IsZero() {
		t.Fatalf("unknown correction: %+v %v", unknown, err)
	}
	terminalWithoutProof := unknownInput
	terminalWithoutProof.Outcome = "LOST"
	terminalWithoutProof.ExpectedSequence = 2
	terminalWithoutProof.SupersedesID = unknown.Outcome.ID
	terminalWithoutProof.CorrectionReason = "Terminal report requires proof."
	terminalWithoutProof.IdempotencyKey = uuid.NewString()
	terminalWithoutProof.CorrelationID = liveID()
	if _, err = outcomes.Report(context.Background(), terminalWithoutProof); err == nil || err.Error() != "outcome_invalid" {
		t.Fatalf("terminal outcome without proof accepted: %v", err)
	}

	proofBody := []byte("%PDF-1.7 synthetic reported outcome proof")
	proofHash := sha256.Sum256(proofBody)
	if _, err = outcomes.InitiateProof(context.Background(), opportunityapp.InitiateOutcomeProofInput{Subject: fixture.subject, BidID: submitted.WorkspaceID, CorrelationID: liveID(), SizeBytes: opportunityapp.MaxTenderDocumentBytes + 1, SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "outcome_proof_invalid" {
		t.Fatalf("oversized outcome proof accepted: %v", err)
	}
	initiated, err := outcomes.InitiateProof(context.Background(), opportunityapp.InitiateOutcomeProofInput{Subject: fixture.subject, BidID: submitted.WorkspaceID, CorrelationID: liveID(), SizeBytes: int64(len(proofBody)), SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()})
	if err != nil || initiated.Session.MediaType != opportunityapp.PDFMediaType || initiated.Session.ExpiresAt.Sub(at.Add(10*time.Minute)) != opportunityapp.UploadInstructionTTL {
		t.Fatalf("outcome proof initiation: %+v %v", initiated, err)
	}
	storage.put(initiated.Session.ObjectKey, "outcome-proof-v1", opportunityapp.PDFMediaType, proofBody, proofHash)
	completed, err := outcomes.CompleteProof(context.Background(), opportunityapp.CompleteOutcomeProofInput{Subject: fixture.subject, BidID: submitted.WorkspaceID, UploadID: initiated.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil || completed.ScanState != "QUARANTINED" {
		t.Fatalf("outcome proof completion: %+v %v", completed, err)
	}
	terminalQuarantined := terminalWithoutProof
	terminalQuarantined.ProofVersionID = completed.VersionID
	if _, err = outcomes.Report(context.Background(), terminalQuarantined); err == nil || err.Error() != "outcome_conflict" {
		t.Fatalf("quarantined terminal proof accepted: %v", err)
	}
	if err = fixture.authorizer.AuthorizeDocumentProcessing(context.Background(), fixture.actorID, fixture.tenantID, 1, authzapp.ReportOutcomeCapability); err != nil {
		t.Fatalf("outcome proof processing authorization unavailable: %v", err)
	}
	processingStore, err := opportunitypersistence.NewDocumentProcessingStore(tenant, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	processor, err := opportunityapp.NewDocumentProcessor(processingStore, storage, syntheticScanner{out: opportunityapp.ScanOutcome{Clean: true, ResultCode: "CLEAN", Engine: "SYNTHETIC_CLAMAV"}}, unusedExtractor{}, fixture.authorizer, randomWorkflowIDs{}, workflowClock{at: at.Add(11 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	clean := false
	for attempt := 0; attempt < 50; attempt++ {
		processed, scanErr := processor.ProcessNextScan(context.Background())
		if rowCount(t, observer, `SELECT count(*) FROM documents.document_versions WHERE id=$1 AND scan_state='CLEAN'`, liveUUID(completed.VersionID)) == 1 {
			if scanErr != nil {
				t.Fatalf("outcome proof scan commit: %v", scanErr)
			}
			clean = true
			break
		}
		if !processed {
			t.Fatalf("scan queue exhausted before outcome proof: %v", scanErr)
		}
		if scanErr != nil && rowCount(t, observer, `SELECT count(*) FROM jobs.jobs WHERE document_version_id=$1 AND state='RUNNING'`, liveUUID(completed.VersionID)) != 0 {
			t.Fatalf("outcome proof scan failed: %v", errors.Unwrap(scanErr))
		}
	}
	if !clean {
		t.Fatal("outcome proof scan was starved")
	}

	outcomes, err = opportunityapp.NewReportedOutcomeService(fixture.authorizer, store, storage, randomWorkflowIDs{}, workflowClock{at: at.Add(12 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	lostInput := terminalQuarantined
	lostInput.Reason = "Buyer notice reports the external bid was unsuccessful."
	lostInput.Lessons = "Improve response lead time."
	lostInput.CorrectionReason = "Buyer notice supplied terminal evidence."
	lostInput.IdempotencyKey = uuid.NewString()
	lostInput.CorrelationID = liveID()
	lost, err := outcomes.Report(context.Background(), lostInput)
	if err != nil || lost.Outcome.Sequence != 3 || lost.Outcome.ProofVersionID != completed.VersionID || !lost.Outcome.ReportedAt.Before(lost.Outcome.RecordedAt) {
		t.Fatalf("terminal outcome: %+v %v", lost, err)
	}
	type outcomeResult struct {
		v   opportunityapp.ReportOutcomeResult
		err error
		key string
	}
	results := make(chan outcomeResult, 2)
	start := make(chan struct{})
	for i, value := range []string{"WON", "DISQUALIFIED"} {
		key := uuid.NewString()
		go func(index int, outcome, key string) {
			<-start
			in := lostInput
			in.Outcome = outcome
			in.Reason = "Concurrent corrected terminal report."
			in.Lessons = "Retain both attempted corrections without overwrite."
			in.SupersedesID = lost.Outcome.ID
			in.ExpectedSequence = 3
			in.CorrectionReason = "New external evidence prompted correction."
			in.IdempotencyKey = key
			in.CorrelationID = liveID()
			v, e := outcomes.Report(context.Background(), in)
			results <- outcomeResult{v, e, key}
		}(i, value, key)
	}
	close(start)
	first, second := <-results, <-results
	if !((first.err == nil && second.err != nil && second.err.Error() == "outcome_conflict") || (second.err == nil && first.err != nil && first.err.Error() == "outcome_conflict")) {
		t.Fatalf("outcome correction race: %+v/%v %+v/%v", first.v, first.err, second.v, second.err)
	}
	winner := first
	if winner.err != nil {
		winner = second
	}
	if winner.v.Outcome.Sequence != 4 || winner.v.Outcome.SupersedesID != lost.Outcome.ID {
		t.Fatalf("winning correction lineage: %+v", winner.v)
	}
	current, err := outcomes.Current(context.Background(), fixture.subject, submitted.WorkspaceID)
	if err != nil || current.ID != winner.v.Outcome.ID {
		t.Fatalf("current outcome: %+v %v", current, err)
	}
	history, err := outcomes.History(context.Background(), fixture.subject, submitted.WorkspaceID)
	if err != nil || len(history) != 4 || history[0].Sequence != 4 || history[3].Sequence != 1 {
		t.Fatalf("outcome history: %+v %v", history, err)
	}
	if rowCount(t, observer, `SELECT count(*) FROM bidora_eventing.outbox_events WHERE aggregate_id=$1 AND event_type='bid.reported_outcome.recorded.v1' AND actor_id=$2 AND payload->>'authoritative_buyer_decision'='false' AND payload->>'contract_award_confirmed'='false'`, liveUUID(submitted.WorkspaceID), liveUUID(fixture.actorID)) != 4 {
		t.Fatal("outcome outbox provenance or non-claim flags drifted")
	}
	denyID := grant(authzapp.ReportOutcomeCapability, "DENY")
	fixture.subject.ContextGeneration++
	denied := lostInput
	denied.ExpectedSequence = 4
	denied.SupersedesID = winner.v.Outcome.ID
	denied.IdempotencyKey = uuid.NewString()
	denied.CorrelationID = liveID()
	if _, err = outcomes.Report(context.Background(), denied); err == nil || err.Error() != "outcome_not_authorized" {
		t.Fatalf("active outcome DENY did not win: %v", err)
	}
	if err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE tenancy.role_grants SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, denyID, at.Add(13*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	fixture.subject.ContextGeneration++

	queuedUpload, err := outcomes.InitiateProof(context.Background(), opportunityapp.InitiateOutcomeProofInput{Subject: fixture.subject, BidID: submitted.WorkspaceID, CorrelationID: liveID(), SizeBytes: int64(len(proofBody)), SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	storage.put(queuedUpload.Session.ObjectKey, "outcome-proof-pending-job", opportunityapp.PDFMediaType, proofBody, proofHash)
	if _, err = outcomes.CompleteProof(context.Background(), opportunityapp.CompleteOutcomeProofInput{Subject: fixture.subject, BidID: submitted.WorkspaceID, UploadID: queuedUpload.Session.ID, CorrelationID: liveID(), IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	if _, err = outcomes.InitiateProof(context.Background(), opportunityapp.InitiateOutcomeProofInput{Subject: fixture.subject, BidID: submitted.WorkspaceID, CorrelationID: liveID(), SizeBytes: int64(len(proofBody)), SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	blockedPreparations, err := opportunityapp.NewArchivePreparationService(fixture.authorizer, store, workflowClock{at: at.Add(13 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := blockedPreparations.Get(context.Background(), fixture.subject, submitted.WorkspaceID)
	if err != nil || blocked.CanArchive || !containsString(blocked.Blockers, "PENDING_UPLOADS") || !containsString(blocked.Blockers, "PENDING_BACKGROUND_JOBS") || !containsString(blocked.Blockers, "WORKSPACE_STATE_OR_OUTCOME_INELIGIBLE") {
		t.Fatalf("submitted archive blockers: %+v %v", blocked, err)
	}

	store = attachWorkflowServices(t, &fixture, tenant, at.Add(21*time.Minute))
	_, noBid := establishBidWorkspace(t, tenant, &fixture, at.Add(20*time.Minute))
	if err = observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		tag, e := db.Exec(ctx, `UPDATE bid.workspaces SET state='NO_BID',version=3,updated_at=$2 WHERE id=$1 AND state='BID' AND version=2`, liveUUID(noBid.WorkspaceID), at.Add(21*time.Minute))
		if e == nil && tag.RowsAffected() != 1 {
			return fmt.Errorf("no-bid prerequisite not updated")
		}
		return e
	}); err != nil {
		t.Fatal(err)
	}
	threadID := uuid.New()
	if err = observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `INSERT INTO bid.review_threads(id,bid_id,target_type,target_id,subject,status,version,created_by,created_at) VALUES($1,$2,'BID',$2,'Synthetic open archive blocker','OPEN',1,$3,$4)`, threadID, liveUUID(noBid.WorkspaceID), liveUUID(fixture.actorID), at.Add(21*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	preparations, err := opportunityapp.NewArchivePreparationService(fixture.authorizer, store, workflowClock{at: at.Add(22 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	openBlocked, err := preparations.Get(context.Background(), fixture.subject, noBid.WorkspaceID)
	if err != nil || openBlocked.CanArchive || !containsString(openBlocked.Blockers, "OPEN_REVIEW_THREADS") {
		t.Fatalf("open-review archive blocker: %+v %v", openBlocked, err)
	}
	if err = observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE bid.review_threads SET status='RESOLVED',version=2,resolved_by=$2,resolved_at=$3 WHERE id=$1 AND status='OPEN'`, threadID, liveUUID(fixture.actorID), at.Add(22*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	ready, err := preparations.Get(context.Background(), fixture.subject, noBid.WorkspaceID)
	if err != nil || !ready.CanArchive || len(ready.Blockers) != 0 || !ready.ArchivePolicyConfigured || !ready.ReadOnlyClosureSupported {
		t.Fatalf("NO_BID archive readiness: %+v %v", ready, err)
	}
	archiveDenyID := grant(authzapp.ArchiveBidCapability, "DENY")
	fixture.subject.ContextGeneration++
	deniedArchives, err := opportunityapp.NewBidArchiveService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(23 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = deniedArchives.Archive(context.Background(), opportunityapp.ArchiveBidInput{Subject: fixture.subject, BidID: noBid.WorkspaceID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 3, Reason: "Active DENY must prevent archive.", IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "bid_archive_not_authorized" {
		t.Fatalf("active archive DENY did not win: %v", err)
	}
	if err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE tenancy.role_grants SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, archiveDenyID, at.Add(23*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	fixture.subject.ContextGeneration++
	archives, err := opportunityapp.NewBidArchiveService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(23 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	archiveInput := func(key string) opportunityapp.ArchiveBidInput {
		return opportunityapp.ArchiveBidInput{Subject: fixture.subject, BidID: noBid.WorkspaceID, CorrelationID: liveID(), ExpectedWorkspaceVersion: 3, Reason: "Close this NO_BID workspace as immutable read-only history.", IdempotencyKey: key}
	}
	archiveResults := make(chan struct {
		v   opportunityapp.BidArchiveResult
		err error
		key string
	}, 2)
	archiveStart := make(chan struct{})
	for range 2 {
		key := uuid.NewString()
		go func() {
			<-archiveStart
			v, e := archives.Archive(context.Background(), archiveInput(key))
			archiveResults <- struct {
				v   opportunityapp.BidArchiveResult
				err error
				key string
			}{v, e, key}
		}()
	}
	close(archiveStart)
	a1, a2 := <-archiveResults, <-archiveResults
	if !((a1.err == nil && a2.err != nil && a2.err.Error() == "bid_archive_conflict") || (a2.err == nil && a1.err != nil && a1.err.Error() == "bid_archive_conflict")) {
		t.Fatalf("archive race: %+v/%v %+v/%v", a1.v, a1.err, a2.v, a2.err)
	}
	archiveWinner := a1
	if archiveWinner.err != nil {
		archiveWinner = a2
	}
	if archiveWinner.v.Archive.WorkspaceVersion != 4 || archiveWinner.v.Archive.ArchivedBy != fixture.actorID {
		t.Fatalf("archive evidence: %+v", archiveWinner.v)
	}
	archiveReplay, err := archives.Archive(context.Background(), archiveInput(archiveWinner.key))
	if err != nil || !archiveReplay.Replay || archiveReplay.Archive.ID != archiveWinner.v.Archive.ID {
		t.Fatalf("archive replay: %+v %v", archiveReplay, err)
	}
	archiveMismatch := archiveInput(archiveWinner.key)
	archiveMismatch.Reason = "Conflicting archive retry body."
	if _, err = archives.Archive(context.Background(), archiveMismatch); err == nil || err.Error() != "bid_archive_conflict" {
		t.Fatalf("archive body mismatch accepted: %v", err)
	}
	archiveRead, err := archives.Get(context.Background(), fixture.subject, noBid.WorkspaceID)
	if err != nil || archiveRead.ID != archiveWinner.v.Archive.ID {
		t.Fatalf("archive read: %+v %v", archiveRead, err)
	}
	bidRead, err := fixture.reader.GetBid(context.Background(), fixture.subject, noBid.WorkspaceID)
	if err != nil || bidRead.ArchivedAt == nil || bidRead.ArchiveReason == nil || *bidRead.ArchiveReason != archiveWinner.v.Archive.Reason {
		t.Fatalf("authorized archived bid history: %+v %v", bidRead, err)
	}
	archivedPrep, err := preparations.Get(context.Background(), fixture.subject, noBid.WorkspaceID)
	if err != nil || archivedPrep.CanArchive || !containsString(archivedPrep.Blockers, "WORKSPACE_ALREADY_ARCHIVED") {
		t.Fatalf("archived preparation: %+v %v", archivedPrep, err)
	}
	if err = observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE bid.workspaces SET updated_at=$2 WHERE id=$1`, liveUUID(noBid.WorkspaceID), at.Add(24*time.Minute))
		return e
	}); err == nil {
		t.Fatal("archived workspace mutation bypassed immutable trigger")
	}
	if _, err = outcomes.InitiateProof(context.Background(), opportunityapp.InitiateOutcomeProofInput{Subject: fixture.subject, BidID: noBid.WorkspaceID, CorrelationID: liveID(), SizeBytes: int64(len(proofBody)), SHA256Hex: fmt.Sprintf("%x", proofHash), IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "outcome_conflict" {
		t.Fatalf("archived workspace accepted new proof work: %v", err)
	}
	if rowCount(t, observer, `SELECT count(*) FROM bidora_eventing.outbox_events WHERE aggregate_id=$1 AND event_type='bid.archived.v1' AND actor_id=$2 AND payload->>'read_only'='true' AND payload->>'deleted'='false'`, liveUUID(noBid.WorkspaceID), liveUUID(fixture.actorID)) != 1 {
		t.Fatal("archive outbox provenance or non-destructive flags drifted")
	}

	storeB, err := opportunitypersistence.NewWorkflowStore(tenantB, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	authority := opportunityapp.WorkflowAuthority{ActorID: fixture.actorID, TenantID: fixture.tenantID, AssignmentGeneration: 1}
	if _, err = storeB.GetCurrentReportedOutcome(context.Background(), authority, submitted.WorkspaceID); !errors.Is(err, opportunityapp.ErrReportedOutcomeNotFound) {
		t.Fatalf("tenant B exposed tenant A outcome: %v", err)
	}
	if _, err = storeB.GetBidArchive(context.Background(), authority, noBid.WorkspaceID); !errors.Is(err, opportunityapp.ErrBidArchiveNotFound) {
		t.Fatalf("tenant B exposed tenant A archive: %v", err)
	}
}
