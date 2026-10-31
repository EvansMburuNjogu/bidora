package httpapi

import (
	commercial "bidora/backend/internal/commercial/domain"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"strings"
	"time"
)

type RegionalPolicyHandler struct{ service *app.RegionalPolicyService }

func NewRegionalPolicyHandler(service *app.RegionalPolicyService) *RegionalPolicyHandler {
	return &RegionalPolicyHandler{service}
}
func RegisterRegionalPolicyRoutes(f *fiber.App, b *BrowserSessionMiddleware, h *RegionalPolicyHandler) {
	f.Get("/v1/platform/audit/actions/:audit_id/request-metadata", b.Platform, h.metadata)
	f.Get("/v1/shared/platform/availability", b.OriginBound, h.availability)
	f.Get("/v1/shared/platform/countries", b.OriginBound, h.sharedCountries)
	f.Get("/v1/platform/countries", b.Platform, h.countries)
	f.Get("/v1/platform/countries/reference", b.Platform, h.references)
	f.Post("/v1/platform/countries", b.Platform, h.saveCountry)
	f.Put("/v1/platform/countries/:code", b.Platform, h.saveCountry)
	f.Get("/v1/platform/countries/:code/history", b.Platform, h.countryHistory)
	f.Get("/v1/platform/operational-policy", b.Platform, h.policy)
	f.Put("/v1/platform/operational-policy", b.Platform, h.savePolicy)
	f.Get("/v1/platform/operational-policy/history", b.Platform, h.policyHistory)
}
func regionalSubject(c fiber.Ctx, write bool) (app.RegionalSubject, string, bool) {
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return app.RegionalSubject{}, "", false
	}
	correlation, e := uuid.Parse(c.GetRespHeader("X-Request-ID"))
	if e != nil || correlation == uuid.Nil {
		_ = fail(c, 503, "SETTINGS_UNAVAILABLE", "Settings are temporarily unavailable.", true)
		return app.RegionalSubject{}, "", false
	}
	out := app.RegionalSubject{Subject: subject, CorrelationID: domain.ID(correlation)}
	if !write {
		return out, "", true
	}
	raw, ok := singleHeader(c, IdempotencyKeyHeader)
	key, e := uuid.Parse(raw)
	if !ok || e != nil || key == uuid.Nil {
		_ = fail(c, 400, "INVALID_REQUEST", "A valid request key is required.", false)
		return out, "", false
	}
	return out, key.String(), true
}
func (h *RegionalPolicyHandler) metadata(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, _, ok := regionalSubject(c, false)
	if !ok {
		return nil
	}
	id, e := uuid.Parse(c.Params("audit_id"))
	if e != nil || id == uuid.Nil {
		return fail(c, 404, "NOT_FOUND", "Request metadata is unavailable.", false)
	}
	out, e := h.service.RequestMetadata(c.Context(), subject, domain.ID(id))
	if e == app.ErrNotFound {
		return fail(c, 404, "NOT_FOUND", "Request metadata is unavailable.", false)
	}
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, out, nil)
}

func (h *RegionalPolicyHandler) availability(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	out, e := h.service.Availability(c.Context())
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, out, nil)
}

func (h *RegionalPolicyHandler) countries(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, _, ok := regionalSubject(c, false)
	if !ok {
		return nil
	}
	rows, e := h.service.Countries(c.Context(), subject, false)
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, rows, nil)
}
func (h *RegionalPolicyHandler) references(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, _, ok := regionalSubject(c, false)
	if !ok {
		return nil
	}
	if _, e := h.service.Countries(c.Context(), subject, false); e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, domain.CountryReferences(), nil)
}
func (h *RegionalPolicyHandler) sharedCountries(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	rows, e := h.service.Countries(c.Context(), app.RegionalSubject{}, true)
	if e != nil {
		return informationFailure(c, e)
	}
	policy, e := h.service.Policy(c.Context(), app.RegionalSubject{}, true)
	if e != nil {
		return informationFailure(c, e)
	}
	defaultCode := ""
	for _, r := range rows {
		if r.Default {
			defaultCode = r.Code
		}
	}
	return platformJSON(c, 200, fiber.Map{"countries": rows, "default_country": defaultCode, "billing_currency": commercial.CurrencyKES, "organization_registration_enabled": policy.RegistrationEnabled && !policy.UnderMaintenance(time.Now().UTC())}, nil)
}
func (h *RegionalPolicyHandler) countryHistory(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, _, ok := regionalSubject(c, false)
	if !ok {
		return nil
	}
	code := c.Params("code")
	if len(code) != 2 || strings.ToUpper(code) != code {
		return fail(c, 400, "INVALID_REQUEST", "Choose a valid country.", false)
	}
	rows, e := h.service.CountryHistory(c.Context(), subject, code)
	if e != nil {
		return informationFailure(c, e)
	}
	if rows == nil {
		rows = []domain.CountryConfiguration{}
	}
	return platformJSON(c, 200, rows, nil)
}
func (h *RegionalPolicyHandler) saveCountry(c fiber.Ctx) error {
	var body struct {
		Code     string `json:"country_code"`
		Currency string `json:"currency"`
		Language string `json:"language"`
		Timezone string `json:"timezone"`
		Enabled  bool   `json:"enabled"`
		Default  bool   `json:"is_default"`
	}
	if !bindManagedSettings(c, &body, []string{"country_code", "currency", "language", "timezone", "enabled", "is_default"}, nil) {
		return nil
	}
	subject, key, ok := regionalSubject(c, true)
	if !ok {
		return nil
	}
	version := int64(0)
	if c.Method() == "PUT" {
		if c.Params("code") != body.Code {
			return fail(c, 400, "INVALID_REQUEST", "The country does not match this request.", false)
		}
		var valid bool
		version, valid = ifMatch(c)
		if !valid {
			return fail(c, 400, "INVALID_REQUEST", "Reload the current country version.", false)
		}
	}
	out, e := h.service.SaveCountry(c.Context(), subject, domain.CountryConfiguration{Code: body.Code, Currency: body.Currency, Language: body.Language, Timezone: body.Timezone, Enabled: body.Enabled, Default: body.Default, Version: version}, key)
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, out, nil)
}
func (h *RegionalPolicyHandler) policy(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, _, ok := regionalSubject(c, false)
	if !ok {
		return nil
	}
	out, e := h.service.Policy(c.Context(), subject, false)
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, out, nil)
}
func (h *RegionalPolicyHandler) policyHistory(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, _, ok := regionalSubject(c, false)
	if !ok {
		return nil
	}
	out, e := h.service.PolicyHistory(c.Context(), subject)
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, out, nil)
}
func (h *RegionalPolicyHandler) savePolicy(c fiber.Ctx) error {
	var body struct {
		Registration bool   `json:"organization_registration_enabled"`
		AI           bool   `json:"ai_features_enabled"`
		Uploads      bool   `json:"document_uploads_enabled"`
		MaxBytes     int64  `json:"max_document_bytes"`
		Maintenance  bool   `json:"maintenance_enabled"`
		Message      string `json:"maintenance_message"`
		Start        string `json:"maintenance_starts_at"`
		End          string `json:"maintenance_ends_at"`
		Retention    int32  `json:"restricted_metadata_retention_days"`
	}
	if !bindManagedSettings(c, &body, []string{"organization_registration_enabled", "ai_features_enabled", "document_uploads_enabled", "max_document_bytes", "maintenance_enabled", "maintenance_message", "maintenance_starts_at", "maintenance_ends_at", "restricted_metadata_retention_days"}, nil) {
		return nil
	}
	subject, key, ok := regionalSubject(c, true)
	if !ok {
		return nil
	}
	version, ok := ifMatch(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "Reload the current policy version.", false)
	}
	parse := func(raw string) (*time.Time, bool) {
		if raw == "" {
			return nil, true
		}
		t, e := time.Parse(time.RFC3339, raw)
		if e != nil {
			return nil, false
		}
		t = t.UTC()
		return &t, true
	}
	start, a := parse(body.Start)
	end, b := parse(body.End)
	if !a || !b {
		return fail(c, 400, "INVALID_REQUEST", "Use explicit UTC timestamps or empty values.", false)
	}
	out, e := h.service.SavePolicy(c.Context(), subject, app.OperationalPolicy{Version: version, RegistrationEnabled: body.Registration, AIEnabled: body.AI, UploadsEnabled: body.Uploads, MaxDocumentBytes: body.MaxBytes, MaintenanceEnabled: body.Maintenance, MaintenanceMessage: body.Message, MaintenanceStartsAt: start, MaintenanceEndsAt: end, MetadataRetentionDays: body.Retention}, key)
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, out, nil)
}
