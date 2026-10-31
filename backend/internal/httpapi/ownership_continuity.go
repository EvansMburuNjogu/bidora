package httpapi

import (
	"bidora/backend/internal/opportunity/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const OwnershipContinuityPath = "/v1/tenderpreneur/organization/ownership-continuity"

type OwnershipContinuityUseCase interface {
	Get(context.Context, application.WorkflowSubject) (application.OwnershipContinuityReport, error)
}
type OwnershipContinuityHandler struct{ service OwnershipContinuityUseCase }

func NewOwnershipContinuityHandler(service OwnershipContinuityUseCase) (*OwnershipContinuityHandler, error) {
	if service == nil {
		return nil, errors.New("ownership_continuity_handler_invalid")
	}
	return &OwnershipContinuityHandler{service}, nil
}
func (h *OwnershipContinuityHandler) Get(c fiber.Ctx) error {
	subject, ok := readSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) > 0 {
		return fail(c, 400, "INVALID_REQUEST", "The ownership continuity request is invalid.", false)
	}
	result, err := h.service.Get(c.Context(), subject)
	if err != nil {
		return ownershipContinuityFailure(c, err)
	}
	members := make([]fiber.Map, len(result.Members))
	for i, row := range result.Members {
		members[i] = fiber.Map{"membership_id": uuid.UUID(row.MembershipID).String(), "user_id": uuid.UUID(row.UserID).String(), "context_generation": row.ContextGeneration, "owned_bid_count": row.OwnedBidCount, "open_requirement_count": row.OpenRequirementCount, "open_review_count": row.OpenReviewCount, "overdue_review_count": row.OverdueReviewCount, "last_collaboration_activity_at": formatOptionalTime(row.LastActivityAt)}
	}
	orphaned := make([]fiber.Map, len(result.OrphanedOwnership))
	for i, row := range result.OrphanedOwnership {
		orphaned[i] = fiber.Map{"user_id": uuid.UUID(row.UserID).String(), "owned_bid_count": row.OwnedBidCount, "open_requirement_count": row.OpenRequirementCount, "open_review_count": row.OpenReviewCount, "overdue_review_count": row.OverdueReviewCount, "last_collaboration_activity_at": formatOptionalTime(row.LastActivityAt)}
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"tenant_id": uuid.UUID(result.TenantID).String(), "assignment_generation": result.AssignmentGeneration, "active_member_count": result.MemberCount, "members": members, "orphaned_ownership": orphaned, "can_revoke_without_review": result.CanRevokeWithoutReview, "automatic_transfer_supported": result.AutomaticTransferSupported, "explicit_offboarding_supported": result.ExplicitOffboardingSupported, "last_active_admin_policy_configured": result.LastActiveAdminPolicyConfigured, "blockers": result.Blockers, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func ownershipContinuityFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "ownership_continuity_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The ownership continuity request is invalid.", false)
	case "ownership_continuity_not_authorized":
		return fail(c, 403, "NOT_AUTHORIZED", "Tenant membership-management authority is required.", false)
	case "ownership_continuity_limit":
		return fail(c, 409, "OWNERSHIP_CONTINUITY_LIMIT", "The member or ownership set exceeds the bounded reconciliation limit.", false)
	case "ownership_continuity_unavailable":
		return fail(c, 503, "OWNERSHIP_CONTINUITY_UNAVAILABLE", "Ownership continuity is temporarily unavailable.", true)
	default:
		return fail(c, 503, "OWNERSHIP_CONTINUITY_UNAVAILABLE", "Ownership continuity is temporarily unavailable.", true)
	}
}
