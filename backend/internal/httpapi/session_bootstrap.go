package httpapi

import (
	"errors"

	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type SessionBootstrapHandler struct{ protection *browser.Protection }

func NewSessionBootstrapHandler(protection *browser.Protection) (*SessionBootstrapHandler, error) {
	if protection == nil {
		return nil, errors.New("session_bootstrap_handler_invalid")
	}
	return &SessionBootstrapHandler{protection}, nil
}

type sessionBootstrapData struct {
	CSRFToken string `json:"csrf_token"`
}
type sessionBootstrapResponse struct {
	Data sessionBootstrapData `json:"data"`
	Meta meta                 `json:"meta"`
}

func (h *SessionBootstrapHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("browser_context_required")
	}
	if h == nil || h.protection == nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	raw := c.Cookies(session.CSRFCookieName)
	if err := h.protection.ValidateBootstrap(c.Get("Origin"), raw, principal.CSRFHash); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	return c.Status(fiber.StatusOK).JSON(sessionBootstrapResponse{Data: sessionBootstrapData{CSRFToken: raw}, Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
