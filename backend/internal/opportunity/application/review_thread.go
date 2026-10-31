package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrReviewThreadConflict = errors.New("review_thread_conflict")
var ErrReviewThreadReadLimit = errors.New("review_thread_read_limit")
var ErrReviewWorkloadReadLimit = errors.New("review_workload_read_limit")
var ErrReviewDashboardNotFound = errors.New("review_dashboard_not_found")

type ReviewComment struct {
	ID, ThreadID, CreatedBy ID
	ThreadVersion           int64
	Kind, Body              string
	CreatedAt               time.Time
}
type ReviewThread struct {
	ID, BidID, TargetID, CreatedBy, ResolvedBy, AssignedTo ID
	TargetType, Subject, Status                            string
	Version                                                int64
	CreatedAt                                              time.Time
	ResolvedAt, DueAt                                      *time.Time
	Comments                                               []ReviewComment
}
type CreateReviewThreadInput struct {
	Subject                                            WorkflowSubject
	BidID, TargetID, CorrelationID                     ID
	TargetType, ThreadSubject, Comment, IdempotencyKey string
}
type AddReviewCommentInput struct {
	Subject                        WorkflowSubject
	BidID, ThreadID, CorrelationID ID
	Comment, IdempotencyKey        string
	ExpectedThreadVersion          int64
}
type ResolveReviewThreadInput struct {
	Subject                        WorkflowSubject
	BidID, ThreadID, CorrelationID ID
	ResolutionNote, IdempotencyKey string
	ExpectedThreadVersion          int64
}
type ReviewThreadCommit struct {
	Action                                                                            string
	CommandID, ThreadID, CommentID, AuditID, OutboxID, BidID, TargetID, CorrelationID ID
	TargetType, ThreadSubject, Body                                                   string
	ExpectedThreadVersion                                                             int64
	KeyHash, RequestHash                                                              [32]byte
	OccurredAt                                                                        time.Time
}
type ReviewThreadResult struct {
	Thread ReviewThread
	Replay bool
}
type ReviewAssignment struct {
	ID, ThreadID, AssignedTo, AssignedBy ID
	ThreadVersion                        int64
	DueAt                                *time.Time
	Reason                               string
	AssignedAt                           time.Time
}
type ReviewAssignmentAuthorizer interface {
	TechnicalResponseAuthorizer
	AuthorizeProposalReview(context.Context, WorkflowSubject) (WorkflowAuthority, error)
	AuthorizeReviewAssignment(context.Context, WorkflowSubject, ID) (WorkflowAuthority, error)
}
type AssignReviewThreadInput struct {
	Subject                                    WorkflowSubject
	BidID, ThreadID, AssigneeID, CorrelationID ID
	DueAt                                      *time.Time
	Reason, IdempotencyKey                     string
	ExpectedThreadVersion                      int64
}
type ReviewAssignmentCommit struct {
	CommandID, AssignmentID, AuditID, OutboxID, BidID, ThreadID, AssigneeID, CorrelationID ID
	DueAt                                                                                  *time.Time
	Reason                                                                                 string
	ExpectedThreadVersion                                                                  int64
	KeyHash, RequestHash                                                                   [32]byte
	OccurredAt                                                                             time.Time
}
type ReviewerWorkload struct {
	ReviewerID                        ID
	OpenCount, OverdueCount, BidCount int64
	EarliestDueAt                     *time.Time
}
type ReviewDashboard struct {
	BidID                                                               ID
	TotalCount, OpenCount, ResolvedCount, UnassignedCount, OverdueCount int64
	ObservedAt                                                          time.Time
	Reviewers                                                           []ReviewerWorkload
}
type TenantReviewWorkloads struct {
	OpenCount, UnassignedCount, OverdueCount, BidCount int64
	ObservedAt                                         time.Time
	Reviewers                                          []ReviewerWorkload
}
type ReviewerOpenWork struct {
	ThreadID, BidID, TargetID ID
	TargetType, Subject       string
	ThreadVersion             int64
	DueAt                     *time.Time
	CreatedAt                 time.Time
}
type ReviewThreadStore interface {
	CommitReviewThread(context.Context, WorkflowAuthority, ReviewThreadCommit) (ReviewThreadResult, error)
	ListReviewThreads(context.Context, WorkflowAuthority, ID) ([]ReviewThread, error)
	AssignReviewThread(context.Context, WorkflowAuthority, ReviewAssignmentCommit) (ReviewThreadResult, error)
	ListReviewAssignments(context.Context, WorkflowAuthority, ID, ID) ([]ReviewAssignment, error)
	GetReviewDashboard(context.Context, WorkflowAuthority, ID, time.Time) (ReviewDashboard, error)
	GetTenantReviewWorkloads(context.Context, WorkflowAuthority, time.Time) (TenantReviewWorkloads, error)
	ListReviewerOpenWork(context.Context, WorkflowAuthority, ID) ([]ReviewerOpenWork, error)
}
type ReviewThreadService struct {
	auth  ReviewAssignmentAuthorizer
	store ReviewThreadStore
	ids   CaptureIDGenerator
	clock CaptureClock
}

func NewReviewThreadService(auth ReviewAssignmentAuthorizer, store ReviewThreadStore, ids CaptureIDGenerator, clock CaptureClock) (*ReviewThreadService, error) {
	if auth == nil || store == nil || ids == nil || clock == nil {
		return nil, errors.New("review_thread_service_invalid")
	}
	return &ReviewThreadService{auth, store, ids, clock}, nil
}
func (s *ReviewThreadService) Create(ctx context.Context, in CreateReviewThreadInput) (ReviewThreadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.TargetID.IsZero() || in.CorrelationID.IsZero() || !validReviewTarget(in.TargetType) || !validRequirementText(in.ThreadSubject, 300) || !validRequirementText(in.Comment, 10000) || in.IdempotencyKey == "" {
		return ReviewThreadResult{}, safe("review_thread_invalid", nil)
	}
	return s.commit(ctx, in.Subject, ReviewThreadCommit{Action: "CREATE", BidID: in.BidID, TargetID: in.TargetID, TargetType: in.TargetType, ThreadSubject: in.ThreadSubject, Body: in.Comment, CorrelationID: in.CorrelationID}, in.IdempotencyKey)
}
func (s *ReviewThreadService) Comment(ctx context.Context, in AddReviewCommentInput) (ReviewThreadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.ThreadID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedThreadVersion < 1 || in.ExpectedThreadVersion >= 199 || !validRequirementText(in.Comment, 10000) || in.IdempotencyKey == "" {
		return ReviewThreadResult{}, safe("review_thread_invalid", nil)
	}
	return s.commit(ctx, in.Subject, ReviewThreadCommit{Action: "COMMENT", BidID: in.BidID, ThreadID: in.ThreadID, Body: in.Comment, ExpectedThreadVersion: in.ExpectedThreadVersion, CorrelationID: in.CorrelationID}, in.IdempotencyKey)
}
func (s *ReviewThreadService) Resolve(ctx context.Context, in ResolveReviewThreadInput) (ReviewThreadResult, error) {
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.ThreadID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedThreadVersion < 1 || in.ExpectedThreadVersion >= 200 || !validRequirementText(in.ResolutionNote, 10000) || in.IdempotencyKey == "" {
		return ReviewThreadResult{}, safe("review_thread_invalid", nil)
	}
	return s.commit(ctx, in.Subject, ReviewThreadCommit{Action: "RESOLVE", BidID: in.BidID, ThreadID: in.ThreadID, Body: in.ResolutionNote, ExpectedThreadVersion: in.ExpectedThreadVersion, CorrelationID: in.CorrelationID}, in.IdempotencyKey)
}
func (s *ReviewThreadService) commit(ctx context.Context, subject WorkflowSubject, c ReviewThreadCommit, key string) (ReviewThreadResult, error) {
	a, err := s.auth.AuthorizeProposalReview(ctx, subject)
	if err != nil {
		return ReviewThreadResult{}, safe("review_thread_not_authorized", err)
	}
	n := 4
	if c.Action == "CREATE" {
		n = 5
	}
	ids, err := responseDraftIDs(s.ids, n)
	if err != nil {
		return ReviewThreadResult{}, safe("review_thread_unavailable", err)
	}
	c.CommandID = ids[0]
	offset := 1
	if c.Action == "CREATE" {
		c.ThreadID = ids[1]
		offset = 2
	}
	c.CommentID = ids[offset]
	c.AuditID = ids[offset+1]
	c.OutboxID = ids[offset+2]
	c.OccurredAt = s.clock.Now().UTC()
	payload, _ := json.Marshal(struct {
		Action                    string
		BidID, ThreadID, TargetID ID
		TargetType, Subject, Body string
		Version                   int64
	}{c.Action, c.BidID, c.ThreadID, c.TargetID, c.TargetType, c.ThreadSubject, c.Body, c.ExpectedThreadVersion})
	c.KeyHash = sha256.Sum256([]byte("review-thread:" + c.Action + ":v1:" + key))
	c.RequestHash = sha256.Sum256(payload)
	result, err := s.store.CommitReviewThread(ctx, a, c)
	if errors.Is(err, ErrReviewThreadConflict) {
		return ReviewThreadResult{}, safe("review_thread_conflict", err)
	}
	if err != nil {
		return ReviewThreadResult{}, safe("review_thread_unavailable", err)
	}
	return result, nil
}
func (s *ReviewThreadService) List(ctx context.Context, subject WorkflowSubject, bidID ID) ([]ReviewThread, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return nil, safe("review_thread_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return nil, safe("review_thread_not_authorized", err)
	}
	rows, err := s.store.ListReviewThreads(ctx, a, bidID)
	if errors.Is(err, ErrReviewThreadReadLimit) {
		return nil, safe("review_thread_read_limit", err)
	}
	if err != nil {
		return nil, safe("review_thread_read_unavailable", err)
	}
	return rows, nil
}
func (s *ReviewThreadService) Assign(ctx context.Context, in AssignReviewThreadInput) (ReviewThreadResult, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validWorkflowSubject(in.Subject) || in.BidID.IsZero() || in.ThreadID.IsZero() || in.AssigneeID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedThreadVersion < 1 || in.ExpectedThreadVersion >= 200 || !validRequirementText(in.Reason, 2000) || in.IdempotencyKey == "" || (in.DueAt != nil && !in.DueAt.After(now)) {
		return ReviewThreadResult{}, safe("review_assignment_invalid", nil)
	}
	if in.DueAt != nil {
		normalized := in.DueAt.UTC()
		in.DueAt = &normalized
	}
	a, err := s.auth.AuthorizeReviewAssignment(ctx, in.Subject, in.AssigneeID)
	if err != nil {
		return ReviewThreadResult{}, safe("review_assignment_not_authorized", err)
	}
	ids, err := responseDraftIDs(s.ids, 4)
	if err != nil {
		return ReviewThreadResult{}, safe("review_assignment_unavailable", err)
	}
	payload, _ := json.Marshal(struct {
		BidID, ThreadID, AssigneeID ID
		DueAt                       *time.Time
		Reason                      string
		Version                     int64
	}{in.BidID, in.ThreadID, in.AssigneeID, in.DueAt, in.Reason, in.ExpectedThreadVersion})
	commit := ReviewAssignmentCommit{CommandID: ids[0], AssignmentID: ids[1], AuditID: ids[2], OutboxID: ids[3], BidID: in.BidID, ThreadID: in.ThreadID, AssigneeID: in.AssigneeID, CorrelationID: in.CorrelationID, DueAt: in.DueAt, Reason: in.Reason, ExpectedThreadVersion: in.ExpectedThreadVersion, KeyHash: sha256.Sum256([]byte("assign-review:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(payload), OccurredAt: now}
	result, err := s.store.AssignReviewThread(ctx, a, commit)
	if errors.Is(err, ErrReviewThreadConflict) {
		return ReviewThreadResult{}, safe("review_assignment_conflict", err)
	}
	if err != nil {
		return ReviewThreadResult{}, safe("review_assignment_unavailable", err)
	}
	return result, nil
}
func (s *ReviewThreadService) AssignmentHistory(ctx context.Context, subject WorkflowSubject, bidID, threadID ID) ([]ReviewAssignment, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() || threadID.IsZero() {
		return nil, safe("review_assignment_read_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return nil, safe("review_assignment_not_authorized", err)
	}
	rows, err := s.store.ListReviewAssignments(ctx, a, bidID, threadID)
	if err != nil {
		return nil, safe("review_assignment_read_unavailable", err)
	}
	return rows, nil
}
func (s *ReviewThreadService) Dashboard(ctx context.Context, subject WorkflowSubject, bidID ID) (ReviewDashboard, error) {
	if ctx == nil || !validWorkflowSubject(subject) || bidID.IsZero() {
		return ReviewDashboard{}, safe("review_dashboard_invalid", nil)
	}
	a, err := s.auth.AuthorizeBidRead(ctx, subject)
	if err != nil {
		return ReviewDashboard{}, safe("review_dashboard_not_authorized", err)
	}
	result, err := s.store.GetReviewDashboard(ctx, a, bidID, s.clock.Now().UTC())
	if errors.Is(err, ErrReviewWorkloadReadLimit) {
		return ReviewDashboard{}, safe("review_dashboard_limit", err)
	}
	if errors.Is(err, ErrReviewDashboardNotFound) {
		return ReviewDashboard{}, safe("review_dashboard_not_found", err)
	}
	if err != nil {
		return ReviewDashboard{}, safe("review_dashboard_unavailable", err)
	}
	return result, nil
}
func (s *ReviewThreadService) Workloads(ctx context.Context, subject WorkflowSubject) (TenantReviewWorkloads, error) {
	if ctx == nil || !validWorkflowSubject(subject) {
		return TenantReviewWorkloads{}, safe("review_workload_invalid", nil)
	}
	a, err := s.auth.AuthorizeProposalReview(ctx, subject)
	if err != nil {
		return TenantReviewWorkloads{}, safe("review_workload_not_authorized", err)
	}
	result, err := s.store.GetTenantReviewWorkloads(ctx, a, s.clock.Now().UTC())
	if errors.Is(err, ErrReviewWorkloadReadLimit) {
		return TenantReviewWorkloads{}, safe("review_workload_limit", err)
	}
	if err != nil {
		return TenantReviewWorkloads{}, safe("review_workload_unavailable", err)
	}
	return result, nil
}
func (s *ReviewThreadService) ReviewerWork(ctx context.Context, subject WorkflowSubject, reviewerID ID) ([]ReviewerOpenWork, error) {
	if ctx == nil || !validWorkflowSubject(subject) || reviewerID.IsZero() {
		return nil, safe("review_workload_invalid", nil)
	}
	a, err := s.auth.AuthorizeReviewAssignment(ctx, subject, reviewerID)
	if err != nil {
		return nil, safe("review_workload_not_authorized", err)
	}
	rows, err := s.store.ListReviewerOpenWork(ctx, a, reviewerID)
	if errors.Is(err, ErrReviewWorkloadReadLimit) {
		return nil, safe("review_workload_limit", err)
	}
	if err != nil {
		return nil, safe("review_workload_unavailable", err)
	}
	return rows, nil
}
func validReviewTarget(v string) bool {
	return v == "BID" || v == "REQUIREMENT" || v == "PROPOSAL_VERSION"
}
