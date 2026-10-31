package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	settingsapp "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformInformationUseCase interface {
	Shared(context.Context) (settingsapp.Information, error)
	Read(context.Context, settingsapp.Subject) (settingsapp.Information, error)
	Save(context.Context, settingsapp.InformationSave) (settingsapp.Information, error)
}
type PlatformInformationHandler struct{ service PlatformInformationUseCase }

func NewPlatformInformationHandler(service PlatformInformationUseCase) (*PlatformInformationHandler, error) {
	if service == nil {
		return nil, errors.New("platform_information_handler_invalid")
	}
	return &PlatformInformationHandler{service}, nil
}
func RegisterPlatformInformationRoutes(app *fiber.App, browser *BrowserSessionMiddleware, handler *PlatformInformationHandler) error {
	if app == nil || browser == nil || handler == nil {
		return errors.New("platform_information_routes_invalid")
	}
	app.Get("/v1/shared/platform/information", browser.OriginBound, handler.shared)
	app.Get("/v1/platform/information", browser.Platform, handler.read)
	app.Put("/v1/platform/information", browser.Platform, handler.save)
	return nil
}
func platformSettingsSubject(c fiber.Ctx) (settingsapp.Subject, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return settingsapp.Subject{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return settingsapp.Subject{}, false
	}
	return settingsapp.Subject{ActorID: domain.ID(principal.UserID), SessionID: domain.ID(principal.SessionID), ContextGeneration: generation}, true
}
func (h *PlatformInformationHandler) read(c fiber.Ctx) error {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	value, err := h.service.Read(c.Context(), subject)
	if err != nil {
		return informationFailure(c, err)
	}
	c.Set("ETag", fmt.Sprintf("\"%d\"", value.Version))
	return platformJSON(c, 200, informationJSON(value), nil)
}

func (h *PlatformInformationHandler) shared(c fiber.Ctx) error {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if _, ok := SessionPrincipalFrom(c); !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	value, err := h.service.Shared(c.Context())
	if err != nil {
		return informationFailure(c, err)
	}
	return platformJSON(c, 200, informationJSON(value), nil)
}

type informationBody struct {
	PlatformName *string `json:"platform_name"`
	ContactPhone *string `json:"contact_phone"`
	SupportEmail string  `json:"support_email"`
	Website      string  `json:"website"`
}

func (h *PlatformInformationHandler) save(c fiber.Ctx) error {
	if len(c.Request().URI().QueryString()) != 0 || len(c.Body()) < 2 || len(c.Body()) > 4096 || strings.Split(c.Get("Content-Type"), ";")[0] != "application/json" {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := platformSettingsSubject(c)
	if !ok {
		return nil
	}
	version, ok := ifMatch(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	raw, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	key, err := uuid.Parse(raw)
	if err != nil || key == uuid.Nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 503, "PLATFORM_INFORMATION_UNAVAILABLE", "Platform information is temporarily unavailable.", true)
	}
	body, err := decodeInformationBody(c.Body())
	if err != nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	value, err := h.service.Save(c.Context(), settingsapp.InformationSave{PlatformName: body.PlatformName, ContactPhone: body.ContactPhone, Subject: subject, SupportEmail: body.SupportEmail, Website: body.Website, ExpectedVersion: version, IdempotencyKey: key.String(), CorrelationID: domain.ID(correlation)})
	if err != nil {
		return informationFailure(c, err)
	}
	c.Set("ETag", fmt.Sprintf("\"%d\"", value.Version))
	return platformJSON(c, 200, informationJSON(value), nil)
}
func decodeInformationBody(raw []byte) (informationBody, error) {
	var body informationBody
	if !utf8.Valid(raw) {
		return body, settingsapp.ErrInformationInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return body, settingsapp.ErrInformationInvalid
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err = decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return body, settingsapp.ErrInformationInvalid
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || string(value) == "null" {
			return body, settingsapp.ErrInformationInvalid
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			return body, settingsapp.ErrInformationInvalid
		}
		switch name {
		case "platform_name":
			body.PlatformName = &text
		case "contact_phone":
			body.ContactPhone = &text
		case "support_email":
			body.SupportEmail = text
		case "website":
			body.Website = text
		default:
			return body, settingsapp.ErrInformationInvalid
		}
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') || !seen["support_email"] || !seen["website"] || decoder.Decode(new(any)) != io.EOF {
		return body, settingsapp.ErrInformationInvalid
	}
	return body, nil
}

func informationJSON(v settingsapp.Information) any {
	return map[string]any{"platform_name": v.PlatformName, "contact_phone": v.ContactPhone, "support_email": v.SupportEmail, "website": v.Website, "version": v.Version, "updated_at": v.UpdatedAt}
}
func informationFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, settingsapp.ErrInformationInvalid):
		return fail(c, 400, "INVALID_REQUEST", "Check the platform name and contact details.", false)
	case errors.Is(err, settingsapp.ErrInformationForbidden):
		return fail(c, 403, "PLATFORM_INFORMATION_FORBIDDEN", "Access to platform information is forbidden.", false)
	case errors.Is(err, settingsapp.ErrInformationConflict):
		return fail(c, 409, "PLATFORM_INFORMATION_CONFLICT", "The setting changed. Refresh before saving again.", false)
	default:
		return fail(c, 503, "PLATFORM_INFORMATION_UNAVAILABLE", "Platform information is temporarily unavailable.", true)
	}
}
