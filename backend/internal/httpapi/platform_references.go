package httpapi

import (
	app "bidora/backend/internal/controlplane/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"net/url"
)

type PlatformReferenceUseCase interface {
	Lookup(context.Context, app.PlatformAuditSubject, string, string) (app.PlatformReferencePage, error)
}

func RegisterPlatformReferenceRoute(server *fiber.App, browser *BrowserSessionMiddleware, service PlatformReferenceUseCase) error {
	if server == nil || browser == nil || service == nil {
		return errors.New("reference_handler_invalid")
	}
	server.Get("/v1/platform/references/:kind", browser.Platform, func(c fiber.Ctx) error {
		query, e := url.ParseQuery(string(c.Request().URI().QueryString()))
		if e != nil || len(c.Body()) != 0 || len(query) > 1 || len(query["search"]) > 1 || len(query) == 1 && !query.Has("search") {
			return fail(c, 400, "INVALID_REQUEST", "The search is invalid.", false)
		}
		p, ok := SessionPrincipalFrom(c)
		if !ok {
			return fail(c, 401, "SESSION_INVALID", "Sign in to continue.", false)
		}
		gen, ok := platformGeneration(c)
		if !ok {
			return nil
		}
		correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
		if !ok {
			return fail(c, 500, "INTERNAL_ERROR", "The request could not be completed.", false)
		}
		result, e := service.Lookup(c.Context(), app.PlatformAuditSubject{UserID: app.ID(p.UserID), SessionID: app.ID(p.SessionID), CorrelationID: app.ID(correlation), ContextGeneration: gen}, c.Params("kind"), query.Get("search"))
		if errors.Is(e, app.ErrPlatformAuditForbidden) {
			return fail(c, 403, "PLATFORM_REFERENCE_FORBIDDEN", "This search is not available for your account.", false)
		}
		if errors.Is(e, app.ErrPlatformAuditInvalid) {
			return fail(c, 400, "INVALID_REQUEST", "The search is invalid.", false)
		}
		if e != nil {
			return fail(c, 503, "PLATFORM_REFERENCE_UNAVAILABLE", "Search is temporarily unavailable.", true)
		}
		rows := make([]any, len(result.Items))
		for i, r := range result.Items {
			rows[i] = fiber.Map{"id": uuid.UUID(r.ID).String(), "label": r.Label}
		}
		return platformJSON(c, 200, fiber.Map{"items": rows, "has_more": result.HasMore}, nil)
	})
	return nil
}
