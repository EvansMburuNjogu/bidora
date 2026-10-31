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

const MFAReadinessPath = "/v1/auth/mfa/preparation"

type MFAReadinessUseCase interface {
	Get(context.Context, identityapp.SessionPrincipal) (identityapp.MFAReadiness, error)
}
type MFAReadinessHandler struct {
	service    MFAReadinessUseCase
	protection *browser.Protection
}

func NewMFAReadinessHandler(service MFAReadinessUseCase, protection *browser.Protection) (*MFAReadinessHandler, error) {
	if service == nil || protection == nil {
		return nil, errors.New("mfa_readiness_handler_invalid")
	}
	return &MFAReadinessHandler{service, protection}, nil
}
func (h *MFAReadinessHandler) Get(c fiber.Ctx) error {
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
		return mfaReadinessFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"email_verified": result.Facts.EmailVerified, "account_disabled": result.Facts.AccountDisabled, "password_credential_present": result.Facts.PasswordCredentialPresent, "google_identity_present": result.Facts.GoogleIdentityPresent, "mfa_enrollment_available": result.MFAEnrollmentAvailable, "mfa_enrolled": result.MFAEnrolled, "step_up_available": result.StepUpAvailable, "step_up_policy_configured": result.StepUpPolicyConfigured, "session_assurance_provenance_available": result.SessionAssuranceProvenanceAvailable, "recovery_codes_available": result.RecoveryCodesAvailable, "session_authenticated_at": result.SessionAuthenticatedAt.Format(time.RFC3339Nano), "session_age_seconds": result.SessionAgeSeconds, "blockers": result.Blockers, "observed_at": result.ObservedAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func mfaReadinessFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "mfa_readiness_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The MFA readiness request is invalid.", false)
	case "mfa_readiness_not_found":
		return fail(c, 404, "MFA_READINESS_NOT_FOUND", "The MFA readiness assessment was not found.", false)
	case "mfa_readiness_unavailable":
		return fail(c, 503, "MFA_READINESS_UNAVAILABLE", "MFA readiness is temporarily unavailable.", true)
	default:
		return fail(c, 503, "MFA_READINESS_UNAVAILABLE", "MFA readiness is temporarily unavailable.", true)
	}
}
