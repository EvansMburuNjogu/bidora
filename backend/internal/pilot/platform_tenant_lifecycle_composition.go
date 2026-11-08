package pilot

import (
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/postgres"
	app "bidora/backend/internal/tenancy/application"
	store "bidora/backend/internal/tenancy/persistence"
	"github.com/gofiber/fiber/v3"
)

func registerPlatformTenantLifecycle(server *fiber.App, browser *httpapi.BrowserSessionMiddleware, pool *postgres.Pool) error {
	persistence, e := store.NewPlatformTenantLifecycleStore(pool)
	if e != nil {
		return e
	}
	service, e := app.NewPlatformTenantLifecycleService(persistence)
	if e != nil {
		return e
	}
	return httpapi.RegisterPlatformTenantLifecycleRoutes(server, browser, service, systemClock{})
}
