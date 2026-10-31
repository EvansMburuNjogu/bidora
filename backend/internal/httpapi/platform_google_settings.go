package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"bidora/backend/internal/identity/browser"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformGoogleSettingsHandler struct {
	service    *app.GoogleService
	protection *browser.Protection
}

func NewPlatformGoogleSettingsHandler(s *app.GoogleService, p *browser.Protection) (*PlatformGoogleSettingsHandler, error) {
	if s == nil || p == nil {
		return nil, errors.New("google_settings_handler_invalid")
	}
	return &PlatformGoogleSettingsHandler{s, p}, nil
}
func RegisterPlatformGoogleSettingsRoutes(f *fiber.App, b *BrowserSessionMiddleware, h *PlatformGoogleSettingsHandler) error {
	if f == nil || b == nil || h == nil {
		return errors.New("google_settings_routes_invalid")
	}
	f.Get("/v1/platform/settings/google", b.Platform, h.read)
	f.Put("/v1/platform/settings/google", b.Platform, h.save)
	f.Get("/v1/platform/settings/google/history", b.Platform, h.history)
	f.Post("/v1/platform/settings/google/test", b.Platform, h.test)
	f.Get("/v1/auth/google/status", h.status)
	return nil
}
func googleConfigurationJSON(v app.GoogleConfiguration) fiber.Map {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "version": v.Version, "enabled": v.Enabled, "allow_signup": v.AllowSignup, "client_id": v.ClientID, "allowed_domains": v.AllowedDomains, "secret_configured": v.SecretConfigured, "connection_status": v.ConnectionStatus, "runtime_configured": v.RuntimeConfigured, "created_at": v.CreatedAt}
}
func (h *PlatformGoogleSettingsHandler) read(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	v, e := h.service.Read(c.Context(), subject)
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, googleConfigurationJSON(v), nil)
}
func (h *PlatformGoogleSettingsHandler) history(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	rows, e := h.service.History(c.Context(), subject)
	if e != nil {
		return informationFailure(c, e)
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, v := range rows {
		out = append(out, googleConfigurationJSON(v))
	}
	return platformJSON(c, 200, out, nil)
}

type googleSettingsBody struct {
	Enabled      bool     `json:"enabled"`
	Signup       bool     `json:"allow_signup"`
	ClientID     string   `json:"client_id"`
	ClientSecret *string  `json:"client_secret"`
	Domains      []string `json:"allowed_domains"`
}

func (h *PlatformGoogleSettingsHandler) save(c fiber.Ctx) error {
	var body googleSettingsBody
	if !bindManagedSettings(c, &body, []string{"enabled", "allow_signup", "client_id", "allowed_domains"}, []string{"client_secret"}) {
		return nil
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	version, key, correlation, ok := managedSettingsMutation(c)
	if !ok {
		return nil
	}
	v, e := h.service.Save(c.Context(), app.GoogleSave{Subject: subject, Configuration: app.GoogleConfiguration{Version: version, Enabled: body.Enabled, AllowSignup: body.Signup, ClientID: body.ClientID, AllowedDomains: body.Domains}, ClientSecret: body.ClientSecret, IdempotencyKey: key, CorrelationID: domain.ID(correlation)})
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, googleConfigurationJSON(v), nil)
}
func (h *PlatformGoogleSettingsHandler) test(c fiber.Ctx) error {
	var body struct{}
	if !bindManagedSettings(c, &body, nil, nil) {
		return nil
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	version, key, correlation, ok := managedSettingsMutation(c)
	if !ok {
		return nil
	}
	result, e := h.service.Test(c.Context(), subject, version, key, domain.ID(correlation))
	if e != nil {
		return informationFailure(c, e)
	}
	return platformJSON(c, 200, fiber.Map{"status": result}, nil)
}
func (h *PlatformGoogleSettingsHandler) status(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	if h.protection.ValidateOrigin(c.Get("Origin")) != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The request was rejected.", false)
	}
	v, e := h.service.Runtime(c.Context())
	if e != nil {
		return informationFailure(c, e)
	}
	view := h.service.PublicStatus(v)
	return platformJSON(c, 200, view, nil)
}
func settingsEmptyRead(c fiber.Ctx) bool {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return false
	}
	return true
}

// Bounded strict JSON binding for these two configuration operations, not a
// settings persistence framework. Reject duplicate, null and unknown fields.
func bindManagedSettings(c fiber.Ctx, target any, required, optional []string) bool {
	raw := c.Body()
	if len(raw) < 2 || len(raw) > 16384 || len(c.Request().URI().QueryString()) != 0 || strings.Split(c.Get("Content-Type"), ";")[0] != "application/json" || !utf8.Valid(raw) {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, e := decoder.Token()
	if e != nil || token != json.Delim('{') {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return false
	}
	allowed := map[string]bool{}
	seen := map[string]bool{}
	for _, k := range append(append([]string{}, required...), optional...) {
		allowed[k] = true
	}
	for decoder.More() {
		token, e = decoder.Token()
		key, ok := token.(string)
		if e != nil || !ok || seen[key] || !allowed[key] {
			_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			return false
		}
		seen[key] = true
		var v json.RawMessage
		if decoder.Decode(&v) != nil || string(v) == "null" {
			_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			return false
		}
	}
	token, e = decoder.Token()
	if e != nil || token != json.Delim('}') || decoder.Decode(new(any)) != io.EOF {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return false
	}
	for _, k := range required {
		if !seen[k] {
			_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			return false
		}
	}
	if json.Unmarshal(raw, target) != nil {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return false
	}
	return true
}
func managedSettingsMutation(c fiber.Ctx) (int64, string, [16]byte, bool) {
	version, ok := ifMatch(c)
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return 0, "", [16]byte{}, false
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	id, e := uuid.Parse(key)
	if !ok || e != nil || id == uuid.Nil {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return 0, "", [16]byte{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 503, "SETTINGS_UNAVAILABLE", "Settings are unavailable.", true)
		return 0, "", [16]byte{}, false
	}
	return version, id.String(), [16]byte(correlation), true
}
