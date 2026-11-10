package httpapi

import (
	"context"
	"errors"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type SessionValidator interface {
	Validate(context.Context, string) (identityapp.SessionPrincipal, error)
}
type SessionToucher interface {
	TouchSession(context.Context, identityapp.ID, time.Time) error
}
type BrowserClock interface{ Now() time.Time }
type BrowserSessionMiddleware struct {
	auditIPs         LoginClientIPResolver
	securityPolicy   SecurityPolicyResolver
	operationalGate  func(context.Context, string) error
	metadataObserver func(fiber.Ctx, identityapp.SessionPrincipal)
	validator        SessionValidator
	toucher          SessionToucher
	protection       *browser.Protection
	clock            BrowserClock
}

func NewBrowserSessionMiddleware(validator SessionValidator, toucher SessionToucher, protection *browser.Protection, clock BrowserClock) (*BrowserSessionMiddleware, error) {
	if validator == nil || toucher == nil || protection == nil || clock == nil {
		return nil, errors.New("browser_session_middleware_invalid")
	}
	return &BrowserSessionMiddleware{validator: validator, toucher: toucher, protection: protection, clock: clock}, nil
}

// ConfigureOperationalHooks adds owner-composed feature/maintenance checks and
// authenticated request metadata observation; authentication remains unchanged.
func (m *BrowserSessionMiddleware) ConfigureOperationalHooks(gate func(context.Context, string) error, observe func(fiber.Ctx, identityapp.SessionPrincipal)) {
	m.operationalGate = gate
	m.metadataObserver = observe
}

type principalKey struct{}

func SessionPrincipalFrom(c fiber.Ctx) (identityapp.SessionPrincipal, bool) {
	if c == nil {
		return identityapp.SessionPrincipal{}, false
	}
	principal, ok := c.Locals(principalKey{}).(identityapp.SessionPrincipal)
	return principal, ok && !principal.SessionID.IsZero() && !principal.UserID.IsZero()
}
func (m *BrowserSessionMiddleware) Handle(c fiber.Ctx) error    { return m.handle(c, false, browser.RequiresCSRF(c.Method())) }
func (m *BrowserSessionMiddleware) Bootstrap(c fiber.Ctx) error { return m.handle(c, true, false) }
func (m *BrowserSessionMiddleware) NoTouch(c fiber.Ctx) error   { return m.handle(c, false, false) }

// OriginBound validates the approved Origin for authenticated browser requests;
// unsafe requests still receive the ordinary session-bound CSRF validation.
func (m *BrowserSessionMiddleware) OriginBound(c fiber.Ctx) error {
	if c == nil {
		return errors.New("browser_context_required")
	}
	if m == nil || m.protection == nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if err := m.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	return m.handle(c, false, browser.RequiresCSRF(c.Method()))
}

// Platform retains the named Platform Administration middleware boundary.
func (m *BrowserSessionMiddleware) Platform(c fiber.Ctx) error { return m.OriginBound(c) }
func (m *BrowserSessionMiddleware) handle(c fiber.Ctx, bootstrap, touch bool) error {
	if c == nil {
		return errors.New("browser_context_required")
	}
	if m == nil || m.validator == nil || m.toucher == nil || m.protection == nil || m.clock == nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if bootstrap && (len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0) {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	principal, err := m.validator.Validate(c.Context(), c.Cookies(session.CookieName))
	if err != nil {
		if err.Error() == "session_invalid" {
			return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		}
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if principal.SessionID.IsZero() || principal.UserID.IsZero() || principal.CSRFHash == [32]byte{} || principal.AuthEpoch < 1 {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if bootstrap {
		if err = m.protection.ValidateBootstrap(c.Get("Origin"), c.Cookies(session.CSRFCookieName), principal.CSRFHash); err != nil {
			return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
		}
	} else if browser.RequiresCSRF(c.Method()) {
		if err = m.protection.ValidateUnsafe(c.Get("Origin"), c.Get(session.CSRFHeader), c.Cookies(session.CSRFCookieName), principal.CSRFHash); err != nil {
			return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
		}
	}
	observedAt := m.clock.Now().UTC()
	if observedAt.IsZero() {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if touch {
		if err = m.toucher.TouchSession(c.Context(), principal.SessionID, observedAt); err != nil {
			if errors.Is(err, identityapp.ErrSessionNotFound) {
				return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
			}
			return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		}
	}
	c.Locals(principalKey{}, principal)
	if m.operationalGate != nil {
		if e := m.operationalGate(c.Context(), c.Path()); e != nil {
			message := "This operation is temporarily unavailable. Please try again later."
			var public interface{ PublicMessage() string }
			if errors.As(e, &public) && public.PublicMessage() != "" {
				message = public.PublicMessage()
			}
			return fail(c, 503, "OPERATION_UNAVAILABLE", message, true)
		}
	}
	err = c.Next()
	if m.metadataObserver != nil {
		m.metadataObserver(c, principal)
	}
	return err
}
