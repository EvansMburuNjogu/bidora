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

const EmailVerificationResendPath = "/v1/auth/verify-email/resend"

type EmailVerificationResendUseCase interface {
	Resend(context.Context, identityapp.EmailVerificationResendInput) (identityapp.EmailVerificationResendResult, error)
}

type EmailVerificationResendHandler struct {
	codes      *EmailCodes
	resend     EmailVerificationResendUseCase
	protection *browser.Protection
	ips        LoginClientIPResolver
	keys       RegistrationNetworkKeyer
}

func NewEmailVerificationResendHandler(resend EmailVerificationResendUseCase, protection *browser.Protection, ips LoginClientIPResolver, keys RegistrationNetworkKeyer) (*EmailVerificationResendHandler, error) {
	if resend == nil || protection == nil || ips == nil || keys == nil {
		return nil, errors.New("email_verification_resend_handler_invalid")
	}
	return &EmailVerificationResendHandler{resend: resend, protection: protection, ips: ips, keys: keys}, nil
}

func (h *EmailVerificationResendHandler) EnableShortCodes(codes *EmailCodes) { h.codes = codes }
func (h *EmailVerificationResendHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("email_verification_resend_http_context_required")
	}
	prepareRecoveryResponse(c)
	if h == nil || h.resend == nil || h.protection == nil || h.ips == nil || h.keys == nil {
		return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
	}
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	if !validJSONMediaType(contentType) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	emailValue, err := decodeVerificationResend(c.Body(), len(c.Request().URI().QueryString()) != 0)
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
	}
	network, err := h.keys.RegistrationNetwork(clientIP)
	if err != nil || network.IsZero() {
		return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	ctx := c.Context()
	reference := ""
	if h.codes != nil {
		ctx, reference, err = h.codes.issue(ctx, "verify")
		if err != nil {
			return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
		}
	}
	result, err := h.resend.Resend(ctx, identityapp.EmailVerificationResendInput{Email: emailValue, CorrelationID: correlation, NetworkScope: network})
	if err != nil {
		if err.Error() == "verification_resend_attempt_denied" {
			setCodeRetryAfter(c, err)
			return fail(c, 429, "VERIFICATION_RESEND_LIMITED", "Verification email is temporarily unavailable.", true)
		}
		return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
	}
	if !result.Accepted {
		return fail(c, 503, "VERIFICATION_RESEND_UNAVAILABLE", "Verification email is temporarily unavailable.", true)
	}
	if reference != "" {
		c.Set(EmailCodeChallengeHeader, reference)
	}
	return c.SendStatus(fiber.StatusAccepted)
}

func decodeVerificationResend(body []byte, hasQuery bool) (string, error) {
	if hasQuery || len(body) == 0 || !utf8.Valid(body) || !validLoginSurrogates(body) {
		return "", errors.New("INVALID_REQUEST")
	}
	if len(body) > LoginBodyLimit {
		return "", errors.New("REQUEST_TOO_LARGE")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') || !decoder.More() {
		return "", errors.New("INVALID_REQUEST")
	}
	key, err := decoder.Token()
	if err != nil || key != "email" {
		return "", errors.New("INVALID_REQUEST")
	}
	var value string
	if err = decoder.Decode(&value); err != nil || value == "" || len(value) > 254 || decoder.More() {
		return "", errors.New("INVALID_REQUEST")
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		return "", errors.New("INVALID_REQUEST")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return "", errors.New("INVALID_REQUEST")
	}
	return value, nil
}
