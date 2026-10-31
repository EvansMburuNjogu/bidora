package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const RequirementCorrectionPath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id"
const RequirementCorrectionHistoryPath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id/history"

type RequirementCorrectionUseCase interface {
	Correct(context.Context, opportunityapp.CorrectRequirementInput) (opportunityapp.RequirementCorrectionResult, error)
	History(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.RequirementContentRevision, error)
}
type RequirementCorrectionHandler struct{ service RequirementCorrectionUseCase }

func NewRequirementCorrectionHandler(v RequirementCorrectionUseCase) (*RequirementCorrectionHandler, error) {
	if v == nil {
		return nil, errors.New("requirement_correction_handler_invalid")
	}
	return &RequirementCorrectionHandler{v}, nil
}

type correctRequirementBody struct {
	SourceLocation             string `json:"source_location"`
	Text                       string `json:"text"`
	Mandatory                  bool   `json:"mandatory"`
	Category                   string `json:"category"`
	CorrectionReason           string `json:"correction_reason"`
	ExpectedRequirementVersion int64  `json:"expected_requirement_version"`
	ExpectedWorkspaceVersion   int64  `json:"expected_workspace_version"`
}

func (h *RequirementCorrectionHandler) Correct(c fiber.Ctx) error {
	var body correctRequirementBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"source_location", "text", "mandatory", "category", "correction_reason", "expected_requirement_version", "expected_workspace_version"}) {
		return fail(c, 400, "INVALID_REQUEST", "The requirement correction request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	requirementID, ok := parseOpportunityID(c.Params("requirement_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.service.Correct(c.Context(), opportunityapp.CorrectRequirementInput{Subject: common.subject, BidID: common.opportunityID, RequirementID: requirementID, CorrelationID: common.correlationID, SourceLocation: body.SourceLocation, Text: body.Text, Mandatory: body.Mandatory, Category: body.Category, CorrectionReason: body.CorrectionReason, ExpectedRequirementVersion: body.ExpectedRequirementVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return requirementCorrectionFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"requirement": requirementView(result.Requirement), "content_revision": requirementCorrectionView(result.ContentRevision), "workspace_version": result.WorkspaceVersion, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *RequirementCorrectionHandler) History(c fiber.Ctx) error {
	if len(c.Body()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, bidID, requirementID, ok := technicalReadCommon(c)
	if !ok {
		return nil
	}
	rows, err := h.service.History(c.Context(), subject, bidID, requirementID)
	if err != nil {
		return requirementCorrectionFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = requirementCorrectionView(row)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"versions": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func requirementCorrectionView(v opportunityapp.RequirementContentRevision) fiber.Map {
	result := fiber.Map{"id": uuid.UUID(v.ID).String(), "requirement_id": uuid.UUID(v.RequirementID).String(), "bid_id": uuid.UUID(v.BidID).String(), "revision": v.Revision, "source_version_id": uuid.UUID(v.SourceVersionID).String(), "source_location": v.SourceLocation, "text": v.Text, "mandatory": v.Mandatory, "category": v.Category, "correction_reason": v.CorrectionReason, "created_by": uuid.UUID(v.CreatedBy).String(), "created_at": v.CreatedAt.Format(time.RFC3339Nano)}
	if v.HasSourceDocument {
		result["source_document_version_id"] = uuid.UUID(v.SourceDocumentVersionID).String()
	}
	return result
}
func requirementCorrectionFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "requirement_correction_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to correct requirements.", false)
	case "requirement_correction_invalid", "requirement_correction_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The requirement correction request is invalid.", false)
	case "requirement_correction_conflict":
		return fail(c, 409, "REQUIREMENT_CORRECTION_CONFLICT", "The requirement is satisfied or its version changed.", false)
	default:
		return fail(c, 503, "REQUIREMENT_CORRECTION_UNAVAILABLE", "Requirement correction is temporarily unavailable.", true)
	}
}
