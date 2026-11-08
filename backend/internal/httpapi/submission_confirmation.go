package httpapi

import (
	app "bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const SubmissionProofUploadsPath = "/v1/tenderpreneur/bids/:bid_id/submission-evidence-reports/:evidence_id/proof/uploads"
const SubmissionProofCompletePath = "/v1/tenderpreneur/bids/:bid_id/submission-evidence-reports/:evidence_id/proof/uploads/:upload_id/complete"
const SubmissionConfirmationPath = "/v1/tenderpreneur/bids/:bid_id/submission-confirmation"

type SubmissionUseCase interface {
	Initiate(context.Context, app.InitiateSubmissionProofInput) (app.InitiateUploadResult, error)
	Complete(context.Context, app.CompleteSubmissionProofInput) (app.SubmissionProofResult, error)
	Confirm(context.Context, app.ConfirmSubmissionInput) (app.SubmissionConfirmationResult, error)
	Get(context.Context, app.WorkflowSubject, app.ID) (app.SubmissionConfirmation, error)
}
type SubmissionHandler struct{ s SubmissionUseCase }

func NewSubmissionHandler(s SubmissionUseCase) (*SubmissionHandler, error) {
	if s == nil {
		return nil, errors.New("submission_handler_invalid")
	}
	return &SubmissionHandler{s}, nil
}

type proofInitBody struct {
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}
type confirmSubmissionBody struct {
	EvidenceID               string `json:"evidence_id"`
	ManifestID               string `json:"manifest_id"`
	ProofDocumentVersionID   string `json:"proof_document_version_id"`
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
	Rationale                string `json:"rationale"`
}

func (h *SubmissionHandler) Initiate(c fiber.Ctx) error {
	var b proofInitBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	if !requireWorkflowFields(c.Body(), []string{"size_bytes", "sha256"}) {
		return fail(c, 400, "INVALID_REQUEST", "The submission proof request is invalid.", false)
	}
	x, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	eid, ok := parseOpportunityID(c.Params("evidence_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The submission proof request is invalid.", false)
	}
	v, e := h.s.Initiate(c.Context(), app.InitiateSubmissionProofInput{Subject: x.subject, BidID: x.opportunityID, EvidenceID: eid, CorrelationID: x.correlationID, SizeBytes: b.SizeBytes, SHA256Hex: b.SHA256, IdempotencyKey: x.idempotencyKey})
	if e != nil {
		return submissionFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(v.Session.ID).String(), "document_id": uuid.UUID(v.Session.DocumentID).String(), "media_type": "application/pdf", "state": v.Session.State, "expires_at": v.Session.ExpiresAt.Format(time.RFC3339Nano), "transfer": fiber.Map{"method": v.Instruction.Method, "url": v.Instruction.URL, "form_fields": v.Instruction.Fields}}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *SubmissionHandler) Complete(c fiber.Ctx) error {
	var b completeUploadBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	if !requireWorkflowFields(c.Body(), nil) {
		return fail(c, 400, "INVALID_REQUEST", "The submission proof request is invalid.", false)
	}
	x, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	eid, eok := parseOpportunityID(c.Params("evidence_id"))
	uid, uok := parseOpportunityID(c.Params("upload_id"))
	if !eok || !uok {
		return fail(c, 400, "INVALID_REQUEST", "The submission proof request is invalid.", false)
	}
	v, e := h.s.Complete(c.Context(), app.CompleteSubmissionProofInput{Subject: x.subject, BidID: x.opportunityID, EvidenceID: eid, UploadID: uid, CorrelationID: x.correlationID, IdempotencyKey: x.idempotencyKey})
	if e != nil {
		return submissionFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("Location", "/v1/jobs/"+uuid.UUID(v.JobID).String())
	return c.Status(202).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(v.UploadID).String(), "document_id": uuid.UUID(v.DocumentID).String(), "document_version_id": uuid.UUID(v.VersionID).String(), "scan_job_id": uuid.UUID(v.JobID).String(), "scan_state": v.ScanState, "replay": v.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *SubmissionHandler) Confirm(c fiber.Ctx) error {
	var b confirmSubmissionBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	if !requireWorkflowFields(c.Body(), []string{"evidence_id", "manifest_id", "proof_document_version_id", "expected_workspace_version", "rationale"}) {
		return fail(c, 400, "INVALID_REQUEST", "The submission confirmation request is invalid.", false)
	}
	x, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	eid, eok := parseOpportunityID(b.EvidenceID)
	mid, mok := parseOpportunityID(b.ManifestID)
	pid, pok := parseOpportunityID(b.ProofDocumentVersionID)
	principal, present := SessionPrincipalFrom(c)
	if !eok || !mok || !pok || !present {
		return fail(c, 400, "INVALID_REQUEST", "The submission confirmation request is invalid.", false)
	}
	v, e := h.s.Confirm(c.Context(), app.ConfirmSubmissionInput{Subject: x.subject, BidID: x.opportunityID, EvidenceID: eid, ManifestID: mid, ProofVersionID: pid, CorrelationID: x.correlationID, ExpectedWorkspaceVersion: b.ExpectedWorkspaceVersion, Rationale: b.Rationale, IdempotencyKey: x.idempotencyKey, SessionAuthenticatedAt: principal.AuthenticatedAt.UTC(), AssuranceLevel: principal.AssuranceLevel, AssuranceMethod: principal.AssuranceMethod, AssuranceAt: principal.AssuredAt})
	if e != nil {
		return submissionFailure(c, e)
	}
	if v.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": submissionView(v.Confirmation), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *SubmissionHandler) Get(c fiber.Ctx) error {
	subject, bid, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	v, e := h.s.Get(c.Context(), subject, bid)
	if e != nil {
		return submissionFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": submissionView(v), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func submissionView(v app.SubmissionConfirmation) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "bid_id": uuid.UUID(v.BidID).String(), "external_action_evidence_id": uuid.UUID(v.EvidenceID).String(), "package_manifest_id": uuid.UUID(v.ManifestID).String(), "proposal_approval_id": uuid.UUID(v.ApprovalID).String(), "proof_document_version_id": uuid.UUID(v.ProofVersionID).String(), "reporter_id": uuid.UUID(v.ReporterID).String(), "verifier_id": uuid.UUID(v.VerifierID).String(), "reported_action_at": v.ReportedActionAt.Format(time.RFC3339Nano), "report_recorded_at": v.ReportRecordedAt.Format(time.RFC3339Nano), "confirmed_at": v.ConfirmedAt.Format(time.RFC3339Nano), "rationale": v.Rationale, "workspace_version": v.WorkspaceVersion, "submitted": true, "submission_basis": v.SubmissionBasis, "bidora_transmitted": false, "external_acceptance_confirmed": false, "on_time_submission_confirmed": false}
}
func submissionFailure(c fiber.Ctx, e error) error {
	switch e.Error() {
	case "submission_proof_invalid", "submission_invalid", "submission_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The submission request is invalid.", false)
	case "submission_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized for this submission operation.", false)
	case "submission_fresh_auth_required":
		return fail(c, 403, "FRESH_AUTHENTICATION_REQUIRED", "Sign in again before confirming submission.", false)
	case "submission_not_found":
		return fail(c, 404, "SUBMISSION_NOT_FOUND", "The submission record was not found.", false)
	case "submission_proof_expired":
		return fail(c, 409, "UPLOAD_EXPIRED", "The upload instructions expired.", false)
	case "submission_proof_mismatch":
		return fail(c, 409, "UPLOAD_MISMATCH", "The proof object does not match its declaration.", false)
	case "submission_blocked", "submission_conflict":
		return fail(c, 409, "SUBMISSION_BLOCKED", "Submission confirmation requirements are not satisfied.", false)
	default:
		return fail(c, 503, "SUBMISSION_UNAVAILABLE", "Submission is temporarily unavailable.", true)
	}
}
