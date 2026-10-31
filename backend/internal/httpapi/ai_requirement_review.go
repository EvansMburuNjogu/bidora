package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const AIRequirementListPath = "/v1/tenderpreneur/opportunities/:opportunity_id/document-versions/:document_version_id/ai-requirements"
const AIRequirementAcceptPath = "/v1/tenderpreneur/opportunities/:opportunity_id/document-versions/:document_version_id/ai-requirements/:suggestion_id/accept"
const AIRequirementRejectPath = "/v1/tenderpreneur/opportunities/:opportunity_id/document-versions/:document_version_id/ai-requirements/:suggestion_id/reject"

type AIRequirementReviewUseCase interface {
	List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.AISuggestion, error)
	Accept(context.Context, opportunityapp.AcceptAISuggestionInput) (opportunityapp.AISuggestionReviewResult, error)
	Reject(context.Context, opportunityapp.RejectAISuggestionInput) (opportunityapp.AISuggestionReviewResult, error)
}
type AIRequirementReviewHandler struct{ reviews AIRequirementReviewUseCase }

func NewAIRequirementReviewHandler(reviews AIRequirementReviewUseCase) (*AIRequirementReviewHandler, error) {
	if reviews == nil {
		return nil, errors.New("ai_requirement_review_handler_invalid")
	}
	return &AIRequirementReviewHandler{reviews}, nil
}

type acceptAISuggestionBody struct {
	BidID                     string  `json:"bid_id"`
	SourceVersionID           string  `json:"source_version_id"`
	RequirementText           string  `json:"requirement_text"`
	Category                  string  `json:"category"`
	Mandatory                 bool    `json:"mandatory"`
	OwnerID                   *string `json:"owner_id"`
	DueAt                     *string `json:"due_at"`
	ReviewNote                string  `json:"review_note"`
	ExpectedSuggestionVersion int64   `json:"expected_suggestion_version"`
	ExpectedWorkspaceVersion  int64   `json:"expected_workspace_version"`
}
type rejectAISuggestionBody struct {
	ReviewNote                string `json:"review_note"`
	ExpectedSuggestionVersion int64  `json:"expected_suggestion_version"`
}

func (h *AIRequirementReviewHandler) List(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	opportunityID, ok := parseOpportunityID(c.Params("opportunity_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	documentID, ok := parseOpportunityID(c.Params("document_version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	rows, err := h.reviews.List(c.Context(), subject, opportunityID, documentID)
	if err != nil {
		return aiReviewFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = aiSuggestionView(row)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"suggestions": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *AIRequirementReviewHandler) Accept(c fiber.Ctx) error {
	var body acceptAISuggestionBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, documentID, suggestionID, ok := aiReviewCommon(c)
	if !ok {
		return nil
	}
	bidID, ok := parseOpportunityID(body.BidID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	sourceID, ok := parseOpportunityID(body.SourceVersionID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	input := opportunityapp.AcceptAISuggestionInput{Subject: common.subject, OpportunityID: common.opportunityID, DocumentVersionID: documentID, SuggestionID: suggestionID, BidID: bidID, SourceVersionID: sourceID, CorrelationID: common.correlationID, RequirementText: body.RequirementText, Category: body.Category, Mandatory: body.Mandatory, ReviewNote: body.ReviewNote, ExpectedSuggestionVersion: body.ExpectedSuggestionVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey}
	if body.OwnerID != nil {
		owner, valid := parseOpportunityID(*body.OwnerID)
		if !valid {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		input.OwnerID = owner
		input.HasOwner = true
	}
	due, valid := parseOptionalInstant(body.DueAt)
	if !valid {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	input.DueAt = due
	result, err := h.reviews.Accept(c.Context(), input)
	if err != nil {
		return aiReviewFailure(c, err)
	}
	if result.Requirement == nil {
		return fail(c, 503, "REVIEW_UNAVAILABLE", "The review service is temporarily unavailable.", true)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"suggestion": aiSuggestionView(result.Suggestion), "requirement": requirementView(*result.Requirement), "workspace_version": result.WorkspaceVersion, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *AIRequirementReviewHandler) Reject(c fiber.Ctx) error {
	var body rejectAISuggestionBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, documentID, suggestionID, ok := aiReviewCommon(c)
	if !ok {
		return nil
	}
	result, err := h.reviews.Reject(c.Context(), opportunityapp.RejectAISuggestionInput{Subject: common.subject, OpportunityID: common.opportunityID, DocumentVersionID: documentID, SuggestionID: suggestionID, CorrelationID: common.correlationID, ReviewNote: body.ReviewNote, ExpectedSuggestionVersion: body.ExpectedSuggestionVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return aiReviewFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"suggestion": aiSuggestionView(result.Suggestion), "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func aiReviewCommon(c fiber.Ctx) (workflowCommonInput, opportunityapp.ID, opportunityapp.ID, bool) {
	common, ok := workflowCommon(c, "opportunity_id")
	if !ok {
		return workflowCommonInput{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	documentID, ok := parseOpportunityID(c.Params("document_version_id"))
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return workflowCommonInput{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	suggestionID, ok := parseOpportunityID(c.Params("suggestion_id"))
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return workflowCommonInput{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	return common, documentID, suggestionID, true
}
func aiSuggestionView(v opportunityapp.AISuggestion) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "document_version_id": uuid.UUID(v.DocumentVersionID).String(), "ordinal": v.Ordinal, "requirement_text": v.RequirementText, "category": v.Category, "mandatory": v.Mandatory, "citation_page": optionalInt32(v.CitationPage), "citation_location": v.CitationLocation, "citation_quote": v.CitationQuote, "confidence": v.Confidence, "review_state": v.ReviewState, "version": v.Version, "accepted_requirement_id": optionalID(v.AcceptedRequirementID, v.HasAcceptedRequirement), "review_note": optionalString(v.ReviewNote), "reviewed_by": optionalID(v.ReviewedBy, v.HasReviewedBy), "created_at": v.CreatedAt.Format(time.RFC3339Nano), "reviewed_at": formatOptionalTime(v.ReviewedAt)}
}
func optionalInt32(v int32) any {
	if v == 0 {
		return nil
	}
	return v
}
func aiReviewFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "ai_requirement_review_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to review AI requirements.", false)
	case "ai_requirement_review_invalid", "ai_requirement_accept_invalid", "ai_requirement_reject_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The review request is invalid.", false)
	case "ai_requirement_review_conflict":
		return fail(c, 409, "AI_REQUIREMENT_REVIEW_CONFLICT", "The suggestion or workspace has changed.", false)
	default:
		return fail(c, 503, "REVIEW_UNAVAILABLE", "The review service is temporarily unavailable.", true)
	}
}
