package opportunity_test

import (
	"context"
	"testing"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
)

func mfaOpportunityID(value byte) opportunityapp.ID {
	var id opportunityapp.ID
	id[15] = value
	return id
}

type mfaApprovalAuthorizer struct{ calls int }

func (a *mfaApprovalAuthorizer) AuthorizeProposalApproval(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	a.calls++
	return opportunityapp.WorkflowAuthority{ActorID: mfaOpportunityID(1), TenantID: mfaOpportunityID(2), AssignmentGeneration: 1}, nil
}
func (a *mfaApprovalAuthorizer) AuthorizeBidRead(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	return opportunityapp.WorkflowAuthority{}, nil
}

type mfaApprovalSource struct {
	readiness opportunityapp.ProposalApprovalReadiness
}

func (*mfaApprovalSource) GetProposalVersion(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalDocument, error) {
	return opportunityapp.ProposalDocument{}, nil
}
func (s *mfaApprovalSource) GetProposalApprovalReadiness(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.ProposalApprovalReadiness, error) {
	return s.readiness, nil
}

type mfaApprovalStore struct {
	manifest opportunityapp.ProposalPackageManifest
	commits  int
}

func (*mfaApprovalStore) FindProposalApprovalReplay(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, [32]byte, [32]byte) (opportunityapp.ProposalApprovalResult, bool, error) {
	return opportunityapp.ProposalApprovalResult{}, false, nil
}
func (s *mfaApprovalStore) CommitProposalApproval(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ProposalApprovalCommit) (opportunityapp.ProposalApprovalResult, error) {
	s.commits++
	return opportunityapp.ProposalApprovalResult{}, nil
}
func (*mfaApprovalStore) GetProposalApprovalByManifest(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalApproval, error) {
	return opportunityapp.ProposalApproval{}, nil
}
func (s *mfaApprovalStore) GetProposalPackageManifest(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalPackageManifest, error) {
	return s.manifest, nil
}

type mfaApprovalIDs struct{ next byte }

func (g *mfaApprovalIDs) NewID() (opportunityapp.ID, error) {
	g.next++
	return mfaOpportunityID(g.next), nil
}

type mfaApprovalClock struct{ now time.Time }

func (c mfaApprovalClock) Now() time.Time { return c.now }

func TestGovernedProposalApprovalRequiresRecentTOTPAAL2(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	bid, manifest, proposalVersion := mfaOpportunityID(10), mfaOpportunityID(11), mfaOpportunityID(12)
	authorizer := &mfaApprovalAuthorizer{}
	source := &mfaApprovalSource{readiness: opportunityapp.ProposalApprovalReadiness{BidID: bid, ProposalVersionID: proposalVersion, ProposalVersion: 3, WorkspaceVersion: 4, Ready: true}}
	store := &mfaApprovalStore{manifest: opportunityapp.ProposalPackageManifest{ID: manifest, BidID: bid, ProposalVersionID: proposalVersion, Status: "UNAPPROVED_PREPARATION_ONLY", ReadinessReady: true}}
	service, err := opportunityapp.NewProposalApprovalService(authorizer, source, store, &mfaApprovalIDs{}, mfaApprovalClock{now: now})
	if err != nil {
		t.Fatal(err)
	}
	base := opportunityapp.ApproveProposalInput{Subject: opportunityapp.WorkflowSubject{IdentityID: mfaOpportunityID(1), SelectedTenantID: mfaOpportunityID(2), ContextGeneration: 1}, BidID: bid, ManifestID: manifest, CorrelationID: mfaOpportunityID(13), ExpectedProposalVersion: 3, ExpectedWorkspaceVersion: 4, IdempotencyKey: "synthetic-approval", SessionAuthenticatedAt: now, AssuranceAt: now, AssuranceLevel: "AAL2"}
	for _, method := range []string{"PRIMARY_ONLY", "RECOVERY_CODE"} {
		input := base
		input.AssuranceMethod = method
		if _, approveErr := service.Approve(context.Background(), input); approveErr == nil || approveErr.Error() != "proposal_approval_fresh_auth_required" {
			t.Fatalf("governed approval accepted %s: %v", method, approveErr)
		}
	}
	stale := base
	stale.AssuranceMethod = "TOTP"
	stale.AssuranceAt = now.Add(-16 * time.Minute)
	if _, approveErr := service.Approve(context.Background(), stale); approveErr == nil || approveErr.Error() != "proposal_approval_fresh_auth_required" {
		t.Fatal("governed approval accepted stale TOTP", approveErr)
	}
	allowed := base
	allowed.AssuranceMethod = "TOTP"
	if _, approveErr := service.Approve(context.Background(), allowed); approveErr != nil {
		t.Fatal(approveErr)
	}
	if store.commits != 1 || authorizer.calls != 4 {
		t.Fatal("governed approval boundary drifted", store.commits, authorizer.calls)
	}
}
