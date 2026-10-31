package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const AIProposalApplyPath = "/v1/tenderpreneur/bids/:bid_id/proposal/ai-writing-suggestions/:suggestion_id/apply"
const ProposalApprovalReadinessPath = "/v1/tenderpreneur/bids/:bid_id/proposal/approval-readiness"

type ProposalApplicationUseCase interface {
	Apply(context.Context, opportunityapp.ApplyProposalSuggestionInput) (opportunityapp.ProposalSaveResult, error)
	Readiness(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.ProposalApprovalReadiness, error)
}
type ProposalApplicationHandler struct{ service ProposalApplicationUseCase }

func NewProposalApplicationHandler(v ProposalApplicationUseCase) (*ProposalApplicationHandler, error) {
	if v == nil {
		return nil, errors.New("proposal_application_handler_invalid")
	}
	return &ProposalApplicationHandler{v}, nil
}

type applyProposalSuggestionBody struct {
	Introduction              string `json:"introduction"`
	Closing                   string `json:"closing"`
	ReviewNote                string `json:"review_note"`
	ExpectedSuggestionVersion int64  `json:"expected_suggestion_version"`
	ExpectedProposalVersion   int64  `json:"expected_proposal_version"`
	ExpectedWorkspaceVersion  int64  `json:"expected_workspace_version"`
}

func (h *ProposalApplicationHandler) Apply(c fiber.Ctx) error {
	var body applyProposalSuggestionBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	suggestionID, ok := parseOpportunityID(c.Params("suggestion_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.service.Apply(c.Context(), opportunityapp.ApplyProposalSuggestionInput{Subject: common.subject, BidID: common.opportunityID, SuggestionID: suggestionID, CorrelationID: common.correlationID, Introduction: body.Introduction, Closing: body.Closing, ReviewNote: body.ReviewNote, ExpectedSuggestionVersion: body.ExpectedSuggestionVersion, ExpectedProposalVersion: body.ExpectedProposalVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return proposalApplicationFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"proposal": proposalView(result.Proposal), "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ProposalApplicationHandler) Readiness(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	result, err := h.service.Readiness(c.Context(), subject, bidID)
	if err != nil {
		return proposalApplicationFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"bid_id": uuid.UUID(result.BidID).String(), "proposal_id": optionalUUID(result.ProposalID), "proposal_version_id": optionalUUID(result.ProposalVersionID), "workspace_state": result.WorkspaceState, "workspace_version": result.WorkspaceVersion, "proposal_version": result.ProposalVersion, "ready_for_approval_review": result.Ready, "blockers": result.Blockers, "requirement_count": result.RequirementCount, "open_review_thread_count": result.OpenReviewThreadCount, "unassigned_review_thread_count": result.UnassignedReviewThreadCount, "overdue_review_thread_count": result.OverdueReviewThreadCount, "mandatory_unsatisfied": result.MandatoryUnsatisfied, "open_unassigned": result.OpenUnassigned, "open_overdue": result.OpenOverdue, "mandatory_response_missing": result.MandatoryResponseMissing, "expired_evidence_count": result.ExpiredEvidenceCount, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func optionalUUID(id opportunityapp.ID) any {
	if id.IsZero() {
		return nil
	}
	return uuid.UUID(id).String()
}
func proposalApplicationFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "proposal_suggestion_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to review this proposal.", false)
	case "proposal_suggestion_apply_invalid", "proposal_approval_readiness_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The proposal review request is invalid.", false)
	case "proposal_suggestion_apply_conflict":
		return fail(c, 409, "PROPOSAL_REVIEW_CONFLICT", "The suggestion, proposal, or workspace changed.", false)
	default:
		return fail(c, 503, "PROPOSAL_REVIEW_UNAVAILABLE", "Proposal review is temporarily unavailable.", true)
	}
}
