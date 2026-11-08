package httpapi

import (
	app "bidora/backend/internal/tenancy/application"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformTenantLifecycleUseCase interface {
	Change(context.Context, app.PlatformTenantCommand) (app.PlatformTenantReceipt, error)
}

func RegisterPlatformTenantLifecycleRoutes(server *fiber.App, browser *BrowserSessionMiddleware, service PlatformTenantLifecycleUseCase, clock BrowserClock) error {
	if server == nil || browser == nil || service == nil || clock == nil {
		return errors.New("tenant_lifecycle_handler_invalid")
	}
	for _, op := range []struct{ path, action string }{{"suspend", "SUSPEND"}, {"reactivate", "REACTIVATE"}} {
		action := op.action
		server.Post("/v1/platform/tenants/:tenant_id/"+op.path, browser.Platform, func(c fiber.Ctx) error {
			if len(c.Request().URI().QueryString()) != 0 {
				return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			}
			var body struct {
				ExpectedVersion int64  `json:"expected_version"`
				Reason          string `json:"reason"`
			}
			if len(c.Body()) > 4096 {
				return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
			}
			if e := bindOrganization(c, &body); e != nil {
				return fail(c, 400, "INVALID_REQUEST", "Enter a reason and reload the organisation before trying again.", false)
			}
			if !requireOrganizationFields(c.Body(), "expected_version", "reason") {
				return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			}
			p, ok := SessionPrincipalFrom(c)
			if !ok {
				return fail(c, 401, "SESSION_INVALID", "Sign in to continue.", false)
			}
			gen, ok := platformGeneration(c)
			if !ok {
				return nil
			}
			target, e := uuid.Parse(c.Params("tenant_id"))
			if e != nil || target == uuid.Nil {
				return fail(c, 404, "PLATFORM_TENANT_NOT_FOUND", "Organisation not found.", false)
			}
			correlation, e := uuid.Parse(c.GetRespHeader("X-Request-ID"))
			if e != nil {
				return fail(c, 500, "INTERNAL_ERROR", "The request could not be completed.", false)
			}
			key, ok := singleHeader(c, IdempotencyKeyHeader)
			if !ok {
				return invalidHeader(c, IdempotencyKeyHeader, "required")
			}
			if parsed, e := uuid.Parse(key); e != nil || parsed == uuid.Nil {
				return invalidHeader(c, IdempotencyKeyHeader, "uuid")
			}
			result, e := service.Change(c.Context(), app.PlatformTenantCommand{Subject: app.PlatformDirectorySubject{ActorID: app.ID(p.UserID), SessionID: app.ID(p.SessionID), PlatformContextGeneration: gen, ObservedAt: clock.Now().UTC()}, TenantID: app.ID(target), CorrelationID: app.ID(correlation), Operation: action, Reason: body.Reason, IdempotencyKey: key, ExpectedVersion: body.ExpectedVersion})
			if errors.Is(e, app.ErrPlatformTenantConflict) {
				return fail(c, 409, "PLATFORM_TENANT_CONFLICT", "Organisation status changed or its services are not ready. Reload before trying again.", false)
			}
			if e != nil {
				return platformTenantDirectoryFailure(c, e)
			}
			if result.Replay {
				c.Set("Idempotency-Replayed", "true")
			}
			return platformJSON(c, 200, fiber.Map{"tenant_id": tenantUUID(result.TenantID), "state": result.State, "version": result.Version}, nil)
		})
	}
	return nil
}
