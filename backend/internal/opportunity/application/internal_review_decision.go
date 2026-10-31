package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrInternalReviewConflict = errors.New("internal_review_conflict")
var ErrInternalReviewReadLimit = errors.New("internal_review_read_limit")
var ErrInternalReviewNotFound = errors.New("internal_review_not_found")

type RecordInternalReviewDecisionInput struct {
	Subject                                 WorkflowSubject
	BidID, ProposalVersionID, CorrelationID ID
	Decision, Rationale, IdempotencyKey     string
	ExpectedPreviousSequence                int32
}
type InternalReviewDecisionCommit struct {
	CommandID, DecisionID, AuditID, OutboxID, BidID, ProposalVersionID, CorrelationID ID
	Decision, Rationale                                                               string
	ExpectedPreviousSequence                                                          int32
	KeyHash, RequestHash                                                              [32]byte
	OccurredAt                                                                        time.Time
}
type InternalReviewDecision struct {
	ID, BidID, ProposalVersionID, ReviewerID ID
	ReviewerSequence                         int32
	Decision, Rationale                      string
	DecidedAt                                time.Time
}
type InternalReviewDecisionResult struct {
	Decision InternalReviewDecision
	Replay   bool
}
type ApprovalPolicyPreparation struct {
	BidID, ProposalVersionID                                                         ID
	ReviewerCount, ReviewCompleteCount, ChangesRequestedCount, OpenReviewThreadCount int64
	ApprovalPolicyConfigured, EligibleForApproval                                    bool
	Blockers                                                                         []string
	ObservedAt                                                                       time.Time
}
type InternalReviewDecisionStore interface {
	CommitInternalReviewDecision(context.Context, WorkflowAuthority, InternalReviewDecisionCommit) (InternalReviewDecisionResult, error)
	ListInternalReviewDecisions(context.Context, WorkflowAuthority, ID, ID) ([]InternalReviewDecision, error)
	GetApprovalPolicyPreparation(context.Context, WorkflowAuthority, ID, time.Time) (ApprovalPolicyPreparation, error)
}
type ProposalReviewAuthorizer interface {
	AuthorizeProposalReview(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeBidRead(context.Context, WorkflowSubject) (WorkflowAuthority, error)
}
type InternalReviewDecisionService struct {
	auth  ProposalReviewAuthorizer
	store InternalReviewDecisionStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewInternalReviewDecisionService(auth ProposalReviewAuthorizer, store InternalReviewDecisionStore, ids CaptureIDGenerator, clock CaptureClock) (*InternalReviewDecisionService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("internal_review_service_invalid")
	}
	return &InternalReviewDecisionService{auth: auth, store: store, ids: ids, clock: clock}, nil
}
func (s *InternalReviewDecisionService) Record(ctx context.Context, in RecordInternalReviewDecisionInput) (InternalReviewDecisionResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.ProposalVersionID.IsZero() || in.CorrelationID.IsZero() || (in.Decision != "REVIEW_COMPLETE" && in.Decision != "CHANGES_REQUESTED") || !validRequirementText(in.Rationale, 5000) || in.IdempotencyKey == "" || in.ExpectedPreviousSequence < 0 || in.ExpectedPreviousSequence >= 100 {
		return InternalReviewDecisionResult{}, safe("internal_review_invalid", nil)
	}
	a, err := s.auth.AuthorizeProposalReview(ctx, in.Subject)
	if err != nil {
		return InternalReviewDecisionResult{}, safe("internal_review_not_authorized", err)
	}
	ids, err := responseDraftIDs(s.ids, 4)
	if err != nil {
		return InternalReviewDecisionResult{}, safe("internal_review_unavailable", err)
	}
	payload, _ := json.Marshal(struct {
		BidID, ProposalVersionID ID
		Decision, Rationale      string
		Previous                 int32
	}{in.BidID, in.ProposalVersionID, in.Decision, in.Rationale, in.ExpectedPreviousSequence})
	commit := InternalReviewDecisionCommit{CommandID: ids[0], DecisionID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, ProposalVersionID: in.ProposalVersionID, CorrelationID: in.CorrelationID, Decision: in.Decision, Rationale: in.Rationale, ExpectedPreviousSequence: in.ExpectedPreviousSequence, KeyHash: sha256.Sum256([]byte("internal-review-decision:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(payload), OccurredAt: s.clock.Now().UTC()}
	result, err := s.store.CommitInternalReviewDecision(ctx, a, commit)
	if errors.Is(err, ErrInternalReviewConflict) {
		return InternalReviewDecisionResult{}, safe("internal_review_conflict", err)
	}
	if err != nil {
		return InternalReviewDecisionResult{}, safe("internal_review_unavailable", err)
	}
	return result, nil
}
func (s *InternalReviewDecisionService) History(ctx context.Context, subject WorkflowSubject, bidID, proposalVersionID ID) ([]InternalReviewDecision, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || proposalVersionID.IsZero() {
		return nil, safe("internal_review_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return nil, safe("internal_review_not_authorized", err)
	}
	rows, err := s.store.ListInternalReviewDecisions(ctx, a, bidID, proposalVersionID)
	if errors.Is(err, ErrInternalReviewReadLimit) {
		return nil, safe("internal_review_read_limit", err)
	}
	if errors.Is(err, ErrInternalReviewNotFound) {
		return nil, safe("internal_review_not_found", err)
	}
	if err != nil {
		return nil, safe("internal_review_read_unavailable", err)
	}
	return rows, nil
}
func (s *InternalReviewDecisionService) Preparation(ctx context.Context, subject WorkflowSubject, bidID ID) (ApprovalPolicyPreparation, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return ApprovalPolicyPreparation{}, safe("approval_policy_preparation_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return ApprovalPolicyPreparation{}, safe("internal_review_not_authorized", err)
	}
	result, err := s.store.GetApprovalPolicyPreparation(ctx, a, bidID, s.clock.Now().UTC())
	if errors.Is(err, ErrInternalReviewNotFound) {
		return ApprovalPolicyPreparation{}, safe("approval_policy_preparation_not_found", err)
	}
	if err != nil {
		return ApprovalPolicyPreparation{}, safe("approval_policy_preparation_unavailable", err)
	}
	return result, nil
}
