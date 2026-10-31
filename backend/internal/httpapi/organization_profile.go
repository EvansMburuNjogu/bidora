package httpapi

import (
	app "bidora/backend/internal/organization/application"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"io"
	"mime"
	"strings"
	"unicode/utf8"
)

const (
	OrganizationProfilePath = "/v1/tenderpreneur/organization/profile"
	LetterheadVersionsPath  = "/v1/tenderpreneur/organization/letterhead/versions"
	LetterheadVersionPath   = "/v1/tenderpreneur/organization/letterhead/versions/:version_id"
	ActiveLetterheadPath    = "/v1/tenderpreneur/organization/letterhead/active"
	ActivateLetterheadPath  = "/v1/tenderpreneur/organization/letterhead/versions/:version_id/activate"
)

type OrganizationUseCase interface {
	SaveProfile(context.Context, app.SaveProfileInput) (app.ProfileResult, error)
	CreateLetterhead(context.Context, app.CreateLetterheadInput) (app.LetterheadResult, error)
	ActivateLetterhead(context.Context, app.ActivateLetterheadInput) (app.ActivationResult, error)
	GetProfile(context.Context, app.Subject) (app.CompanyProfile, error)
	GetLetterhead(context.Context, app.Subject, app.ID) (app.LetterheadVersion, error)
	GetActiveLetterhead(context.Context, app.Subject) (app.LetterheadVersion, app.LetterheadConfiguration, error)
}
type OrganizationHandler struct{ usecase OrganizationUseCase }

func NewOrganizationHandler(usecase OrganizationUseCase) (*OrganizationHandler, error) {
	if usecase == nil {
		return nil, errors.New("organization_handler_invalid")
	}
	return &OrganizationHandler{usecase}, nil
}
func RegisterOrganizationRoutes(fiberApp *fiber.App, browser *BrowserSessionMiddleware, handler *OrganizationHandler) error {
	if fiberApp == nil || browser == nil || handler == nil {
		return errors.New("organization_routes_invalid")
	}
	fiberApp.Get(OrganizationProfilePath, browser.OriginBound, handler.GetProfile)
	fiberApp.Put(OrganizationProfilePath, browser.OriginBound, handler.SaveProfile)
	fiberApp.Post(LetterheadVersionsPath, browser.OriginBound, handler.CreateLetterhead)
	fiberApp.Get(LetterheadVersionPath, browser.OriginBound, handler.GetLetterhead)
	fiberApp.Get(ActiveLetterheadPath, browser.OriginBound, handler.GetActiveLetterhead)
	fiberApp.Post(ActivateLetterheadPath, browser.OriginBound, handler.ActivateLetterhead)
	return nil
}

type profileBody struct {
	RegisteredName         string  `json:"registered_name"`
	TradingName            *string `json:"trading_name"`
	BusinessType           string  `json:"business_type"`
	PrimaryIndustry        string  `json:"primary_industry"`
	Description            string  `json:"description"`
	ServicesCapabilities   string  `json:"services_capabilities"`
	WorkEmail              string  `json:"work_email"`
	WorkPhone              string  `json:"work_phone"`
	PreferredLanguage      string  `json:"preferred_language"`
	Locale                 string  `json:"locale"`
	Website                *string `json:"website"`
	PostalAddress          *string `json:"postal_address"`
	PhysicalAddress        *string `json:"physical_address"`
	RegistrationIdentifier *string `json:"registration_identifier"`
	TaxIdentifier          *string `json:"tax_identifier"`
	ExpectedVersion        int64   `json:"expected_version"`
}
type letterheadBody struct {
	RegisteredName               string  `json:"registered_name"`
	TradingName                  *string `json:"trading_name"`
	RegistrationIdentifier       *string `json:"registration_identifier"`
	TaxIdentifier                *string `json:"tax_identifier"`
	PostalAddress                *string `json:"postal_address"`
	PhysicalAddress              *string `json:"physical_address"`
	Phone                        string  `json:"phone"`
	Email                        string  `json:"email"`
	Website                      *string `json:"website"`
	Slogan                       *string `json:"slogan"`
	HeaderLayout                 string  `json:"header_layout"`
	FooterLayout                 string  `json:"footer_layout"`
	PageNumberStyle              string  `json:"page_number_style"`
	PrimaryColor                 string  `json:"primary_color"`
	SecondaryColor               string  `json:"secondary_color"`
	Disclaimer                   *string `json:"disclaimer"`
	TemplateKey                  string  `json:"template_key"`
	ExpectedConfigurationVersion int64   `json:"expected_configuration_version"`
}
type activationBody struct {
	ExpectedConfigurationVersion int64  `json:"expected_configuration_version"`
	Reason                       string `json:"reason"`
}

func (h *OrganizationHandler) SaveProfile(c fiber.Ctx) error {
	var body profileBody
	if err := bindOrganization(c, &body); err != nil {
		return err
	}
	if !requireOrganizationFields(c.Body(), "registered_name", "trading_name", "business_type", "primary_industry", "description", "services_capabilities", "work_email", "work_phone", "preferred_language", "locale", "website", "postal_address", "physical_address", "registration_identifier", "tax_identifier", "expected_version") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	common, ok := organizationMutation(c)
	if !ok {
		return nil
	}
	result, err := h.usecase.SaveProfile(c.Context(), app.SaveProfileInput{Subject: common.subject, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, ExpectedVersion: body.ExpectedVersion, Fields: app.CompanyProfileFields{RegisteredName: body.RegisteredName, TradingName: body.TradingName, BusinessType: body.BusinessType, PrimaryIndustry: body.PrimaryIndustry, Description: body.Description, ServicesCapabilities: body.ServicesCapabilities, WorkEmail: body.WorkEmail, WorkPhone: body.WorkPhone, PreferredLanguage: body.PreferredLanguage, Locale: body.Locale, Website: body.Website, PostalAddress: body.PostalAddress, PhysicalAddress: body.PhysicalAddress, RegistrationIdentifier: body.RegistrationIdentifier, TaxIdentifier: body.TaxIdentifier}})
	if err != nil {
		return organizationFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	return organizationJSON(c, 200, "profile", profileView(result.Profile))
}
func (h *OrganizationHandler) GetProfile(c fiber.Ctx) error {
	subject, ok := organizationRead(c)
	if !ok {
		return nil
	}
	result, err := h.usecase.GetProfile(c.Context(), subject)
	if err != nil {
		return organizationFailure(c, err)
	}
	return organizationJSON(c, 200, "profile", profileView(result))
}
func (h *OrganizationHandler) CreateLetterhead(c fiber.Ctx) error {
	var body letterheadBody
	if err := bindOrganization(c, &body); err != nil {
		return err
	}
	if !requireOrganizationFields(c.Body(), "registered_name", "trading_name", "registration_identifier", "tax_identifier", "postal_address", "physical_address", "phone", "email", "website", "slogan", "header_layout", "footer_layout", "page_number_style", "primary_color", "secondary_color", "disclaimer", "template_key", "expected_configuration_version") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	common, ok := organizationMutation(c)
	if !ok {
		return nil
	}
	result, err := h.usecase.CreateLetterhead(c.Context(), app.CreateLetterheadInput{Subject: common.subject, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, ExpectedConfigurationVersion: body.ExpectedConfigurationVersion, Fields: letterheadFields(body)})
	if err != nil {
		return organizationFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	return organizationJSON(c, 201, "letterhead", letterheadView(result.Version, result.ConfigurationVersion))
}
func (h *OrganizationHandler) ActivateLetterhead(c fiber.Ctx) error {
	var body activationBody
	if err := bindOrganization(c, &body); err != nil {
		return err
	}
	if !requireOrganizationFields(c.Body(), "expected_configuration_version", "reason") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	common, ok := organizationMutation(c)
	if !ok {
		return nil
	}
	versionID, ok := organizationID(c.Params("version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.usecase.ActivateLetterhead(c.Context(), app.ActivateLetterheadInput{Subject: common.subject, VersionID: versionID, CorrelationID: common.correlation, IdempotencyKey: common.idempotency, ExpectedConfigurationVersion: body.ExpectedConfigurationVersion, Reason: body.Reason})
	if err != nil {
		return organizationFailure(c, err)
	}
	if result.Replay {
		c.Set("Idempotency-Replayed", "true")
	}
	return organizationJSON(c, 200, "letterhead", letterheadView(result.Version, result.ConfigurationVersion))
}
func (h *OrganizationHandler) GetLetterhead(c fiber.Ctx) error {
	subject, ok := organizationRead(c)
	if !ok {
		return nil
	}
	versionID, ok := organizationID(c.Params("version_id"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.usecase.GetLetterhead(c.Context(), subject, versionID)
	if err != nil {
		return organizationFailure(c, err)
	}
	return organizationJSON(c, 200, "letterhead", letterheadView(result, 0))
}
func (h *OrganizationHandler) GetActiveLetterhead(c fiber.Ctx) error {
	subject, ok := organizationRead(c)
	if !ok {
		return nil
	}
	result, configuration, err := h.usecase.GetActiveLetterhead(c.Context(), subject)
	if err != nil {
		return organizationFailure(c, err)
	}
	return organizationJSON(c, 200, "letterhead", letterheadView(result, configuration.Version))
}

type organizationCommon struct {
	subject     app.Subject
	correlation app.ID
	idempotency string
}

func organizationMutation(c fiber.Ctx) (organizationCommon, bool) {
	subject, ok := organizationSubject(c)
	if !ok {
		return organizationCommon{}, false
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		_ = invalidHeader(c, IdempotencyKeyHeader, "required")
		return organizationCommon{}, false
	}
	if _, err := uuid.Parse(key); err != nil {
		_ = invalidHeader(c, IdempotencyKeyHeader, "uuid")
		return organizationCommon{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		return organizationCommon{}, false
	}
	return organizationCommon{subject: subject, correlation: app.ID(correlation), idempotency: key}, true
}
func organizationRead(c fiber.Ctx) (app.Subject, bool) {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return app.Subject{}, false
	}
	return organizationSubject(c)
}
func organizationSubject(c fiber.Ctx) (app.Subject, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return app.Subject{}, false
	}
	tenantRaw, ok := singleHeader(c, TenantIDHeader)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "required")
		return app.Subject{}, false
	}
	tenant, ok := parseIdentityID(tenantRaw)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "uuid")
		return app.Subject{}, false
	}
	generationRaw, ok := singleHeader(c, ContextGenerationHeader)
	if !ok {
		_ = invalidHeader(c, ContextGenerationHeader, "required")
		return app.Subject{}, false
	}
	generation, ok := positiveInt64(generationRaw)
	if !ok {
		_ = invalidHeader(c, ContextGenerationHeader, "positive_integer")
		return app.Subject{}, false
	}
	return app.Subject{IdentityID: app.ID(principal.UserID), TenantID: app.ID(tenant), ContextGeneration: generation}, true
}
func bindOrganization(c fiber.Ctx, target any) error {
	c.Set("Cache-Control", "private, no-store")
	if len(c.Body()) > 16384 {
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
	if err != nil || mediaType != "application/json" || (len(params) > 1) || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
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
	return nil
}
func requireOrganizationFields(body []byte, names ...string) bool {
	var values map[string]json.RawMessage
	if json.Unmarshal(body, &values) != nil {
		return false
	}
	for _, name := range names {
		if _, ok := values[name]; !ok {
			return false
		}
	}
	return true
}
func organizationFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, app.ErrNotFound):
		return fail(c, 404, "ORGANIZATION_NOT_FOUND", "The resource was not found.", false)
	case errors.Is(err, app.ErrConflict):
		return fail(c, 409, "ORGANIZATION_CONFLICT", "The organization resource changed.", false)
	case err.Error() == "organization_not_authorized":
		return fail(c, 403, "ORGANIZATION_NOT_AUTHORIZED", "You are not authorized to manage this organization.", false)
	case strings.Contains(err.Error(), "invalid"):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	default:
		return fail(c, 503, "ORGANIZATION_UNAVAILABLE", "Organization settings are temporarily unavailable.", true)
	}
}
func organizationJSON(c fiber.Ctx, status int, key string, value any) error {
	c.Set("Cache-Control", "private, no-store")
	return c.Status(status).JSON(fiber.Map{"data": fiber.Map{key: value}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func profileView(v app.CompanyProfile) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "registered_name": v.RegisteredName, "trading_name": v.TradingName, "business_type": v.BusinessType, "primary_industry": v.PrimaryIndustry, "description": v.Description, "services_capabilities": v.ServicesCapabilities, "work_email": v.WorkEmail, "work_phone": v.WorkPhone, "preferred_language": v.PreferredLanguage, "locale": v.Locale, "website": v.Website, "postal_address": v.PostalAddress, "physical_address": v.PhysicalAddress, "registration_identifier": v.RegistrationIdentifier, "tax_identifier": v.TaxIdentifier, "version": v.Version, "created_by": uuid.UUID(v.CreatedBy).String(), "updated_by": uuid.UUID(v.UpdatedBy).String(), "created_at": v.CreatedAt, "updated_at": v.UpdatedAt}
}
func letterheadFields(v letterheadBody) app.LetterheadFields {
	return app.LetterheadFields{RegisteredName: v.RegisteredName, TradingName: v.TradingName, RegistrationIdentifier: v.RegistrationIdentifier, TaxIdentifier: v.TaxIdentifier, PostalAddress: v.PostalAddress, PhysicalAddress: v.PhysicalAddress, Phone: v.Phone, Email: v.Email, Website: v.Website, Slogan: v.Slogan, HeaderLayout: v.HeaderLayout, FooterLayout: v.FooterLayout, PageNumberStyle: v.PageNumberStyle, PrimaryColor: v.PrimaryColor, SecondaryColor: v.SecondaryColor, Disclaimer: v.Disclaimer, TemplateKey: v.TemplateKey}
}
func letterheadView(v app.LetterheadVersion, configurationVersion int64) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "version_number": v.VersionNumber, "registered_name": v.RegisteredName, "trading_name": v.TradingName, "registration_identifier": v.RegistrationIdentifier, "tax_identifier": v.TaxIdentifier, "postal_address": v.PostalAddress, "physical_address": v.PhysicalAddress, "phone": v.Phone, "email": v.Email, "website": v.Website, "slogan": v.Slogan, "header_layout": v.HeaderLayout, "footer_layout": v.FooterLayout, "page_number_style": v.PageNumberStyle, "primary_color": v.PrimaryColor, "secondary_color": v.SecondaryColor, "disclaimer": v.Disclaimer, "template_key": v.TemplateKey, "content_hash": hex.EncodeToString(v.ContentHash[:]), "created_by": uuid.UUID(v.CreatedBy).String(), "created_at": v.CreatedAt, "configuration_version": configurationVersion}
}
func organizationID(raw string) (app.ID, bool) {
	id, err := uuid.Parse(raw)
	return app.ID(id), err == nil && id != uuid.Nil
}
