package httpapi

import (
	app "bidora/backend/internal/controlplane/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
)

type PlatformOverviewUseCase interface {
	Read(context.Context, app.PlatformAuditSubject) (app.PlatformOverview, error)
}

func RegisterPlatformOverviewRoute(server *fiber.App, browser *BrowserSessionMiddleware, service PlatformOverviewUseCase) error {
	if server == nil || browser == nil || service == nil {
		return errors.New("overview_handler_invalid")
	}
	server.Get("/v1/platform/overview", browser.Platform, func(c fiber.Ctx) error {
		if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		p, ok := SessionPrincipalFrom(c)
		if !ok {
			return fail(c, 401, "SESSION_INVALID", "Sign in to continue.", false)
		}
		generation, ok := platformGeneration(c)
		if !ok {
			return nil
		}
		correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
		if !ok {
			return fail(c, 500, "INTERNAL_ERROR", "The request could not be completed.", false)
		}
		result, e := service.Read(c.Context(), app.PlatformAuditSubject{UserID: app.ID(p.UserID), SessionID: app.ID(p.SessionID), CorrelationID: app.ID(correlation), ContextGeneration: generation})
		if errors.Is(e, app.ErrPlatformAuditForbidden) {
			return fail(c, 403, "PLATFORM_OVERVIEW_FORBIDDEN", "Verify your account to view the overview.", false)
		}
		if e != nil {
			return fail(c, 503, "PLATFORM_OVERVIEW_UNAVAILABLE", "The overview is temporarily unavailable.", true)
		}
		return platformJSON(c, 200, result, nil)
	})
	return nil
}
