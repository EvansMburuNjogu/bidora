package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

type ApplyProposalSuggestionInput struct {
	Subject                                                                      WorkflowSubject
	BidID, SuggestionID, CorrelationID                                           ID
	Introduction, Closing, ReviewNote                                            string
	ExpectedSuggestionVersion, ExpectedProposalVersion, ExpectedWorkspaceVersion int64
	IdempotencyKey                                                               string
}
type ApplyProposalSuggestionCommit struct {
	CommandID, ProposalVersionID, ProposalAuditID, ProposalOutboxID, ReviewAuditID, ReviewOutboxID, BidID, SuggestionID, CorrelationID ID
	Introduction, Closing, ReviewNote                                                                                                  string
	ExpectedSuggestionVersion, ExpectedProposalVersion, ExpectedWorkspaceVersion                                                       int64
	KeyHash, RequestHash                                                                                                               [32]byte
	OccurredAt                                                                                                                         time.Time
}
type ProposalApplicationStore interface {
	ApplyProposalSuggestion(context.Context, WorkflowAuthority, ApplyProposalSuggestionCommit) (ProposalSaveResult, error)
	GetProposalApprovalReadiness(context.Context, WorkflowAuthority, ID, time.Time) (ProposalApprovalReadiness, error)
}
type ProposalApprovalReadiness struct {
	BidID, ProposalID, ProposalVersionID                                                                                                                                                              ID
	WorkspaceState                                                                                                                                                                                    string
	WorkspaceVersion, ProposalVersion                                                                                                                                                                 int64
	Ready                                                                                                                                                                                             bool
	Blockers                                                                                                                                                                                          []string
	RequirementCount, OpenReviewThreadCount, UnassignedReviewThreadCount, OverdueReviewThreadCount, MandatoryUnsatisfied, OpenUnassigned, OpenOverdue, MandatoryResponseMissing, ExpiredEvidenceCount int64
	ObservedAt                                                                                                                                                                                        time.Time
}
type ProposalApplicationService struct {
	auth  TechnicalResponseAuthorizer
	store ProposalApplicationStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewProposalApplicationService(auth TechnicalResponseAuthorizer, store ProposalApplicationStore, ids CaptureIDGenerator, clock CaptureClock) (*ProposalApplicationService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("proposal_application_service_invalid")
	}
	return &ProposalApplicationService{auth, store, ids, clock}, nil
}
func (s *ProposalApplicationService) Apply(ctx context.Context, in ApplyProposalSuggestionInput) (ProposalSaveResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.SuggestionID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedSuggestionVersion < 1 || in.ExpectedProposalVersion < 1 || in.ExpectedWorkspaceVersion < 1 || in.IdempotencyKey == "" || !validPlainResponse(in.Introduction, 10000) || !validPlainResponse(in.Closing, 10000) || !validRequirementText(in.ReviewNote, 2000) {
		return ProposalSaveResult{}, safe("proposal_suggestion_apply_invalid", nil)
	}
	a, err := s.auth.AuthorizeResponseEdit(ctx, in.Subject)
	if err != nil {
		return ProposalSaveResult{}, safe("proposal_suggestion_not_authorized", err)
	}
	ids, err := responseDraftIDs(s.ids, 6)
	if err != nil {
		return ProposalSaveResult{}, safe("proposal_suggestion_apply_unavailable", err)
	}
	payload, _ := json.Marshal(struct {
		Bid, Suggestion                                      string
		Introduction, Closing, ReviewNote                    string
		SuggestionVersion, ProposalVersion, WorkspaceVersion int64
	}{string(in.BidID[:]), string(in.SuggestionID[:]), in.Introduction, in.Closing, in.ReviewNote, in.ExpectedSuggestionVersion, in.ExpectedProposalVersion, in.ExpectedWorkspaceVersion})
	result, err := s.store.ApplyProposalSuggestion(ctx, a, ApplyProposalSuggestionCommit{CommandID: ids[0], ProposalVersionID: ids[1], ProposalAuditID: ids[2], ProposalOutboxID: ids[3], ReviewAuditID: ids[4], ReviewOutboxID: ids[5], BidID: in.BidID, SuggestionID: in.SuggestionID, CorrelationID: in.CorrelationID, Introduction: in.Introduction, Closing: in.Closing, ReviewNote: in.ReviewNote, ExpectedSuggestionVersion: in.ExpectedSuggestionVersion, ExpectedProposalVersion: in.ExpectedProposalVersion, ExpectedWorkspaceVersion: in.ExpectedWorkspaceVersion, KeyHash: sha256.Sum256([]byte("apply-proposal-suggestion:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(append([]byte("apply-proposal-suggestion:v1:"), payload...)), OccurredAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrProposalConflict) {
		return ProposalSaveResult{}, safe("proposal_suggestion_apply_conflict", err)
	}
	if err != nil {
		return ProposalSaveResult{}, safe("proposal_suggestion_apply_unavailable", err)
	}
	return result, nil
}
func (s *ProposalApplicationService) Readiness(ctx context.Context, subject WorkflowSubject, bidID ID) (ProposalApprovalReadiness, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return ProposalApprovalReadiness{}, safe("proposal_approval_readiness_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return ProposalApprovalReadiness{}, safe("proposal_suggestion_not_authorized", err)
	}
	result, err := s.store.GetProposalApprovalReadiness(ctx, a, bidID, s.clock.Now().UTC())
	if err != nil {
		return ProposalApprovalReadiness{}, safe("proposal_approval_readiness_unavailable", err)
	}
	result.Ready = len(result.Blockers) == 0
	return result, nil
}
