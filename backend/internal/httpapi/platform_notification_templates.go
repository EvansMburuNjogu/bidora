package httpapi

import (
	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	"errors"
	"github.com/gofiber/fiber/v3"
)

type PlatformNotificationTemplatesHandler struct{ service *app.TemplateService }

func NewPlatformNotificationTemplatesHandler(s *app.TemplateService) (*PlatformNotificationTemplatesHandler, error) {
	if s == nil {
		return nil, errors.New("notification_templates_handler_invalid")
	}
	return &PlatformNotificationTemplatesHandler{s}, nil
}
func RegisterPlatformNotificationTemplateRoutes(f *fiber.App, b *BrowserSessionMiddleware, h *PlatformNotificationTemplatesHandler) error {
	if f == nil || b == nil || h == nil {
		return errors.New("notification_templates_routes_invalid")
	}
	f.Get("/v1/platform/notification-templates", b.Platform, h.list)
	f.Get("/v1/platform/notification-templates/:template_key", b.Platform, h.read)
	f.Put("/v1/platform/notification-templates/:template_key", b.Platform, h.save)
	f.Get("/v1/platform/notification-templates/:template_key/history", b.Platform, h.history)
	f.Post("/v1/platform/notification-templates/:template_key/preview", b.Platform, h.preview)
	f.Post("/v1/platform/notification-templates/:template_key/test", b.Platform, h.test)
	return nil
}
func templateSubject(c fiber.Ctx) (app.TemplateSubject, bool) {
	s, ok := platformSettingsSubject(c)
	if !ok {
		return app.TemplateSubject{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = templateFailure(c, app.ErrTemplateUnavailable)
		return app.TemplateSubject{}, false
	}
	return app.TemplateSubject{ActorID: domain.ID(s.ActorID), SessionID: domain.ID(s.SessionID), ContextGeneration: s.ContextGeneration, CorrelationID: domain.ID(correlation)}, true
}
func templateFailure(c fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, app.ErrTemplateInvalid):
		return fail(c, 400, "TEMPLATE_INVALID", "Check the template and placeholders.", false)
	case errors.Is(e, app.ErrTemplateForbidden):
		return fail(c, 403, "TEMPLATE_FORBIDDEN", "This operation is not permitted.", false)
	case errors.Is(e, app.ErrTemplateConflict):
		return fail(c, 409, "TEMPLATE_CONFLICT", "Settings changed or the test limit was reached. Reload before retrying.", false)
	default:
		return fail(c, 503, "TEMPLATE_UNAVAILABLE", "Notifications are temporarily unavailable.", true)
	}
}
func (h *PlatformNotificationTemplatesHandler) list(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	s, ok := templateSubject(c)
	if !ok {
		return nil
	}
	v, e := h.service.List(c.Context(), s)
	if e != nil {
		return templateFailure(c, e)
	}
	return platformJSON(c, 200, v, nil)
}
func (h *PlatformNotificationTemplatesHandler) read(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	s, ok := templateSubject(c)
	if !ok {
		return nil
	}
	v, e := h.service.Read(c.Context(), s, c.Params("template_key"))
	if e != nil {
		return templateFailure(c, e)
	}
	return platformJSON(c, 200, v, nil)
}
func (h *PlatformNotificationTemplatesHandler) history(c fiber.Ctx) error {
	if !settingsEmptyRead(c) {
		return nil
	}
	s, ok := templateSubject(c)
	if !ok {
		return nil
	}
	v, e := h.service.History(c.Context(), s, c.Params("template_key"))
	if e != nil {
		return templateFailure(c, e)
	}
	return platformJSON(c, 200, v, nil)
}

type notificationTemplateBody struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Enabled bool   `json:"enabled"`
}

func (h *PlatformNotificationTemplatesHandler) save(c fiber.Ctx) error {
	var body notificationTemplateBody
	if !bindManagedSettings(c, &body, []string{"name", "subject", "body", "enabled"}, nil) {
		return nil
	}
	s, ok := templateSubject(c)
	if !ok {
		return nil
	}
	version, key, _, ok := managedSettingsMutation(c)
	if !ok {
		return nil
	}
	v, e := h.service.Save(c.Context(), app.TemplateSave{Subject: s, Template: app.Template{Key: c.Params("template_key"), Name: body.Name, Subject: body.Subject, Body: body.Body, Enabled: body.Enabled, Version: version}, IdempotencyKey: key})
	if e != nil {
		return templateFailure(c, e)
	}
	return platformJSON(c, 200, v, nil)
}
func (h *PlatformNotificationTemplatesHandler) preview(c fiber.Ctx) error {
	var body notificationTemplateBody
	if !bindManagedSettings(c, &body, []string{"name", "subject", "body", "enabled"}, nil) {
		return nil
	}
	s, ok := templateSubject(c)
	if !ok {
		return nil
	}
	current, e := h.service.Read(c.Context(), s, c.Params("template_key"))
	if e != nil {
		return templateFailure(c, e)
	}
	draft := app.Template{Key: current.Key, Name: body.Name, Subject: body.Subject, Body: body.Body, Enabled: true, Version: current.Version}
	v, e := app.RenderTemplate(draft, app.TemplatePreviewValues())
	if e != nil {
		return templateFailure(c, e)
	}
	return platformJSON(c, 200, fiber.Map{"subject": v.Subject, "body": v.Text, "preview_only": true}, nil)
}
func (h *PlatformNotificationTemplatesHandler) test(c fiber.Ctx) error {
	var body struct{}
	if !bindManagedSettings(c, &body, nil, nil) {
		return nil
	}
	s, ok := templateSubject(c)
	if !ok {
		return nil
	}
	version, key, _, ok := managedSettingsMutation(c)
	if !ok {
		return nil
	}
	status, e := h.service.Test(c.Context(), s, c.Params("template_key"), version, key)
	if e != nil {
		return templateFailure(c, e)
	}
	return platformJSON(c, 200, fiber.Map{"status": status}, nil)
}
