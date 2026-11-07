package pilot

import (
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/browser"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/postgres"
	tenant "bidora/backend/internal/tenancy/application"
	"context"
	"github.com/gofiber/fiber/v3"
	"time"
)

func registerRegionalRuntime(f *fiber.App, b *httpapi.BrowserSessionMiddleware, pool *postgres.Pool, config Config) error {
	store, e := persistence.NewRegionalPolicyStore(pool)
	if e != nil {
		return e
	}
	service, e := app.NewRegionalPolicyService(store, platformSettingIDs{}, systemClock{})
	if e != nil {
		return e
	}
	httpapi.RegisterRegionalPolicyRoutes(f, b, httpapi.NewRegionalPolicyHandler(service))
	ips, e := browser.NewClientIPResolver(config.TrustedProxies)
	if e != nil {
		return e
	}
	if e = b.ConfigureRequestAudit(ips); e != nil {
		return e
	}
	b.ConfigureOperationalHooks(store.Gate, nil)
	return nil
}
func organizationRegionalPolicy(ctx context.Context, db postgres.DBTX, c tenant.OrganizationCreationCommit) error {
	owner, e := persistence.NewRegionalPolicyTx(db)
	if e != nil {
		return e
	}
	policy, e := owner.Policy(ctx, true)
	if e != nil || !policy.Valid() {
		return tenant.ErrOrganizationCreationUnavailable
	}
	if !policy.RegistrationEnabled || policy.UnderMaintenance(time.Now().UTC()) {
		return tenant.ErrOrganizationCreationUnavailable
	}
	country, e := owner.Country(ctx, c.Country, true)
	if e != nil || !country.Enabled || c.Timezone != country.Timezone || c.Locale != country.Locale || c.PreferredLanguage != country.Language {
		return tenant.ErrOrganizationCreationInvalid
	}
	return nil
}
