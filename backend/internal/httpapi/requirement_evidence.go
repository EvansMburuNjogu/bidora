package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const EvidenceUploadInitiatePath = "/v1/tenderpreneur/bids/:bid_id/evidence/uploads"
const EvidenceUploadCompletePath = "/v1/tenderpreneur/bids/:bid_id/evidence/uploads/:upload_id/complete"
const RequirementSatisfyPath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id/satisfy"
const RequirementEvidenceListPath = "/v1/tenderpreneur/bids/:bid_id/requirements/:requirement_id/evidence"

type RequirementEvidenceUseCase interface {
	InitiateUpload(context.Context, opportunityapp.InitiateEvidenceUploadInput) (opportunityapp.InitiateUploadResult, error)
	CompleteUpload(context.Context, opportunityapp.CompleteEvidenceUploadInput) (opportunityapp.EvidenceUploadResult, error)
	Satisfy(context.Context, opportunityapp.SatisfyRequirementInput) (opportunityapp.SatisfyRequirementResult, error)
	List(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID, opportunityapp.ID) ([]opportunityapp.RequirementEvidence, error)
}
type RequirementEvidenceHandler struct{ evidence RequirementEvidenceUseCase }

func NewRequirementEvidenceHandler(e RequirementEvidenceUseCase) (*RequirementEvidenceHandler, error) {
	if e == nil {
		return nil, errors.New("requirement_evidence_handler_invalid")
	}
	return &RequirementEvidenceHandler{e}, nil
}

type initiateEvidenceUploadBody struct {
	Title             string  `json:"title"`
	MediaType         string  `json:"media_type"`
	SizeBytes         int64   `json:"size_bytes"`
	SHA256            string  `json:"sha256"`
	EvidenceExpiresAt *string `json:"evidence_expires_at"`
}
type satisfyRequirementBody struct {
	DocumentVersionID          string `json:"document_version_id"`
	EvidenceLocation           string `json:"evidence_location"`
	Justification              string `json:"justification"`
	ExpectedRequirementVersion int64  `json:"expected_requirement_version"`
	ExpectedWorkspaceVersion   int64  `json:"expected_workspace_version"`
}

func (h *RequirementEvidenceHandler) Initiate(c fiber.Ctx) error {
	var body initiateEvidenceUploadBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"title", "media_type", "size_bytes", "sha256"}, "evidence_expires_at") {
		return fail(c, 400, "INVALID_REQUEST", "The evidence request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	result, err := h.evidence.InitiateUpload(c.Context(), opportunityapp.InitiateEvidenceUploadInput{Subject: common.subject, BidID: common.opportunityID, CorrelationID: common.correlationID, Title: body.Title, MediaType: body.MediaType, SizeBytes: body.SizeBytes, SHA256Hex: body.SHA256, EvidenceExpiresAt: body.EvidenceExpiresAt, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return evidenceFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(result.Session.ID).String(), "document_id": uuid.UUID(result.Session.DocumentID).String(), "state": result.Session.State, "expires_at": result.Session.ExpiresAt.Format(time.RFC3339Nano), "evidence_expires_at": formatOptionalTime(result.Session.EvidenceExpiresAt), "transfer": fiber.Map{"method": result.Instruction.Method, "url": result.Instruction.URL, "form_fields": result.Instruction.Fields}}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *RequirementEvidenceHandler) Complete(c fiber.Ctx) error {
	var body completeUploadBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), nil) {
		return fail(c, 400, "INVALID_REQUEST", "The evidence request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	uploadID, ok := parseOpportunityID(c.Params("upload_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.evidence.CompleteUpload(c.Context(), opportunityapp.CompleteEvidenceUploadInput{Subject: common.subject, BidID: common.opportunityID, UploadID: uploadID, CorrelationID: common.correlationID, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return evidenceFailure(c, err)
	}
	location := "/v1/jobs/" + uuid.UUID(result.ScanJobID).String()
	c.Set("Location", location)
	c.Set("Cache-Control", "private, no-store")
	return c.Status(202).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(result.UploadID).String(), "document_id": uuid.UUID(result.DocumentID).String(), "document_version_id": uuid.UUID(result.DocumentVersionID).String(), "scan_job_id": uuid.UUID(result.ScanJobID).String(), "scan_state": result.ScanState, "evidence_expires_at": formatOptionalTime(result.EvidenceExpiresAt), "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *RequirementEvidenceHandler) Satisfy(c fiber.Ctx) error {
	var body satisfyRequirementBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"document_version_id", "evidence_location", "justification", "expected_requirement_version", "expected_workspace_version"}) {
		return fail(c, 400, "INVALID_REQUEST", "The evidence request is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	requirementID, ok := parseOpportunityID(c.Params("requirement_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	versionID, ok := parseOpportunityID(body.DocumentVersionID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.evidence.Satisfy(c.Context(), opportunityapp.SatisfyRequirementInput{Subject: common.subject, BidID: common.opportunityID, RequirementID: requirementID, DocumentVersionID: versionID, CorrelationID: common.correlationID, EvidenceLocation: body.EvidenceLocation, Justification: body.Justification, ExpectedRequirementVersion: body.ExpectedRequirementVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return evidenceFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"requirement": requirementView(result.Requirement), "evidence": evidenceView(result.Evidence), "workspace_version": result.WorkspaceVersion, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *RequirementEvidenceHandler) List(c fiber.Ctx) error {
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
	requirementID, ok := parseOpportunityID(c.Params("requirement_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	rows, err := h.evidence.List(c.Context(), subject, bidID, requirementID)
	if err != nil {
		return evidenceFailure(c, err)
	}
	items := make([]fiber.Map, len(rows))
	for i, row := range rows {
		items[i] = evidenceView(row)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"evidence": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func evidenceView(v opportunityapp.RequirementEvidence) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "requirement_id": uuid.UUID(v.RequirementID).String(), "bid_id": uuid.UUID(v.BidID).String(), "document_version_id": uuid.UUID(v.DocumentVersionID).String(), "evidence_location": v.EvidenceLocation, "justification": v.Justification, "added_by": uuid.UUID(v.AddedBy).String(), "added_at": v.AddedAt.Format(time.RFC3339Nano), "expires_at": formatOptionalTime(v.ExpiresAt), "expired": v.Expired}
}
func evidenceFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "evidence_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to manage requirement evidence.", false)
	case "evidence_upload_invalid", "evidence_upload_complete_invalid", "requirement_satisfaction_invalid", "requirement_evidence_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The evidence request is invalid.", false)
	case "evidence_document_expired":
		return fail(c, 409, "EVIDENCE_EXPIRED", "The evidence document has expired.", false)
	case "evidence_upload_expired":
		return fail(c, 409, "UPLOAD_EXPIRED", "The upload instructions have expired.", false)
	case "evidence_upload_mismatch":
		return fail(c, 409, "UPLOAD_MISMATCH", "The uploaded object does not match the declared evidence.", false)
	case "evidence_conflict":
		return fail(c, 409, "EVIDENCE_CONFLICT", "The requirement, workspace, or evidence has changed.", false)
	case "evidence_upload_unavailable", "requirement_satisfaction_unavailable", "requirement_evidence_read_unavailable":
		return fail(c, 503, "EVIDENCE_UNAVAILABLE", "The evidence service is temporarily unavailable.", true)
	default:
		return fail(c, 503, "EVIDENCE_UNAVAILABLE", "The evidence service is temporarily unavailable.", true)
	}
}
