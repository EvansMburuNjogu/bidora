package httpapi

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const InternalReviewDecisionPath = "/v1/tenderpreneur/bids/:bid_id/proposal/versions/:proposal_version_id/internal-review-decisions"
const ApprovalPolicyPreparationPath = "/v1/tenderpreneur/bids/:bid_id/proposal/approval-policy-preparation"

type InternalReviewDecisionUseCase interface {
	Record(context.Context, application.RecordInternalReviewDecisionInput) (application.InternalReviewDecisionResult, error)
	History(context.Context, application.WorkflowSubject, application.ID, application.ID) ([]application.InternalReviewDecision, error)
	Preparation(context.Context, application.WorkflowSubject, application.ID) (application.ApprovalPolicyPreparation, error)
}
type InternalReviewDecisionHandler struct{ service InternalReviewDecisionUseCase }

func NewInternalReviewDecisionHandler(service InternalReviewDecisionUseCase) (*InternalReviewDecisionHandler, error) {
	if service == nil {
		return nil, errors.New("internal_review_handler_invalid")
	}
	return &InternalReviewDecisionHandler{service}, nil
}

type internalReviewDecisionBody struct {
	Decision                 string `json:"decision"`
	Rationale                string `json:"rationale"`
	ExpectedPreviousSequence int32  `json:"expected_previous_sequence"`
}

func (h *InternalReviewDecisionHandler) Record(c fiber.Ctx) error {
	var body internalReviewDecisionBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"decision", "rationale", "expected_previous_sequence"}) {
		return fail(c, 400, "INVALID_REQUEST", "The internal review decision is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	versionID, ok := parseOpportunityID(c.Params("proposal_version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The internal review decision is invalid.", false)
	}
	result, err := h.service.Record(c.Context(), application.RecordInternalReviewDecisionInput{Subject: common.subject, BidID: common.opportunityID, ProposalVersionID: versionID, CorrelationID: common.correlationID, Decision: body.Decision, Rationale: body.Rationale, IdempotencyKey: common.idempotencyKey, ExpectedPreviousSequence: body.ExpectedPreviousSequence})
	if err != nil {
		return internalReviewFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": internalReviewDecisionView(result.Decision), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *InternalReviewDecisionHandler) History(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	versionID, ok := parseOpportunityID(c.Params("proposal_version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The internal review request is invalid.", false)
	}
	rows, err := h.service.History(c.Context(), subject, bidID, versionID)
	if err != nil {
		return internalReviewFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = internalReviewDecisionView(row)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"decisions": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *InternalReviewDecisionHandler) Preparation(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	result, err := h.service.Preparation(c.Context(), subject, bidID)
	if err != nil {
		return internalReviewFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"bid_id": uuid.UUID(result.BidID).String(), "proposal_version_id": uuid.UUID(result.ProposalVersionID).String(), "reviewer_count": result.ReviewerCount, "review_complete_count": result.ReviewCompleteCount, "changes_requested_count": result.ChangesRequestedCount, "open_review_thread_count": result.OpenReviewThreadCount, "approval_policy_configured": result.ApprovalPolicyConfigured, "eligible_for_approval": result.EligibleForApproval, "blockers": result.Blockers, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func internalReviewDecisionView(row application.InternalReviewDecision) fiber.Map {
	return fiber.Map{"id": uuid.UUID(row.ID).String(), "bid_id": uuid.UUID(row.BidID).String(), "proposal_version_id": uuid.UUID(row.ProposalVersionID).String(), "reviewer_id": uuid.UUID(row.ReviewerID).String(), "reviewer_sequence": row.ReviewerSequence, "decision": row.Decision, "rationale": row.Rationale, "decided_at": row.DecidedAt.Format(time.RFC3339Nano)}
}
func internalReviewFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "internal_review_invalid", "internal_review_read_invalid", "approval_policy_preparation_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The internal review request is invalid.", false)
	case "internal_review_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized for this internal review operation.", false)
	case "internal_review_not_found", "approval_policy_preparation_not_found":
		return fail(c, 404, "INTERNAL_REVIEW_NOT_FOUND", "The proposal review record was not found.", false)
	case "internal_review_conflict":
		return fail(c, 409, "INTERNAL_REVIEW_CONFLICT", "The proposal revision or reviewer sequence changed.", false)
	case "internal_review_read_limit":
		return fail(c, 409, "INTERNAL_REVIEW_LIMIT", "The internal review history exceeds the bounded read limit.", false)
	case "internal_review_unavailable", "internal_review_read_unavailable", "approval_policy_preparation_unavailable":
		return fail(c, 503, "INTERNAL_REVIEW_UNAVAILABLE", "Internal review is temporarily unavailable.", true)
	default:
		return fail(c, 503, "INTERNAL_REVIEW_UNAVAILABLE", "Internal review is temporarily unavailable.", true)
	}
}
