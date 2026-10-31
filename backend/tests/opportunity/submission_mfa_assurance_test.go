package opportunity_test

import (
	"context"
	"testing"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
)

type submissionMFAAuthorizer struct{ verifyCalls int }

func (*submissionMFAAuthorizer) AuthorizeSubmissionReport(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	return opportunityapp.WorkflowAuthority{}, nil
}
func (a *submissionMFAAuthorizer) AuthorizeSubmissionVerify(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	a.verifyCalls++
	return opportunityapp.WorkflowAuthority{ActorID: mfaOpportunityID(1), TenantID: mfaOpportunityID(2), AssignmentGeneration: 1}, nil
}
func (*submissionMFAAuthorizer) AuthorizeBidRead(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	return opportunityapp.WorkflowAuthority{}, nil
}

type submissionMFAStore struct{ commits int }

func (*submissionMFAStore) ReserveSubmissionProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.SubmissionProofReservation) (opportunityapp.SubmissionProofUpload, error) {
	return opportunityapp.SubmissionProofUpload{}, nil
}
func (*submissionMFAStore) GetSubmissionProofUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID, opportunityapp.ID) (opportunityapp.SubmissionProofUpload, error) {
	return opportunityapp.SubmissionProofUpload{}, nil
}
func (*submissionMFAStore) GetCompletedSubmissionProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID, opportunityapp.ID) (opportunityapp.SubmissionProofResult, error) {
	return opportunityapp.SubmissionProofResult{}, nil
}
func (*submissionMFAStore) CompleteSubmissionProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.SubmissionProofCompletion) (opportunityapp.SubmissionProofResult, error) {
	return opportunityapp.SubmissionProofResult{}, nil
}
func (*submissionMFAStore) FindSubmissionReplay(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, [32]byte, [32]byte) (opportunityapp.SubmissionConfirmationResult, bool, error) {
	return opportunityapp.SubmissionConfirmationResult{}, false, nil
}
func (s *submissionMFAStore) CommitSubmissionConfirmation(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.SubmissionConfirmationCommit) (opportunityapp.SubmissionConfirmationResult, error) {
	s.commits++
	return opportunityapp.SubmissionConfirmationResult{}, nil
}
func (*submissionMFAStore) GetSubmissionConfirmationByBid(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.SubmissionConfirmation, error) {
	return opportunityapp.SubmissionConfirmation{}, nil
}

type submissionMFAStorage struct{}

func (submissionMFAStorage) PresignPut(context.Context, string, string, int64, [32]byte, time.Duration) (opportunityapp.UploadInstruction, error) {
	return opportunityapp.UploadInstruction{}, nil
}
func (submissionMFAStorage) Inspect(context.Context, string) (opportunityapp.StoredObject, error) {
	return opportunityapp.StoredObject{}, nil
}

func TestSubmissionConfirmationRequiresRecentTOTPAAL2(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	bid := mfaOpportunityID(10)
	authorizer := &submissionMFAAuthorizer{}
	store := &submissionMFAStore{}
	readiness := &mfaApprovalSource{readiness: opportunityapp.ProposalApprovalReadiness{BidID: bid, WorkspaceVersion: 4, Ready: true}}
	service, err := opportunityapp.NewSubmissionService(authorizer, store, readiness, submissionMFAStorage{}, &mfaApprovalIDs{}, mfaApprovalClock{now: now})
	if err != nil {
		t.Fatal(err)
	}
	base := opportunityapp.ConfirmSubmissionInput{Subject: opportunityapp.WorkflowSubject{IdentityID: mfaOpportunityID(1), SelectedTenantID: mfaOpportunityID(2), ContextGeneration: 1}, BidID: bid, EvidenceID: mfaOpportunityID(11), ManifestID: mfaOpportunityID(12), ProofVersionID: mfaOpportunityID(13), CorrelationID: mfaOpportunityID(14), ExpectedWorkspaceVersion: 4, Rationale: "Human verified external submission proof.", IdempotencyKey: "submission-assurance", SessionAuthenticatedAt: now, AssuranceAt: now, AssuranceLevel: "AAL2"}
	for _, method := range []string{"PRIMARY_ONLY", "RECOVERY_CODE"} {
		input := base
		input.AssuranceMethod = method
		if _, confirmErr := service.Confirm(context.Background(), input); confirmErr == nil || confirmErr.Error() != "submission_fresh_auth_required" {
			t.Fatalf("submission confirmation accepted %s: %v", method, confirmErr)
		}
	}
	stale := base
	stale.AssuranceMethod = "TOTP"
	stale.AssuranceAt = now.Add(-16 * time.Minute)
	if _, confirmErr := service.Confirm(context.Background(), stale); confirmErr == nil || confirmErr.Error() != "submission_fresh_auth_required" {
		t.Fatal("submission confirmation accepted stale TOTP", confirmErr)
	}
	future := base
	future.AssuranceMethod = "TOTP"
	future.AssuranceAt = now.Add(time.Second)
	if _, confirmErr := service.Confirm(context.Background(), future); confirmErr == nil || confirmErr.Error() != "submission_fresh_auth_required" {
		t.Fatal("submission confirmation accepted future TOTP", confirmErr)
	}
	allowed := base
	allowed.AssuranceMethod = "TOTP"
	if _, confirmErr := service.Confirm(context.Background(), allowed); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	if store.commits != 1 || authorizer.verifyCalls != 5 {
		t.Fatal("submission assurance boundary drifted", store.commits, authorizer.verifyCalls)
	}
}
