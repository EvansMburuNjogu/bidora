package httpapi

import (
	"context"
	"errors"

	platformapp "bidora/backend/internal/platformadmin/application"
	tenancyapp "bidora/backend/internal/tenancy/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformTenantDirectoryUseCase interface {
	List(context.Context, tenancyapp.PlatformDirectorySubject, tenancyapp.PlatformTenantFilter, tenancyapp.PlatformTenantPage) (tenancyapp.PlatformTenantList, error)
	Get(context.Context, tenancyapp.PlatformDirectorySubject, tenancyapp.ID) (tenancyapp.PlatformTenant, error)
}

type PlatformTenantDirectoryHandler struct {
	usecase PlatformTenantDirectoryUseCase
	clock   BrowserClock
}

func NewPlatformTenantDirectoryHandler(usecase PlatformTenantDirectoryUseCase, clock BrowserClock) (*PlatformTenantDirectoryHandler, error) {
	if usecase == nil || clock == nil {
		return nil, errors.New("platform_tenant_directory_handler_invalid")
	}
	return &PlatformTenantDirectoryHandler{usecase: usecase, clock: clock}, nil
}

func RegisterPlatformTenantDirectoryRoutes(app *fiber.App, middleware *BrowserSessionMiddleware, handler *PlatformTenantDirectoryHandler) error {
	if app == nil || middleware == nil || handler == nil {
		return errors.New("platform_tenant_directory_routes_invalid")
	}
	app.Get("/v1/platform/tenants", middleware.Platform, handler.list)
	app.Get("/v1/platform/tenants/:tenant_id", middleware.Platform, handler.get)
	return nil
}

func (h *PlatformTenantDirectoryHandler) list(c fiber.Ctx) error {
	subject, ok := h.subject(c)
	if !ok {
		return nil
	}
	page, values, ok := platformPage(c, "state", "search", "country")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var filter tenancyapp.PlatformTenantFilter
	if value := values.Get("state"); value != "" {
		filter.State = &value
	}
	if values.Has("search") {
		value := values.Get("search")
		filter.Search = &value
	}
	if values.Has("country") {
		value := values.Get("country")
		filter.Country = &value
	}
	tenantPage := tenancyapp.PlatformTenantPage{Limit: page.Limit}
	if page.BeforeTime != nil && page.BeforeID != nil {
		at, id := page.BeforeTime.UTC(), tenancyapp.ID(*page.BeforeID)
		tenantPage.BeforeCreatedAt, tenantPage.BeforeID = &at, &id
	}
	result, err := h.usecase.List(c.Context(), subject, filter, tenantPage)
	if err != nil {
		return platformTenantDirectoryFailure(c, err)
	}
	items := make([]any, len(result.Tenants))
	for index, tenant := range result.Tenants {
		items[index] = platformTenantJSON(tenant)
	}
	return platformJSON(c, 200, map[string]any{"tenants": items}, platformTenantNextCursor(result.NextCursor))
}

func (h *PlatformTenantDirectoryHandler) get(c fiber.Ctx) error {
	subject, ok := h.subject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	parsed, err := uuid.Parse(c.Params("tenant_id"))
	if err != nil || parsed == uuid.Nil {
		return fail(c, 404, "PLATFORM_TENANT_NOT_FOUND", "The tenant was not found.", false)
	}
	tenant, err := h.usecase.Get(c.Context(), subject, tenancyapp.ID(parsed))
	if err != nil {
		return platformTenantDirectoryFailure(c, err)
	}
	return platformJSON(c, 200, platformTenantJSON(tenant), nil)
}

func (h *PlatformTenantDirectoryHandler) subject(c fiber.Ctx) (tenancyapp.PlatformDirectorySubject, bool) {
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return tenancyapp.PlatformDirectorySubject{}, false
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return tenancyapp.PlatformDirectorySubject{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return tenancyapp.PlatformDirectorySubject{}, false
	}
	observedAt := h.clock.Now().UTC()
	if observedAt.IsZero() {
		_ = fail(c, 503, "PLATFORM_TENANT_DIRECTORY_UNAVAILABLE", "The tenant directory is temporarily unavailable.", true)
		return tenancyapp.PlatformDirectorySubject{}, false
	}
	return tenancyapp.PlatformDirectorySubject{ActorID: tenancyapp.ID(principal.UserID), SessionID: tenancyapp.ID(principal.SessionID), PlatformContextGeneration: generation, ObservedAt: observedAt}, true
}

func platformTenantDirectoryFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, tenancyapp.ErrPlatformDirectoryInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(err, tenancyapp.ErrPlatformDirectoryForbidden):
		return fail(c, 403, "PLATFORM_TENANT_DIRECTORY_FORBIDDEN", "Tenant directory access is forbidden.", false)
	case errors.Is(err, tenancyapp.ErrPlatformDirectoryAssuranceRequired):
		return fail(c, 403, "PLATFORM_ASSURANCE_REQUIRED", "Recent TOTP authentication is required.", false)
	case errors.Is(err, tenancyapp.ErrPlatformDirectoryContextStale):
		return fail(c, 409, "PLATFORM_CONTEXT_STALE", "The platform context is stale.", false)
	case errors.Is(err, tenancyapp.ErrPlatformTenantNotFound):
		return fail(c, 404, "PLATFORM_TENANT_NOT_FOUND", "The tenant was not found.", false)
	default:
		return fail(c, 503, "PLATFORM_TENANT_DIRECTORY_UNAVAILABLE", "The tenant directory is temporarily unavailable.", true)
	}
}

func platformTenantJSON(tenant tenancyapp.PlatformTenant) any {
	return map[string]any{
		"id": tenantUUID(tenant.ID), "name": tenant.Name, "tenant_type": tenant.Type, "state": tenant.State,
		"country": tenant.Country, "timezone": tenant.Timezone, "locale": tenant.Locale,
		"assignment_generation": tenant.AssignmentGeneration, "version": tenant.Version,
		"assignment_present": tenant.AssignmentPresent, "common_schema": platformTenantSchemaJSON(tenant.CommonSchema),
		"tenant_schema": platformTenantSchemaJSON(tenant.TenantSchema), "created_at": tenant.CreatedAt, "updated_at": tenant.UpdatedAt,
	}
}

func platformTenantSchemaJSON(schema *tenancyapp.PlatformTenantSchema) any {
	if schema == nil {
		return nil
	}
	return map[string]any{"current_version": schema.CurrentVersion, "generation": schema.Generation, "state": schema.State, "updated_at": schema.UpdatedAt}
}

func platformTenantNextCursor(page *tenancyapp.PlatformTenantPage) *platformapp.PageCursor {
	if page == nil || page.BeforeCreatedAt == nil || page.BeforeID == nil {
		return nil
	}
	at, id := page.BeforeCreatedAt.UTC(), platformapp.ID(*page.BeforeID)
	return &platformapp.PageCursor{BeforeTime: &at, BeforeID: &id, Limit: page.Limit}
}

func tenantUUID(id tenancyapp.ID) string { return uuid.UUID(id).String() }
