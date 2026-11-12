package pilot

import (
	"errors"

	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/postgres"
	tenantapp "bidora/backend/internal/tenancy/application"
	"bidora/backend/internal/tenancy/creationredis"
	tenantpersistence "bidora/backend/internal/tenancy/persistence"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// BuildTenderpreneurOrganizationCreationHandler composes the approved intake
// boundary. Schema migrations remain an explicit maintenance operation.
func BuildTenderpreneurOrganizationCreationHandler(config Config, control *postgres.Pool, redisClient *attemptredis.ClientRunner) (*httpapi.TenderpreneurOrganizationCreationHandler, error) {
	if control == nil || redisClient == nil {
		return nil, errors.New("organization_creation_composition_invalid")
	}
	deriver, err := attempt.NewDeriver(config.AttemptHMACSecret)
	if err != nil {
		return nil, errors.New("organization_creation_composition_invalid")
	}
	var protection *browser.Protection
	if config.DevelopmentLoopbackHTTP {
		protection, err = browser.NewLoopbackDevelopmentProtection(config.AllowedOrigins)
	} else {
		protection, err = browser.NewProtection(config.AllowedOrigins)
	}
	if err != nil {
		return nil, errors.New("organization_creation_composition_invalid")
	}
	ips, err := browser.NewClientIPResolver(config.TrustedProxies)
	if err != nil {
		return nil, errors.New("organization_creation_composition_invalid")
	}
	limiter, err := creationredis.New(redisClient, config.RedisPrefix)
	if err != nil {
		return nil, err
	}
	store, err := tenantpersistence.NewOrganizationCreationStore(control)
	if err != nil {
		return nil, err
	}
	if err = store.EnableRegionalPolicy(organizationRegionalPolicy); err != nil {
		return nil, err
	}
	service, err := tenantapp.NewOrganizationCreationService(store, limiter, organizationCreationIDs{}, systemClock{})
	if err != nil {
		return nil, err
	}
	return httpapi.NewTenderpreneurOrganizationCreationHandler(service, protection, ips, deriver)
}

func registerOrganizationCreationRuntime(app *fiber.App, browserMiddleware *httpapi.BrowserSessionMiddleware, config Config, control *postgres.Pool, redisClient *attemptredis.ClientRunner) error {
	if app == nil || browserMiddleware == nil {
		return errors.New("organization_creation_composition_invalid")
	}
	handler, err := BuildTenderpreneurOrganizationCreationHandler(config, control, redisClient)
	if err != nil {
		return err
	}
	app.Get(httpapi.TenderpreneurOrganizationCreationPath, browserMiddleware.OriginBound, handler.Status)
	// Creation stays unmounted until a provisioner can advance the resulting
	// durable jobs. Accepting them now would strand new organizations.
	return nil
}

type organizationCreationIDs struct{}

func (organizationCreationIDs) NewID() (tenantapp.ID, error) { return tenantapp.ID(uuid.New()), nil }
