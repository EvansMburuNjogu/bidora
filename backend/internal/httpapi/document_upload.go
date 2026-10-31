package httpapi

import (
	"context"
	"errors"
	"time"

	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const DocumentUploadInitiatePath = "/v1/tenderpreneur/opportunities/:opportunity_id/documents/uploads"
const DocumentUploadCompletePath = "/v1/tenderpreneur/opportunities/:opportunity_id/documents/uploads/:upload_id/complete"
const DocumentJobPath = "/v1/jobs/:job_id"

type DocumentUploadUseCase interface {
	Initiate(context.Context, opportunityapp.InitiateUploadInput) (opportunityapp.InitiateUploadResult, error)
	Complete(context.Context, opportunityapp.CompleteUploadInput) (opportunityapp.UploadResult, error)
	Job(context.Context, opportunityapp.WorkflowSubject, opportunityapp.ID) (opportunityapp.DocumentJob, error)
}
type DocumentUploadHandler struct{ uploads DocumentUploadUseCase }

func NewDocumentUploadHandler(uploads DocumentUploadUseCase) (*DocumentUploadHandler, error) {
	if uploads == nil {
		return nil, errors.New("document_upload_handler_invalid")
	}
	return &DocumentUploadHandler{uploads}, nil
}

type initiateUploadBody struct {
	Title     string `json:"title"`
	MediaType string `json:"media_type"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}
type completeUploadBody struct{}

func (h *DocumentUploadHandler) Initiate(c fiber.Ctx) error {
	var body initiateUploadBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "opportunity_id")
	if !ok {
		return nil
	}
	result, err := h.uploads.Initiate(c.Context(), opportunityapp.InitiateUploadInput{Subject: common.subject, OpportunityID: common.opportunityID, CorrelationID: common.correlationID, Title: body.Title, MediaType: body.MediaType, SizeBytes: body.SizeBytes, SHA256Hex: body.SHA256, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return documentUploadFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(result.Session.ID).String(), "document_id": uuid.UUID(result.Session.DocumentID).String(), "state": result.Session.State, "expires_at": result.Session.ExpiresAt.Format(time.RFC3339Nano), "transfer": fiber.Map{"method": result.Instruction.Method, "url": result.Instruction.URL, "form_fields": result.Instruction.Fields}}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *DocumentUploadHandler) Complete(c fiber.Ctx) error {
	var body completeUploadBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	common, ok := workflowCommon(c, "opportunity_id")
	if !ok {
		return nil
	}
	uploadID, ok := parseOpportunityID(c.Params("upload_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.uploads.Complete(c.Context(), opportunityapp.CompleteUploadInput{Subject: common.subject, OpportunityID: common.opportunityID, UploadID: uploadID, CorrelationID: common.correlationID, IdempotencyKey: common.idempotencyKey})
	if err != nil {
		return documentUploadFailure(c, err)
	}
	location := "/v1/jobs/" + uuid.UUID(result.AnalysisJobID).String()
	c.Set("Location", location)
	c.Set("Cache-Control", "private, no-store")
	return c.Status(202).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(result.UploadID).String(), "document_id": uuid.UUID(result.DocumentID).String(), "document_version_id": uuid.UUID(result.DocumentVersionID).String(), "scan_job_id": uuid.UUID(result.ScanJobID).String(), "scan_state": result.ScanState, "analysis_job_id": uuid.UUID(result.AnalysisJobID).String(), "analysis_state": result.AnalysisState, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *DocumentUploadHandler) Job(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request query is invalid.", false)
	}
	jobID, ok := parseOpportunityID(c.Params("job_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	job, err := h.uploads.Job(c.Context(), subject, jobID)
	if err != nil {
		return documentUploadFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"id": uuid.UUID(job.ID).String(), "kind": job.Kind, "document_id": uuid.UUID(job.DocumentID).String(), "document_version_id": uuid.UUID(job.DocumentVersionID).String(), "opportunity_id": optionalID(job.OpportunityID, job.HasOpportunity), "bid_id": optionalID(job.BidID, job.HasBid), "document_kind": job.DocumentKind, "state": job.State, "blocked_reason": optionalString(job.BlockedReason), "attempts": job.Attempts, "created_at": job.CreatedAt.Format(time.RFC3339Nano), "started_at": formatOptionalTime(job.StartedAt), "completed_at": formatOptionalTime(job.CompletedAt)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func documentUploadFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "document_upload_invalid", "document_upload_complete_invalid", "document_job_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The upload request is invalid.", false)
	case "document_upload_not_authorized", "document_upload_complete_not_authorized", "document_job_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to upload this tender document.", false)
	case "document_job_not_found":
		return fail(c, 404, "NOT_FOUND", "The job was not found.", false)
	case "document_upload_expired":
		return fail(c, 409, "UPLOAD_EXPIRED", "The upload instructions have expired.", false)
	case "document_upload_mismatch":
		return fail(c, 409, "UPLOAD_MISMATCH", "The uploaded object does not match the declared file.", false)
	default:
		return fail(c, 503, "UPLOAD_UNAVAILABLE", "The upload service is temporarily unavailable.", true)
	}
}
