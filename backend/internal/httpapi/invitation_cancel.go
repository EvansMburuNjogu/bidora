package httpapi

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	"github.com/gofiber/fiber/v3"
)

type InvitationCancellationUseCase interface {
	Cancel(context.Context, authzapp.InvitationCancellationInput) error
}
type InvitationCancellationHandler struct{ cancellation InvitationCancellationUseCase }

func NewInvitationCancellationHandler(cancellation InvitationCancellationUseCase) (*InvitationCancellationHandler, error) {
	if cancellation == nil {
		return nil, errors.New("invitation_cancellation_handler_invalid")
	}
	return &InvitationCancellationHandler{cancellation}, nil
}
func (h *InvitationCancellationHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("invitation_cancellation_http_context_required")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.cancellation == nil {
		return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	invitationID, ok := parseOpportunityID(c.Params("invitation_id"))
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
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	err := h.cancellation.Cancel(c.Context(), authzapp.InvitationCancellationInput{ActorID: authzapp.ID(principal.UserID), TenantID: authzapp.ID(tenantID), InvitationID: authzapp.ID(invitationID), CorrelationID: authzapp.ID(correlation), ContextGeneration: generation})
	if err != nil {
		if err.Error() == "invitation_cancellation_not_found" {
			return fail(c, 404, "INVITATION_NOT_FOUND", "The invitation was not found.", false)
		}
		if err.Error() == "invitation_cancellation_invalid" {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		return fail(c, 503, "INVITATION_UNAVAILABLE", "Invitation is temporarily unavailable.", true)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
