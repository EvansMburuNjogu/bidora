package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/netip"
	"strings"
	"unicode/utf8"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

type OpenRegistrationUseCase interface {
	Register(context.Context, identityapp.OpenRegistrationInput) (identityapp.OpenRegistrationResult, error)
}
type RegistrationNetworkKeyer interface {
	RegistrationNetwork(netip.Addr) (attempt.Key, error)
}
type RegistrationHandler struct {
	registration OpenRegistrationUseCase
	protection   *browser.Protection
	ips          LoginClientIPResolver
	keys         RegistrationNetworkKeyer
}

func NewRegistrationHandler(registration OpenRegistrationUseCase, protection *browser.Protection, ips LoginClientIPResolver, keys RegistrationNetworkKeyer) (*RegistrationHandler, error) {
	if registration == nil || protection == nil || ips == nil || keys == nil {
		return nil, errors.New("registration_handler_invalid")
	}
	return &RegistrationHandler{registration, protection, ips, keys}, nil
}
func (h *RegistrationHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("registration_http_context_required")
	}
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.registration == nil || h.protection == nil || h.ips == nil || h.keys == nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok || !validJSONMediaType(contentType) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	firstName, lastName, emailValue, passwordValue, err := decodeRegistration(c.Body())
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	passwordBytes := []byte(passwordValue)
	defer func() {
		for index := range passwordBytes {
			passwordBytes[index] = 0
		}
	}()
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	networkScope, err := h.keys.RegistrationNetwork(clientIP)
	if err != nil || networkScope.IsZero() {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	correlationID, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.registration.Register(c.Context(), identityapp.OpenRegistrationInput{Registration: identityapp.SelfRegistrationInput{FirstName: firstName, LastName: lastName, Email: emailValue, Password: passwordBytes, CorrelationID: correlationID}, NetworkScope: networkScope})
	if err != nil {
		return registrationFailure(c, err)
	}
	if !result.Accepted {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	return c.SendStatus(fiber.StatusAccepted)
}
func decodeRegistration(body []byte) (string, string, string, string, error) {
	if len(body) > LoginBodyLimit || !utf8.Valid(body) || !validLoginSurrogates(body) {
		code := "INVALID_REQUEST"
		if len(body) > LoginBodyLimit {
			code = "REQUEST_TOO_LARGE"
		}
		return "", "", "", "", errors.New(code)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", "", "", "", errors.New("INVALID_REQUEST")
	}
	seen := map[string]bool{}
	values := map[string]string{}
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
		seen[key] = true
		switch key {
		case "first_name", "last_name", "email", "password":
		default:
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
		var value string
		if err = decoder.Decode(&value); err != nil {
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
		values[key] = value
	}
	if _, err = decoder.Token(); err != nil {
		return "", "", "", "", errors.New("INVALID_REQUEST")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return "", "", "", "", errors.New("INVALID_REQUEST")
	}
	for _, key := range []string{"first_name", "last_name", "email", "password"} {
		if !seen[key] {
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
	}
	return values["first_name"], values["last_name"], values["email"], values["password"], nil
}

func registrationFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "registration_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case "registration_attempt_denied":
		c.Set("Retry-After", "900")
		return fail(c, 429, "REGISTRATION_LIMITED", "Registration is temporarily unavailable.", true)
	case "registration_context_required", "registration_unavailable":
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	default:
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
}
func validJSONMediaType(value string) bool {
	mediaType, params, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json" && (len(params) == 0 || len(params) == 1 && strings.EqualFold(params["charset"], "utf-8"))
}
