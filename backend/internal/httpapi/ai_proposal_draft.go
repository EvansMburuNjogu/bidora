package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const AIProposalDraftPath = "/v1/tenderpreneur/bids/:bid_id/proposal/versions/:proposal_version_id/ai-writing-suggestions"

type AIProposalDraftUseCase interface {
	Request(context.Context, opportunityapp.RequestProposalDraftInput) (opportunityapp.ProposalDraftRequestResult, error)
	List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ProposalDraftSuggestion, error)
}
type AIProposalDraftHandler struct{ drafts AIProposalDraftUseCase }

func NewAIProposalDraftHandler(v AIProposalDraftUseCase) (*AIProposalDraftHandler, error) {
	if v == nil {
		return nil, errors.New("ai_proposal_draft_handler_invalid")
	}
	return &AIProposalDraftHandler{v}, nil
}

type requestAIProposalDraftBody struct {
	ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
	ExpectedProposalVersion  int64 `json:"expected_proposal_version"`
}

func (h *AIProposalDraftHandler) Request(c fiber.Ctx) error {
	var body requestAIProposalDraftBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	versionID, ok := parseOpportunityID(c.Params("proposal_version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.drafts.Request(c.Context(), opportunityapp.RequestProposalDraftInput{Subject: common.subject, BidID: common.opportunityID, ProposalVersionID: versionID, CorrelationID: common.correlationID, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, ExpectedProposalVersion: body.ExpectedProposalVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return aiProposalDraftFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"data": fiber.Map{"run_id": uuid.UUID(result.RunID).String(), "job_id": uuid.UUID(result.JobID).String(), "state": result.State, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *AIProposalDraftHandler) List(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	versionID, ok := parseOpportunityID(c.Params("proposal_version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	rows, err := h.drafts.List(c.Context(), subject, bidID, versionID)
	if err != nil {
		return aiProposalDraftFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		citations := make([]fiber.Map, len(row.Citations))
		for j, v := range row.Citations {
			citations[j] = fiber.Map{"ordinal": v.Ordinal, "document_version_id": uuid.UUID(v.DocumentVersionID).String(), "location": v.Location, "quote": v.Quote}
		}
		item := fiber.Map{"run_id": uuid.UUID(row.RunID).String(), "proposal_version_id": uuid.UUID(row.ProposalVersionID).String(), "run_state": row.RunState, "requested_workspace_version": row.RequestedWorkspaceVersion, "requested_proposal_version": row.RequestedProposalVersion, "created_at": row.CreatedAt.Format(time.RFC3339Nano)}
		if !row.ID.IsZero() {
			item["suggestion_id"] = uuid.UUID(row.ID).String()
			item["suggestion_version"] = row.SuggestionVersion
			item["introduction"] = row.Introduction
			item["closing"] = row.Closing
			item["rationale"] = row.Rationale
			item["review_state"] = row.ReviewState
			item["citations"] = citations
			if !row.AppliedProposalVersionID.IsZero() {
				item["applied_proposal_version_id"] = uuid.UUID(row.AppliedProposalVersionID).String()
				item["reviewed_by"] = uuid.UUID(row.ReviewedBy).String()
				item["reviewed_at"] = row.ReviewedAt.Format(time.RFC3339Nano)
				item["review_note"] = row.ReviewNote
			}
		}
		items[i] = item
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"suggestions": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func aiProposalDraftFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "proposal_draft_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to access AI proposal suggestions.", false)
	case "proposal_draft_invalid", "proposal_draft_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The AI proposal request is invalid.", false)
	case "proposal_draft_conflict":
		return fail(c, 409, "AI_PROPOSAL_CONFLICT", "The proposal or workspace changed or has no clean tender source.", false)
	case "proposal_draft_unavailable", "proposal_draft_read_unavailable":
		return fail(c, 503, "AI_PROPOSAL_UNAVAILABLE", "AI proposal writing is temporarily unavailable.", true)
	default:
		return fail(c, 503, "AI_PROPOSAL_UNAVAILABLE", "AI proposal writing is temporarily unavailable.", true)
	}
}
