package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const RequirementListPath = "/v1/tenderpreneur/bids/:bid_id/requirements"
const RequirementAssignmentPath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id/assignment"
const ReadinessPath = "/v1/tenderpreneur/bids/:bid_id/readiness"

type RequirementUseCase interface {
	Create(context.Context, opportunityapp.CreateRequirementInput) (opportunityapp.RequirementResult, error)
	Assign(context.Context, opportunityapp.AssignRequirementInput) (opportunityapp.RequirementResult, error)
	List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) ([]opportunityapp.Requirement, error)
	Readiness(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.Readiness, error)
}
type RequirementHandler struct{ requirements RequirementUseCase }

func NewRequirementHandler(requirements RequirementUseCase) (*RequirementHandler, error) {
	if requirements == nil {
		return nil, errors.New("requirement_handler_invalid")
	}
	return &RequirementHandler{requirements}, nil
}

type createRequirementBody struct {
	SourceVersionID          string  `json:"source_version_id"`
	SourceLocation           string  `json:"source_location"`
	Text                     string  `json:"text"`
	Category                 string  `json:"category"`
	Mandatory                bool    `json:"mandatory"`
	OwnerID                  *string `json:"owner_id"`
	DueAt                    *string `json:"due_at"`
	ExpectedWorkspaceVersion int64   `json:"expected_workspace_version"`
}
type assignRequirementBody struct {
	OwnerID                    string  `json:"owner_id"`
	DueAt                      *string `json:"due_at"`
	ExpectedWorkspaceVersion   int64   `json:"expected_workspace_version"`
	ExpectedRequirementVersion int64   `json:"expected_requirement_version"`
}

func (h *RequirementHandler) Create(c fiber.Ctx) error {
	var body createRequirementBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"source_version_id", "source_location", "text", "category", "mandatory", "expected_workspace_version"}, "owner_id", "due_at") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	sourceID, ok := parseOpportunityID(body.SourceVersionID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	input := opportunityapp.CreateRequirementInput{Subject: common.subject, BidID: common.opportunityID, SourceVersionID: sourceID, CorrelationID: common.correlationID, SourceLocation: body.SourceLocation, Text: body.Text, Category: body.Category, Mandatory: body.Mandatory, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey}
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
	result, err := h.requirements.Create(c.Context(), input)
	if err != nil {
		return requirementFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"requirement": requirementView(result.Requirement), "workspace_version": result.WorkspaceVersion, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *RequirementHandler) Assign(c fiber.Ctx) error {
	var body assignRequirementBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"owner_id", "expected_workspace_version", "expected_requirement_version"}, "due_at") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	requirementID, valid := parseOpportunityID(c.Params("requirement_id"))
	if !valid {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	ownerID, valid := parseOpportunityID(body.OwnerID)
	if !valid {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	due, valid := parseOptionalInstant(body.DueAt)
	if !valid {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.requirements.Assign(c.Context(), opportunityapp.AssignRequirementInput{Subject: common.subject, BidID: common.opportunityID, RequirementID: requirementID, OwnerID: ownerID, CorrelationID: common.correlationID, DueAt: due, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, ExpectedRequirementVersion: body.ExpectedRequirementVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return requirementFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"requirement": requirementView(result.Requirement), "workspace_version": result.WorkspaceVersion, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *RequirementHandler) List(c fiber.Ctx) error {
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
	rows, err := h.requirements.List(c.Context(), subject, bidID)
	if err != nil {
		return requirementFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = requirementView(row)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"requirements": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *RequirementHandler) Readiness(c fiber.Ctx) error {
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
	result, err := h.requirements.Readiness(c.Context(), subject, bidID)
	if err != nil {
		return requirementFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"bid_id": uuid.UUID(result.BidID).String(), "workspace_state": result.WorkspaceState, "workspace_version": result.WorkspaceVersion, "ready": result.Ready, "open_mandatory_count": result.OpenMandatoryCount, "unassigned_mandatory_count": result.UnassignedMandatoryCount, "overdue_count": result.OverdueCount, "expired_evidence_count": result.ExpiredEvidenceCount, "blockers": result.Blockers}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func parseOptionalInstant(value *string) (*time.Time, bool) {
	if value == nil {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}
func requirementView(r opportunityapp.Requirement) fiber.Map {
	return fiber.Map{"id": uuid.UUID(r.ID).String(), "bid_id": uuid.UUID(r.BidID).String(), "source_version_id": uuid.UUID(r.SourceVersionID).String(), "source_location": r.SourceLocation, "text": r.Text, "mandatory": r.Mandatory, "category": r.Category, "status": r.Status, "owner_id": optionalID(r.OwnerID, r.HasOwner), "due_at": formatOptionalTime(r.DueAt), "version": r.Version, "created_at": r.CreatedAt.Format(time.RFC3339Nano), "updated_at": r.UpdatedAt.Format(time.RFC3339Nano)}
}
func requirementFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "requirement_not_authorized", "requirement_assignment_not_authorized", "requirement_read_not_authorized", "readiness_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to perform this operation.", false)
	case "requirement_invalid", "requirement_assignment_invalid", "requirement_read_invalid", "readiness_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case "requirement_conflict", "requirement_assignment_conflict":
		return fail(c, 409, "REQUIREMENT_CONFLICT", "The requirement operation conflicts with the current bid version.", false)
	case "requirement_unavailable", "requirement_assignment_unavailable", "requirement_read_unavailable", "readiness_unavailable":
		return fail(c, 503, "REQUIREMENT_UNAVAILABLE", "The resource is temporarily unavailable.", true)
	default:
		return fail(c, 503, "REQUIREMENT_UNAVAILABLE", "The resource is temporarily unavailable.", true)
	}
}
