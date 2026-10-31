package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	commercialdomain "bidora/backend/internal/commercial/domain"
	opportunityapp "bidora/backend/internal/opportunity/application"
	opportunityintegration "bidora/backend/internal/opportunity/integration"
	opportunitypersistence "bidora/backend/internal/opportunity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type randomWorkflowIDs struct{}

func (randomWorkflowIDs) NewID() (opportunityapp.ID, error) {
	return opportunityapp.ID(uuid.New()), nil
}

type workflowClock struct{ at time.Time }

func (c workflowClock) Now() time.Time { return c.at }

type paidWriteGuard struct {
	mu   sync.Mutex
	deny bool
}

func (g *paidWriteGuard) RequirePaidWriteAccess(context.Context, commercialdomain.ID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.deny {
		return errors.New("synthetic_paid_write_denied")
	}
	return nil
}

func (g *paidWriteGuard) setDenied(denied bool) {
	g.mu.Lock()
	g.deny = denied
	g.mu.Unlock()
}

type fixedWorkflowAuthorizer struct {
	authority opportunityapp.WorkflowAuthority
}

func (a fixedWorkflowAuthorizer) AuthorizeVerification(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	return a.authority, nil
}
func (a fixedWorkflowAuthorizer) AuthorizeWorkspaceCreation(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.WorkflowAuthority, error) {
	return a.authority, nil
}
func (a fixedWorkflowAuthorizer) AuthorizeDecision(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	return a.authority, nil
}
func (a fixedWorkflowAuthorizer) AuthorizeOpportunityRead(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	return a.authority, nil
}
func (a fixedWorkflowAuthorizer) AuthorizeBidRead(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	return a.authority, nil
}

type workflowFixture struct {
	tenantID, actorID, ownerID, inactiveOwnerID opportunityapp.ID
	membershipID                                uuid.UUID
	subject                                     opportunityapp.WorkflowSubject
	service                                     *opportunityapp.WorkflowService
	reader                                      *opportunityapp.WorkflowReadService
	authorizer                                  *opportunityintegration.WorkflowAuthorizationAdapter
	guard                                       *paidWriteGuard
}

func insertWorkflowAuthority(t *testing.T, control *postgres.Pool, databaseName string) workflowFixture {
	t.Helper()
	tenantID, actorID, ownerID, inactiveOwnerID := liveID(), liveID(), liveID(), liveID()
	membershipID, ownerMembershipID, inactiveMembershipID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	clusterID := uuid.New()
	capabilities := []string{
		authzapp.VerifyExternalOpportunityCapability,
		authzapp.CreateBidWorkspaceCapability,
		authzapp.RecordBidDecisionCapability,
		authzapp.ReadOpportunityCapability,
		authzapp.ReadBidCapability,
		authzapp.ManageRequirementCapability,
		authzapp.AssignRequirementCapability,
		authzapp.ReadReadinessCapability,
		authzapp.UploadTenderDocumentCapability,
		authzapp.EditTechnicalResponseCapability,
	}
	err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		for _, user := range []opportunityapp.ID{actorID, ownerID} {
			email := "v4-" + uuid.NewString() + "@example.invalid"
			if _, err := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$2,$3,NULL,1,$3,$3)`, liveUUID(user), email, now); err != nil {
				return err
			}
		}
		inactiveEmail := "v4-disabled-" + uuid.NewString() + "@example.invalid"
		if _, err := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$2,$3,$3,1,$3,$3)`, liveUUID(inactiveOwnerID), inactiveEmail, now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenants (id,tenant_type,state,name,country,timezone,locale,assignment_generation,version,created_at,updated_at) VALUES ($1,'TENDERPRENEUR','ACTIVE','Synthetic V4 Tenant','KE','Africa/Nairobi','en-KE',1,1,$2,$2)`, liveUUID(tenantID), now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.database_clusters (id,region,endpoint_ref,state,capacity_policy,version,created_at,updated_at) VALUES ($1,'local-test',$2,'ACTIVE','{}'::jsonb,1,$3,$3)`, clusterID, "synthetic/v4/"+uuid.NewString(), now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO bidora_tenancy.tenant_database_assignments (id,tenant_id,cluster_id,database_name,secret_ref,generation,active,created_at,deactivated_at) VALUES ($1,$2,$3,$4,$5,1,true,$6,NULL)`, uuid.New(), liveUUID(tenantID), clusterID, databaseName, "synthetic/v4/"+uuid.NewString(), now); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO tenancy.memberships (id,user_id,tenant_id,state,context_generation,created_at,updated_at) VALUES ($1,$2,$3,'ACTIVE',1,$4,$4),($5,$6,$3,'ACTIVE',1,$4,$4),($7,$8,$3,'ACTIVE',1,$4,$4)`, membershipID, liveUUID(actorID), liveUUID(tenantID), now, ownerMembershipID, liveUUID(ownerID), inactiveMembershipID, liveUUID(inactiveOwnerID)); err != nil {
			return err
		}
		for _, capability := range capabilities {
			if _, err := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at) VALUES ($1,$2,'BID_MANAGER',$3,'ALLOW',$4,$5)`, uuid.New(), membershipID, capability, liveUUID(actorID), now); err != nil {
				return err
			}
		}
		for _, ownerMembership := range []uuid.UUID{ownerMembershipID, inactiveMembershipID} {
			if _, err := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at) VALUES ($1,$2,'BID_MANAGER',$3,'ALLOW',$4,$5)`, uuid.New(), ownerMembership, authzapp.ReadBidCapability, liveUUID(actorID), now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	guard := &paidWriteGuard{}
	return workflowFixture{
		tenantID: tenantID, actorID: actorID, ownerID: ownerID, inactiveOwnerID: inactiveOwnerID,
		membershipID: membershipID,
		subject:      opportunityapp.WorkflowSubject{IdentityID: actorID, SelectedTenantID: tenantID, ContextGeneration: int64(1 + len(capabilities))},
		guard:        guard,
		// Service and reader are attached after the tenant-private store is constructed.
		service: nil, reader: nil,
	}
}

func attachWorkflowServices(t *testing.T, fixture *workflowFixture, tenant *postgres.Pool, at time.Time) *opportunitypersistence.WorkflowStore {
	t.Helper()
	store, err := opportunitypersistence.NewWorkflowStore(tenant, fixture.tenantID, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Rebuild the production authorization chain for the already-persisted fixture.
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	grantReader, err := authzpersistence.NewOpportunityWorkflowGrantReader(control)
	if err != nil {
		t.Fatal(err)
	}
	controlAuthorizer, err := authzapp.NewOpportunityWorkflowAuthorizer(grantReader)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := opportunityintegration.NewPaidWorkflowAuthorizationAdapter(controlAuthorizer, fixture.guard)
	if err != nil {
		t.Fatal(err)
	}
	fixture.authorizer = authorizer
	fixture.service, err = opportunityapp.NewWorkflowService(authorizer, store, randomWorkflowIDs{}, workflowClock{at: at})
	if err != nil {
		t.Fatal(err)
	}
	fixture.reader, err = opportunityapp.NewWorkflowReadService(authorizer, store)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func captureForWorkflow(t *testing.T, tenant *postgres.Pool, tenantID, actorID opportunityapp.ID, title string, at time.Time) opportunityapp.CaptureResult {
	t.Helper()
	result, err := executeLive(t, tenant, tenantID, actorID, 1, uuid.NewString(), liveCommand(t, title), idBatch(), at)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestLiveOpportunityWorkspaceJourneyAuthorizationReplayAndIsolation(t *testing.T) {
	controlMigrator := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	tenantA := livePool(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN")
	tenantAObserver := livePool(t, "BIDORA_TEST_TENANT_A_MIGRATOR_DSN")
	tenantB := livePool(t, "BIDORA_TEST_TENANT_B_RUNTIME_DSN")

	at := time.Now().UTC().Truncate(time.Microsecond)
	fixture := insertWorkflowAuthority(t, controlMigrator, livePostgresConfig(t, "BIDORA_TEST_TENANT_A_RUNTIME_DSN").Database)
	storeA := attachWorkflowServices(t, &fixture, tenantA, at.Add(time.Minute))
	fixtureCount := rowCount(t, controlMigrator, `SELECT count(*) FROM tenancy.memberships actor JOIN identity.users actor_user ON actor_user.id=actor.user_id JOIN bidora_tenancy.tenants tenant ON tenant.id=actor.tenant_id WHERE actor.user_id=$1 AND actor.tenant_id=$2 AND actor.context_generation=$4 AND actor.state='ACTIVE' AND actor_user.email_verified_at IS NOT NULL AND actor_user.disabled_at IS NULL AND tenant.tenant_type='TENDERPRENEUR' AND tenant.state='ACTIVE' AND EXISTS (SELECT 1 FROM tenancy.role_grants grant_row WHERE grant_row.membership_id=actor.id AND grant_row.capability=$3 AND grant_row.effect='ALLOW' AND grant_row.revoked_at IS NULL) AND NOT EXISTS (SELECT 1 FROM tenancy.role_grants grant_row WHERE grant_row.membership_id=actor.id AND grant_row.capability=$3 AND grant_row.effect='DENY' AND grant_row.revoked_at IS NULL)`, liveUUID(fixture.actorID), liveUUID(fixture.tenantID), authzapp.VerifyExternalOpportunityCapability, fixture.subject.ContextGeneration)
	if fixtureCount != 1 {
		t.Fatalf("synthetic workflow authority fixture is incomplete: joined=%d user=%d membership=%d tenant=%d allow=%d", fixtureCount,
			rowCount(t, controlMigrator, `SELECT count(*) FROM identity.users WHERE id=$1 AND email_verified_at IS NOT NULL AND disabled_at IS NULL`, liveUUID(fixture.actorID)),
			rowCount(t, controlMigrator, `SELECT count(*) FROM tenancy.memberships WHERE id=$1 AND state='ACTIVE' AND context_generation=$2`, fixture.membershipID, fixture.subject.ContextGeneration),
			rowCount(t, controlMigrator, `SELECT count(*) FROM bidora_tenancy.tenants WHERE id=$1 AND tenant_type='TENDERPRENEUR' AND state='ACTIVE'`, liveUUID(fixture.tenantID)),
			rowCount(t, controlMigrator, `SELECT count(*) FROM tenancy.role_grants WHERE membership_id=$1 AND capability=$2 AND effect='ALLOW' AND revoked_at IS NULL`, fixture.membershipID, authzapp.VerifyExternalOpportunityCapability))
	}

	title := "Synthetic V4 opportunity " + uuid.NewString()
	captured := captureForWorkflow(t, tenantA, fixture.tenantID, fixture.actorID, title, at)
	verifyInput := opportunityapp.VerifyOpportunityInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, SourceVersionID: captured.SourceVersionID, CorrelationID: liveID(), ExpectedVersion: 1, IdempotencyKey: uuid.NewString()}
	type verifyOutcome struct {
		result opportunityapp.VerificationResult
		err    error
	}
	verifyOutcomes := make(chan verifyOutcome, 2)
	verifyStart := make(chan struct{})
	var verifyReady sync.WaitGroup
	verifyReady.Add(2)
	for range 2 {
		go func() {
			verifyReady.Done()
			<-verifyStart
			result, verifyErr := fixture.service.Verify(context.Background(), verifyInput)
			verifyOutcomes <- verifyOutcome{result: result, err: verifyErr}
		}()
	}
	verifyReady.Wait()
	close(verifyStart)
	firstVerify, secondVerify := <-verifyOutcomes, <-verifyOutcomes
	if firstVerify.err != nil || secondVerify.err != nil || firstVerify.result.SourceVersionID != secondVerify.result.SourceVersionID || firstVerify.result.Replay == secondVerify.result.Replay {
		firstError, secondError := "<nil>", "<nil>"
		if firstVerify.err != nil {
			firstError = firstVerify.err.Error()
		}
		if secondVerify.err != nil {
			secondError = secondVerify.err.Error()
		}
		t.Fatalf("concurrent verification did not converge: first=%s second=%s", firstError, secondError)
	}
	verified := firstVerify.result
	if verified.Replay {
		verified = secondVerify.result
	}
	if verified.OpportunityID != captured.OpportunityID || verified.SourceVersionID.IsZero() || verified.Version != 2 {
		t.Fatal("verification result drifted", verified)
	}
	verifyReplay, err := fixture.service.Verify(context.Background(), verifyInput)
	if err != nil || !verifyReplay.Replay || verifyReplay.SourceVersionID != verified.SourceVersionID || verifyReplay.Version != 2 {
		t.Fatal("verification replay did not converge", verifyReplay, err)
	}
	staleVerify := verifyInput
	staleVerify.IdempotencyKey = uuid.NewString()
	if _, err = fixture.service.Verify(context.Background(), staleVerify); err == nil || err.Error() != "opportunity_verification_conflict" {
		t.Fatal("stale verification was accepted", err)
	}

	workspaceInput := opportunityapp.CreateWorkspaceInput{Subject: fixture.subject, OpportunityID: captured.OpportunityID, SourceVersionID: verified.SourceVersionID, OwnerID: fixture.ownerID, CorrelationID: liveID(), ExpectedVersion: 2, IdempotencyKey: uuid.NewString()}
	ineligibleOwnerInput := workspaceInput
	ineligibleOwnerInput.OwnerID = fixture.inactiveOwnerID
	ineligibleOwnerInput.IdempotencyKey = uuid.NewString()
	if _, err = fixture.service.CreateWorkspace(context.Background(), ineligibleOwnerInput); err == nil || err.Error() != "workspace_creation_not_authorized" {
		t.Fatal("inactive workspace owner was accepted", err)
	}
	workspace, err := fixture.service.CreateWorkspace(context.Background(), workspaceInput)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.WorkspaceID.IsZero() || workspace.Version != 1 || workspace.Replay {
		t.Fatal("workspace result drifted", workspace)
	}
	workspaceReplay, err := fixture.service.CreateWorkspace(context.Background(), workspaceInput)
	if err != nil || !workspaceReplay.Replay || workspaceReplay.WorkspaceID != workspace.WorkspaceID {
		t.Fatal("workspace replay did not converge", workspaceReplay, err)
	}
	secondWorkspace := workspaceInput
	secondWorkspace.IdempotencyKey = uuid.NewString()
	if _, err = fixture.service.CreateWorkspace(context.Background(), secondWorkspace); err == nil || err.Error() != "workspace_creation_conflict" {
		t.Fatal("second active workspace was accepted", err)
	}

	decisionInput := opportunityapp.RecordDecisionInput{Subject: fixture.subject, WorkspaceID: workspace.WorkspaceID, CorrelationID: liveID(), ExpectedVersion: 1, Decision: "BID", Rationale: "Proceed with the synthetic response plan.", IdempotencyKey: uuid.NewString()}
	decision, err := fixture.service.RecordDecision(context.Background(), decisionInput)
	if err != nil {
		t.Fatal(err)
	}
	decisionReplay, err := fixture.service.RecordDecision(context.Background(), decisionInput)
	if err != nil || !decisionReplay.Replay || decisionReplay.DecisionID != decision.DecisionID || decisionReplay.Version != 2 {
		t.Fatal("decision replay did not converge", decisionReplay, err)
	}
	conflictingReplay := decisionInput
	conflictingReplay.Rationale = "A conflicting retry must not overwrite the original rationale."
	if _, err = fixture.service.RecordDecision(context.Background(), conflictingReplay); err == nil || err.Error() != "bid_decision_conflict" {
		t.Fatal("conflicting decision idempotency reuse was accepted", err)
	}
	if _, err = fixture.service.RecordDecision(context.Background(), opportunityapp.RecordDecisionInput{Subject: fixture.subject, WorkspaceID: workspace.WorkspaceID, CorrelationID: liveID(), ExpectedVersion: 1, Decision: "NO_BID", Rationale: "Stale synthetic decision.", IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "bid_decision_conflict" {
		t.Fatal("stale decision was accepted", err)
	}
	secondDecision, err := fixture.service.RecordDecision(context.Background(), opportunityapp.RecordDecisionInput{Subject: fixture.subject, WorkspaceID: workspace.WorkspaceID, CorrelationID: liveID(), ExpectedVersion: 2, Decision: "NO_BID", Rationale: "New evidence changed the human decision.", IdempotencyKey: uuid.NewString()})
	if err != nil || secondDecision.Version != 3 || secondDecision.Decision != "NO_BID" {
		t.Fatal("second human decision failed", secondDecision, err)
	}

	for range 21 {
		captureForWorkflow(t, tenantA, fixture.tenantID, fixture.actorID, title, at.Add(90*time.Second))
	}
	detail, err := fixture.reader.GetOpportunity(context.Background(), fixture.subject, captured.OpportunityID)
	if err != nil || detail.VerificationState != "VERIFIED" || detail.Version != 2 || detail.VerifiedBy != fixture.actorID || len(detail.PossibleDuplicateIDs) != 20 || !detail.DuplicatesTruncated {
		t.Fatal("opportunity detail or bounded duplicate hints drifted", detail, err)
	}
	bid, err := fixture.reader.GetBid(context.Background(), fixture.subject, workspace.WorkspaceID)
	if err != nil || bid.State != "NO_BID" || bid.Version != 3 || len(bid.Decisions) != 2 || bid.Decisions[0].Decision != "NO_BID" || bid.Decisions[1].Decision != "BID" {
		t.Fatal("bid history drifted", bid, err)
	}
	page, err := fixture.reader.ListOpportunities(context.Background(), opportunityapp.ReadQuery{Subject: fixture.subject, Limit: 1})
	if err != nil || len(page.Items) != 1 || !page.HasMore {
		t.Fatal("bounded opportunity page failed", len(page.Items), page.HasMore, err)
	}
	bidPage, err := fixture.reader.ListBids(context.Background(), opportunityapp.BidReadQuery{ReadQuery: opportunityapp.ReadQuery{Subject: fixture.subject, Limit: 1}, State: "NO_BID", ArchiveState: "ACTIVE", OwnerID: fixture.ownerID, FilterOwner: true})
	if err != nil || len(bidPage.Items) != 1 || bidPage.Items[0].ID != workspace.WorkspaceID {
		t.Fatal("bounded bid filter failed", bidPage, err)
	}
	if rowCount(t, tenantAObserver, `SELECT count(*) FROM bidora_audit.audit_events WHERE target_id IN ($1,$2)`, liveUUID(captured.OpportunityID), liveUUID(workspace.WorkspaceID)) != 5 {
		t.Fatal("workflow audit lineage missing")
	}
	if rowCount(t, tenantA, `SELECT count(*) FROM bidora_eventing.outbox_events WHERE aggregate_id IN ($1,$2)`, liveUUID(captured.OpportunityID), liveUUID(workspace.WorkspaceID)) != 5 {
		t.Fatal("workflow outbox lineage missing")
	}

	// Active DENY wins even while the matching ALLOW remains current.
	denyID := uuid.New()
	if err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `INSERT INTO tenancy.role_grants (id,membership_id,role_key,capability,effect,created_by,created_at) VALUES ($1,$2,'BID_MANAGER',$3,'DENY',$4,$5)`, denyID, fixture.membershipID, authzapp.ReadOpportunityCapability, liveUUID(fixture.actorID), at)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	fixture.subject.ContextGeneration++
	if _, err = fixture.reader.GetOpportunity(context.Background(), fixture.subject, captured.OpportunityID); err == nil || err.Error() != "opportunity_read_not_authorized" {
		t.Fatal("active DENY did not win", err)
	}
	if err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE tenancy.role_grants SET revoked_at=$2 WHERE id=$1`, denyID, at.Add(time.Second))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	fixture.subject.ContextGeneration++

	staleSubject := fixture.subject
	staleSubject.ContextGeneration++
	if _, err = fixture.reader.GetOpportunity(context.Background(), staleSubject, captured.OpportunityID); err == nil || err.Error() != "opportunity_read_not_authorized" {
		t.Fatal("stale context was accepted", err)
	}

	// Commercial writes fail closed before tenant persistence, while reads remain available.
	fixture.guard.setDenied(true)
	blockedCapture := captureForWorkflow(t, tenantA, fixture.tenantID, fixture.actorID, "Synthetic V4 paid gate "+uuid.NewString(), at.Add(2*time.Minute))
	if _, err = fixture.service.Verify(context.Background(), opportunityapp.VerifyOpportunityInput{Subject: fixture.subject, OpportunityID: blockedCapture.OpportunityID, SourceVersionID: blockedCapture.SourceVersionID, CorrelationID: liveID(), ExpectedVersion: 1, IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "opportunity_verification_not_authorized" {
		t.Fatal("paid-write denial was bypassed", err)
	}
	if rowCount(t, tenantA, `SELECT count(*) FROM bidora_opportunity.source_versions WHERE opportunity_id=$1 AND verification_state='VERIFIED'`, liveUUID(blockedCapture.OpportunityID)) != 0 {
		t.Fatal("paid-write denial persisted verification")
	}
	fixture.guard.setDenied(false)

	// A changed server assignment generation cannot enter the generation-1 tenant store.
	if err = controlMigrator.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		_, e := db.Exec(ctx, `UPDATE bidora_tenancy.tenants SET assignment_generation=2,version=version+1,updated_at=$2 WHERE id=$1`, liveUUID(fixture.tenantID), at.Add(3*time.Minute))
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.service.Verify(context.Background(), opportunityapp.VerifyOpportunityInput{Subject: fixture.subject, OpportunityID: blockedCapture.OpportunityID, SourceVersionID: blockedCapture.SourceVersionID, CorrelationID: liveID(), ExpectedVersion: 1, IdempotencyKey: uuid.NewString()}); err == nil || err.Error() != "opportunity_verification_unavailable" {
		t.Fatal("stale tenant assignment entered persistence", err)
	}

	// Tenant B's production store cannot retrieve tenant A's private IDs.
	tenantBID, tenantBActor := liveID(), liveID()
	storeB, err := opportunitypersistence.NewWorkflowStore(tenantB, tenantBID, 1)
	if err != nil {
		t.Fatal(err)
	}
	readerB, err := opportunityapp.NewWorkflowReadService(fixedWorkflowAuthorizer{authority: opportunityapp.WorkflowAuthority{ActorID: tenantBActor, TenantID: tenantBID, AssignmentGeneration: 1}}, storeB)
	if err != nil {
		t.Fatal(err)
	}
	subjectB := opportunityapp.WorkflowSubject{IdentityID: tenantBActor, SelectedTenantID: tenantBID, ContextGeneration: 1}
	if _, err = readerB.GetOpportunity(context.Background(), subjectB, captured.OpportunityID); err == nil || err.Error() != "opportunity_not_found" {
		t.Fatal("tenant B observed tenant A opportunity", err)
	}
	if _, err = readerB.GetBid(context.Background(), subjectB, workspace.WorkspaceID); err == nil || err.Error() != "bid_not_found" {
		t.Fatal("tenant B observed tenant A bid", err)
	}

	// Ensure the production store itself rejects a mismatched authority independently of control reads.
	if _, err = storeA.GetOpportunity(context.Background(), opportunityapp.WorkflowAuthority{ActorID: fixture.actorID, TenantID: fixture.tenantID, AssignmentGeneration: 2}, captured.OpportunityID); err == nil {
		t.Fatal("generation-mismatched direct store read succeeded")
	}
}
