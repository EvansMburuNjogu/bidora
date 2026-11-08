package pilot

import (
	app "bidora/backend/internal/controlplane/application"
	integration "bidora/backend/internal/controlplane/integration"
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/postgres"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type overviewIDs struct{}

func (overviewIDs) NewID() (app.ID, error) { id, e := uuid.NewRandom(); return app.ID(id), e }
func registerPlatformOverview(server *fiber.App, browser *httpapi.BrowserSessionMiddleware, pool *postgres.Pool) error {
	store, e := integration.NewPlatformOverviewStore(pool)
	if e != nil {
		return e
	}
	service, e := app.NewPlatformOverviewService(store, overviewIDs{}, systemClock{})
	if e != nil {
		return e
	}
	if e = httpapi.RegisterPlatformReferenceRoute(server, browser, service); e != nil {
		return e
	}
	return httpapi.RegisterPlatformOverviewRoute(server, browser, service)
}
