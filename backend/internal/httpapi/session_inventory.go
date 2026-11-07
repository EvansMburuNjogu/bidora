package httpapi

import (
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"time"
)

const SessionInventoryPath = "/v1/auth/sessions"

type SessionInventoryUseCase interface {
	Get(context.Context, identityapp.SessionPrincipal) (identityapp.SessionInventory, error)
}
type SessionInventoryHandler struct {
	service    SessionInventoryUseCase
	protection *browser.Protection
}

func NewSessionInventoryHandler(service SessionInventoryUseCase, protection *browser.Protection) (*SessionInventoryHandler, error) {
	if service == nil || protection == nil {
		return nil, errors.New("session_inventory_handler_invalid")
	}
	return &SessionInventoryHandler{service: service, protection: protection}, nil
}
func (h *SessionInventoryHandler) Get(c fiber.Ctx) error {
	if len(c.Body()) > 0 || len(c.Request().URI().QueryString()) > 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if err := h.protection.ValidateBootstrap(c.Get("Origin"), c.Cookies(session.CSRFCookieName), principal.CSRFHash); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	result, err := h.service.Get(c.Context(), principal)
	if err != nil {
		return sessionInventoryFailure(c, err)
	}
	items := make([]fiber.Map, len(result.Sessions))
	for i, row := range result.Sessions {
		items[i] = fiber.Map{"session_id": uuid.UUID(row.SessionID).String(), "rotated_from_session_id": optionalIdentityUUID(row.RotatedFromSessionID), "current": row.Current, "created_at": row.CreatedAt.Format(time.RFC3339Nano), "last_seen_at": row.LastSeenAt.Format(time.RFC3339Nano), "idle_expires_at": row.IdleExpiresAt.Format(time.RFC3339Nano), "absolute_expires_at": row.AbsoluteExpiresAt.Format(time.RFC3339Nano)}
	}
	// Empty collections must remain JSON arrays for strict session clients.
	blockers := result.Blockers
	if blockers == nil {
		blockers = []string{}
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"active_session_count": result.ActiveSessionCount, "other_session_count": result.OtherSessionCount, "sessions": items, "current_session_logout_available": result.CurrentSessionLogoutAvailable, "other_session_revocation_available": result.OtherSessionRevocationAvailable, "revoke_all_available": result.RevokeAllAvailable, "revocation_policy_configured": result.RevocationPolicyConfigured, "device_metadata_collected": false, "blockers": blockers, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func optionalIdentityUUID(id identityapp.ID) any {
	if id.IsZero() {
		return nil
	}
	return uuid.UUID(id).String()
}
func sessionInventoryFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "session_inventory_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The session inventory request is invalid.", false)
	case "session_inventory_limit":
		return fail(c, 409, "SESSION_INVENTORY_LIMIT", "The active session count exceeds the bounded inventory limit.", false)
	case "session_inventory_unavailable":
		return fail(c, 503, "SESSION_INVENTORY_UNAVAILABLE", "Session inventory is temporarily unavailable.", true)
	default:
		return fail(c, 503, "SESSION_INVENTORY_UNAVAILABLE", "Session inventory is temporarily unavailable.", true)
	}
}
