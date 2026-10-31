package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	settingsapp "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformIntegrationSettingsReader interface {
	Active(context.Context, settingsapp.Subject, domain.Kind) (settingsapp.RedactedVersion, error)
	Versions(context.Context, settingsapp.Subject, domain.Kind, *int64, int32) (settingsapp.VersionPage, error)
	Change(context.Context, settingsapp.Subject, domain.ID) (settingsapp.RedactedChange, error)
}
type PlatformIntegrationSettingsHandler struct {
	reader PlatformIntegrationSettingsReader
	clock  BrowserClock
}

func NewPlatformIntegrationSettingsHandler(reader PlatformIntegrationSettingsReader, clock BrowserClock) (*PlatformIntegrationSettingsHandler, error) {
	if reader == nil || clock == nil {
		return nil, errors.New("platform_integration_settings_handler_invalid")
	}
	return &PlatformIntegrationSettingsHandler{reader, clock}, nil
}
func RegisterPlatformIntegrationSettingsReadRoutes(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformIntegrationSettingsHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_integration_settings_routes_invalid")
	}
	app.Get("/v1/platform/integration-settings/:setting_kind", middleware.Platform, handler.active)
	app.Get("/v1/platform/integration-settings/:setting_kind/versions", middleware.Platform, handler.versions)
	app.Get("/v1/platform/integration-setting-changes/:change_id", middleware.Platform, handler.change)
	return nil
}
func (h *PlatformIntegrationSettingsHandler) active(c fiber.Ctx) error {
	subject, ok := h.subject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	kind, ok := settingKind(c.Params("setting_kind"))
	if !ok {
		return fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting was not found.", false)
	}
	result, err := h.reader.Active(c.Context(), subject, kind)
	if err != nil {
		return platformSettingFailure(c, err)
	}
	return platformJSON(c, 200, platformSettingVersionJSON(result), nil)
}
func (h *PlatformIntegrationSettingsHandler) versions(c fiber.Ctx) error {
	subject, ok := h.subject(c)
	if !ok {
		return nil
	}
	kind, ok := settingKind(c.Params("setting_kind"))
	if !ok {
		return fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting was not found.", false)
	}
	values, err := parseStrictQuery(c, "limit", "cursor")
	if err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	limit := int32(25)
	if raw := values.Get("limit"); raw != "" {
		parsed, e := strconv.Atoi(raw)
		if e != nil || parsed < 1 || parsed > 100 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		limit = int32(parsed)
	}
	before, ok := decodeSettingCursor(values.Get("cursor"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.reader.Versions(c.Context(), subject, kind, before, limit)
	if err != nil {
		return platformSettingFailure(c, err)
	}
	items := make([]any, len(result.Versions))
	for i, version := range result.Versions {
		items[i] = platformSettingVersionJSON(version)
	}
	cursor, err := encodeSettingCursor(result.NextSequence)
	if err != nil {
		return platformSettingFailure(c, settingsapp.ErrUnavailable)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(200).JSON(platformResponse{Data: items, Meta: platformMeta{TraceID: c.GetRespHeader("X-Request-ID"), NextCursor: cursor}})
}
func (h *PlatformIntegrationSettingsHandler) change(c fiber.Ctx) error {
	subject, ok := h.subject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	parsed, err := uuid.Parse(c.Params("change_id"))
	if err != nil || parsed == uuid.Nil {
		return fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting change was not found.", false)
	}
	result, err := h.reader.Change(c.Context(), subject, domain.ID(parsed))
	if err != nil {
		return platformSettingFailure(c, err)
	}
	return platformJSON(c, 200, platformSettingChangeJSON(result), nil)
}
func (h *PlatformIntegrationSettingsHandler) subject(c fiber.Ctx) (settingsapp.Subject, bool) {
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return settingsapp.Subject{}, false
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return settingsapp.Subject{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return settingsapp.Subject{}, false
	}
	if h.clock.Now().UTC().IsZero() {
		_ = fail(c, 503, "PLATFORM_SETTINGS_UNAVAILABLE", "Platform settings are temporarily unavailable.", true)
		return settingsapp.Subject{}, false
	}
	return settingsapp.Subject{ActorID: domain.ID(principal.UserID), SessionID: domain.ID(principal.SessionID), ContextGeneration: generation}, true
}
func settingKind(value string) (domain.Kind, bool) {
	kind := domain.Kind(value)
	return kind, kind == domain.SMTP || kind == domain.OpenAI || kind == domain.PesaPal
}
func platformSettingFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, settingsapp.ErrInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(err, settingsapp.ErrForbidden):
		return fail(c, 403, "PLATFORM_SETTING_FORBIDDEN", "Platform setting access is forbidden.", false)
	case errors.Is(err, settingsapp.ErrNotFound):
		return fail(c, 404, "PLATFORM_SETTING_NOT_FOUND", "The setting was not found.", false)
	case errors.Is(err, settingsapp.ErrConflict):
		return fail(c, 409, "SETTING_STATE_CONFLICT", "The setting state changed or conflicts with this request.", false)
	case errors.Is(err, settingsapp.ErrInProgress):
		return fail(c, 409, "SETTING_STATE_CONFLICT", "The setting operation is already in progress.", true)
	default:
		return fail(c, 503, "PLATFORM_SETTINGS_UNAVAILABLE", "Platform settings are temporarily unavailable.", true)
	}
}
func platformSettingVersionJSON(v settingsapp.RedactedVersion) any {
	configuration := any(nil)
	if v.SMTP != nil {
		configuration = map[string]any{"setting_kind": "SMTP", "host": v.SMTP.Host, "port": v.SMTP.Port, "credential_reference_configured": true, "trust_roots_reference_configured": true, "starttls_required": true, "sender_name": v.SMTP.SenderName, "sender_address": v.SMTP.SenderAddress, "reply_to_address": v.SMTP.ReplyToAddress, "connect_timeout_seconds": v.SMTP.ConnectTimeoutSeconds, "command_timeout_seconds": v.SMTP.CommandTimeoutSeconds, "delivery_timeout_seconds": v.SMTP.DeliveryTimeoutSeconds, "max_concurrency": v.SMTP.MaxConcurrency}
	}
	if v.OpenAI != nil {
		configuration = map[string]any{"setting_kind": "OPENAI", "enabled": v.OpenAI.Enabled, "endpoint": domain.OpenAIEndpoint, "model": v.OpenAI.Model, "api_key_reference_configured": true, "trust_roots_reference_configured": true, "store": false, "timeout_seconds": v.OpenAI.TimeoutSeconds, "max_concurrency": v.OpenAI.MaxConcurrency}
	}
	if v.PesaPal != nil {
		c := v.PesaPal
		configuration = map[string]any{"setting_kind": "PESAPAL", "enabled": c.Enabled, "environment": c.Environment, "merchant_name": c.MerchantName, "credential_reference_configured": c.CredentialsConfigured, "notification_id": optionalSettingID(c.NotificationID), "api_endpoint": domain.PesaPalConfiguration{Environment: c.Environment}.API()}
	}
	return map[string]any{"id": uuid.UUID(v.ID).String(), "setting_kind": v.Kind, "sequence": v.Sequence, "state": v.State, "version": v.Version, "configuration": configuration, "test_status": v.TestStatus, "test_code": nullableText(v.TestCode), "created_at": v.CreatedAt, "tested_at": v.TestedAt, "activated_at": v.ActivatedAt, "superseded_at": v.SupersededAt}
}
func platformSettingChangeJSON(v settingsapp.RedactedChange) any {
	return map[string]any{"id": uuid.UUID(v.ID).String(), "setting_kind": v.Kind, "target_version_id": uuid.UUID(v.TargetVersionID).String(), "action": v.Action, "state": v.State, "version": v.Version, "maker_user_id": uuid.UUID(v.MakerUserID).String(), "checker_user_id": optionalSettingID(v.CheckerUserID), "created_at": v.CreatedAt, "expires_at": v.ExpiresAt, "decided_at": v.DecidedAt}
}
func optionalSettingID(id domain.ID) any {
	if id.IsZero() {
		return nil
	}
	return uuid.UUID(id).String()
}
func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func parseStrictQuery(c fiber.Ctx, allowed ...string) (url.Values, error) {
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		set[key] = true
	}
	for key, entries := range values {
		if !set[key] || len(entries) != 1 {
			return nil, errors.New("invalid_query")
		}
	}
	return values, nil
}
func decodeSettingCursor(raw string) (*int64, bool) {
	if raw == "" {
		return nil, true
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) > 128 {
		return nil, false
	}
	var value struct {
		Before int64 `json:"before"`
	}
	if json.Unmarshal(decoded, &value) != nil || value.Before < 1 {
		return nil, false
	}
	return &value.Before, true
}
func encodeSettingCursor(before *int64) (*string, error) {
	if before == nil {
		return nil, nil
	}
	raw, err := json.Marshal(struct {
		Before int64 `json:"before"`
	}{*before})
	if err != nil {
		return nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	return &value, nil
}
