package httpapi

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const ExternalActionEvidencePath = "/v1/tenderpreneur/bids/:bid_id/submission-evidence-reports"
const ExternalActionEvidenceDetailPath = "/v1/tenderpreneur/bids/:bid_id/submission-evidence-reports/:evidence_id"
const ExternalEvidenceReconciliationPath = "/v1/tenderpreneur/bids/:bid_id/submission-evidence-reconciliation"

type ExternalActionEvidenceUseCase interface {
	Report(context.Context, application.ReportExternalActionEvidenceInput) (application.ExternalActionEvidenceResult, error)
	Get(context.Context, application.WorkflowSubject, application.ID, application.ID) (application.ExternalActionEvidence, error)
	List(context.Context, application.ExternalActionEvidenceHistoryInput) (application.ExternalActionEvidencePage, error)
	Reconciliation(context.Context, application.WorkflowSubject, application.ID) (application.ExternalEvidenceReconciliation, error)
}
type ExternalActionEvidenceHandler struct{ service ExternalActionEvidenceUseCase }

func NewExternalActionEvidenceHandler(service ExternalActionEvidenceUseCase) (*ExternalActionEvidenceHandler, error) {
	if service == nil {
		return nil, errors.New("external_action_evidence_handler_invalid")
	}
	return &ExternalActionEvidenceHandler{service}, nil
}

type externalActionEvidenceBody struct {
	PackageManifestID        string  `json:"package_manifest_id"`
	ExternalSystemName       string  `json:"external_system_name"`
	ExternalReference        *string `json:"external_reference"`
	ReportedActionTime       string  `json:"reported_action_time"`
	ReportNote               string  `json:"report_note"`
	SupersedesEvidenceID     *string `json:"supersedes_evidence_id"`
	ExpectedWorkspaceVersion int64   `json:"expected_workspace_version"`
}

func (h *ExternalActionEvidenceHandler) Report(c fiber.Ctx) error {
	var body externalActionEvidenceBody
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"package_manifest_id", "external_system_name", "reported_action_time", "report_note", "expected_workspace_version"}, "external_reference", "supersedes_evidence_id") {
		return fail(c, 400, "INVALID_REQUEST", "The external action evidence report is invalid.", false)
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	manifestID, ok := parseOpportunityID(body.PackageManifestID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The external action evidence report is invalid.", false)
	}
	var supersedes application.ID
	if body.SupersedesEvidenceID != nil {
		parsed, valid := parseOpportunityID(*body.SupersedesEvidenceID)
		if !valid {
			return fail(c, 400, "INVALID_REQUEST", "The external action evidence report is invalid.", false)
		}
		supersedes = parsed
	}
	result, err := h.service.Report(c.Context(), application.ReportExternalActionEvidenceInput{Subject: common.subject, BidID: common.opportunityID, PackageManifestID: manifestID, SupersedesEvidenceID: supersedes, CorrelationID: common.correlationID, ExternalSystemName: body.ExternalSystemName, ExternalReference: body.ExternalReference, ReportedActionTimeText: body.ReportedActionTime, ReportNote: body.ReportNote, IdempotencyKey: common.idempotencyKey, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion})
	if err != nil {
		return externalActionEvidenceFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": externalActionEvidenceView(result.Evidence), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ExternalActionEvidenceHandler) List(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommonWithoutQuery(c)
	if !ok {
		return nil
	}
	limit, cursor, values, ok := readPage(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The external action evidence history request is invalid.", false)
	}
	for key := range values {
		if key != "limit" && key != "cursor" {
			return fail(c, 400, "INVALID_REQUEST", "The external action evidence history request is invalid.", false)
		}
	}
	page, err := h.service.List(c.Context(), application.ExternalActionEvidenceHistoryInput{Subject: subject, BidID: bidID, Cursor: cursor, Limit: int32(limit)})
	if err != nil {
		return externalActionEvidenceFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	items := make([]fiber.Map, len(page.Items))
	for i, row := range page.Items {
		items[i] = externalActionEvidenceView(row)
	}
	var next *string
	if page.HasMore && len(page.Items) > 0 {
		encoded, err := encodeReadCursor(page.Items[len(page.Items)-1].RecordedAt, page.Items[len(page.Items)-1].ID)
		if err != nil {
			return fail(c, 503, "EXTERNAL_ACTION_EVIDENCE_UNAVAILABLE", "Evidence history is temporarily unavailable.", true)
		}
		next = &encoded
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"reports": items, "next_cursor": next}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ExternalActionEvidenceHandler) Reconciliation(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	result, err := h.service.Reconciliation(c.Context(), subject, bidID)
	if err != nil {
		return externalActionEvidenceFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	items := make([]fiber.Map, len(result.ActiveReports))
	for i, row := range result.ActiveReports {
		items[i] = fiber.Map{"evidence_id": uuid.UUID(row.EvidenceID).String(), "package_manifest_id": uuid.UUID(row.PackageManifestID).String(), "proposal_version_id": uuid.UUID(row.ProposalVersionID).String(), "package_is_current": !result.CurrentProposalVersionID.IsZero() && row.ProposalVersionID == result.CurrentProposalVersionID, "verification_status": row.VerificationStatus, "package_status": row.PackageStatus, "ready_for_approval_review_at_preparation": row.ReadinessReady, "package_readiness_observed_at": row.ReadinessObservedAt.Format(time.RFC3339Nano), "reported_action_at": row.ReportedActionAt.Format(time.RFC3339Nano), "recorded_at": row.RecordedAt.Format(time.RFC3339Nano), "supersedes_evidence_id": optionalUUID(row.SupersedesEvidenceID), "correction_lineage_depth": row.LineageDepth, "correction_lineage_truncated": row.LineageTruncated}
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"bid_id": uuid.UUID(result.BidID).String(), "current_proposal_version_id": optionalUUID(result.CurrentProposalVersionID), "report_count": result.ReportCount, "active_report_count": result.ActiveReportCount, "active_reports": items, "proof_policy_configured": result.ProofPolicyConfigured, "can_mark_submitted": result.CanMarkSubmitted, "submitted": result.Submitted, "blockers": result.Blockers, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ExternalActionEvidenceHandler) Get(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	evidenceID, ok := parseOpportunityID(c.Params("evidence_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The external action evidence request is invalid.", false)
	}
	result, err := h.service.Get(c.Context(), subject, bidID, evidenceID)
	if err != nil {
		return externalActionEvidenceFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": externalActionEvidenceView(result), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func externalActionEvidenceView(v application.ExternalActionEvidence) fiber.Map {
	return fiber.Map{"evidence_id": uuid.UUID(v.ID).String(), "bid_id": uuid.UUID(v.BidID).String(), "package_manifest_id": uuid.UUID(v.PackageManifestID).String(), "report_type": v.ReportType, "verification_status": v.VerificationStatus, "external_system_name": v.ExternalSystemName, "external_reference": v.ExternalReference, "reported_action_time_text": v.ReportedActionTimeText, "reported_action_at": v.ReportedActionAt.Format(time.RFC3339Nano), "report_note": v.ReportNote, "supersedes_evidence_id": optionalUUID(v.SupersedesEvidenceID), "recorded_by": uuid.UUID(v.RecordedBy).String(), "recorded_at": v.RecordedAt.Format(time.RFC3339Nano), "bid_state_changed": false, "submitted": false, "transmitted_by_bidora": false, "external_acceptance_confirmed": false}
}
func externalActionEvidenceFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "external_action_evidence_invalid", "external_action_evidence_read_invalid", "external_evidence_reconciliation_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The external action evidence request is invalid.", false)
	case "external_action_evidence_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized for this external action evidence operation.", false)
	case "external_action_evidence_not_found":
		return fail(c, 404, "EXTERNAL_ACTION_EVIDENCE_NOT_FOUND", "The external action evidence report was not found.", false)
	case "external_action_evidence_conflict":
		return fail(c, 409, "EXTERNAL_ACTION_EVIDENCE_CONFLICT", "The package, workspace, correction target or idempotency request changed.", false)
	case "external_evidence_reconciliation_limit":
		return fail(c, 409, "EXTERNAL_EVIDENCE_RECONCILIATION_LIMIT", "The active evidence set exceeds the reconciliation limit.", false)
	case "external_action_evidence_unavailable", "external_action_evidence_read_unavailable", "external_evidence_reconciliation_unavailable":
		return fail(c, 503, "EXTERNAL_ACTION_EVIDENCE_UNAVAILABLE", "External action evidence is temporarily unavailable.", true)
	default:
		return fail(c, 503, "EXTERNAL_ACTION_EVIDENCE_UNAVAILABLE", "External action evidence is temporarily unavailable.", true)
	}
}
