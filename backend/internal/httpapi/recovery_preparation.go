package httpapi

import (
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"time"
)

const RecoveryPreparationPath = "/v1/auth/recovery/preparation"

type RecoveryPreparationUseCase interface {
	Get(context.Context, identityapp.SessionPrincipal) (identityapp.RecoveryPreparation, error)
}
type RecoveryPreparationHandler struct {
	service    RecoveryPreparationUseCase
	protection *browser.Protection
}

func NewRecoveryPreparationHandler(service RecoveryPreparationUseCase, protection *browser.Protection) (*RecoveryPreparationHandler, error) {
	if service == nil || protection == nil {
		return nil, errors.New("recovery_preparation_handler_invalid")
	}
	return &RecoveryPreparationHandler{service: service, protection: protection}, nil
}
func (h *RecoveryPreparationHandler) Get(c fiber.Ctx) error {
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
		return recoveryPreparationFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"email_verified": result.Facts.EmailVerified, "account_disabled": result.Facts.AccountDisabled, "password_credential_present": result.Facts.PasswordCredentialPresent, "google_identity_present": result.Facts.GoogleIdentityPresent, "recovery_request_available": result.RecoveryRequestAvailable, "password_reset_available": result.PasswordResetAvailable, "token_policy_configured": result.TokenPolicyConfigured, "delivery_policy_configured": result.DeliveryPolicyConfigured, "session_effect_policy_configured": result.SessionEffectPolicyConfigured, "unauthenticated_account_lookup_available": false, "blockers": result.Blockers, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func recoveryPreparationFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "recovery_preparation_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The recovery preparation request is invalid.", false)
	case "recovery_preparation_not_found":
		return fail(c, 404, "RECOVERY_PREPARATION_NOT_FOUND", "The recovery preparation was not found.", false)
	case "recovery_preparation_unavailable":
		return fail(c, 503, "RECOVERY_PREPARATION_UNAVAILABLE", "Recovery preparation is temporarily unavailable.", true)
	default:
		return fail(c, 503, "RECOVERY_PREPARATION_UNAVAILABLE", "Recovery preparation is temporarily unavailable.", true)
	}
}
