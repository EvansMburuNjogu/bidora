package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

type EmailVerificationUseCase interface {
	Verify(context.Context, identityapp.EmailVerificationInput) (identityapp.EmailVerificationResult, error)
}

type EmailVerificationHandler struct {
	codes        *EmailCodes
	verification EmailVerificationUseCase
	protection   *browser.Protection
}

func NewEmailVerificationHandler(verification EmailVerificationUseCase, protection *browser.Protection) (*EmailVerificationHandler, error) {
	if verification == nil || protection == nil {
		return nil, errors.New("email_verification_handler_invalid")
	}
	return &EmailVerificationHandler{verification: verification, protection: protection}, nil
}

func (h *EmailVerificationHandler) EnableShortCodes(codes *EmailCodes) { h.codes = codes }
func (h *EmailVerificationHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("email_verification_http_context_required")
	}
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.verification == nil || h.protection == nil {
		return fail(c, 503, "VERIFICATION_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
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
	var reference string
	var token string
	var err error
	if h.codes != nil {
		var body struct {
			Token     string `json:"token"`
			Reference string `json:"challenge_ref"`
		}
		if e := decodeRecoveryJSON(c, &body); e != nil {
			return e
		}
		if c.Response().StatusCode() >= 400 {
			return nil
		}
		if !requireWorkflowFields(c.Body(), []string{"token"}, "challenge_ref") {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		token, reference = body.Token, body.Reference
	} else {
		token, err = decodeVerification(c.Body())
	}
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	correlationID, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	var reservation identityapp.AttemptReservation
	if len(token) == 6 && h.codes != nil {
		token, reservation, err = h.codes.redeem(c, "verify", reference, token)
		if err != nil {
			if !invalidEmailCode(err) {
				return fail(c, 503, "VERIFICATION_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
			}
			return fail(c, 400, "VERIFICATION_INVALID", "The code is invalid, expired or temporarily blocked.", false)
		}
	}
	_, err = h.verification.Verify(c.Context(), identityapp.EmailVerificationInput{Token: token, CorrelationID: correlationID})
	if reservation != nil {
		if finishErr := reservation.Finish(c.Context(), err == nil); finishErr != nil {
			return fail(c, 503, "VERIFICATION_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
		}
	}
	if err != nil {
		if err.Error() == "verification_invalid" {
			return fail(c, 400, "VERIFICATION_INVALID", "The verification code is invalid or expired.", false)
		}
		return fail(c, 503, "VERIFICATION_UNAVAILABLE", "Email verification is temporarily unavailable.", true)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func decodeVerification(body []byte) (string, error) {
	if len(body) > LoginBodyLimit {
		return "", errors.New("REQUEST_TOO_LARGE")
	}
	if !utf8.Valid(body) {
		return "", errors.New("INVALID_REQUEST")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') || !decoder.More() {
		return "", errors.New("INVALID_REQUEST")
	}
	key, err := decoder.Token()
	if err != nil || key != "token" {
		return "", errors.New("INVALID_REQUEST")
	}
	var token string
	if err = decoder.Decode(&token); err != nil || token == "" || decoder.More() {
		return "", errors.New("INVALID_REQUEST")
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		return "", errors.New("INVALID_REQUEST")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return "", errors.New("INVALID_REQUEST")
	}
	return token, nil
}
