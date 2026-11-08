package httpapi

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const ProposalApprovalPath = "/v1/tenderpreneur/bids/:bid_id/proposal/package-preparations/:manifest_id/approval"

type ProposalApprovalUseCase interface {
	Approve(context.Context, application.ApproveProposalInput) (application.ProposalApprovalResult, error)
	Get(context.Context, application.WorkflowSubject, application.ID, application.ID) (application.ProposalApproval, error)
}
type ProposalApprovalHandler struct{ service ProposalApprovalUseCase }

func NewProposalApprovalHandler(s ProposalApprovalUseCase) (*ProposalApprovalHandler, error) {
	if s == nil {
		return nil, errors.New("proposal_approval_handler_invalid")
	}
	return &ProposalApprovalHandler{s}, nil
}

type proposalApprovalBody struct {
	ExpectedProposalVersion  int64  `json:"expected_proposal_version"`
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
	Rationale                string `json:"rationale"`
}

func (h *ProposalApprovalHandler) Approve(c fiber.Ctx) error {
	var body proposalApprovalBody
	if e := bindWorkflow(c, &body); e != nil {
		return e
	}
	common, ok := workflowCommon(c, "bid_id")
	if !ok {
		return nil
	}
	manifest, ok := parseOpportunityID(c.Params("manifest_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The proposal approval request is invalid.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	authenticatedAt := principal.AuthenticatedAt.UTC()
	result, e := h.service.Approve(c.Context(), application.ApproveProposalInput{Subject: common.subject, BidID: common.opportunityID, ManifestID: manifest, CorrelationID: common.correlationID, ExpectedProposalVersion: body.ExpectedProposalVersion, ExpectedWorkspaceVersion: body.ExpectedWorkspaceVersion, IdempotencyKey: common.idempotencyKey, Rationale: body.Rationale, SessionAuthenticatedAt: authenticatedAt, AssuranceLevel: principal.AssuranceLevel, AssuranceMethod: principal.AssuranceMethod, AssuranceAt: principal.AssuredAt})
	if e != nil {
		return proposalApprovalFailure(c, e)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Status(201)
	return c.JSON(fiber.Map{"data": proposalApprovalView(result.Approval), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *ProposalApprovalHandler) Get(c fiber.Ctx) error {
	subject, bidID, ok := proposalReadCommon(c)
	if !ok {
		return nil
	}
	manifest, ok := parseOpportunityID(c.Params("manifest_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The proposal approval request is invalid.", false)
	}
	v, e := h.service.Get(c.Context(), subject, bidID, manifest)
	if e != nil {
		return proposalApprovalFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": proposalApprovalView(v), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func proposalApprovalView(v application.ProposalApproval) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "bid_id": uuid.UUID(v.BidID).String(), "proposal_version_id": uuid.UUID(v.ProposalVersionID).String(), "package_manifest_id": uuid.UUID(v.ManifestID).String(), "review_decision_id": uuid.UUID(v.ReviewDecisionID).String(), "approver_id": uuid.UUID(v.ApproverID).String(), "policy_version": v.PolicyVersion, "rationale": v.Rationale, "approved_at": v.ApprovedAt.Format(time.RFC3339Nano), "approved": true, "current_applicability_recheck_required": true, "signed": false, "submitted": false}
}
func proposalApprovalFailure(c fiber.Ctx, e error) error {
	switch e.Error() {
	case "proposal_approval_invalid", "proposal_approval_read_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The proposal approval request is invalid.", false)
	case "proposal_approval_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "You are not authorized to approve this proposal package.", false)
	case "proposal_approval_fresh_auth_required":
		return fail(c, 403, "FRESH_AUTHENTICATION_REQUIRED", "Sign in again before approving this proposal package.", false)
	case "proposal_approval_not_found":
		return fail(c, 404, "PROPOSAL_APPROVAL_NOT_FOUND", "The proposal approval record was not found.", false)
	case "proposal_approval_conflict":
		return fail(c, 409, "PROPOSAL_APPROVAL_CONFLICT", "The package, proposal, review or workspace changed.", false)
	case "proposal_approval_blocked":
		return fail(c, 409, "PROPOSAL_APPROVAL_BLOCKED", "The proposal is not ready for approval.", false)
	default:
		return fail(c, 503, "PROPOSAL_APPROVAL_UNAVAILABLE", "Proposal approval is temporarily unavailable.", true)
	}
}
