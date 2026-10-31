package pilot

import (
	"bidora/backend/internal/httpapi"
	identity "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	app "bidora/backend/internal/platformsettings/application"
	"bidora/backend/internal/platformsettings/domain"
	"bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/postgres"
	tenant "bidora/backend/internal/tenancy/application"
	"context"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"net/netip"
	"strings"
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
	var observe func(fiber.Ctx, identity.SessionPrincipal)
	if config.DevelopmentLoopbackHTTP {
		ips, e := browser.NewClientIPResolver(config.TrustedProxies)
		if e != nil {
			return e
		}
		observe = func(c fiber.Ctx, principal identity.SessionPrincipal) {
			if !strings.HasPrefix(c.Path(), "/v1/platform/") || c.Response().StatusCode() < 200 || c.Response().StatusCode() >= 300 || c.RemoteAddr() == nil {
				return
			}
			peer, e := netip.ParseAddrPort(c.RemoteAddr().String())
			if e != nil {
				return
			}
			ip, e := ips.Resolve(peer, c.Get("X-Forwarded-For"))
			if e != nil {
				return
			}
			correlation, e := uuid.Parse(c.GetRespHeader("X-Request-ID"))
			if e != nil {
				return
			}
			ctx, cancel := context.WithTimeout(c.Context(), time.Second)
			defer cancel()
			_ = store.ObserveRequest(ctx, domain.ID(principal.UserID), domain.ID(correlation), ip, c.Get("User-Agent"), time.Now().UTC())
		}
	}
	b.ConfigureOperationalHooks(store.Gate, observe)
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
