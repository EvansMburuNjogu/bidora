package httpapi

import (
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const SessionRevokePath = "/v1/auth/sessions/:session_id"
const SessionRevokeAllPath = "/v1/auth/sessions/revoke-all"

type SessionAdministrationUseCase interface {
	RevokeOther(context.Context, app.RevokeOtherSessionInput) (app.RevokeOtherSessionResult, error)
	RevokeAll(context.Context, app.SessionPrincipal, app.ID, time.Time) (app.RevokeAllSessionsResult, error)
}
type SessionAdministrationHandler struct{ s SessionAdministrationUseCase }

func NewSessionAdministrationHandler(s SessionAdministrationUseCase) (*SessionAdministrationHandler, error) {
	if s == nil {
		return nil, errors.New("session_administration_handler_invalid")
	}
	return &SessionAdministrationHandler{s}, nil
}
func (h *SessionAdministrationHandler) RevokeOther(c fiber.Ctx) error {
	if !emptySessionAdminRequest(c) {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	p, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	target, ok := parseIdentityID(c.Params("session_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		return invalidHeader(c, IdempotencyKeyHeader, "required")
	}
	if _, e := uuid.Parse(key); e != nil {
		return invalidHeader(c, IdempotencyKeyHeader, "uuid")
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	v, e := h.s.RevokeOther(c.Context(), app.RevokeOtherSessionInput{Principal: p, TargetSessionID: app.ID(target), CorrelationID: correlation, IdempotencyKey: key, SessionAuthenticatedAt: p.AbsoluteExpiresAt.UTC().Add(-12 * time.Hour)})
	if e != nil {
		return sessionAdministrationFailure(c, e)
	}
	if v.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.SendStatus(204)
}
func (h *SessionAdministrationHandler) RevokeAll(c fiber.Ctx) error {
	if !emptySessionAdminRequest(c) {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	p, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	if _, e := h.s.RevokeAll(c.Context(), p, correlation, p.AbsoluteExpiresAt.UTC().Add(-12*time.Hour)); e != nil {
		return sessionAdministrationFailure(c, e)
	}
	setLoginCookie(c, browser.ClearSessionCookie())
	setLoginCookie(c, browser.ClearCSRFCookie())
	c.Set("Cache-Control", "private, no-store")
	return c.SendStatus(204)
}
func emptySessionAdminRequest(c fiber.Ctx) bool {
	return c != nil && len(c.Body()) == 0 && len(c.Request().URI().QueryString()) == 0
}
func sessionAdministrationFailure(c fiber.Ctx, e error) error {
	switch e.Error() {
	case "session_administration_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The session administration request is invalid.", false)
	case "session_administration_fresh_auth_required":
		return fail(c, 403, "FRESH_AUTHENTICATION_REQUIRED", "Sign in again before administering sessions.", false)
	case "session_administration_current_requires_logout":
		return fail(c, 409, "CURRENT_SESSION_REQUIRES_LOGOUT", "Use the logout operation for the current session.", false)
	case "session_administration_conflict":
		return fail(c, 409, "IDEMPOTENCY_CONFLICT", "The idempotency key was already used for another request.", false)
	case "session_administration_limit":
		return fail(c, 409, "SESSION_ADMINISTRATION_LIMIT", "The active session count exceeds the bounded limit.", false)
	default:
		return fail(c, 503, "SESSION_ADMINISTRATION_UNAVAILABLE", "Session administration is temporarily unavailable.", true)
	}
}
