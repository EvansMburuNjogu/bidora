package pilot

import (
	"bidora/backend/internal/commercial"
	commercialapp "bidora/backend/internal/commercial/application"
	money "bidora/backend/internal/commercial/domain"
	commercialpersistence "bidora/backend/internal/commercial/persistence"
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/pesapalruntime"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"context"
	"github.com/gofiber/fiber/v3"
)

func registerManagedPesaPal(f *fiber.App, b *httpapi.BrowserSessionMiddleware, pool *postgres.Pool, module *commercial.Module, store *commercialpersistence.Store, config Config) error {
	versions, e := persistence.NewActivePesaPalStore(pool)
	if e != nil {
		return e
	}
	policies, e := persistence.NewRegionalPolicyStore(pool)
	if e != nil {
		return e
	}
	var secrets smtpsecret.Resolver
	if len(config.SMTPVaultKey) == 32 && config.SMTPVaultKeyVersion > 0 {
		secrets, e = smtpsecret.NewVault(pool, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if e != nil {
			return e
		}
	}
	provider, e := pesapalruntime.New(versions, secrets, policies, config.PesaPal, nil)
	if e != nil {
		return e
	}
	if e = module.ConfigurePayments(store, provider, provider, commercialIDs{}, systemClock{}); e != nil {
		return e
	}
	financial, e := httpapi.NewCommercialFinancialHandler(module)
	if e != nil {
		return e
	}
	httpapi.RegisterCommercialPaymentInitiationRoute(f, b, financial)
	callbacks, e := commercialapp.NewPaymentCallbackService(store, commercialIDs{}, systemClock{})
	if e != nil {
		return e
	}
	httpapi.RegisterPesaPalCallbackRoute(f, httpapi.NewPesaPalCallbackHandler(callbacks, systemClock{}))
	return nil
}

// These explicit steps reuse durable claims, leases, exact-version credentials,
// bounded authoritative queries and existing accounting/reversal decisions.
// No provider worker or financial operation is silently started at boot.
func (r *Runtime) ReconcileNextPayment(ctx context.Context, execution money.ID) (bool, error) {
	if r == nil || r.payments == nil {
		return false, commercialapp.ErrPaymentUnavailable
	}
	return r.payments.ReconcileNextPayment(ctx, execution)
}
func (r *IdentityRuntime) ReconcileNextPayment(ctx context.Context, execution money.ID) (bool, error) {
	if r == nil || r.payments == nil {
		return false, commercialapp.ErrPaymentUnavailable
	}
	return r.payments.ReconcileNextPayment(ctx, execution)
}
