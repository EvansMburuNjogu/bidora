package httpapi

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
	"github.com/gofiber/fiber/v3"
)

type MembershipRevocationUseCase interface {
	Revoke(context.Context, authzapp.MembershipRevocationInput) (authzapp.MembershipRevocationResult, error)
}
type MembershipRevocationContinuity interface {
	Get(context.Context, opportunityapp.WorkflowSubject) (opportunityapp.OwnershipContinuityReport, error)
}
type MembershipRevocationHandler struct {
	revocation MembershipRevocationUseCase
	continuity MembershipRevocationContinuity
}

func NewMembershipRevocationHandler(revocation MembershipRevocationUseCase) (*MembershipRevocationHandler, error) {
	if revocation == nil {
		return nil, errors.New("membership_revocation_handler_invalid")
	}
	return &MembershipRevocationHandler{revocation: revocation}, nil
}
func NewMembershipRevocationHandlerWithContinuity(revocation MembershipRevocationUseCase, continuity MembershipRevocationContinuity) (*MembershipRevocationHandler, error) {
	if revocation == nil || continuity == nil {
		return nil, errors.New("membership_revocation_handler_invalid")
	}
	return &MembershipRevocationHandler{revocation: revocation, continuity: continuity}, nil
}
func (h *MembershipRevocationHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("membership_revocation_http_context_required")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.revocation == nil {
		return fail(c, 503, "MEMBERSHIP_UNAVAILABLE", "Membership is temporarily unavailable.", true)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	targetText := c.Params("membership_id")
	targetID, ok := parseOpportunityID(targetText)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	tenantText, ok := singleHeader(c, TenantIDHeader)
	if !ok {
		return invalidHeader(c, TenantIDHeader, "required")
	}
	tenantID, ok := parseOpportunityID(tenantText)
	if !ok {
		return invalidHeader(c, TenantIDHeader, "uuid")
	}
	generationText, ok := singleHeader(c, ContextGenerationHeader)
	if !ok {
		return invalidHeader(c, ContextGenerationHeader, "required")
	}
	generation, ok := positiveInt64(generationText)
	if !ok {
		return invalidHeader(c, ContextGenerationHeader, "positive_integer")
	}
	if h.continuity != nil {
		report, err := h.continuity.Get(c.Context(), opportunityapp.WorkflowSubject{IdentityID: opportunityapp.ID(principal.UserID), SelectedTenantID: opportunityapp.ID(tenantID), ContextGeneration: generation})
		if err != nil {
			return fail(c, 503, "MEMBERSHIP_UNAVAILABLE", "Membership is temporarily unavailable.", true)
		}
		found := false
		for _, member := range report.Members {
			if member.MembershipID == opportunityapp.ID(targetID) {
				found = true
				if member.OwnedBidCount > 0 || member.OpenRequirementCount > 0 || member.OpenReviewCount > 0 {
					return fail(c, 409, "OFFBOARDING_REQUIRED", "Transfer active work through membership offboarding before revocation.", false)
				}
				break
			}
		}
		if !found {
			return fail(c, 404, "MEMBERSHIP_NOT_FOUND", "The membership was not found.", false)
		}
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.revocation.Revoke(c.Context(), authzapp.MembershipRevocationInput{ActorID: authzapp.ID(principal.UserID), TenantID: authzapp.ID(tenantID), TargetMembershipID: authzapp.ID(targetID), CorrelationID: authzapp.ID(correlation), ActorContextGeneration: generation})
	if err != nil {
		if err.Error() == "membership_revocation_not_authorized" {
			return fail(c, 404, "MEMBERSHIP_NOT_FOUND", "The membership was not found.", false)
		}
		if err.Error() == "membership_revocation_invalid" {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		return fail(c, 503, "MEMBERSHIP_UNAVAILABLE", "Membership is temporarily unavailable.", true)
	}
	if result.MembershipID.IsZero() || result.TenantID.IsZero() || result.ContextGeneration < 1 {
		return fail(c, 503, "MEMBERSHIP_UNAVAILABLE", "Membership is temporarily unavailable.", true)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
