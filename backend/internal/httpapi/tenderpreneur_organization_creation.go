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

	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	tenantapp "bidora/backend/internal/tenancy/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const TenderpreneurOrganizationCreationPath = "/v1/tenderpreneur/organizations"

type TenderpreneurOrganizationCreationUseCase interface {
	Create(context.Context, tenantapp.OrganizationCreationInput) (tenantapp.OrganizationCreationResult, error)
}
type OrganizationCreationScopeKeyer interface {
	OrganizationCreationIdentity([16]byte) (attempt.Key, error)
	OrganizationCreationNetwork(netip.Addr) (attempt.Key, error)
}
type TenderpreneurOrganizationCreationHandler struct {
	service    TenderpreneurOrganizationCreationUseCase
	protection *browser.Protection
	ips        LoginClientIPResolver
	keys       OrganizationCreationScopeKeyer
}

func NewTenderpreneurOrganizationCreationHandler(service TenderpreneurOrganizationCreationUseCase, protection *browser.Protection, ips LoginClientIPResolver, keys OrganizationCreationScopeKeyer) (*TenderpreneurOrganizationCreationHandler, error) {
	if service == nil || protection == nil || ips == nil || keys == nil {
		return nil, errors.New("organization_creation_handler_invalid")
	}
	return &TenderpreneurOrganizationCreationHandler{service: service, protection: protection, ips: ips, keys: keys}, nil
}

type tenderpreneurOrganizationCreationBody struct {
	Name              string `json:"name"`
	Country           string `json:"country"`
	Timezone          string `json:"timezone"`
	Locale            string `json:"locale"`
	BusinessType      string `json:"business_type"`
	PrimaryIndustry   string `json:"primary_industry"`
	PreferredLanguage string `json:"preferred_language"`
	Currency          string `json:"currency"`
	CommercialPath    string `json:"commercial_path"`
}

func (h *TenderpreneurOrganizationCreationHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("organization_creation_http_context_required")
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.service == nil || h.protection == nil || h.ips == nil || h.keys == nil {
		return organizationCreationFailure(c, tenantapp.ErrOrganizationCreationUnavailable)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if err := h.protection.ValidateUnsafe(c.Get("Origin"), c.Get(session.CSRFHeader), c.Cookies(session.CSRFCookieName), principal.CSRFHash); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	var body tenderpreneurOrganizationCreationBody
	if err := bindOrganizationCreation(c, &body); err != nil {
		return err
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		return invalidHeader(c, IdempotencyKeyHeader, "required")
	}
	if _, err := uuid.Parse(key); err != nil {
		return invalidHeader(c, IdempotencyKeyHeader, "uuid")
	}
	remote := c.RemoteAddr()
	if remote == nil {
		return organizationCreationFailure(c, tenantapp.ErrOrganizationCreationUnavailable)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return organizationCreationFailure(c, tenantapp.ErrOrganizationCreationUnavailable)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return organizationCreationFailure(c, tenantapp.ErrOrganizationCreationUnavailable)
	}
	identityScope, err := h.keys.OrganizationCreationIdentity([16]byte(principal.UserID))
	if err != nil || identityScope.IsZero() {
		return organizationCreationFailure(c, tenantapp.ErrOrganizationCreationUnavailable)
	}
	networkScope, err := h.keys.OrganizationCreationNetwork(clientIP)
	if err != nil || networkScope.IsZero() {
		return organizationCreationFailure(c, tenantapp.ErrOrganizationCreationUnavailable)
	}
	correlationID, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.service.Create(c.Context(), tenantapp.OrganizationCreationInput{
		CreatorUserID: tenantapp.ID(principal.UserID), CorrelationID: tenantapp.ID(correlationID), IdempotencyKey: key,
		IdentityScopeHash: [32]byte(identityScope), NetworkScopeHash: [32]byte(networkScope),
		Name: body.Name, Country: body.Country, Timezone: body.Timezone, Locale: body.Locale,
		BusinessType: body.BusinessType, PrimaryIndustry: body.PrimaryIndustry, PreferredLanguage: body.PreferredLanguage,
		Currency: body.Currency, CommercialPath: tenantapp.OrganizationCommercialPath(body.CommercialPath),
		AssuranceLevel: principal.AssuranceLevel, AssuranceMethod: principal.AssuranceMethod, AssuredAt: principal.AssuredAt,
	})
	if err != nil {
		return organizationCreationFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"data": fiber.Map{"operation_id": uuid.UUID(result.OperationID).String(), "tenant_id": uuid.UUID(result.TenantID).String(), "state": result.State}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}

func bindOrganizationCreation(c fiber.Ctx, target *tenderpreneurOrganizationCreationBody) error {
	if len(c.Body()) > 4096 {
		return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" || len(params) > 1 || len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8") {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	if !utf8.Valid(c.Body()) || !validLoginSurrogates(c.Body()) || hasDuplicateWorkflowField(c.Body()) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(target); err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if !requireOrganizationFields(c.Body(), "name", "country", "timezone", "locale", "business_type", "primary_industry", "preferred_language", "currency", "commercial_path") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	return nil
}
func organizationCreationFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "organization_creation_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case "organization_creation_assurance_required":
		return fail(c, 403, "ASSURANCE_REQUIRED", "Recent authentication assurance is required.", false)
	case "organization_creation_ineligible":
		return fail(c, 403, "ORGANIZATION_CREATION_INELIGIBLE", "Organization creation is not available for this account.", false)
	case "organization_creation_conflict":
		return fail(c, 409, "ORGANIZATION_CREATION_CONFLICT", "The organization creation request conflicts with existing state.", false)
	case "organization_creation_rate_limited":
		c.Set("Retry-After", "900")
		return fail(c, 429, "ORGANIZATION_CREATION_RATE_LIMITED", "Organization creation is temporarily unavailable.", true)
	default:
		return fail(c, 503, "ORGANIZATION_CREATION_UNAVAILABLE", "Organization creation is temporarily unavailable.", true)
	}
}
