package pilot

import (
	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/googleoidc"
	app "bidora/backend/internal/platformsettings/application"
	persistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	"github.com/gofiber/fiber/v3"
)

func composeManagedConfiguration(config Config, pool *postgres.Pool, result *identityComposition) error {
	templates, renderer, e := buildNotificationTemplateService(pool, config)
	if e != nil {
		return e
	}
	result.templates = templates
	result.templateRenderer = renderer
	var vault app.GoogleSealer
	if len(config.SMTPVaultKey) == 32 && config.SMTPVaultKeyVersion > 0 {
		v, e := smtpsecret.NewVault(pool, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if e != nil {
			return e
		}
		vault = v
	}
	var tester app.GoogleConnectionTester
	if config.Google != nil {
		transport, e := googleoidc.NewTransport(config.Google.Transport)
		if e != nil {
			return e
		}
		result.googleTransport = transport
		tester = googleKeyTester{transport}
	}
	store, e := persistence.NewGoogleStore(pool)
	if e != nil {
		return e
	}
	service, e := app.NewGoogleService(store, vault, tester, platformSettingIDs{}, systemClock{}, config.Google != nil)
	if e != nil {
		return e
	}
	result.google = service
	return nil
}
func registerManagedSettings(app *fiber.App, browser *httpapi.BrowserSessionMiddleware, identity *identityComposition) error {
	google, e := httpapi.NewPlatformGoogleSettingsHandler(identity.google, identity.protection)
	if e != nil {
		return e
	}
	if e = httpapi.RegisterPlatformGoogleSettingsRoutes(app, browser, google); e != nil {
		return e
	}
	templates, e := httpapi.NewPlatformNotificationTemplatesHandler(identity.templates)
	if e != nil {
		return e
	}
	return httpapi.RegisterPlatformNotificationTemplateRoutes(app, browser, templates)
}
