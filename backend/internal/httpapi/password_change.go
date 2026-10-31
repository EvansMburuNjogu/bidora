package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"unicode/utf8"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

const PasswordChangePath = "/v1/auth/password"

type PasswordChangeUseCase interface {
	Change(context.Context, identityapp.PasswordChangeInput) (identityapp.LoginResult, error)
}
type PasswordChangeHandler struct {
	change PasswordChangeUseCase
	ips    LoginClientIPResolver
	keys   LoginNetworkKeyer
}

func NewPasswordChangeHandler(change PasswordChangeUseCase, ips LoginClientIPResolver, keys LoginNetworkKeyer) (*PasswordChangeHandler, error) {
	if change == nil || ips == nil || keys == nil {
		return nil, errors.New("password_change_handler_invalid")
	}
	return &PasswordChangeHandler{change, ips, keys}, nil
}
func (h *PasswordChangeHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("password_change_http_context_required")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.change == nil || h.ips == nil || h.keys == nil {
		return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok || !validJSONMediaType(contentType) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	emailValue, currentValue, newValue, err := decodePasswordChange(c.Body())
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	currentPassword, newPassword := []byte(currentValue), []byte(newValue)
	defer func() {
		for index := range currentPassword {
			currentPassword[index] = 0
		}
		for index := range newPassword {
			newPassword[index] = 0
		}
	}()
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
	}
	networkScope, err := h.keys.Network(clientIP)
	if err != nil || networkScope.IsZero() {
		return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.change.Change(c.Context(), identityapp.PasswordChangeInput{Principal: principal, Credentials: identityapp.AuthenticateInput{Email: emailValue, Password: currentPassword, NetworkScope: networkScope}, NewPassword: newPassword, CorrelationID: correlation})
	if err != nil {
		switch err.Error() {
		case "attempt_denied":
			c.Set("Retry-After", "900")
			return fail(c, 429, "ATTEMPT_DENIED", "Authentication is temporarily unavailable.", true)
		case "password_change_credentials_invalid", "password_change_invalid":
			return fail(c, 401, "CREDENTIALS_INVALID", "The credentials are invalid.", false)
		case "password_change_new_password_invalid":
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		default:
			return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
		}
	}
	sessionCookie, err := browser.SessionCookie(result.Session.RevealForCookie(), result.AbsoluteExpiresAt)
	if err != nil {
		return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
	}
	csrfCookie, err := browser.CSRFCookie(result.CSRF.RevealForBootstrap(), result.AbsoluteExpiresAt)
	if err != nil {
		return fail(c, 503, "PASSWORD_CHANGE_UNAVAILABLE", "Password change is temporarily unavailable.", true)
	}
	setLoginCookie(c, sessionCookie)
	setLoginCookie(c, csrfCookie)
	return c.SendStatus(fiber.StatusNoContent)
}
func decodePasswordChange(body []byte) (string, string, string, error) {
	if len(body) > LoginBodyLimit {
		return "", "", "", errors.New("REQUEST_TOO_LARGE")
	}
	if !utf8.Valid(body) || !validLoginSurrogates(body) {
		return "", "", "", errors.New("INVALID_REQUEST")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", "", "", errors.New("INVALID_REQUEST")
	}
	seen := map[string]bool{}
	values := map[string]string{}
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] || (key != "email" && key != "current_password" && key != "new_password") {
			return "", "", "", errors.New("INVALID_REQUEST")
		}
		seen[key] = true
		var value string
		if decoder.Decode(&value) != nil {
			return "", "", "", errors.New("INVALID_REQUEST")
		}
		values[key] = value
	}
	if _, err = decoder.Token(); err != nil {
		return "", "", "", errors.New("INVALID_REQUEST")
	}
	if _, err = decoder.Token(); err != io.EOF || !seen["email"] || !seen["current_password"] || !seen["new_password"] {
		return "", "", "", errors.New("INVALID_REQUEST")
	}
	return values["email"], values["current_password"], values["new_password"], nil
}
