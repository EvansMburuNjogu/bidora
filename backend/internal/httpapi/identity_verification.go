package httpapi

import (
	"errors"

	"bidora/backend/internal/health"
	"github.com/gofiber/fiber/v3"
)

// NewIdentityVerificationApp mounts only the registration verification boundary.
// The full pilot continues to use NewPilot and its complete dependency checks.
func NewIdentityVerificationApp(state *health.State, login *LoginHandler, registration *RegistrationHandler, verification *EmailVerificationHandler) (*fiber.App, error) {
	if state == nil || login == nil || registration == nil || verification == nil {
		return nil, errors.New("identity_verification_routes_invalid")
	}
	app := newBase(4096)
	ready := func(c fiber.Ctx) error {
		if !state.Ready() {
			return fail(c, 503, "SERVICE_NOT_READY", "The service is not ready to accept requests.", true)
		}
		return c.Next()
	}
	app.Post(LoginPath, ready, login.Handle)
	app.Post(RegistrationPath, ready, registration.Handle)
	app.Post(EmailVerificationPath, ready, verification.Handle)
	return app, nil
}
