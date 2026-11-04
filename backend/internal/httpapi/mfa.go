package httpapi

import (
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"net/netip"
	"time"
)

const MFAEnrollPath = "/v1/auth/mfa/totp/enroll"
const MFAConfirmPath = "/v1/auth/mfa/totp/confirm"
const MFAStepUpPath = "/v1/auth/mfa/step-up"
const MFAChallengePath = "/v1/auth/login-code/verify"
const EmailLoginResendPath = "/v1/auth/login-code/resend"
const EmailLoginCancelPath = "/v1/auth/login-code/cancel"
const MFARecoveryRegeneratePath = "/v1/auth/mfa/recovery-codes/regenerate"
const MFAReplacePath = "/v1/auth/mfa/totp/replace"

type MFAUseCase interface {
	Enroll(context.Context, app.SessionPrincipal, app.ID) (app.MFAEnrollment, error)
	Confirm(context.Context, app.SessionPrincipal, app.ID, string) (app.MFAActivationResult, error)
	StepUp(context.Context, app.SessionPrincipal, app.ID, string, string) (app.LoginResult, error)
	CompleteLogin(context.Context, string, string, string, app.ID) (app.LoginResult, error)
	ResendLogin(context.Context, string, [32]byte) (app.MFALoginGateResult, error)
	CancelLogin(context.Context, string) error
	RegenerateRecoveryCodes(context.Context, app.SessionPrincipal, app.ID) ([]string, error)
	BeginReplacement(context.Context, app.SessionPrincipal, app.ID) (app.MFAEnrollment, error)
	ConfirmReplacement(context.Context, app.SessionPrincipal, app.ID, string) (app.MFAActivationResult, error)
}
type EmailLoginResendNetworkKeyer interface {
	EmailLoginResendNetwork(netip.Addr) ([32]byte, error)
}
type MFAHandler struct {
	s          MFAUseCase
	protection *browser.Protection
	ips        LoginClientIPResolver
	resendKeys EmailLoginResendNetworkKeyer
}

func NewMFAHandler(s MFAUseCase, p *browser.Protection) (*MFAHandler, error) {
	if s == nil || p == nil {
		return nil, errors.New("mfa_handler_invalid")
	}
	return &MFAHandler{s: s, protection: p}, nil
}
func (h *MFAHandler) EnableEmailLoginResend(ips LoginClientIPResolver, keys EmailLoginResendNetworkKeyer) error {
	if h == nil || ips == nil || keys == nil {
		return errors.New("email_login_resend_handler_invalid")
	}
	h.ips = ips
	h.resendKeys = keys
	return nil
}

type mfaEmptyBody struct{}

func (h *MFAHandler) Enroll(c fiber.Ctx) error {
	var body mfaEmptyBody
	ok, err := bindMFAWorkflow(c, &body)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	p, cor, ok := mfaCommon(c)
	if !ok {
		return nil
	}
	v, e := h.s.Enroll(c.Context(), p, cor)
	if e != nil {
		return mfaFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"factor_id": uuid.UUID(v.FactorID).String(), "secret": v.Secret, "otpauth_uri": v.URI, "expires_at": v.ExpiresAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}

type mfaCodeBody struct {
	Code   string `json:"code"`
	Method string `json:"method,omitempty"`
}

func (h *MFAHandler) Confirm(c fiber.Ctx) error {
	var b mfaCodeBody
	ok, err := bindMFAWorkflow(c, &b)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if b.Method != "" {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	p, cor, ok := mfaCommon(c)
	if !ok {
		return nil
	}
	v, e := h.s.Confirm(c.Context(), p, cor, b.Code)
	if e != nil {
		return mfaFailure(c, e)
	}
	if e = setMFAResultCookies(c, v.Login); e != nil {
		return e
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"recovery_codes": v.RecoveryCodes, "assurance_level": "AAL2", "assurance_method": "TOTP"}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *MFAHandler) CancelLoginCode(c fiber.Ctx) error {
	if e := h.protection.ValidateOrigin(c.Get("Origin")); e != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	raw, _ := singleGoogleCookie(c.Get("Cookie"), "__Host-bidora_login_code")
	if e := h.s.CancelLogin(c.Context(), raw); e != nil {
		return fail(c, 503, "LOGIN_CODE_UNAVAILABLE", "Sign-in restart is temporarily unavailable.", true)
	}
	c.Cookie(&fiber.Cookie{Name: "__Host-bidora_login_code", Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), Secure: true, HTTPOnly: true, SameSite: "Lax"})
	c.Set("Cache-Control", "private, no-store")
	return c.SendStatus(204)
}
func (h *MFAHandler) ResendLoginCode(c fiber.Ctx) error {
	if e := h.protection.ValidateOrigin(c.Get("Origin")); e != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	var body mfaEmptyBody
	ok, err := bindMFAWorkflow(c, &body)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	raw, ok := singleGoogleCookie(c.Get("Cookie"), "__Host-bidora_login_code")
	if !ok {
		return fail(c, 401, "LOGIN_CODE_INVALID", "The login verification code is invalid.", false)
	}
	if h.ips == nil || h.resendKeys == nil {
		return fail(c, 503, "LOGIN_CODE_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
	}
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "LOGIN_CODE_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
	}
	peer, e := netip.ParseAddrPort(remote.String())
	if e != nil {
		return fail(c, 503, "LOGIN_CODE_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
	}
	clientIP, e := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if e != nil {
		return fail(c, 503, "LOGIN_CODE_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
	}
	networkScope, e := h.resendKeys.EmailLoginResendNetwork(clientIP)
	if e != nil || networkScope == ([32]byte{}) {
		return fail(c, 503, "LOGIN_CODE_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
	}
	result, e := h.s.ResendLogin(c.Context(), raw, networkScope)
	if e != nil {
		return emailLoginFailure(c, e)
	}
	c.Cookie(&fiber.Cookie{Name: "__Host-bidora_login_code", Value: result.Token, Path: "/", Expires: result.ExpiresAt.Add(app.EmailLoginResendGrace), Secure: true, HTTPOnly: true, SameSite: "Lax"})
	c.Set("X-Bidora-Challenge-Expires-At", result.ExpiresAt.UTC().Format(time.RFC3339Nano))
	c.Set("Cache-Control", "private, no-store")
	return c.SendStatus(202)
}
func (h *MFAHandler) Challenge(c fiber.Ctx) error {
	if e := h.protection.ValidateOrigin(c.Get("Origin")); e != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	var b mfaCodeBody
	ok, err := bindMFAWorkflow(c, &b)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	raw, ok := singleGoogleCookie(c.Get("Cookie"), "__Host-bidora_login_code")
	if !ok {
		return fail(c, 401, "LOGIN_CODE_INVALID", "The login verification code is invalid.", false)
	}
	cor, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	v, e := h.s.CompleteLogin(c.Context(), raw, b.Code, b.Method, cor)
	if e != nil {
		return emailLoginFailure(c, e)
	}
	if e = setMFAResultCookies(c, v); e != nil {
		return e
	}
	c.Cookie(&fiber.Cookie{Name: "__Host-bidora_login_code", Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0), Secure: true, HTTPOnly: true, SameSite: "Lax"})
	return c.SendStatus(204)
}
func (h *MFAHandler) Regenerate(c fiber.Ctx) error {
	var body mfaEmptyBody
	ok, err := bindMFAWorkflow(c, &body)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	p, cor, ok := mfaCommon(c)
	if !ok {
		return nil
	}
	codes, e := h.s.RegenerateRecoveryCodes(c.Context(), p, cor)
	if e != nil {
		return mfaFailure(c, e)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"recovery_codes": codes}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}

type mfaReplaceBody struct {
	Code string `json:"code,omitempty"`
}

func (h *MFAHandler) Replace(c fiber.Ctx) error {
	var b mfaReplaceBody
	ok, err := bindMFAWorkflow(c, &b)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	p, cor, ok := mfaCommon(c)
	if !ok {
		return nil
	}
	if b.Code == "" {
		v, e := h.s.BeginReplacement(c.Context(), p, cor)
		if e != nil {
			return mfaFailure(c, e)
		}
		c.Set("Cache-Control", "private, no-store")
		return c.Status(201).JSON(fiber.Map{"data": fiber.Map{"factor_id": uuid.UUID(v.FactorID).String(), "secret": v.Secret, "otpauth_uri": v.URI, "expires_at": v.ExpiresAt.Format(time.RFC3339Nano)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
	}
	v, e := h.s.ConfirmReplacement(c.Context(), p, cor, b.Code)
	if e != nil {
		return mfaFailure(c, e)
	}
	if e = setMFAResultCookies(c, v.Login); e != nil {
		return e
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{"recovery_codes": v.RecoveryCodes, "assurance_level": "AAL2", "assurance_method": "TOTP"}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *MFAHandler) StepUp(c fiber.Ctx) error {
	var b mfaCodeBody
	ok, err := bindMFAWorkflow(c, &b)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	p, cor, ok := mfaCommon(c)
	if !ok {
		return nil
	}
	v, e := h.s.StepUp(c.Context(), p, cor, b.Code, b.Method)
	if e != nil {
		return mfaChallengeFailure(c, e)
	}
	if e = setMFAResultCookies(c, v); e != nil {
		return e
	}
	return c.SendStatus(204)
}
func bindMFAWorkflow(c fiber.Ctx, target any) (bool, error) {
	if err := bindWorkflow(c, target); err != nil {
		return false, err
	}
	return c.Response().StatusCode() < 400, nil
}
func mfaCommon(c fiber.Ctx) (app.SessionPrincipal, app.ID, bool) {
	p, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return p, app.ID{}, false
	}
	cor, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		return p, app.ID{}, false
	}
	return p, cor, true
}
func setMFAResultCookies(c fiber.Ctx, v app.LoginResult) error {
	sc, e := browser.SessionCookie(v.Session.RevealForCookie(), v.AbsoluteExpiresAt)
	if e != nil {
		return fail(c, 503, "MFA_UNAVAILABLE", "MFA is temporarily unavailable.", true)
	}
	cc, e := browser.CSRFCookie(v.CSRF.RevealForBootstrap(), v.AbsoluteExpiresAt)
	if e != nil {
		return fail(c, 503, "MFA_UNAVAILABLE", "MFA is temporarily unavailable.", true)
	}
	setLoginCookie(c, sc)
	setLoginCookie(c, cc)
	return nil
}
func emailLoginFailure(c fiber.Ctx, e error) error {
	if e.Error() == "mfa_invalid" {
		return fail(c, 401, "LOGIN_CODE_INVALID", "The login verification code is invalid.", false)
	}
	if e.Error() == "email_login_resend_denied" {
		c.Set("Retry-After", "900")
		return fail(c, 429, "LOGIN_CODE_RESEND_LIMITED", "Another login verification code cannot be sent yet.", false)
	}
	if e.Error() == "mfa_attempt_denied" {
		c.Set("Retry-After", "600")
		return fail(c, 429, "LOGIN_CODE_ATTEMPT_DENIED", "The login verification code is temporarily unavailable.", false)
	}
	return fail(c, 503, "LOGIN_CODE_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
}
func mfaChallengeFailure(c fiber.Ctx, e error) error {
	if e.Error() == "mfa_invalid" {
		return fail(c, 401, "MFA_INVALID", "The MFA challenge is invalid.", false)
	}
	return mfaFailure(c, e)
}
func mfaFailure(c fiber.Ctx, e error) error {
	switch e.Error() {
	case "mfa_enrollment_invalid", "mfa_invalid":
		return fail(c, 400, "MFA_INVALID", "The MFA request is invalid.", false)
	case "mfa_conflict":
		return fail(c, 409, "MFA_CONFLICT", "MFA state changed or already exists.", false)
	case "mfa_assurance_required":
		return fail(c, 403, "MFA_ASSURANCE_REQUIRED", "Recent MFA assurance is required.", false)
	case "mfa_attempt_denied":
		c.Set("Retry-After", "600")
		return fail(c, 429, "MFA_ATTEMPT_DENIED", "The MFA challenge is temporarily unavailable.", false)
	default:
		return fail(c, 503, "MFA_UNAVAILABLE", "MFA is temporarily unavailable.", true)
	}
}
