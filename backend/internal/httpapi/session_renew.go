package httpapi

import (
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
	"time"
)

const SessionRenewPath = "/v1/auth/session/renew"

// Renew is a CSRF/Origin-protected, empty-body activity signal. It extends only
// the existing server lease, never its absolute bound, token or assurance time.
func (m *BrowserSessionMiddleware) Renew(c fiber.Ctx) error {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "Session renewal accepts no body or query.", false)
	}
	return m.OriginBound(c)
}
func (m *BrowserSessionMiddleware) RenewResult(c fiber.Ctx) error {
	// Read back authoritative post-touch limits, including concurrent tightening
	// or revocation. Never calculate expiry from client clocks or reset assurance.
	p, e := m.validator.Validate(c.Context(), c.Cookies(session.CookieName))
	if e != nil {
		if e.Error() == "session_invalid" {
			return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		}
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"idle_expires_at": p.IdleExpiresAt.UTC().Format(time.RFC3339Nano), "absolute_expires_at": p.AbsoluteExpiresAt.UTC().Format(time.RFC3339Nano), "observed_at": m.clock.Now().UTC().Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
