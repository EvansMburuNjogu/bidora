package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

const LoginBodyLimit = 4096

type LoginUseCase interface {
	Login(context.Context, identityapp.LoginInput) (identityapp.LoginResult, error)
}
type LoginClientIPResolver interface {
	Resolve(netip.AddrPort, string) (netip.Addr, error)
}
type LoginNetworkKeyer interface {
	Network(netip.Addr) (attempt.Key, error)
}
type LoginHandler struct {
	login      LoginUseCase
	protection *browser.Protection
	ips        LoginClientIPResolver
	keys       LoginNetworkKeyer
}

func NewLoginHandler(login LoginUseCase, protection *browser.Protection, ips LoginClientIPResolver, keys LoginNetworkKeyer) (*LoginHandler, error) {
	if login == nil || protection == nil || ips == nil || keys == nil {
		return nil, errors.New("login_handler_invalid")
	}
	return &LoginHandler{login, protection, ips, keys}, nil
}
func (h *LoginHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("login_http_context_required")
	}
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.login == nil || h.protection == nil || h.ips == nil || h.keys == nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" || len(params) > 1 || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	emailValue, passwordValue, err := decodeLogin(c.Body())
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	passwordBytes := []byte(passwordValue)
	defer func() {
		for i := range passwordBytes {
			passwordBytes[i] = 0
		}
	}()
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	networkScope, err := h.keys.Network(clientIP)
	if err != nil || networkScope.IsZero() {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	correlationID, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.login.Login(c.Context(), identityapp.LoginInput{Credentials: identityapp.AuthenticateInput{Email: emailValue, Password: passwordBytes, NetworkScope: networkScope}, CorrelationID: correlationID})
	if err != nil {
		return loginFailure(c, err)
	}
	if result.MFARequired {
		c.Cookie(&fiber.Cookie{Name: "__Host-bidora_login_code", Value: result.MFAToken, Path: "/", Expires: result.MFAExpiresAt.Add(identityapp.EmailLoginResendGrace), Secure: true, HTTPOnly: true, SameSite: "Lax"})
		c.Set("X-Bidora-Challenge-Expires-At", result.MFAExpiresAt.UTC().Format(time.RFC3339Nano))
		return c.Status(202).JSON(fiber.Map{"data": fiber.Map{"mfa_required": true, "expires_at": result.MFAExpiresAt}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
	}
	sessionCookie, err := browser.SessionCookie(result.Session.RevealForCookie(), result.AbsoluteExpiresAt)
	if err != nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	csrfCookie, err := browser.CSRFCookie(result.CSRF.RevealForBootstrap(), result.AbsoluteExpiresAt)
	if err != nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	setLoginCookie(c, sessionCookie)
	setLoginCookie(c, csrfCookie)
	return c.SendStatus(fiber.StatusNoContent)
}
func writeInvitationSession(c fiber.Ctx, result identityapp.LoginResult) error {
	sessionCookie, err := browser.SessionCookie(result.Session.RevealForCookie(), result.AbsoluteExpiresAt)
	if err != nil {
		return fail(c, 503, "INVITATION_COMPLETION_UNCONFIRMED", "Sign-in could not be completed. Please sign in.", true)
	}
	csrfCookie, err := browser.CSRFCookie(result.CSRF.RevealForBootstrap(), result.AbsoluteExpiresAt)
	if err != nil {
		return fail(c, 503, "INVITATION_COMPLETION_UNCONFIRMED", "Sign-in could not be completed. Please sign in.", true)
	}
	setLoginCookie(c, sessionCookie)
	setLoginCookie(c, csrfCookie)
	return c.SendStatus(fiber.StatusNoContent)
}

func setLoginCookie(c fiber.Ctx, cookie http.Cookie) {
	c.Cookie(&fiber.Cookie{Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Domain: cookie.Domain, Expires: cookie.Expires, MaxAge: cookie.MaxAge, Secure: cookie.Secure, HTTPOnly: cookie.HttpOnly, SameSite: "Lax"})
}
func decodeLogin(body []byte) (string, string, error) {
	if len(body) > LoginBodyLimit {
		return "", "", errors.New("REQUEST_TOO_LARGE")
	}
	if !utf8.Valid(body) {
		return "", "", errors.New("INVALID_REQUEST")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", "", errors.New("INVALID_REQUEST")
	}
	seen := map[string]bool{}
	emailValue, passwordValue := "", ""
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return "", "", errors.New("INVALID_REQUEST")
		}
		seen[key] = true
		if key != "email" && key != "password" {
			return "", "", errors.New("INVALID_REQUEST")
		}
		var value string
		if err = decoder.Decode(&value); err != nil {
			return "", "", errors.New("INVALID_REQUEST")
		}
		if key == "email" {
			emailValue = value
		} else {
			passwordValue = value
		}
	}
	if _, err = decoder.Token(); err != nil {
		return "", "", errors.New("INVALID_REQUEST")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return "", "", errors.New("INVALID_REQUEST")
	}
	if !seen["email"] || !seen["password"] || !validLoginSurrogates(body) {
		return "", "", errors.New("INVALID_REQUEST")
	}
	return emailValue, passwordValue, nil
}
func validLoginSurrogates(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' || i+1 >= len(raw) {
			continue
		}
		i++
		if raw[i] != 'u' || i+4 >= len(raw) {
			continue
		}
		value, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if value >= 0xDC00 && value <= 0xDFFF {
			return false
		}
		if value >= 0xD800 && value <= 0xDBFF {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return true
}
func errorStatus(code string) int {
	if code == "REQUEST_TOO_LARGE" {
		return 413
	}
	return 400
}
func parseIdentityID(value string) (identityapp.ID, bool) {
	parsed, ok := parseOpportunityID(value)
	return identityapp.ID(parsed), ok
}
func loginFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "credentials_invalid":
		return fail(c, 401, "CREDENTIALS_INVALID", "The credentials are invalid.", false)
	case "attempt_denied":
		c.Set("Retry-After", strconv.Itoa(15*60))
		return fail(c, 429, "ATTEMPT_DENIED", "Authentication is temporarily unavailable.", true)
	default:
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
}
