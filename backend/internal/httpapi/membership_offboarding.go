package httpapi

import (
	authapp "bidora/backend/internal/authorization/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const MembershipOffboardingPath = "/v1/shared/memberships/:membership_id/offboarding"
const MembershipOffboardingOperationPath = "/v1/shared/membership-offboarding/:operation_id"
const MembershipOffboardingRetargetPath = "/v1/shared/membership-offboarding/:operation_id/retarget"
const MembershipOffboardingCancelPath = "/v1/shared/membership-offboarding/:operation_id/cancel"

type MembershipOffboardingUseCase interface {
	Begin(context.Context, authapp.MembershipOffboardingInput) (authapp.MembershipOffboardingOperation, error)
	Get(context.Context, authapp.ID, authapp.ID, authapp.ID, int64) (authapp.MembershipOffboardingOperation, error)
	Retarget(context.Context, authapp.MembershipOffboardingChange) (authapp.MembershipOffboardingOperation, error)
	Cancel(context.Context, authapp.MembershipOffboardingChange) (authapp.MembershipOffboardingOperation, error)
}
type MembershipOffboardingHandler struct{ s MembershipOffboardingUseCase }

func NewMembershipOffboardingHandler(s MembershipOffboardingUseCase) (*MembershipOffboardingHandler, error) {
	if s == nil {
		return nil, errors.New("membership_offboarding_handler_invalid")
	}
	return &MembershipOffboardingHandler{s}, nil
}

type beginOffboardingBody struct {
	SuccessorMembershipID           string `json:"successor_membership_id"`
	ExpectedTargetContextGeneration int64  `json:"expected_target_context_generation"`
	Reason                          string `json:"reason"`
}
type changeOffboardingBody struct {
	SuccessorMembershipID    string `json:"successor_membership_id,omitempty"`
	ExpectedOperationVersion int64  `json:"expected_operation_version"`
	Reason                   string `json:"reason"`
}

func (h *MembershipOffboardingHandler) Begin(c fiber.Ctx) error {
	var b beginOffboardingBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	x, ok := workflowCommon(c, "membership_id")
	if !ok {
		return nil
	}
	succ, ok := parseOpportunityID(b.SuccessorMembershipID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The offboarding request is invalid.", false)
	}
	p, _ := SessionPrincipalFrom(c)
	authAt := p.AbsoluteExpiresAt.UTC().Add(-12 * time.Hour)
	v, e := h.s.Begin(c.Context(), authapp.MembershipOffboardingInput{ActorID: authapp.ID(x.subject.IdentityID), TenantID: authapp.ID(x.subject.SelectedTenantID), TargetMembershipID: authapp.ID(x.opportunityID), SuccessorMembershipID: authapp.ID(succ), CorrelationID: authapp.ID(x.correlationID), ActorContextGeneration: x.subject.ContextGeneration, ExpectedTargetGeneration: b.ExpectedTargetContextGeneration, Reason: b.Reason, IdempotencyKey: x.idempotencyKey, SessionAuthenticatedAt: authAt, AssuranceLevel: p.AssuranceLevel, AssuranceMethod: p.AssuranceMethod, AssuranceAt: p.AssuredAt})
	if e != nil {
		return membershipOffboardingFailure(c, e)
	}
	if v.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	status := fiber.StatusAccepted
	if v.State == "COMPLETED" {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(fiber.Map{"data": membershipOffboardingView(v), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *MembershipOffboardingHandler) Get(c fiber.Ctx) error {
	sub, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	id, ok := parseOpportunityID(c.Params("operation_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	p, _ := SessionPrincipalFrom(c)
	v, e := h.s.Get(c.Context(), authapp.ID(p.UserID), authapp.ID(sub.SelectedTenantID), authapp.ID(id), sub.ContextGeneration)
	if e != nil {
		return membershipOffboardingFailure(c, e)
	}
	return c.JSON(fiber.Map{"data": membershipOffboardingView(v), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *MembershipOffboardingHandler) Retarget(c fiber.Ctx) error { return h.change(c, true) }
func (h *MembershipOffboardingHandler) Cancel(c fiber.Ctx) error   { return h.change(c, false) }
func (h *MembershipOffboardingHandler) change(c fiber.Ctx, retarget bool) error {
	var b changeOffboardingBody
	if e := bindWorkflow(c, &b); e != nil {
		return e
	}
	x, ok := workflowCommon(c, "operation_id")
	if !ok {
		return nil
	}
	var succ authapp.ID
	if retarget {
		id, yes := parseOpportunityID(b.SuccessorMembershipID)
		if !yes {
			return fail(c, 400, "INVALID_REQUEST", "The offboarding request is invalid.", false)
		}
		succ = authapp.ID(id)
	} else if b.SuccessorMembershipID != "" {
		return fail(c, 400, "INVALID_REQUEST", "The offboarding request is invalid.", false)
	}
	p, _ := SessionPrincipalFrom(c)
	in := authapp.MembershipOffboardingChange{ActorID: authapp.ID(p.UserID), TenantID: authapp.ID(x.subject.SelectedTenantID), OperationID: authapp.ID(x.opportunityID), SuccessorMembershipID: succ, CorrelationID: authapp.ID(x.correlationID), ActorContextGeneration: x.subject.ContextGeneration, ExpectedVersion: b.ExpectedOperationVersion, Reason: b.Reason, IdempotencyKey: x.idempotencyKey, SessionAuthenticatedAt: p.AbsoluteExpiresAt.UTC().Add(-12 * time.Hour), AssuranceLevel: p.AssuranceLevel, AssuranceMethod: p.AssuranceMethod, AssuranceAt: p.AssuredAt}
	var v authapp.MembershipOffboardingOperation
	var e error
	if retarget {
		v, e = h.s.Retarget(c.Context(), in)
	} else {
		v, e = h.s.Cancel(c.Context(), in)
	}
	if e != nil {
		return membershipOffboardingFailure(c, e)
	}
	return c.JSON(fiber.Map{"data": membershipOffboardingView(v), "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func membershipOffboardingView(v authapp.MembershipOffboardingOperation) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "target_membership_id": uuid.UUID(v.TargetMembershipID).String(), "successor_membership_id": uuid.UUID(v.SuccessorMembershipID).String(), "state": v.State, "version": v.Version, "assignment_generation": v.AssignmentGeneration, "reason": v.Reason, "workspace_count": v.WorkspaceCount, "requirement_count": v.RequirementCount, "review_count": v.ReviewCount, "started_at": v.StartedAt.Format(time.RFC3339Nano), "tenant_transferred_at": formatOptionalTime(v.TenantTransferredAt), "completed_at": formatOptionalTime(v.CompletedAt), "cancelled_at": formatOptionalTime(v.CancelledAt), "automatic_successor": false, "restoration_supported": false, "platform_override": false}
}
func membershipOffboardingFailure(c fiber.Ctx, e error) error {
	switch e.Error() {
	case "membership_offboarding_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The offboarding request is invalid.", false)
	case "membership_offboarding_not_found":
		return fail(c, 404, "OFFBOARDING_NOT_FOUND", "The offboarding operation was not found.", false)
	case "membership_last_admin":
		return fail(c, 409, "LAST_ADMIN_BLOCKED", "The final continuity administrator cannot be removed.", false)
	case "membership_transfer_limit":
		return fail(c, 409, "TRANSFER_LIMIT_EXCEEDED", "The active workload exceeds the pilot transfer limit.", false)
	case "membership_offboarding_conflict":
		return fail(c, 409, "OFFBOARDING_CONFLICT", "Membership or ownership facts changed.", false)
	default:
		return fail(c, 503, "OFFBOARDING_UNAVAILABLE", "Membership offboarding is temporarily unavailable.", true)
	}
}
