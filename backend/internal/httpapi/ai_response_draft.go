package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const AIResponseDraftPath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id/ai-response-drafts"

type AIResponseDraftUseCase interface {
	Request(context.Context, opportunityapp.RequestResponseDraftInput) (opportunityapp.ResponseDraftRequestResult, error)
	List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.ResponseDraftSuggestion, error)
}
type AIResponseDraftHandler struct {
	drafts          AIResponseDraftUseCase
	providerEnabled bool
}

func NewAIResponseDraftHandler(d AIResponseDraftUseCase, providerEnabled bool) (*AIResponseDraftHandler, error) {
	if d == nil {
		return nil, errors.New("ai_response_draft_handler_invalid")
	}
	return &AIResponseDraftHandler{drafts: d, providerEnabled: providerEnabled}, nil
}

type requestAIResponseDraftBody struct {
	ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
	ExpectedSectionVersion   int64 `json:"expected_section_version"`
}

func (h *AIResponseDraftHandler) Request(c fiber.Ctx) error {
	if !h.providerEnabled {
		return fail(c, 503, "AI_RESPONSE_DRAFT_ENTITLEMENT_POLICY_REQUIRED", "AI response drafting is temporarily unavailable.", false)
	}
	var body requestAIResponseDraftBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"expected_workspace_version", "expected_section_version"}) {
		return fail(c, 400, "INVALID_REQUEST", "The AI response draft request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	requirementID, ok := parseOpportunityID(c.Params("requirement_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.drafts.Request(c.Context(), opportunityapp.RequestResponseDraftInput{Subject: common.subject, BidID: common.opportunityID, RequirementID: requirementID, CorrelationID: common.correlationID, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, ExpectedSectionVersion: body.ExpectedSectionVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return aiResponseDraftFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"data": fiber.Map{"run_id": uuid.UUID(result.RunID).String(), "job_id": uuid.UUID(result.JobID).String(), "state": result.State, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *AIResponseDraftHandler) List(c fiber.Ctx) error {
	subject, bidID, requirementID, ok := technicalReadCommon(c)
	if !ok {
		return nil
	}
	rows, err := h.drafts.List(c.Context(), subject, bidID, requirementID)
	if err != nil {
		return aiResponseDraftFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		citations := make([]fiber.Map, len(row.Citations))
		for j, v := range row.Citations {
			citations[j] = fiber.Map{"ordinal": v.Ordinal, "document_version_id": uuid.UUID(v.DocumentVersionID).String(), "location": v.Location, "quote": v.Quote}
		}
		gaps := make([]fiber.Map, len(row.Gaps))
		for j, v := range row.Gaps {
			gap := fiber.Map{"ordinal": v.Ordinal, "code": v.Code, "description": v.Description}
			if v.HasCitation {
				gap["document_version_id"] = uuid.UUID(v.DocumentVersionID).String()
				gap["location"] = v.Location
				gap["quote"] = v.Quote
			}
			gaps[j] = gap
		}
		item := fiber.Map{"run_id": uuid.UUID(row.RunID).String(), "run_state": row.RunState, "requested_workspace_version": row.RequestedWorkspaceVersion, "requested_section_version": row.RequestedSectionVersion, "created_at": row.CreatedAt.Format(time.RFC3339Nano)}
		if !row.ID.IsZero() {
			item["suggestion_id"] = uuid.UUID(row.ID).String()
			item["content"] = row.Content
			item["rationale"] = row.Rationale
			item["review_state"] = row.ReviewState
			item["citations"] = citations
			item["gaps"] = gaps
		}
		items[i] = item
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"drafts": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func aiResponseDraftFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "response_draft_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to access AI response drafts.", false)
	case "response_draft_invalid", "response_draft_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The AI response draft request is invalid.", false)
	case "response_draft_conflict":
		return fail(c, 409, "AI_RESPONSE_DRAFT_CONFLICT", "The requirement, response, or workspace has changed or lacks a clean source.", false)
	case "response_draft_unavailable", "response_draft_read_unavailable":
		return fail(c, 503, "AI_RESPONSE_DRAFT_UNAVAILABLE", "AI response drafting is temporarily unavailable.", true)
	default:
		return fail(c, 503, "AI_RESPONSE_DRAFT_UNAVAILABLE", "AI response drafting is temporarily unavailable.", true)
	}
}
