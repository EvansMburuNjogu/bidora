package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const ReviewThreadListPath = "/v1/tenderpreneur/bids/:bid_id/review-threads"
const ReviewThreadCommentPath = "/v1/tenderpreneur/bids/:bid_id/review-threads/:thread_id/comments"
const ReviewThreadResolvePath = "/v1/tenderpreneur/bids/:bid_id/review-threads/:thread_id/resolve"
const ReviewThreadAssignmentPath = "/v1/tenderpreneur/bids/:bid_id/review-threads/:thread_id/assignment"
const ReviewThreadAssignmentHistoryPath = "/v1/tenderpreneur/bids/:bid_id/review-threads/:thread_id/assignment/history"
const ReviewDashboardPath = "/v1/tenderpreneur/bids/:bid_id/review-dashboard"
const ReviewWorkloadsPath = "/v1/tenderpreneur/review-workloads"
const ReviewerOpenWorkPath = "/v1/tenderpreneur/review-workloads/:reviewer_id"

type ReviewThreadUseCase interface {
	Create(context.Context, opportunityapp.CreateReviewThreadInput) (opportunityapp.ReviewThreadResult, error)
	Comment(context.Context, opportunityapp.AddReviewCommentInput) (opportunityapp.ReviewThreadResult, error)
	Resolve(context.Context, opportunityapp.ResolveReviewThreadInput) (opportunityapp.ReviewThreadResult, error)
	List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.ReviewThread, error)
	Assign(context.Context, opportunityapp.AssignReviewThreadInput) (opportunityapp.ReviewThreadResult, error)
	AssignmentHistory(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ReviewAssignment, error)
	Dashboard(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ReviewDashboard, error)
	Workloads(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.TenantReviewWorkloads, error)
	ReviewerWork(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.ReviewerOpenWork, error)
}
type ReviewThreadHandler struct{ service ReviewThreadUseCase }

func NewReviewThreadHandler(v ReviewThreadUseCase) (*ReviewThreadHandler, error) {
	if v == nil {
		return nil, errors.New("review_thread_handler_invalid")
	}
	return &ReviewThreadHandler{v}, nil
}

type createReviewThreadBody struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Subject    string `json:"subject"`
	Comment    string `json:"comment"`
}
type addReviewCommentBody struct {
	Comment               string `json:"comment"`
	ExpectedThreadVersion int64  `json:"expected_thread_version"`
}
type assignReviewThreadBody struct {
	AssigneeID            string  `json:"assignee_id"`
	DueAt                 *string `json:"due_at"`
	Reason                string  `json:"reason"`
	ExpectedThreadVersion int64   `json:"expected_thread_version"`
}
type resolveReviewThreadBody struct {
	ResolutionNote        string `json:"resolution_note"`
	ExpectedThreadVersion int64  `json:"expected_thread_version"`
}

func (h *ReviewThreadHandler) Create(c fiber.Ctx) error {
	var body createReviewThreadBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	targetID, ok := parseOpportunityID(body.TargetID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The review request is invalid.", false)
	}
	result, err := h.service.Create(c.Context(), opportunityapp.CreateReviewThreadInput{Subject: common.subject, BidID: common.opportunityID, TargetID: targetID, CorrelationID: common.correlationID, TargetType: body.TargetType, ThreadSubject: body.Subject, Comment: body.Comment, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	return reviewThreadResponse(c, result)
}
func (h *ReviewThreadHandler) Comment(c fiber.Ctx) error {
	var body addReviewCommentBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	threadID, ok := parseOpportunityID(c.Params("thread_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The review request is invalid.", false)
	}
	result, err := h.service.Comment(c.Context(), opportunityapp.AddReviewCommentInput{Subject: common.subject, BidID: common.opportunityID, ThreadID: threadID, CorrelationID: common.correlationID, Comment: body.Comment, ExpectedThreadVersion: body.ExpectedThreadVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	return reviewThreadResponse(c, result)
}
func (h *ReviewThreadHandler) Resolve(c fiber.Ctx) error {
	var body resolveReviewThreadBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	threadID, ok := parseOpportunityID(c.Params("thread_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The review request is invalid.", false)
	}
	result, err := h.service.Resolve(c.Context(), opportunityapp.ResolveReviewThreadInput{Subject: common.subject, BidID: common.opportunityID, ThreadID: threadID, CorrelationID: common.correlationID, ResolutionNote: body.ResolutionNote, ExpectedThreadVersion: body.ExpectedThreadVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	return reviewThreadResponse(c, result)
}
func (h *ReviewThreadHandler) Assign(c fiber.Ctx) error {
	var body assignReviewThreadBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	threadID, ok := parseOpportunityID(c.Params("thread_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The review assignment is invalid.", false)
	}
	assigneeID, ok := parseOpportunityID(body.AssigneeID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The review assignment is invalid.", false)
	}
	var dueAt *time.Time
	if body.DueAt != nil {
		parsed, err := time.Parse(time.RFC3339, *body.DueAt)
		if err != nil {
			return fail(c, 400, "INVALID_REQUEST", "The review assignment is invalid.", false)
		}
		normalized := parsed.UTC()
		dueAt = &normalized
	}
	result, err := h.service.Assign(c.Context(), opportunityapp.AssignReviewThreadInput{Subject: common.subject, BidID: common.opportunityID, ThreadID: threadID, AssigneeID: assigneeID, CorrelationID: common.correlationID, DueAt: dueAt, Reason: body.Reason, IdempotencyKey: common.idempotencyKey, ExpectedThreadVersion: body.ExpectedThreadVersion})
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	return reviewThreadResponse(c, result)
}
func (h *ReviewThreadHandler) AssignmentHistory(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	threadID, ok := parseOpportunityID(c.Params("thread_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The review assignment is invalid.", false)
	}
	rows, err := h.service.AssignmentHistory(c.Context(), subject, bidID, threadID)
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = fiber.Map{"id": uuid.UUID(row.ID).String(), "thread_version": row.ThreadVersion, "assigned_to": uuid.UUID(row.AssignedTo).String(), "due_at": formatOptionalTime(row.DueAt), "reason": row.Reason, "assigned_by": uuid.UUID(row.AssignedBy).String(), "assigned_at": row.AssignedAt.Format(time.RFC3339Nano)}
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"assignments": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ReviewThreadHandler) Dashboard(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	result, err := h.service.Dashboard(c.Context(), subject, bidID)
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"bid_id": uuid.UUID(result.BidID).String(), "total_count": result.TotalCount, "open_count": result.OpenCount, "resolved_count": result.ResolvedCount, "unassigned_count": result.UnassignedCount, "overdue_count": result.OverdueCount, "reviewers": reviewerWorkloadViews(result.Reviewers), "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ReviewThreadHandler) Workloads(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) > 0 {
		return fail(c, 400, "INVALID_REQUEST", "Review workload query parameters are not supported.", false)
	}
	result, err := h.service.Workloads(c.Context(), subject)
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"open_count": result.OpenCount, "unassigned_count": result.UnassignedCount, "overdue_count": result.OverdueCount, "bid_count": result.BidCount, "reviewers": reviewerWorkloadViews(result.Reviewers), "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ReviewThreadHandler) ReviewerWork(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) > 0 {
		return fail(c, 400, "INVALID_REQUEST", "Review workload query parameters are not supported.", false)
	}
	reviewerID, ok := parseOpportunityID(c.Params("reviewer_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The reviewer workload request is invalid.", false)
	}
	rows, err := h.service.ReviewerWork(c.Context(), subject, reviewerID)
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = fiber.Map{"thread_id": uuid.UUID(row.ThreadID).String(), "bid_id": uuid.UUID(row.BidID).String(), "target_type": row.TargetType, "target_id": uuid.UUID(row.TargetID).String(), "subject": row.Subject, "thread_version": row.ThreadVersion, "due_at": formatOptionalTime(row.DueAt), "created_at": row.CreatedAt.Format(time.RFC3339Nano)}
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"reviewer_id": uuid.UUID(reviewerID).String(), "open_work": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func reviewerWorkloadViews(rows []opportunityapp.ReviewerWorkload) []fiber.Map {
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = fiber.Map{"reviewer_id": uuid.UUID(row.ReviewerID).String(), "open_count": row.OpenCount, "overdue_count": row.OverdueCount, "bid_count": row.BidCount, "earliest_due_at": formatOptionalTime(row.EarliestDueAt)}
	}
	return items
}
func (h *ReviewThreadHandler) List(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	rows, err := h.service.List(c.Context(), subject, bidID)
	if err != nil {
		return reviewThreadFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = reviewThreadView(row)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"threads": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func reviewThreadResponse(c fiber.Ctx, v opportunityapp.ReviewThreadResult) error {
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"thread": reviewThreadView(v.Thread), "replay": v.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func reviewThreadView(v opportunityapp.ReviewThread) fiber.Map {
	comments := make([]fiber.Map, len(v.Comments))
	for i, row := range v.Comments {
		comments[i] = fiber.Map{"id": uuid.UUID(row.ID).String(), "thread_version": row.ThreadVersion, "kind": row.Kind, "body": row.Body, "created_by": uuid.UUID(row.CreatedBy).String(), "created_at": row.CreatedAt.Format(time.RFC3339Nano)}
	}
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "bid_id": uuid.UUID(v.BidID).String(), "target_type": v.TargetType, "target_id": uuid.UUID(v.TargetID).String(), "subject": v.Subject, "status": v.Status, "version": v.Version, "created_by": uuid.UUID(v.CreatedBy).String(), "created_at": v.CreatedAt.Format(time.RFC3339Nano), "resolved_by": optionalUUID(v.ResolvedBy), "resolved_at": formatOptionalTime(v.ResolvedAt), "assigned_to": optionalUUID(v.AssignedTo), "due_at": formatOptionalTime(v.DueAt), "comments": comments}
}
func reviewThreadFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "review_thread_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to access bid reviews.", false)
	case "review_thread_invalid", "review_thread_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The review request is invalid.", false)
	case "review_dashboard_invalid", "review_workload_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The review workload request is invalid.", false)
	case "review_dashboard_not_found":
		return fail(c, 404, "REVIEW_DASHBOARD_NOT_FOUND", "The bid review dashboard was not found.", false)
	case "review_dashboard_not_authorized", "review_workload_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to read this review workload.", false)
	case "review_dashboard_limit", "review_workload_limit":
		return fail(c, 409, "REVIEW_WORKLOAD_LIMIT", "The review workload exceeds the bounded read limit.", false)
	case "review_dashboard_unavailable", "review_workload_unavailable":
		return fail(c, 503, "REVIEW_WORKLOAD_UNAVAILABLE", "Review workload is temporarily unavailable.", true)
	case "review_assignment_invalid", "review_assignment_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The review assignment is invalid.", false)
	case "review_assignment_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to assign this reviewer.", false)
	case "review_assignment_conflict":
		return fail(c, 409, "REVIEW_ASSIGNMENT_CONFLICT", "The review thread changed or is closed.", false)
	case "review_assignment_unavailable", "review_assignment_read_unavailable":
		return fail(c, 503, "REVIEW_ASSIGNMENT_UNAVAILABLE", "Review assignment is temporarily unavailable.", true)
	case "review_thread_read_limit":
		return fail(c, 413, "REVIEW_READ_LIMIT", "The review discussion is too large for this bounded read.", false)
	case "review_thread_conflict":
		return fail(c, 409, "REVIEW_THREAD_CONFLICT", "The target or review thread changed.", false)
	default:
		return fail(c, 503, "REVIEW_THREAD_UNAVAILABLE", "Bid reviews are temporarily unavailable.", true)
	}
}
