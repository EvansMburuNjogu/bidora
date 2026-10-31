package httpapi

import (
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"io"
	"mime"
	"net/netip"
	"unicode/utf8"
)

const PasswordRecoveryRequestPath = "/v1/auth/password-recovery"
const PasswordRecoveryResetPath = "/v1/auth/password-recovery/reset"

type PublicPasswordRecoveryUseCase interface {
	Request(context.Context, identityapp.PublicPasswordRecoveryInput) error
}
type PasswordRecoveryResetUseCase interface {
	Reset(context.Context, identityapp.PasswordRecoveryResetInput) error
}
type RecoveryNetworkKeyer interface {
	RecoveryNetwork(netip.Addr) (attempt.Key, error)
}
type PasswordRecoveryHandler struct {
	request    PublicPasswordRecoveryUseCase
	reset      PasswordRecoveryResetUseCase
	protection *browser.Protection
	ips        LoginClientIPResolver
	keys       RecoveryNetworkKeyer
}

func NewPasswordRecoveryHandler(request PublicPasswordRecoveryUseCase, reset PasswordRecoveryResetUseCase, protection *browser.Protection, ips LoginClientIPResolver, keys RecoveryNetworkKeyer) (*PasswordRecoveryHandler, error) {
	if request == nil || reset == nil || protection == nil || ips == nil || keys == nil {
		return nil, errors.New("password_recovery_handler_invalid")
	}
	return &PasswordRecoveryHandler{request, reset, protection, ips, keys}, nil
}
func (h *PasswordRecoveryHandler) Request(c fiber.Ctx) error {
	prepareRecoveryResponse(c)
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeRecoveryJSON(c, &body); err != nil {
		return err
	}
	if c.Response().StatusCode() >= 400 {
		return nil
	}
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "PASSWORD_RECOVERY_UNAVAILABLE", "Password recovery is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "PASSWORD_RECOVERY_UNAVAILABLE", "Password recovery is temporarily unavailable.", true)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "PASSWORD_RECOVERY_UNAVAILABLE", "Password recovery is temporarily unavailable.", true)
	}
	network, err := h.keys.RecoveryNetwork(clientIP)
	if err != nil {
		return fail(c, 503, "PASSWORD_RECOVERY_UNAVAILABLE", "Password recovery is temporarily unavailable.", true)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	err = h.request.Request(c.Context(), identityapp.PublicPasswordRecoveryInput{Email: body.Email, CorrelationID: correlation, NetworkScope: network})
	if err != nil {
		return passwordRecoveryFailure(c, err)
	}
	return c.SendStatus(202)
}
func (h *PasswordRecoveryHandler) Reset(c fiber.Ctx) error {
	prepareRecoveryResponse(c)
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeRecoveryJSON(c, &body); err != nil {
		return err
	}
	if c.Response().StatusCode() >= 400 {
		return nil
	}
	password := []byte(body.NewPassword)
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	if err := h.reset.Reset(c.Context(), identityapp.PasswordRecoveryResetInput{Token: body.Token, NewPassword: password, CorrelationID: correlation}); err != nil {
		return passwordRecoveryFailure(c, err)
	}
	return c.SendStatus(204)
}
func prepareRecoveryResponse(c fiber.Ctx) {
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
}
func decodeRecoveryJSON(c fiber.Ctx, dst any) error {
	if len(c.Request().URI().QueryString()) > 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	media, params, err := mime.ParseMediaType(contentType)
	if err != nil || media != "application/json" || len(params) > 0 {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	if len(c.Body()) == 0 || len(c.Body()) > 4096 || !utf8.Valid(c.Body()) || hasDuplicateWorkflowField(c.Body()) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(dst); err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	return nil
}
func passwordRecoveryFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "password_recovery_attempt_denied":
		c.Set("Retry-After", "900")
		return fail(c, 429, "PASSWORD_RECOVERY_LIMITED", "Password recovery is temporarily unavailable.", true)
	case "password_recovery_reset_invalid":
		return fail(c, 400, "PASSWORD_RECOVERY_INVALID", "The recovery token or password is invalid.", false)
	case "password_recovery_request_invalid":
		return c.SendStatus(202)
	case "password_recovery_request_unavailable", "password_recovery_reset_unavailable":
		return fail(c, 503, "PASSWORD_RECOVERY_UNAVAILABLE", "Password recovery is temporarily unavailable.", true)
	default:
		return fail(c, 503, "PASSWORD_RECOVERY_UNAVAILABLE", "Password recovery is temporarily unavailable.", true)
	}
}
