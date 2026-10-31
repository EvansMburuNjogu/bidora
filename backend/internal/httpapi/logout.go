package httpapi

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

type LogoutUseCase interface {
	Logout(context.Context, identityapp.SessionPrincipal, identityapp.ID) error
}
type LogoutHandler struct{ logout LogoutUseCase }

func NewLogoutHandler(logout LogoutUseCase) (*LogoutHandler, error) {
	if logout == nil {
		return nil, errors.New("logout_handler_invalid")
	}
	return &LogoutHandler{logout}, nil
}
func (h *LogoutHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("logout_http_context_required")
	}
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.logout == nil {
		return fail(c, 503, "LOGOUT_UNAVAILABLE", "Logout is temporarily unavailable.", true)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	correlationID, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	if err := h.logout.Logout(c.Context(), principal, correlationID); err != nil {
		return fail(c, 503, "LOGOUT_UNAVAILABLE", "Logout is temporarily unavailable.", true)
	}
	setLoginCookie(c, browser.ClearSessionCookie())
	setLoginCookie(c, browser.ClearCSRFCookie())
	return c.SendStatus(fiber.StatusNoContent)
}
