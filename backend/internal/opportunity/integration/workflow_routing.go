package integration

import (
	"context"
	"errors"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	tenancyapp "bidora/backend/internal/tenancy/application"
)

type TenantWorkflowStore interface {
	TransferMembershipOwnership(context.Context, authzapp.MembershipOffboardingOperation, authzapp.ID, string, authzapp.ID, authzapp.MembershipOffboardingIDs, time.Time) (authzapp.OwnershipTransferResult, error)
	ClearMembershipOwnershipFence(context.Context, authzapp.MembershipOffboardingOperation, time.Time) error
	Verify(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.VerifyCommit) (opportunityapp.VerificationResult, error)
	CreateWorkspace(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.WorkspaceCommit) (opportunityapp.WorkspaceResult, error)
	RecordDecision(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.DecisionCommit) (opportunityapp.DecisionResult, error)
	ListOpportunities(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ReadCursor, int32) ([]opportunityapp.OpportunitySummary, error)
	GetOpportunity(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.OpportunityDetail, error)
	ListBids(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.BidReadQuery, int32) ([]opportunityapp.BidSummary, error)
	GetBid(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.BidDetail, error)
	CreateRequirement(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.RequirementCommit) (opportunityapp.RequirementResult, error)
	AssignRequirement(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.AssignmentCommit) (opportunityapp.RequirementResult, error)
	ListRequirements(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) ([]opportunityapp.Requirement, error)
	GetReadiness(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.Readiness, error)
	ReserveUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.UploadReservation) (opportunityapp.UploadSession, error)
	GetUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.UploadSession, error)
	GetCompletedUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.UploadResult, error)
	CompleteUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.UploadCompletion) (opportunityapp.UploadResult, error)
	GetDocumentJob(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.DocumentJob, error)
	ListAISuggestions(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.AISuggestion, error)
	AcceptAISuggestion(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.AIAcceptCommit) (opportunityapp.AISuggestionReviewResult, error)
	RejectAISuggestion(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.AIRejectCommit) (opportunityapp.AISuggestionReviewResult, error)
	ReserveEvidenceUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.EvidenceUploadReservation) (opportunityapp.EvidenceUploadSession, error)
	GetEvidenceUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.EvidenceUploadSession, error)
	GetCompletedEvidenceUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.EvidenceUploadResult, error)
	CompleteEvidenceUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.EvidenceUploadCompletion) (opportunityapp.EvidenceUploadResult, error)
	SatisfyRequirement(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.SatisfyRequirementCommit) (opportunityapp.SatisfyRequirementResult, error)
	ListRequirementEvidence(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.RequirementEvidence, error)
	SaveTechnicalResponse(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.TechnicalResponseCommit) (opportunityapp.TechnicalResponseResult, error)
	GetTechnicalResponse(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.TechnicalResponse, error)
	ListTechnicalResponseHistory(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.TechnicalResponse, error)
	GetGapReview(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.GapReview, error)
	PrepareResponseDraft(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID, int64, int64, time.Time) (opportunityapp.PreparedResponseDraft, error)
	RequestResponseDraft(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ResponseDraftRequestCommit) (opportunityapp.ResponseDraftRequestResult, error)
	ListResponseDrafts(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ResponseDraftSuggestion, error)
	SaveProposal(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ProposalCommit) (opportunityapp.ProposalSaveResult, error)
	GetProposal(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.ProposalDocument, error)
	GetProposalVersion(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalDocument, error)
	ListProposalHistory(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) ([]opportunityapp.ProposalHistoryItem, error)
	PrepareProposalDraft(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID, int64, int64, time.Time) (opportunityapp.PreparedProposalDraft, error)
	RequestProposalDraft(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ProposalDraftCommit) (opportunityapp.ProposalDraftRequestResult, error)
	ListProposalDrafts(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ProposalDraftSuggestion, error)
	ApplyProposalSuggestion(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ApplyProposalSuggestionCommit) (opportunityapp.ProposalSaveResult, error)
	GetProposalApprovalReadiness(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.ProposalApprovalReadiness, error)
	CorrectRequirement(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.RequirementCorrectionCommit) (opportunityapp.RequirementCorrectionResult, error)
	ListRequirementCorrections(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.RequirementContentRevision, error)
	CommitReviewThread(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ReviewThreadCommit) (opportunityapp.ReviewThreadResult, error)
	ListReviewThreads(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) ([]opportunityapp.ReviewThread, error)
	AssignReviewThread(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ReviewAssignmentCommit) (opportunityapp.ReviewThreadResult, error)
	ListReviewAssignments(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ReviewAssignment, error)
	GetReviewDashboard(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.ReviewDashboard, error)
	GetTenantReviewWorkloads(context.Context, opportunityapp.WorkflowAuthority, time.Time) (opportunityapp.TenantReviewWorkloads, error)
	ListReviewerOpenWork(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) ([]opportunityapp.ReviewerOpenWork, error)
	CommitInternalReviewDecision(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.InternalReviewDecisionCommit) (opportunityapp.InternalReviewDecisionResult, error)
	ListInternalReviewDecisions(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.InternalReviewDecision, error)
	GetApprovalPolicyPreparation(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.ApprovalPolicyPreparation, error)
	ListCollaborationTimeline(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ReadCursor, int32) ([]opportunityapp.CollaborationActivity, error)
	FindProposalPackageReplay(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, [32]byte, [32]byte) (opportunityapp.ProposalPackageResult, bool, error)
	CommitProposalPackage(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ProposalPackageCommit) (opportunityapp.ProposalPackageResult, error)
	GetProposalPackageManifest(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalPackageManifest, error)
	ListProposalPackageManifests(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ReadCursor, int32) ([]opportunityapp.ProposalPackageManifest, error)
	FindProposalApprovalReplay(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, [32]byte, [32]byte) (opportunityapp.ProposalApprovalResult, bool, error)
	CommitProposalApproval(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ProposalApprovalCommit) (opportunityapp.ProposalApprovalResult, error)
	GetProposalApprovalByManifest(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ProposalApproval, error)
	ReserveSubmissionProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.SubmissionProofReservation) (opportunityapp.SubmissionProofUpload, error)
	GetSubmissionProofUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID, opportunityapp.ID) (opportunityapp.SubmissionProofUpload, error)
	GetCompletedSubmissionProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID, opportunityapp.ID) (opportunityapp.SubmissionProofResult, error)
	CompleteSubmissionProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.SubmissionProofCompletion) (opportunityapp.SubmissionProofResult, error)
	FindSubmissionReplay(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, [32]byte, [32]byte) (opportunityapp.SubmissionConfirmationResult, bool, error)
	CommitSubmissionConfirmation(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.SubmissionConfirmationCommit) (opportunityapp.SubmissionConfirmationResult, error)
	GetSubmissionConfirmationByBid(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.SubmissionConfirmation, error)
	ReserveOutcomeProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.OutcomeProofReservation) (opportunityapp.OutcomeProofUpload, error)
	GetOutcomeProofUpload(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.OutcomeProofUpload, error)
	GetCompletedOutcomeProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.OutcomeProofResult, error)
	CompleteOutcomeProof(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.OutcomeProofCompletion) (opportunityapp.OutcomeProofResult, error)
	FindReportedOutcomeReplay(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, [32]byte, [32]byte) (opportunityapp.ReportOutcomeResult, bool, error)
	CommitReportedOutcome(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ReportOutcomeCommit) (opportunityapp.ReportOutcomeResult, error)
	GetCurrentReportedOutcome(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.ReportedOutcome, error)
	ListReportedOutcomes(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) ([]opportunityapp.ReportedOutcome, error)
	FindBidArchiveReplay(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, [32]byte, [32]byte) (opportunityapp.BidArchiveResult, bool, error)
	CommitBidArchive(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.BidArchiveCommit) (opportunityapp.BidArchiveResult, error)
	GetBidArchive(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID) (opportunityapp.BidArchive, error)
	CommitExternalActionEvidence(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ExternalActionEvidenceCommit) (opportunityapp.ExternalActionEvidenceResult, error)
	GetExternalActionEvidence(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ID) (opportunityapp.ExternalActionEvidence, error)
	ListExternalActionEvidence(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, opportunityapp.ReadCursor, int32) ([]opportunityapp.ExternalActionEvidence, error)
	GetExternalEvidenceReconciliation(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.ExternalEvidenceReconciliation, error)
	GetBidArchivePreparation(context.Context, opportunityapp.WorkflowAuthority, opportunityapp.ID, time.Time) (opportunityapp.BidArchivePreparation, error)
	ListOwnershipContinuityActivity(context.Context, opportunityapp.WorkflowAuthority, time.Time) ([]opportunityapp.OwnershipActivity, error)
}
type WorkflowStoreRegistration struct {
	Assignment tenancyapp.Assignment
	Store      TenantWorkflowStore
}
type RoutedWorkflowStore struct {
	assignments CaptureAssignmentResolver
	stores      map[tenancyapp.Assignment]TenantWorkflowStore
}

func NewRoutedWorkflowStore(assignments CaptureAssignmentResolver, registrations []WorkflowStoreRegistration) (*RoutedWorkflowStore, error) {
	if assignments == nil || len(registrations) == 0 {
		return nil, errors.New("workflow_router_invalid")
	}
	stores := make(map[tenancyapp.Assignment]TenantWorkflowStore, len(registrations))
	for _, registration := range registrations {
		if !registration.Assignment.Valid() || registration.Store == nil {
			return nil, errors.New("workflow_router_invalid")
		}
		if _, exists := stores[registration.Assignment]; exists {
			return nil, errors.New("workflow_router_invalid")
		}
		stores[registration.Assignment] = registration.Store
	}
	return &RoutedWorkflowStore{assignments, stores}, nil
}
func (r *RoutedWorkflowStore) resolve(ctx context.Context, authority opportunityapp.WorkflowAuthority) (TenantWorkflowStore, error) {
	if ctx == nil || authority.TenantID.IsZero() || authority.AssignmentGeneration < 1 {
		return nil, errors.New("workflow_routing_invalid")
	}
	assignment, err := r.assignments.ResolveCaptureAssignment(ctx, tenancyapp.ID(authority.TenantID), authority.AssignmentGeneration)
	if err != nil {
		return nil, err
	}
	store, ok := r.stores[assignment]
	if !ok {
		return nil, errors.New("workflow_store_unavailable")
	}
	return store, nil
}
func (r *RoutedWorkflowStore) Verify(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.VerifyCommit) (opportunityapp.VerificationResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.VerificationResult{}, err
	}
	return store.Verify(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) ReserveUpload(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.UploadReservation) (opportunityapp.UploadSession, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.UploadSession{}, err
	}
	return store.ReserveUpload(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) GetUpload(ctx context.Context, authority opportunityapp.WorkflowAuthority, opportunityID, uploadID opportunityapp.ID) (opportunityapp.UploadSession, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.UploadSession{}, err
	}
	return store.GetUpload(ctx, authority, opportunityID, uploadID)
}
func (r *RoutedWorkflowStore) GetCompletedUpload(ctx context.Context, authority opportunityapp.WorkflowAuthority, opportunityID, uploadID opportunityapp.ID) (opportunityapp.UploadResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.UploadResult{}, err
	}
	return store.GetCompletedUpload(ctx, authority, opportunityID, uploadID)
}
func (r *RoutedWorkflowStore) CompleteUpload(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.UploadCompletion) (opportunityapp.UploadResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.UploadResult{}, err
	}
	return store.CompleteUpload(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) PrepareResponseDraft(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, requirementID opportunityapp.ID, workspaceVersion, sectionVersion int64, observed time.Time) (opportunityapp.PreparedResponseDraft, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.PreparedResponseDraft{}, e
	}
	return s.PrepareResponseDraft(ctx, a, bidID, requirementID, workspaceVersion, sectionVersion, observed)
}
func (r *RoutedWorkflowStore) RequestResponseDraft(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ResponseDraftRequestCommit) (opportunityapp.ResponseDraftRequestResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ResponseDraftRequestResult{}, e
	}
	return s.RequestResponseDraft(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListResponseDrafts(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, requirementID opportunityapp.ID) ([]opportunityapp.ResponseDraftSuggestion, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListResponseDrafts(ctx, a, bidID, requirementID)
}
func (r *RoutedWorkflowStore) ListOwnershipContinuityActivity(ctx context.Context, a opportunityapp.WorkflowAuthority, at time.Time) ([]opportunityapp.OwnershipActivity, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListOwnershipContinuityActivity(ctx, a, at)
}
func (r *RoutedWorkflowStore) GetBidArchivePreparation(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, at time.Time) (opportunityapp.BidArchivePreparation, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.BidArchivePreparation{}, e
	}
	return s.GetBidArchivePreparation(ctx, a, bidID, at)
}
func (r *RoutedWorkflowStore) GetExternalEvidenceReconciliation(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, at time.Time) (opportunityapp.ExternalEvidenceReconciliation, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ExternalEvidenceReconciliation{}, e
	}
	return s.GetExternalEvidenceReconciliation(ctx, a, bidID, at)
}
func (r *RoutedWorkflowStore) CommitExternalActionEvidence(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ExternalActionEvidenceCommit) (opportunityapp.ExternalActionEvidenceResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ExternalActionEvidenceResult{}, e
	}
	return s.CommitExternalActionEvidence(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetExternalActionEvidence(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, evidenceID opportunityapp.ID) (opportunityapp.ExternalActionEvidence, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ExternalActionEvidence{}, e
	}
	return s.GetExternalActionEvidence(ctx, a, bidID, evidenceID)
}
func (r *RoutedWorkflowStore) ListExternalActionEvidence(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, cursor opportunityapp.ReadCursor, limit int32) ([]opportunityapp.ExternalActionEvidence, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListExternalActionEvidence(ctx, a, bidID, cursor, limit)
}
func (r *RoutedWorkflowStore) GetProposalPackageManifest(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, manifestID opportunityapp.ID) (opportunityapp.ProposalPackageManifest, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalPackageManifest{}, e
	}
	return s.GetProposalPackageManifest(ctx, a, bidID, manifestID)
}
func (r *RoutedWorkflowStore) ListProposalPackageManifests(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, cursor opportunityapp.ReadCursor, limit int32) ([]opportunityapp.ProposalPackageManifest, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListProposalPackageManifests(ctx, a, bidID, cursor, limit)
}
func (r *RoutedWorkflowStore) FindProposalPackageReplay(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, keyHash, requestHash [32]byte) (opportunityapp.ProposalPackageResult, bool, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalPackageResult{}, false, e
	}
	return s.FindProposalPackageReplay(ctx, a, bidID, keyHash, requestHash)
}
func (r *RoutedWorkflowStore) CommitProposalPackage(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ProposalPackageCommit) (opportunityapp.ProposalPackageResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalPackageResult{}, e
	}
	return s.CommitProposalPackage(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListCollaborationTimeline(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, cursor opportunityapp.ReadCursor, limit int32) ([]opportunityapp.CollaborationActivity, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListCollaborationTimeline(ctx, a, bidID, cursor, limit)
}
func (r *RoutedWorkflowStore) CommitInternalReviewDecision(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.InternalReviewDecisionCommit) (opportunityapp.InternalReviewDecisionResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.InternalReviewDecisionResult{}, e
	}
	return s.CommitInternalReviewDecision(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListInternalReviewDecisions(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, versionID opportunityapp.ID) ([]opportunityapp.InternalReviewDecision, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListInternalReviewDecisions(ctx, a, bidID, versionID)
}
func (r *RoutedWorkflowStore) GetApprovalPolicyPreparation(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, at time.Time) (opportunityapp.ApprovalPolicyPreparation, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ApprovalPolicyPreparation{}, e
	}
	return s.GetApprovalPolicyPreparation(ctx, a, bidID, at)
}
func (r *RoutedWorkflowStore) GetReviewDashboard(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, at time.Time) (opportunityapp.ReviewDashboard, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ReviewDashboard{}, e
	}
	return s.GetReviewDashboard(ctx, a, bidID, at)
}
func (r *RoutedWorkflowStore) GetTenantReviewWorkloads(ctx context.Context, a opportunityapp.WorkflowAuthority, at time.Time) (opportunityapp.TenantReviewWorkloads, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.TenantReviewWorkloads{}, e
	}
	return s.GetTenantReviewWorkloads(ctx, a, at)
}
func (r *RoutedWorkflowStore) ListReviewerOpenWork(ctx context.Context, a opportunityapp.WorkflowAuthority, reviewerID opportunityapp.ID) ([]opportunityapp.ReviewerOpenWork, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListReviewerOpenWork(ctx, a, reviewerID)
}
func (r *RoutedWorkflowStore) AssignReviewThread(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ReviewAssignmentCommit) (opportunityapp.ReviewThreadResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ReviewThreadResult{}, e
	}
	return s.AssignReviewThread(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListReviewAssignments(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, threadID opportunityapp.ID) ([]opportunityapp.ReviewAssignment, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListReviewAssignments(ctx, a, bidID, threadID)
}
func (r *RoutedWorkflowStore) CommitReviewThread(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ReviewThreadCommit) (opportunityapp.ReviewThreadResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ReviewThreadResult{}, e
	}
	return s.CommitReviewThread(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListReviewThreads(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID) ([]opportunityapp.ReviewThread, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListReviewThreads(ctx, a, bidID)
}
func (r *RoutedWorkflowStore) CorrectRequirement(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.RequirementCorrectionCommit) (opportunityapp.RequirementCorrectionResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.RequirementCorrectionResult{}, e
	}
	return s.CorrectRequirement(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListRequirementCorrections(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, requirementID opportunityapp.ID) ([]opportunityapp.RequirementContentRevision, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListRequirementCorrections(ctx, a, bidID, requirementID)
}
func (r *RoutedWorkflowStore) ApplyProposalSuggestion(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ApplyProposalSuggestionCommit) (opportunityapp.ProposalSaveResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalSaveResult{}, e
	}
	return s.ApplyProposalSuggestion(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetProposalApprovalReadiness(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, at time.Time) (opportunityapp.ProposalApprovalReadiness, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalApprovalReadiness{}, e
	}
	return s.GetProposalApprovalReadiness(ctx, a, bidID, at)
}
func (r *RoutedWorkflowStore) PrepareProposalDraft(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, versionID opportunityapp.ID, workspaceVersion, proposalVersion int64, observed time.Time) (opportunityapp.PreparedProposalDraft, error) {
	s, err := r.resolve(ctx, a)
	if err != nil {
		return opportunityapp.PreparedProposalDraft{}, err
	}
	return s.PrepareProposalDraft(ctx, a, bidID, versionID, workspaceVersion, proposalVersion, observed)
}

func (r *RoutedWorkflowStore) RequestProposalDraft(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ProposalDraftCommit) (opportunityapp.ProposalDraftRequestResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalDraftRequestResult{}, e
	}
	return s.RequestProposalDraft(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListProposalDrafts(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, versionID opportunityapp.ID) ([]opportunityapp.ProposalDraftSuggestion, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListProposalDrafts(ctx, a, bidID, versionID)
}
func (r *RoutedWorkflowStore) SaveProposal(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ProposalCommit) (opportunityapp.ProposalSaveResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalSaveResult{}, e
	}
	return s.SaveProposal(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetProposal(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID) (opportunityapp.ProposalDocument, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalDocument{}, e
	}
	return s.GetProposal(ctx, a, bidID)
}
func (r *RoutedWorkflowStore) GetProposalVersion(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, versionID opportunityapp.ID) (opportunityapp.ProposalDocument, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalDocument{}, e
	}
	return s.GetProposalVersion(ctx, a, bidID, versionID)
}
func (r *RoutedWorkflowStore) ListProposalHistory(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID) ([]opportunityapp.ProposalHistoryItem, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListProposalHistory(ctx, a, bidID)
}
func (r *RoutedWorkflowStore) SaveTechnicalResponse(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.TechnicalResponseCommit) (opportunityapp.TechnicalResponseResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.TechnicalResponseResult{}, e
	}
	return s.SaveTechnicalResponse(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetTechnicalResponse(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, requirementID opportunityapp.ID) (opportunityapp.TechnicalResponse, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.TechnicalResponse{}, e
	}
	return s.GetTechnicalResponse(ctx, a, bidID, requirementID)
}
func (r *RoutedWorkflowStore) ListTechnicalResponseHistory(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, requirementID opportunityapp.ID) ([]opportunityapp.TechnicalResponse, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListTechnicalResponseHistory(ctx, a, bidID, requirementID)
}
func (r *RoutedWorkflowStore) GetGapReview(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, at time.Time) (opportunityapp.GapReview, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.GapReview{}, e
	}
	return s.GetGapReview(ctx, a, bidID, at)
}
func (r *RoutedWorkflowStore) ReserveEvidenceUpload(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.EvidenceUploadReservation) (opportunityapp.EvidenceUploadSession, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.EvidenceUploadSession{}, e
	}
	return s.ReserveEvidenceUpload(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetEvidenceUpload(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, uploadID opportunityapp.ID) (opportunityapp.EvidenceUploadSession, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.EvidenceUploadSession{}, e
	}
	return s.GetEvidenceUpload(ctx, a, bidID, uploadID)
}
func (r *RoutedWorkflowStore) GetCompletedEvidenceUpload(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, uploadID opportunityapp.ID) (opportunityapp.EvidenceUploadResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.EvidenceUploadResult{}, e
	}
	return s.GetCompletedEvidenceUpload(ctx, a, bidID, uploadID)
}
func (r *RoutedWorkflowStore) CompleteEvidenceUpload(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.EvidenceUploadCompletion) (opportunityapp.EvidenceUploadResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.EvidenceUploadResult{}, e
	}
	return s.CompleteEvidenceUpload(ctx, a, c)
}
func (r *RoutedWorkflowStore) SatisfyRequirement(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.SatisfyRequirementCommit) (opportunityapp.SatisfyRequirementResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SatisfyRequirementResult{}, e
	}
	return s.SatisfyRequirement(ctx, a, c)
}
func (r *RoutedWorkflowStore) ListRequirementEvidence(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, requirementID opportunityapp.ID) ([]opportunityapp.RequirementEvidence, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListRequirementEvidence(ctx, a, bidID, requirementID)
}
func (r *RoutedWorkflowStore) ListAISuggestions(ctx context.Context, authority opportunityapp.WorkflowAuthority, opportunityID, documentVersionID opportunityapp.ID) ([]opportunityapp.AISuggestion, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return nil, err
	}
	return store.ListAISuggestions(ctx, authority, opportunityID, documentVersionID)
}
func (r *RoutedWorkflowStore) AcceptAISuggestion(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.AIAcceptCommit) (opportunityapp.AISuggestionReviewResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.AISuggestionReviewResult{}, err
	}
	return store.AcceptAISuggestion(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) RejectAISuggestion(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.AIRejectCommit) (opportunityapp.AISuggestionReviewResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.AISuggestionReviewResult{}, err
	}
	return store.RejectAISuggestion(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) GetDocumentJob(ctx context.Context, authority opportunityapp.WorkflowAuthority, jobID opportunityapp.ID) (opportunityapp.DocumentJob, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.DocumentJob{}, err
	}
	return store.GetDocumentJob(ctx, authority, jobID)
}
func (r *RoutedWorkflowStore) CreateRequirement(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.RequirementCommit) (opportunityapp.RequirementResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.RequirementResult{}, err
	}
	return store.CreateRequirement(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) AssignRequirement(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.AssignmentCommit) (opportunityapp.RequirementResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.RequirementResult{}, err
	}
	return store.AssignRequirement(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) ListRequirements(ctx context.Context, authority opportunityapp.WorkflowAuthority, id opportunityapp.ID) ([]opportunityapp.Requirement, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return nil, err
	}
	return store.ListRequirements(ctx, authority, id)
}
func (r *RoutedWorkflowStore) GetReadiness(ctx context.Context, authority opportunityapp.WorkflowAuthority, id opportunityapp.ID, at time.Time) (opportunityapp.Readiness, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.Readiness{}, err
	}
	return store.GetReadiness(ctx, authority, id, at)
}
func (r *RoutedWorkflowStore) ListOpportunities(ctx context.Context, authority opportunityapp.WorkflowAuthority, cursor opportunityapp.ReadCursor, maxRows int32) ([]opportunityapp.OpportunitySummary, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return nil, err
	}
	return store.ListOpportunities(ctx, authority, cursor, maxRows)
}
func (r *RoutedWorkflowStore) GetOpportunity(ctx context.Context, authority opportunityapp.WorkflowAuthority, id opportunityapp.ID) (opportunityapp.OpportunityDetail, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.OpportunityDetail{}, err
	}
	return store.GetOpportunity(ctx, authority, id)
}
func (r *RoutedWorkflowStore) ListBids(ctx context.Context, authority opportunityapp.WorkflowAuthority, query opportunityapp.BidReadQuery, maxRows int32) ([]opportunityapp.BidSummary, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return nil, err
	}
	return store.ListBids(ctx, authority, query, maxRows)
}
func (r *RoutedWorkflowStore) GetBid(ctx context.Context, authority opportunityapp.WorkflowAuthority, id opportunityapp.ID) (opportunityapp.BidDetail, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.BidDetail{}, err
	}
	return store.GetBid(ctx, authority, id)
}
func (r *RoutedWorkflowStore) RecordDecision(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.DecisionCommit) (opportunityapp.DecisionResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.DecisionResult{}, err
	}
	return store.RecordDecision(ctx, authority, commit)
}
func (r *RoutedWorkflowStore) FindProposalApprovalReplay(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID opportunityapp.ID, key, request [32]byte) (opportunityapp.ProposalApprovalResult, bool, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalApprovalResult{}, false, e
	}
	return s.FindProposalApprovalReplay(ctx, a, bidID, key, request)
}
func (r *RoutedWorkflowStore) CommitProposalApproval(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ProposalApprovalCommit) (opportunityapp.ProposalApprovalResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalApprovalResult{}, e
	}
	return s.CommitProposalApproval(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetProposalApprovalByManifest(ctx context.Context, a opportunityapp.WorkflowAuthority, bidID, manifestID opportunityapp.ID) (opportunityapp.ProposalApproval, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ProposalApproval{}, e
	}
	return s.GetProposalApprovalByManifest(ctx, a, bidID, manifestID)
}
func (r *RoutedWorkflowStore) ReserveSubmissionProof(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.SubmissionProofReservation) (opportunityapp.SubmissionProofUpload, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SubmissionProofUpload{}, e
	}
	return s.ReserveSubmissionProof(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetSubmissionProofUpload(ctx context.Context, a opportunityapp.WorkflowAuthority, b, eid, u opportunityapp.ID) (opportunityapp.SubmissionProofUpload, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SubmissionProofUpload{}, e
	}
	return s.GetSubmissionProofUpload(ctx, a, b, eid, u)
}
func (r *RoutedWorkflowStore) GetCompletedSubmissionProof(ctx context.Context, a opportunityapp.WorkflowAuthority, b, eid, u opportunityapp.ID) (opportunityapp.SubmissionProofResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SubmissionProofResult{}, e
	}
	return s.GetCompletedSubmissionProof(ctx, a, b, eid, u)
}
func (r *RoutedWorkflowStore) CompleteSubmissionProof(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.SubmissionProofCompletion) (opportunityapp.SubmissionProofResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SubmissionProofResult{}, e
	}
	return s.CompleteSubmissionProof(ctx, a, c)
}
func (r *RoutedWorkflowStore) FindSubmissionReplay(ctx context.Context, a opportunityapp.WorkflowAuthority, b opportunityapp.ID, k, h [32]byte) (opportunityapp.SubmissionConfirmationResult, bool, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SubmissionConfirmationResult{}, false, e
	}
	return s.FindSubmissionReplay(ctx, a, b, k, h)
}
func (r *RoutedWorkflowStore) CommitSubmissionConfirmation(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.SubmissionConfirmationCommit) (opportunityapp.SubmissionConfirmationResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SubmissionConfirmationResult{}, e
	}
	return s.CommitSubmissionConfirmation(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetSubmissionConfirmationByBid(ctx context.Context, a opportunityapp.WorkflowAuthority, b opportunityapp.ID) (opportunityapp.SubmissionConfirmation, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.SubmissionConfirmation{}, e
	}
	return s.GetSubmissionConfirmationByBid(ctx, a, b)
}
func (r *RoutedWorkflowStore) ReserveOutcomeProof(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.OutcomeProofReservation) (opportunityapp.OutcomeProofUpload, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.OutcomeProofUpload{}, e
	}
	return s.ReserveOutcomeProof(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetOutcomeProofUpload(ctx context.Context, a opportunityapp.WorkflowAuthority, b, u opportunityapp.ID) (opportunityapp.OutcomeProofUpload, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.OutcomeProofUpload{}, e
	}
	return s.GetOutcomeProofUpload(ctx, a, b, u)
}
func (r *RoutedWorkflowStore) GetCompletedOutcomeProof(ctx context.Context, a opportunityapp.WorkflowAuthority, b, u opportunityapp.ID) (opportunityapp.OutcomeProofResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.OutcomeProofResult{}, e
	}
	return s.GetCompletedOutcomeProof(ctx, a, b, u)
}
func (r *RoutedWorkflowStore) CompleteOutcomeProof(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.OutcomeProofCompletion) (opportunityapp.OutcomeProofResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.OutcomeProofResult{}, e
	}
	return s.CompleteOutcomeProof(ctx, a, c)
}
func (r *RoutedWorkflowStore) FindReportedOutcomeReplay(ctx context.Context, a opportunityapp.WorkflowAuthority, b opportunityapp.ID, k, h [32]byte) (opportunityapp.ReportOutcomeResult, bool, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ReportOutcomeResult{}, false, e
	}
	return s.FindReportedOutcomeReplay(ctx, a, b, k, h)
}
func (r *RoutedWorkflowStore) CommitReportedOutcome(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.ReportOutcomeCommit) (opportunityapp.ReportOutcomeResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ReportOutcomeResult{}, e
	}
	return s.CommitReportedOutcome(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetCurrentReportedOutcome(ctx context.Context, a opportunityapp.WorkflowAuthority, b opportunityapp.ID) (opportunityapp.ReportedOutcome, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.ReportedOutcome{}, e
	}
	return s.GetCurrentReportedOutcome(ctx, a, b)
}
func (r *RoutedWorkflowStore) ListReportedOutcomes(ctx context.Context, a opportunityapp.WorkflowAuthority, b opportunityapp.ID) ([]opportunityapp.ReportedOutcome, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return nil, e
	}
	return s.ListReportedOutcomes(ctx, a, b)
}
func (r *RoutedWorkflowStore) FindBidArchiveReplay(ctx context.Context, a opportunityapp.WorkflowAuthority, b opportunityapp.ID, k, h [32]byte) (opportunityapp.BidArchiveResult, bool, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.BidArchiveResult{}, false, e
	}
	return s.FindBidArchiveReplay(ctx, a, b, k, h)
}
func (r *RoutedWorkflowStore) CommitBidArchive(ctx context.Context, a opportunityapp.WorkflowAuthority, c opportunityapp.BidArchiveCommit) (opportunityapp.BidArchiveResult, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.BidArchiveResult{}, e
	}
	return s.CommitBidArchive(ctx, a, c)
}
func (r *RoutedWorkflowStore) GetBidArchive(ctx context.Context, a opportunityapp.WorkflowAuthority, b opportunityapp.ID) (opportunityapp.BidArchive, error) {
	s, e := r.resolve(ctx, a)
	if e != nil {
		return opportunityapp.BidArchive{}, e
	}
	return s.GetBidArchive(ctx, a, b)
}
func (r *RoutedWorkflowStore) TransferMembershipOwnership(ctx context.Context, op authzapp.MembershipOffboardingOperation, actor authzapp.ID, reason string, correlation authzapp.ID, ids authzapp.MembershipOffboardingIDs, at time.Time) (authzapp.OwnershipTransferResult, error) {
	a := opportunityapp.WorkflowAuthority{TenantID: opportunityapp.ID(op.TenantID), ActorID: opportunityapp.ID(actor), AssignmentGeneration: op.AssignmentGeneration}
	s, e := r.resolve(ctx, a)
	if e != nil {
		return authzapp.OwnershipTransferResult{}, e
	}
	return s.TransferMembershipOwnership(ctx, op, actor, reason, correlation, ids, at)
}
func (r *RoutedWorkflowStore) ClearMembershipOwnershipFence(ctx context.Context, op authzapp.MembershipOffboardingOperation, at time.Time) error {
	a := opportunityapp.WorkflowAuthority{TenantID: opportunityapp.ID(op.TenantID), ActorID: opportunityapp.ID(op.ActorID), AssignmentGeneration: op.AssignmentGeneration}
	s, e := r.resolve(ctx, a)
	if e != nil {
		return e
	}
	return s.ClearMembershipOwnershipFence(ctx, op, at)
}
func (r *RoutedWorkflowStore) CreateWorkspace(ctx context.Context, authority opportunityapp.WorkflowAuthority, commit opportunityapp.WorkspaceCommit) (opportunityapp.WorkspaceResult, error) {
	store, err := r.resolve(ctx, authority)
	if err != nil {
		return opportunityapp.WorkspaceResult{}, err
	}
	return store.CreateWorkspace(ctx, authority, commit)
}
