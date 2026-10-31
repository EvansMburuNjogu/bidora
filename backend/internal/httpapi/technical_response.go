package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const TechnicalResponsePath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id/response"
const TechnicalResponseHistoryPath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id/response/history"
const GapReviewPath = "/v1/tenderpreneur/bids/:bid_id/gap-review"

type TechnicalResponseUseCase interface {
	Save(context.Context, opportunityapp.SaveTechnicalResponseInput) (opportunityapp.TechnicalResponseResult, error)
	Get(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) (opportunityapp.TechnicalResponse, error)
	History(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.TechnicalResponse, error)
	GapReview(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.GapReview, error)
}
type TechnicalResponseHandler struct{ responses TechnicalResponseUseCase }

func NewTechnicalResponseHandler(r TechnicalResponseUseCase) (*TechnicalResponseHandler, error) {
	if r == nil {
		return nil, errors.New("technical_response_handler_invalid")
	}
	return &TechnicalResponseHandler{r}, nil
}

type saveTechnicalResponseBody struct {
	Content                  string   `json:"content"`
	ChangeNote               string   `json:"change_note"`
	EvidenceIDs              []string `json:"evidence_ids"`
	ExpectedSectionVersion   int64    `json:"expected_section_version"`
	ExpectedWorkspaceVersion int64    `json:"expected_workspace_version"`
}

func (h *TechnicalResponseHandler) Save(c fiber.Ctx) error {
	var body saveTechnicalResponseBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"content", "change_note", "evidence_ids", "expected_section_version", "expected_workspace_version"}) {
		return fail(c, 400, "INVALID_REQUEST", "The response request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	requirementID, ok := parseOpportunityID(c.Params("requirement_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	evidenceIDs := make([]opportunityapp.ID, len(body.EvidenceIDs))
	for i, value := range body.EvidenceIDs {
		evidenceIDs[i], ok = parseOpportunityID(value)
		if !ok {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	result, err := h.responses.Save(c.Context(), opportunityapp.SaveTechnicalResponseInput{Subject: common.subject, BidID: common.opportunityID, RequirementID: requirementID, CorrelationID: common.correlationID, Content: body.Content, ChangeNote: body.ChangeNote, EvidenceIDs: evidenceIDs, ExpectedSectionVersion: body.ExpectedSectionVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return technicalResponseFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"response": technicalResponseView(result.Response), "workspace_version": result.WorkspaceVersion, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *TechnicalResponseHandler) Get(c fiber.Ctx) error {
	subject, bidID, requirementID, ok := technicalReadCommon(c)
	if !ok {
		return nil
	}
	result, err := h.responses.Get(c.Context(), subject, bidID, requirementID)
	if err != nil {
		return technicalResponseFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"response": technicalResponseView(result)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *TechnicalResponseHandler) History(c fiber.Ctx) error {
	subject, bidID, requirementID, ok := technicalReadCommon(c)
	if !ok {
		return nil
	}
	rows, err := h.responses.History(c.Context(), subject, bidID, requirementID)
	if err != nil {
		return technicalResponseFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = technicalResponseView(row)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"versions": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *TechnicalResponseHandler) GapReview(c fiber.Ctx) error {
	if len(c.Body()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	bidID, ok := parseOpportunityID(c.Params("bid_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.responses.GapReview(c.Context(), subject, bidID)
	if err != nil {
		return technicalResponseFailure(c, err)
	}
	requirements := make([]fiber.Map, len(result.Requirements))
	for i, row := range result.Requirements {
		requirements[i] = fiber.Map{"requirement_id": uuid.UUID(row.RequirementID).String(), "requirement_text": row.RequirementText, "mandatory": row.Mandatory, "requirement_status": row.RequirementStatus, "requirement_version": row.RequirementVersion, "blockers": row.Blockers}
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"bid_id": uuid.UUID(result.BidID).String(), "workspace_state": result.WorkspaceState, "ready": result.Ready, "workspace_blockers": result.WorkspaceBlockers, "requirement_count": result.RequirementCount, "blocker_count": result.BlockerCount, "requirements": requirements, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func technicalReadCommon(c fiber.Ctx) (opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID, bool) {
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	subject, ok := readSubject(c)
	if !ok {
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	if len(c.Request().URI().QueryString()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	bidID, ok := parseOpportunityID(c.Params("bid_id"))
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	requirementID, ok := parseOpportunityID(c.Params("requirement_id"))
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return opportunityapp.WorkflowSubject{}, opportunityapp.ID{}, opportunityapp.ID{}, false
	}
	return subject, bidID, requirementID, true
}
func technicalResponseView(v opportunityapp.TechnicalResponse) fiber.Map {
	evidence := make([]string, len(v.EvidenceIDs))
	for i, id := range v.EvidenceIDs {
		evidence[i] = uuid.UUID(id).String()
	}
	return fiber.Map{"section_id": uuid.UUID(v.SectionID).String(), "bid_id": uuid.UUID(v.BidID).String(), "requirement_id": uuid.UUID(v.RequirementID).String(), "section_version": v.SectionVersion, "response_version_id": uuid.UUID(v.ResponseVersionID).String(), "revision": v.Revision, "content": v.Content, "change_note": v.ChangeNote, "evidence_ids": evidence, "created_by": uuid.UUID(v.CreatedBy).String(), "created_at": v.CreatedAt.Format(time.RFC3339Nano)}
}
func technicalResponseFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "technical_response_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to access technical responses.", false)
	case "technical_response_invalid", "technical_response_read_invalid", "gap_review_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The response request is invalid.", false)
	case "technical_response_not_found":
		return fail(c, 404, "NOT_FOUND", "The technical response was not found.", false)
	case "technical_response_conflict":
		return fail(c, 409, "RESPONSE_CONFLICT", "The response or workspace has changed.", false)
	case "technical_response_read_unavailable", "technical_response_unavailable", "gap_review_unavailable":
		return fail(c, 503, "RESPONSE_UNAVAILABLE", "The response service is temporarily unavailable.", true)
	default:
		return fail(c, 503, "RESPONSE_UNAVAILABLE", "The response service is temporarily unavailable.", true)
	}
}
