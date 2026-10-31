package httpapi

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

const MePortalPath = "/v1/me"
const AccountTypeSelectionPath = "/v1/me/account-type"

type AccountTypeUseCase interface {
	Resolve(context.Context, identityapp.SessionPrincipal) (identityapp.PortalResolution, error)
	Select(context.Context, identityapp.SelectAccountTypeInput) (identityapp.AccountTypeSelectionResult, error)
}

type AccountTypeHandler struct {
	service    AccountTypeUseCase
	protection *browser.Protection
}

func NewAccountTypeHandler(service AccountTypeUseCase, protection *browser.Protection) (*AccountTypeHandler, error) {
	if service == nil || protection == nil {
		return nil, errors.New("account_type_handler_invalid")
	}
	return &AccountTypeHandler{service: service, protection: protection}, nil
}

func (h *AccountTypeHandler) Get(c fiber.Ctx) error {
	prepareAccountTypeResponse(c)
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	result, err := h.service.Resolve(c.Context(), principal)
	if err != nil {
		return accountTypeFailure(c, err)
	}
	var portal any
	if result.Portal != nil {
		portal = fiber.Map{"key": result.Portal.Key, "path": result.Portal.Path}
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"account_type":                string(result.AccountType),
			"portal":                      portal,
			"platform_context_generation": result.PlatformContextGeneration,
			"capabilities":                result.Capabilities,
			"session": fiber.Map{
				"assurance_level":     result.AssuranceLevel,
				"idle_expires_at":     result.IdleExpiresAt.Format(time.RFC3339Nano),
				"absolute_expires_at": result.AbsoluteExpiresAt.Format(time.RFC3339Nano),
			},
			"required_actions": result.RequiredActions,
		},
		"meta": meta{TraceID: c.GetRespHeader("X-Request-ID")},
	})
}

func (h *AccountTypeHandler) Select(c fiber.Ctx) error {
	prepareAccountTypeResponse(c)
	var request struct {
		AccountType string `json:"account_type"`
	}
	if err := bindWorkflow(c, &request); err != nil {
		return err
	}
	if c.Response().StatusCode() >= 400 {
		return nil
	}
	if !utf8.ValidString(request.AccountType) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok || len(key) > 128 || !utf8.ValidString(key) {
		return invalidHeader(c, IdempotencyKeyHeader, "required_1_to_128_bytes")
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.service.Select(c.Context(), identityapp.SelectAccountTypeInput{
		Principal: principal, AccountType: request.AccountType, IdempotencyKey: key,
		CorrelationID: correlation,
	})
	if err != nil {
		return accountTypeFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	return c.SendStatus(204)
}

func prepareAccountTypeResponse(c fiber.Ctx) {
	c.Set("Cache-Control", "private, no-store")
	c.Set("Vary", "Cookie, Origin")
	c.Set("X-Content-Type-Options", "nosniff")
}

func accountTypeFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "account_type_selection_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The account type selection is invalid.", false)
	case "session_invalid":
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	case "platform_access_forbidden":
		return fail(c, 403, "PLATFORM_ACCESS_FORBIDDEN", "Platform access is not available for this account.", false)
	case "account_type_already_selected":
		return fail(c, 409, "ACCOUNT_TYPE_ALREADY_SELECTED", "The account type has already been selected.", false)
	case "account_type_selection_conflict":
		return fail(c, 409, "ACCOUNT_TYPE_SELECTION_CONFLICT", "The idempotency key was already used for another selection.", false)
	default:
		return fail(c, 503, "PORTAL_RESOLUTION_UNAVAILABLE", "Portal resolution is temporarily unavailable.", true)
	}
}
