package integration_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	opportunitypersistence "bidora/backend/internal/opportunity/persistence"
	"bidora/backend/internal/opportunity/proposalexport"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func TestLiveReviewPackageAndApprovalJourney(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenant := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	tenantB := livePool(t, "BIDORA_TEST_TENANT_B_RUNTIME_DSN")
	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := insertWorkflowAuthority(t, control, livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN").Database)

	grant := func(user opportunityapp.ID, capability string, effect string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
			_, err := db.Exec(ctx, `INSERT INTO tenancy.role_grants(id,membership_id,role_key,capability,effect,created_by,created_at) SELECT $1,m.id,'BID_REVIEWER',$4,$5,$2,$6 FROM tenancy.memberships m WHERE m.user_id=$2 AND m.tenant_id=$3 AND m.state='ACTIVE'`, id, liveUUID(user), liveUUID(fixture.tenantID), capability, effect, at)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	grant(fixture.actorID, authzapp.ReviewProposalCapability, "ALLOW")
	grant(fixture.actorID, authzapp.ApproveProposalCapability, "ALLOW")
	grant(fixture.ownerID, authzapp.ReviewProposalCapability, "ALLOW")
	grant(fixture.ownerID, authzapp.ApproveProposalCapability, "ALLOW")
	fixture.subject.ContextGeneration += 2
	ownerSubject := opportunityapp.WorkflowSubject{IdentityID: fixture.ownerID, SelectedTenantID: fixture.tenantID, ContextGeneration: 4}

	store := attachWorkflowServices(t, &fixture, tenant, at.Add(time.Minute))
	captured, workspace := establishBidWorkspace(t, tenant, &fixture, at)
	seedV7OrganizationAndResponse(t, observer, fixture, captured.SourceVersionID, workspace.WorkspaceID, at)

	proposals, err := opportunityapp.NewProposalService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := proposals.Save(context.Background(), opportunityapp.SaveProposalInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, CorrelationID: liveID(), Title: "Synthetic exact-package proposal", Recipient: "Evaluation Committee", LetterheadLines: []string{"Nairobi, Kenya"}, Introduction: "Human-authored introduction.", Closing: "Human-authored closing.", ChangeNote: "Initial review candidate.", ExpectedProposalVersion: 0, ExpectedWorkspaceVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil || proposal.Proposal.ProposalVersion != 1 || proposal.Proposal.WorkspaceVersion != 3 {
		var currentVersion int64
		readErr := observer.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
			return db.QueryRow(ctx, `SELECT version FROM bid.workspaces WHERE id=$1`, liveUUID(workspace.WorkspaceID)).Scan(&currentVersion)
		})
		t.Fatalf("seed proposal: %+v %v; workspace_version=%d/%v subject_generation=%d", proposal, err, currentVersion, readErr, fixture.subject.ContextGeneration)
	}

	reviews, err := opportunityapp.NewReviewThreadService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(3 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	thread, err := reviews.Create(context.Background(), opportunityapp.CreateReviewThreadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, TargetID: proposal.Proposal.ProposalVersionID, TargetType: "PROPOSAL_VERSION", ThreadSubject: "Confirm exact package wording", Comment: "Review the immutable proposal revision.", CorrelationID: liveID(), IdempotencyKey: uuid.NewString()})
	if err != nil || thread.Thread.Version != 1 || len(thread.Thread.Comments) != 1 {
		t.Fatalf("create review: %+v %v", thread, err)
	}

	type commentOutcome struct {
		result opportunityapp.ReviewThreadResult
		err    error
	}
	outcomes := make(chan commentOutcome, 2)
	start := make(chan struct{})
	for i := range 2 {
		input := opportunityapp.AddReviewCommentInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, ThreadID: thread.Thread.ID, CorrelationID: liveID(), Comment: "Concurrent immutable review comment " + string(rune('A'+i)), ExpectedThreadVersion: 1, IdempotencyKey: uuid.NewString()}
		go func() {
			<-start
			result, callErr := reviews.Comment(context.Background(), input)
			outcomes <- commentOutcome{result, callErr}
		}()
	}
	close(start)
	first, second := <-outcomes, <-outcomes
	if !((first.err == nil && second.err != nil && second.err.Error() == "review_thread_conflict") || (second.err == nil && first.err != nil && first.err.Error() == "review_thread_conflict")) {
		t.Fatalf("review race did not preserve one winner: %+v/%v %+v/%v", first.result, first.err, second.result, second.err)
	}

	due := at.Add(24 * time.Hour)
	assigned, err := reviews.Assign(context.Background(), opportunityapp.AssignReviewThreadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, ThreadID: thread.Thread.ID, AssigneeID: fixture.ownerID, CorrelationID: liveID(), DueAt: &due, Reason: "Independent human review.", ExpectedThreadVersion: 2, IdempotencyKey: uuid.NewString()})
	if err != nil || assigned.Thread.Version != 3 || assigned.Thread.AssignedTo != fixture.ownerID {
		t.Fatalf("assign review: %+v %v", assigned, err)
	}
	dashboard, err := reviews.Dashboard(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || dashboard.OpenCount != 1 || dashboard.UnassignedCount != 0 || len(dashboard.Reviewers) != 1 {
		t.Fatalf("review dashboard: %+v %v", dashboard, err)
	}
	workload, err := reviews.ReviewerWork(context.Background(), fixture.subject, fixture.ownerID)
	if err != nil || len(workload) != 1 || workload[0].ThreadID != thread.Thread.ID {
		t.Fatalf("review workload: %+v %v", workload, err)
	}
	resolved, err := reviews.Resolve(context.Background(), opportunityapp.ResolveReviewThreadInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, ThreadID: thread.Thread.ID, CorrelationID: liveID(), ResolutionNote: "Independent reviewer confirmed the discussion is addressed.", ExpectedThreadVersion: 3, IdempotencyKey: uuid.NewString()})
	if err != nil || resolved.Thread.Status != "RESOLVED" || resolved.Thread.Version != 4 {
		t.Fatalf("resolve review: %+v %v", resolved, err)
	}
	history, err := reviews.List(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || len(history) != 1 || len(history[0].Comments) != 3 || history[0].Comments[0].Body == "" {
		t.Fatalf("immutable review history: %+v %v", history, err)
	}

	decisions, err := opportunityapp.NewInternalReviewDecisionService(fixture.authorizer, store, randomWorkflowIDs{}, workflowClock{at: at.Add(4 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	changes, err := decisions.Record(context.Background(), opportunityapp.RecordInternalReviewDecisionInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, ProposalVersionID: proposal.Proposal.ProposalVersionID, CorrelationID: liveID(), Decision: "CHANGES_REQUESTED", Rationale: "Record the initial independent concern.", ExpectedPreviousSequence: 0, IdempotencyKey: uuid.NewString()})
	if err != nil || changes.Decision.ReviewerSequence != 1 {
		t.Fatalf("changes requested: %+v %v", changes, err)
	}
	complete, err := decisions.Record(context.Background(), opportunityapp.RecordInternalReviewDecisionInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, ProposalVersionID: proposal.Proposal.ProposalVersionID, CorrelationID: liveID(), Decision: "REVIEW_COMPLETE", Rationale: "The corrected exact revision is ready for approval review.", ExpectedPreviousSequence: 1, IdempotencyKey: uuid.NewString()})
	if err != nil || complete.Decision.ReviewerSequence != 2 {
		t.Fatalf("review complete: %+v %v", complete, err)
	}
	decisionHistory, err := decisions.History(context.Background(), fixture.subject, workspace.WorkspaceID, proposal.Proposal.ProposalVersionID)
	if err != nil || len(decisionHistory) != 2 {
		t.Fatalf("decision history: %+v %v", decisionHistory, err)
	}
	seenDecisions := map[int32]string{}
	for _, decision := range decisionHistory {
		seenDecisions[decision.ReviewerSequence] = decision.Decision
	}
	if seenDecisions[1] != "CHANGES_REQUESTED" || seenDecisions[2] != "REVIEW_COMPLETE" {
		t.Fatalf("immutable decision sequence drifted: %+v", decisionHistory)
	}

	packages, err := opportunityapp.NewProposalPackageService(fixture.authorizer, store, store, proposalexport.New(), randomWorkflowIDs{}, workflowClock{at: at.Add(5 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	packageKey := uuid.NewString()
	packageInput := opportunityapp.PrepareProposalPackageInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, ProposalVersionID: proposal.Proposal.ProposalVersionID, CorrelationID: liveID(), ExpectedProposalVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: packageKey}
	prepared, err := packages.Prepare(context.Background(), packageInput)
	if err != nil || !prepared.Manifest.ReadinessReady || prepared.Manifest.ManifestVersion != 2 || prepared.Manifest.ManifestSHA256 == ([32]byte{}) {
		t.Fatalf("prepare exact package: %+v %v", prepared, err)
	}
	replay, err := packages.Prepare(context.Background(), packageInput)
	if err != nil || !replay.Replay || replay.Manifest.ID != prepared.Manifest.ID || replay.Manifest.ManifestSHA256 != prepared.Manifest.ManifestSHA256 || replay.Manifest.PDFSHA256 != prepared.Manifest.PDFSHA256 || replay.Manifest.DOCXSHA256 != prepared.Manifest.DOCXSHA256 {
		t.Fatalf("package replay drifted: %+v %v", replay, err)
	}

	approvals, err := opportunityapp.NewProposalApprovalService(fixture.authorizer, store, store, randomWorkflowIDs{}, workflowClock{at: at.Add(6 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	approvalInput := func(subject opportunityapp.WorkflowSubject, key string) opportunityapp.ApproveProposalInput {
		return opportunityapp.ApproveProposalInput{Subject: subject, BidID: workspace.WorkspaceID, ManifestID: prepared.Manifest.ID, CorrelationID: liveID(), ExpectedProposalVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: key, Rationale: "Independent human approval of the exact package.", SessionAuthenticatedAt: at.Add(5 * time.Minute), AssuranceAt: at.Add(5 * time.Minute), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP"}
	}
	if _, err = approvals.Approve(context.Background(), approvalInput(fixture.subject, uuid.NewString())); err == nil || err.Error() != "proposal_approval_conflict" {
		t.Fatalf("proposal creator self-approval was not blocked: %v", err)
	}
	staleAssurance := approvalInput(ownerSubject, uuid.NewString())
	staleAssurance.AssuranceAt = at.Add(-20 * time.Minute)
	if _, err = approvals.Approve(context.Background(), staleAssurance); err == nil || err.Error() != "proposal_approval_fresh_auth_required" {
		t.Fatalf("stale TOTP assurance accepted: %v", err)
	}

	denyID := grant(fixture.ownerID, authzapp.ApproveProposalCapability, "DENY")
	ownerSubject.ContextGeneration = 5
	if _, err = approvals.Approve(context.Background(), approvalInput(ownerSubject, uuid.NewString())); err == nil || err.Error() != "proposal_approval_not_authorized" {
		t.Fatalf("active approval DENY did not win: %v", err)
	}
	if err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE tenancy.role_grants SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, denyID, at.Add(5*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	ownerSubject.ContextGeneration = 6

	type approvalOutcome struct {
		result opportunityapp.ProposalApprovalResult
		err    error
		key    string
	}
	approvalOutcomes := make(chan approvalOutcome, 2)
	approvalStart := make(chan struct{})
	for range 2 {
		key := uuid.NewString()
		go func() {
			<-approvalStart
			result, callErr := approvals.Approve(context.Background(), approvalInput(ownerSubject, key))
			approvalOutcomes <- approvalOutcome{result, callErr, key}
		}()
	}
	close(approvalStart)
	a, b := <-approvalOutcomes, <-approvalOutcomes
	if !((a.err == nil && b.err != nil && b.err.Error() == "proposal_approval_conflict") || (b.err == nil && a.err != nil && a.err.Error() == "proposal_approval_conflict")) {
		t.Fatalf("approval race did not preserve one winner: %+v/%v %+v/%v", a.result, a.err, b.result, b.err)
	}
	winner := a
	if winner.err != nil {
		winner = b
	}
	approved, err := approvals.Get(context.Background(), fixture.subject, workspace.WorkspaceID, prepared.Manifest.ID)
	if err != nil || approved.ID != winner.result.Approval.ID || approved.ApproverID != fixture.ownerID || approved.ReviewDecisionID != complete.Decision.ID {
		t.Fatalf("immutable approval read: %+v %v", approved, err)
	}
	replayedApproval, err := approvals.Approve(context.Background(), approvalInput(ownerSubject, winner.key))
	if err != nil || !replayedApproval.Replay || replayedApproval.Approval.ID != approved.ID {
		t.Fatalf("approval replay drifted: %+v %v", replayedApproval, err)
	}

	secondProposal, err := proposals.Save(context.Background(), opportunityapp.SaveProposalInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, CorrelationID: liveID(), Title: "Synthetic exact-package proposal revision two", Recipient: "Evaluation Committee", LetterheadLines: []string{"Nairobi, Kenya"}, Introduction: "Human-authored revised introduction.", Closing: "Human-authored revised closing.", ChangeNote: "Material revision after historical approval.", ExpectedProposalVersion: 1, ExpectedWorkspaceVersion: 3, IdempotencyKey: uuid.NewString()})
	if err != nil || secondProposal.Proposal.ProposalVersion != 2 || secondProposal.Proposal.WorkspaceVersion != 4 {
		t.Fatalf("second proposal revision: %+v %v", secondProposal, err)
	}
	secondReview, err := decisions.Record(context.Background(), opportunityapp.RecordInternalReviewDecisionInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, ProposalVersionID: secondProposal.Proposal.ProposalVersionID, CorrelationID: liveID(), Decision: "REVIEW_COMPLETE", Rationale: "Reviewed the second exact revision.", ExpectedPreviousSequence: 0, IdempotencyKey: uuid.NewString()})
	if err != nil || secondReview.Decision.ReviewerSequence != 1 {
		t.Fatalf("second proposal review: %+v %v", secondReview, err)
	}
	secondPackage, err := packages.Prepare(context.Background(), opportunityapp.PrepareProposalPackageInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, ProposalVersionID: secondProposal.Proposal.ProposalVersionID, CorrelationID: liveID(), ExpectedProposalVersion: 2, ExpectedWorkspaceVersion: 4, IdempotencyKey: uuid.NewString()})
	if err != nil || !secondPackage.Manifest.ReadinessReady {
		t.Fatalf("second exact package: %+v %v", secondPackage, err)
	}
	raceStart := make(chan struct{})
	approvalRace := make(chan error, 1)
	editRace := make(chan error, 1)
	go func() {
		<-raceStart
		_, raceErr := approvals.Approve(context.Background(), opportunityapp.ApproveProposalInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, ManifestID: secondPackage.Manifest.ID, CorrelationID: liveID(), ExpectedProposalVersion: 2, ExpectedWorkspaceVersion: 4, IdempotencyKey: uuid.NewString(), Rationale: "Race exact revision approval.", SessionAuthenticatedAt: at.Add(5 * time.Minute), AssuranceAt: at.Add(5 * time.Minute), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP"})
		approvalRace <- raceErr
	}()
	go func() {
		<-raceStart
		_, raceErr := proposals.Save(context.Background(), opportunityapp.SaveProposalInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, CorrelationID: liveID(), Title: "Synthetic exact-package proposal revision three", Recipient: "Evaluation Committee", LetterheadLines: []string{"Nairobi, Kenya"}, Introduction: "Concurrent human revision.", Closing: "Concurrent human closing.", ChangeNote: "Invalidate the second exact package as current.", ExpectedProposalVersion: 2, ExpectedWorkspaceVersion: 4, IdempotencyKey: uuid.NewString()})
		editRace <- raceErr
	}()
	close(raceStart)
	approvalRaceErr, editRaceErr := <-approvalRace, <-editRace
	if editRaceErr != nil || (approvalRaceErr != nil && approvalRaceErr.Error() != "proposal_approval_blocked" && approvalRaceErr.Error() != "proposal_approval_conflict") {
		t.Fatalf("edit/approval race drifted: approval=%v edit=%v", approvalRaceErr, editRaceErr)
	}
	stalePackageApproval := opportunityapp.ApproveProposalInput{Subject: ownerSubject, BidID: workspace.WorkspaceID, ManifestID: secondPackage.Manifest.ID, CorrelationID: liveID(), ExpectedProposalVersion: 3, ExpectedWorkspaceVersion: 5, IdempotencyKey: uuid.NewString(), Rationale: "Stale package must not become current.", SessionAuthenticatedAt: at.Add(5 * time.Minute), AssuranceAt: at.Add(5 * time.Minute), AssuranceLevel: "AAL2", AssuranceMethod: "TOTP"}
	if _, err = approvals.Approve(context.Background(), stalePackageApproval); err == nil || (err.Error() != "proposal_approval_blocked" && err.Error() != "proposal_approval_conflict") {
		t.Fatalf("stale package became currently approvable after revision: %v", err)
	}
	stillHistorical, err := approvals.Get(context.Background(), fixture.subject, workspace.WorkspaceID, prepared.Manifest.ID)
	if err != nil || stillHistorical.ID != approved.ID {
		t.Fatalf("historical approval was mutated by later proposal revisions: %+v %v", stillHistorical, err)
	}
	packageHistory, err := packages.List(context.Background(), opportunityapp.ProposalPackageHistoryInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, Limit: 10})
	if err != nil || len(packageHistory.Items) != 2 || packageHistory.HasMore {
		t.Fatalf("bounded package history drifted: %+v %v", packageHistory, err)
	}
	timeline, err := opportunityapp.NewCollaborationTimelineService(fixture.authorizer, store)
	if err != nil {
		t.Fatal(err)
	}
	activity, err := timeline.List(context.Background(), opportunityapp.CollaborationTimelineInput{Subject: fixture.subject, BidID: workspace.WorkspaceID, Limit: 100})
	if err != nil || len(activity.Items) < 7 || activity.HasMore {
		t.Fatalf("bounded collaboration timeline drifted: %+v %v", activity, err)
	}
	for _, item := range activity.Items {
		if item.ActorID.IsZero() || item.TargetID.IsZero() || item.EventType == "" || item.DetailCode == "" {
			t.Fatalf("collaboration activity lost minimal provenance: %+v", item)
		}
	}

	storeB, err := opportunitypersistence.NewWorkflowStore(tenantB, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	authority := opportunityapp.WorkflowAuthority{ActorID: fixture.actorID, TenantID: fixture.tenantID, AssignmentGeneration: 1}
	if _, err = storeB.GetProposalPackageManifest(context.Background(), authority, workspace.WorkspaceID, prepared.Manifest.ID); !errors.Is(err, opportunityapp.ErrProposalPackageNotFound) {
		t.Fatalf("tenant B exposed tenant A package: %v", err)
	}
	if _, err = storeB.GetProposalApprovalByManifest(context.Background(), authority, workspace.WorkspaceID, prepared.Manifest.ID); !errors.Is(err, opportunityapp.ErrProposalApprovalNotFound) {
		t.Fatalf("tenant B exposed tenant A approval: %v", err)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM bidora_eventing.outbox_events WHERE actor_id IS NOT NULL AND event_type IN ('bid.review_thread.created.v1','bid.review_comment.added.v1','bid.review_thread.assigned.v1','bid.review_thread.resolved.v1','bid.internal_review_decision.recorded.v1','bid.proposal_package.prepared.v1','bid.proposal_package.approved.v1') AND payload->>'bid_id'=$1`, liveUUID(workspace.WorkspaceID).String()); count < 8 {
		t.Fatalf("V7 actor-provenance outbox lineage incomplete: %d", count)
	}
}

func seedV7OrganizationAndResponse(t *testing.T, observer *postgres.Pool, fixture workflowFixture, sourceVersionID, bidID opportunityapp.ID, at time.Time) {
	t.Helper()
	letterheadID, requirementID, requirementVersionID, sectionID, responseID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	hash := sha256.Sum256(append([]byte("v7 immutable synthetic letterhead:"), fixture.tenantID[:]...))
	if err := observer.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenant.organization_profile(singleton_key,id,tenant_registry_id,legal_name,country,timezone,currency,policy_version,created_at,updated_at) VALUES(true,$1,$2,'Synthetic Bidora Limited','KE','Africa/Nairobi','KES',1,$3,$3) ON CONFLICT(singleton_key) DO UPDATE SET legal_name=EXCLUDED.legal_name,updated_at=EXCLUDED.updated_at`, uuid.New(), liveUUID(fixture.tenantID), at); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO organization.letterhead_versions(id,version_number,registered_name,phone,email,header_layout,footer_layout,page_number_style,primary_color,secondary_color,template_key,content_hash,created_by,created_at) SELECT $1,next_version_number,'Bidora Synthetic Ltd','+254700000000','synthetic@example.invalid','STANDARD','CONTACT','PAGE_X_OF_Y','#112233','#445566','STANDARD_PROPOSAL',$2,$3,$4 FROM organization.letterhead_configuration WHERE singleton_key=true`, letterheadID, hash[:], liveUUID(fixture.actorID), at); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `UPDATE organization.letterhead_configuration SET active_version_id=$1,next_version_number=next_version_number+1,version=version+1,updated_by=$2,updated_at=$3 WHERE singleton_key=true`, letterheadID, liveUUID(fixture.actorID), at); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bid.requirements(id,bid_id,source_version_id,source_location,text,mandatory,category,status,owner_id,version,created_by,created_at,updated_at,current_content_revision_id) VALUES($1,$2,$3,'Synthetic source','Provide a reviewed technical response.',false,'TECHNICAL','OPEN',$4,1,$4,$5,$5,$6)`, requirementID, liveUUID(bidID), liveUUID(sourceVersionID), liveUUID(fixture.actorID), at, requirementVersionID); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bid.requirement_versions(id,requirement_id,revision,source_version_id,source_location,text,mandatory,category,correction_reason,created_by,created_at) VALUES($1,$2,1,$3,'Synthetic source','Provide a reviewed technical response.',false,'TECHNICAL','Synthetic V7 prerequisite',$4,$5)`, requirementVersionID, requirementID, liveUUID(sourceVersionID), liveUUID(fixture.actorID), at); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bid.response_sections(id,bid_id,requirement_id,current_revision_id,version,created_by,created_at,updated_at) VALUES($1,$2,$3,NULL,1,$4,$5,$5)`, sectionID, liveUUID(bidID), requirementID, liveUUID(fixture.actorID), at); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bid.response_versions(id,section_id,revision,requirement_version_at_creation,content,change_note,created_by,created_at) VALUES($1,$2,1,1,'Human-authored reviewed technical response.','Synthetic V7 prerequisite.',$3,$4)`, responseID, sectionID, liveUUID(fixture.actorID), at); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `UPDATE bid.response_sections SET current_revision_id=$2 WHERE id=$1`, sectionID, responseID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
