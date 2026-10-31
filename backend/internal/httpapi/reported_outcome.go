package httpapi

import (
	app "bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const OutcomeProofUploadsPath = "/v1/tenderpreneur/bids/:bid_id/outcome-proof/uploads"
const OutcomeProofCompletePath = "/v1/tenderpreneur/bids/:bid_id/outcome-proof/uploads/:upload_id/complete"
const ReportedOutcomesPath = "/v1/tenderpreneur/bids/:bid_id/outcomes"
const CurrentReportedOutcomePath = "/v1/tenderpreneur/bids/:bid_id/outcome"

type ReportedOutcomeUseCase interface {
	InitiateProof(context.Context, app.InitiateOutcomeProofInput) (app.InitiateUploadResult, error)
	CompleteProof(context.Context, app.CompleteOutcomeProofInput) (app.OutcomeProofResult, error)
	Report(context.Context, app.ReportOutcomeInput) (app.ReportOutcomeResult, error)
	Current(context.Context, app.WorkflowSubject, app.ID) (app.ReportedOutcome, error)
	History(context.Context, app.WorkflowSubject, app.ID) ([]app.ReportedOutcome, error)
}
type ReportedOutcomeHandler struct{ s ReportedOutcomeUseCase }

func NewReportedOutcomeHandler(s ReportedOutcomeUseCase) (*ReportedOutcomeHandler, error) {
	if s == nil {
		return nil, errors.New("reported_outcome_handler_invalid")
	}
	return &ReportedOutcomeHandler{s}, nil
}

type outcomeProofBody struct {
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}
type reportOutcomeBody struct {
	Outcome                  string `json:"outcome"`
	SourceClass              string `json:"source_class"`
	Reason                   string `json:"reason"`
	Lessons                  string `json:"lessons"`
	ReportedTime             string `json:"reported_outcome_at"`
	ProofVersionID           string `json:"proof_document_version_id"`
	SupersedesID             string `json:"supersedes_outcome_id"`
	CorrectionReason         string `json:"correction_reason"`
	ExpectedSequence         int32  `json:"expected_sequence"`
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
}

func (h *ReportedOutcomeHandler) Initiate(c fiber.Ctx) error {
	var b outcomeProofBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	if !requireWorkflowFields(c.Body(), []string{"size_bytes", "sha256"}) {
		return fail(c, 400, "INVALID_REQUEST", "The outcome proof request is invalid.", false)
	}
	x, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	v, e := h.s.InitiateProof(c.Context(), app.InitiateOutcomeProofInput{Subject: x.subject, BidID: x.opportunityID, CorrelationID: x.correlationID, SizeBytes: b.SizeBytes, SHA256Hex: b.SHA256, IdempotencyKey: x.idempotencyKey})
	if e != nil {
		return outcomeFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(v.Session.ID).String(), "document_id": uuid.UUID(v.Session.DocumentID).String(), "media_type": "application/pdf", "state": v.Session.State, "expires_at": v.Session.ExpiresAt.Format(time.RFC3339Nano), "transfer": fiber.Map{"method": v.Instruction.Method, "url": v.Instruction.URL, "form_fields": v.Instruction.Fields}}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ReportedOutcomeHandler) Complete(c fiber.Ctx) error {
	var b completeUploadBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	if !requireWorkflowFields(c.Body(), nil) {
		return fail(c, 400, "INVALID_REQUEST", "The outcome proof request is invalid.", false)
	}
	x, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	u, ok := parseOpportunityID(c.Params("upload_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The outcome proof request is invalid.", false)
	}
	v, e := h.s.CompleteProof(c.Context(), app.CompleteOutcomeProofInput{Subject: x.subject, BidID: x.opportunityID, UploadID: u, CorrelationID: x.correlationID, IdempotencyKey: x.idempotencyKey})
	if e != nil {
		return outcomeFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("Location", "/v1/jobs/"+uuid.UUID(v.JobID).String())
	return c.Status(202).JSON(fiber.Map{"data": fiber.Map{"upload_id": uuid.UUID(v.UploadID).String(), "document_id": uuid.UUID(v.DocumentID).String(), "document_version_id": uuid.UUID(v.VersionID).String(), "scan_job_id": uuid.UUID(v.JobID).String(), "scan_state": v.ScanState, "replay": v.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ReportedOutcomeHandler) Report(c fiber.Ctx) error {
	var b reportOutcomeBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	if !requireWorkflowFields(c.Body(), []string{"outcome", "source_class", "reason", "reported_outcome_at", "expected_sequence", "expected_workspace_version"}, "lessons", "proof_document_version_id", "supersedes_outcome_id", "correction_reason") {
		return fail(c, 400, "INVALID_REQUEST", "The outcome request is invalid.", false)
	}
	x, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	proof, ok := optionalOpportunityID(b.ProofVersionID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The outcome request is invalid.", false)
	}
	super, ok := optionalOpportunityID(b.SupersedesID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The outcome request is invalid.", false)
	}
	v, e := h.s.Report(c.Context(), app.ReportOutcomeInput{Subject: x.subject, BidID: x.opportunityID, ProofVersionID: proof, SupersedesID: super, CorrelationID: x.correlationID, Outcome: b.Outcome, SourceClass: b.SourceClass, Reason: b.Reason, Lessons: b.Lessons, ReportedTimeText: b.ReportedTime, CorrectionReason: b.CorrectionReason, IdempotencyKey: x.idempotencyKey, ExpectedSequence: b.ExpectedSequence, ExpectedWorkspaceVersion: b.ExpectedWorkspaceVersion})
	if e != nil {
		return outcomeFailure(c, e)
	}
	if v.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": outcomeView(v.Outcome), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ReportedOutcomeHandler) Current(c fiber.Ctx) error {
	sub, bid, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	v, e := h.s.Current(c.Context(), sub, bid)
	if e != nil {
		return outcomeFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": outcomeView(v), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ReportedOutcomeHandler) History(c fiber.Ctx) error {
	sub, bid, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	rows, e := h.s.History(c.Context(), sub, bid)
	if e != nil {
		return outcomeFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	items := make([]fiber.Map, len(rows))
	for i, v := range rows {
		items[i] = outcomeView(v)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"outcomes": items}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func optionalOpportunityID(v string) (app.ID, bool) {
	if v == "" {
		return app.ID{}, true
	}
	return parseOpportunityID(v)
}
func outcomeView(v app.ReportedOutcome) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "bid_id": uuid.UUID(v.BidID).String(), "sequence": v.Sequence, "reported_outcome": v.Outcome, "verification_status": v.VerificationStatus, "provenance": v.Provenance, "source_class": v.SourceClass, "reason": v.Reason, "lessons": v.Lessons, "reported_outcome_time_text": v.ReportedTimeText, "reported_outcome_at": v.ReportedAt.Format(time.RFC3339Nano), "recorded_at": v.RecordedAt.Format(time.RFC3339Nano), "reported_by": uuid.UUID(v.ReportedBy).String(), "proof_document_version_id": optionalUUID(v.ProofVersionID), "supersedes_outcome_id": optionalUUID(v.SupersedesID), "correction_reason": v.CorrectionReason, "authoritative_buyer_decision": false, "contract_award_confirmed": false, "withdrawal_executed": false, "analytics_accepted": false}
}
func outcomeFailure(c fiber.Ctx, e error) error {
	switch e.Error() {
	case "outcome_proof_invalid", "outcome_invalid", "outcome_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The reported outcome request is invalid.", false)
	case "outcome_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to manage reported outcomes.", false)
	case "outcome_not_found":
		return fail(c, 404, "OUTCOME_NOT_FOUND", "No reported outcome was found.", false)
	case "outcome_proof_expired":
		return fail(c, 409, "UPLOAD_EXPIRED", "The outcome-proof upload expired.", false)
	case "outcome_proof_mismatch":
		return fail(c, 409, "UPLOAD_MISMATCH", "The outcome proof does not match its declaration.", false)
	case "outcome_read_limit":
		return fail(c, 409, "OUTCOME_READ_LIMIT", "The reported outcome history exceeds the bounded read limit.", false)
	case "outcome_conflict":
		return fail(c, 409, "OUTCOME_CONFLICT", "The current outcome, proof or expected sequence changed.", false)
	default:
		return fail(c, 503, "OUTCOME_UNAVAILABLE", "Reported outcomes are temporarily unavailable.", true)
	}
}
