package httpapi

import (
	"errors"
	"net/netip"
	"strings"

	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/postgres"
	"github.com/gofiber/fiber/v3"
)

func (m *BrowserSessionMiddleware) ConfigureRequestAudit(ips LoginClientIPResolver) error {
	if m == nil || ips == nil {
		return errors.New("audit_request_resolver_invalid")
	}
	m.auditIPs = ips
	return nil
}

// CaptureAuditContext runs BEFORE auth and Platform handlers, including the
// email-code completion route. The transaction trigger gets the actual actor
// from the committed audit event, not from an unverified cookie or request body.
func (m *BrowserSessionMiddleware) CaptureAuditContext(c fiber.Ctx) error {
	if m == nil || m.auditIPs == nil {
		return c.Next()
	} // Uncomposed test applications.
	path := strings.ToLower(c.Path()) // Fiber routes are case-insensitive by default.
	if !strings.HasPrefix(path, "/v1/auth/") && !strings.HasPrefix(path, "/v1/platform/") {
		return c.Next()
	}
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "AUDIT_CONTEXT_UNAVAILABLE", "Request information is unavailable. Please try again.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "AUDIT_CONTEXT_UNAVAILABLE", "Request information is unavailable. Please try again.", true)
	}
	ip, err := m.auditIPs.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "AUDIT_CONTEXT_UNAVAILABLE", "Request information is unavailable. Please try again.", true)
	}
	name := browser.Name(c.Get("User-Agent"))
	if name == "" {
		name = "Unknown browser"
	}
	ctx, err := postgres.WithRequestAudit(c.Context(), ip, name)
	if err != nil {
		return fail(c, 503, "AUDIT_CONTEXT_UNAVAILABLE", "Request information is unavailable. Please try again.", true)
	}
	c.SetContext(ctx)
	return c.Next()
}
