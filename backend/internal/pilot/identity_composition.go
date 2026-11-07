package pilot

import (
	"bytes"
	"context"
	"errors"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	"bidora/backend/internal/commercial"
	commercialpersistence "bidora/backend/internal/commercial/persistence"
	controlapp "bidora/backend/internal/controlplane/application"
	controlintegration "bidora/backend/internal/controlplane/integration"
	"bidora/backend/internal/dashboard"
	dashboardintegration "bidora/backend/internal/dashboard/integration"
	"bidora/backend/internal/health"
	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/loginresendredis"
	"bidora/backend/internal/identity/mfaredis"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/recoveryredis"
	"bidora/backend/internal/identity/registrationredis"
	notificationapp "bidora/backend/internal/notification/application"
	notificationpersistence "bidora/backend/internal/notification/persistence"
	platformadmin "bidora/backend/internal/platformadmin"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	settingsapp "bidora/backend/internal/platformsettings/application"
	settingspersistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/smtpdelivery"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	tenancypersistence "bidora/backend/internal/tenancy/persistence"
	tenancysql "bidora/backend/internal/tenancy/persistence/sqlc"
	"github.com/gofiber/fiber/v3"
)

type identityComposition struct {
	google                   *settingsapp.GoogleService
	googleTransport          *googleoidc.Transport
	templates                *notificationapp.TemplateService
	templateRenderer         notificationapp.TemplateRenderer
	hasher                   *password.Hasher
	deriver                  *attempt.Deriver
	protection               *browser.Protection
	ipResolver               *browser.ClientIPResolver
	passwordReader           *identitypersistence.PasswordReader
	authenticator            *identityapp.Authenticator
	rehasher                 *identitypersistence.PasswordRehasher
	loginCommitter           *identitypersistence.LoginCommitter
	mfaStore                 *identitypersistence.MFAStore
	mfaService               *identityapp.MFAService
	login                    *httpapi.LoginHandler
	registration             *httpapi.RegistrationHandler
	verification             *httpapi.EmailVerificationHandler
	resend                   *httpapi.EmailVerificationResendHandler
	recovery                 *httpapi.PasswordRecoveryHandler
	delivery                 smtpDelivery
	tenantInvitationDelivery authzapp.InvitationDelivery
}
type smtpDelivery interface {
	identityapp.EmailLoginAvailability
	identityapp.EmailVerificationDelivery
	identityapp.PasswordRecoveryDelivery
	identityapp.EmailLoginCodeDelivery
	DeliverTenantRoleInvitation(context.Context, email.Address, string, string, time.Time) error
	DeliverRendered(context.Context, email.Address, string, string, string) error
}

func composeIdentity(config Config, control *postgres.Pool, redisClient *attemptredis.ClientRunner) (*identityComposition, error) {
	if control == nil || redisClient == nil || config.DummyPasswordPHC == "" || config.MFAKeyRing == nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	if len(config.NotificationFingerprintKey) != 0 || config.NotificationFingerprintKeyVersion != 0 {
		if len(config.NotificationFingerprintKey) != 32 || config.NotificationFingerprintKeyVersion < 1 || len(config.SMTPVaultKey) != 32 || config.SMTPVaultKeyVersion < 1 || bytes.Equal(config.NotificationFingerprintKey, config.SMTPVaultKey) || bytes.Equal(config.NotificationFingerprintKey, config.AttemptHMACSecret) || bytes.Equal(config.NotificationFingerprintKey, config.EmailLoginHMACSecret) {
			return nil, errors.New("pilot_notification_config_invalid")
		}
	}
	hasher, err := password.New(config.PasswordPolicy)
	if err != nil || hasher.ValidatePHC(config.DummyPasswordPHC) != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	deriver, err := attempt.NewDeriver(config.AttemptHMACSecret)
	if err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	var protection *browser.Protection
	if config.DevelopmentLoopbackHTTP {
		protection, err = browser.NewLoopbackDevelopmentProtection(config.AllowedOrigins)
	} else {
		protection, err = browser.NewProtection(config.AllowedOrigins)
	}
	if err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	ipResolver, err := browser.NewClientIPResolver(config.TrustedProxies)
	if err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	attempts, err := attemptredis.NewSecureController(redisClient, config.RedisPrefix)
	if err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	passwordReader, err := identitypersistence.NewPasswordReader(control)
	if err != nil {
		return nil, err
	}
	authenticator, err := identityapp.NewAuthenticator(passwordReader, attempts, deriver, hasher, config.DummyPasswordPHC)
	if err != nil {
		return nil, err
	}
	rehasher, err := identitypersistence.NewPasswordRehasher(control, hasher, systemClock{})
	if err != nil {
		return nil, err
	}
	loginCommitter, err := identitypersistence.NewLoginCommitter(control)
	if err != nil {
		return nil, err
	}
	mfaStore, err := identitypersistence.NewMFAStore(control)
	if err != nil {
		return nil, err
	}
	mfaAttempts, err := mfaredis.New(redisClient, config.RedisPrefix)
	if err != nil {
		return nil, err
	}
	mfaService, err := identityapp.NewMFAService(mfaStore, config.MFAKeyRing, identityapp.SecureSessionTokenGenerator{}, identityIDs{}, systemClock{}, mfaAttemptAdapter{mfaAttempts}, mfaScopeAdapter{deriver})
	if err != nil {
		return nil, err
	}
	mfaService.RequireEmailLogin()
	loginService, err := identityapp.NewLoginService(authenticator, rehasher, identityapp.SecureSessionTokenGenerator{}, identityIDs{}, systemClock{}, loginCommitter)
	if err != nil {
		return nil, err
	}
	if err = loginService.EnableMFA(mfaService); err != nil {
		return nil, err
	}
	loginHandler, err := httpapi.NewLoginHandler(loginService, protection, ipResolver, deriver)
	if err != nil {
		return nil, err
	}
	result := &identityComposition{hasher: hasher, deriver: deriver, protection: protection, ipResolver: ipResolver, passwordReader: passwordReader, authenticator: authenticator, rehasher: rehasher, loginCommitter: loginCommitter, mfaStore: mfaStore, mfaService: mfaService, login: loginHandler}
	if err = composeManagedConfiguration(config, control, result); err != nil {
		return nil, err
	}
	var delivery smtpDelivery
	if len(config.SMTPVaultKey) == 32 && config.SMTPVaultKeyVersion > 0 {
		vault, vaultErr := smtpsecret.NewVault(control, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if vaultErr != nil {
			return nil, errors.New("pilot_runtime_invalid")
		}
		activeStore, storeErr := settingspersistence.NewActiveSMTPStore(control)
		if storeErr != nil {
			return nil, storeErr
		}
		delivery, err = smtpdelivery.New(activeStore, vault)
		if err != nil {
			return nil, err
		}
	}
	if delivery == nil {
		if config.PlatformLoginEmail != nil {
			return nil, errors.New("platform_login_email_config_invalid")
		}
		return result, nil
	}
	if len(config.NotificationFingerprintKey) != 32 || config.NotificationFingerprintKeyVersion < 1 {
		return nil, errors.New("notification_evidence_configuration_required")
	}
	delivery = &configuredSMTPDelivery{base: delivery, renderer: result.templateRenderer}
	var loginDelivery identityapp.EmailLoginCodeDelivery = delivery
	var verificationDelivery identityapp.EmailVerificationDelivery = delivery
	var recoveryDelivery identityapp.PasswordRecoveryDelivery = delivery
	if len(config.NotificationFingerprintKey) != 0 {
		vault, vaultErr := smtpsecret.NewVault(control, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if vaultErr != nil {
			return nil, vaultErr
		}
		recorded, recordErr := BuildIdentityNotificationDelivery(control, vault, config.NotificationFingerprintKey, config.NotificationFingerprintKeyVersion)
		if recordErr != nil {
			return nil, recordErr
		}
		if err = recorded.EnableTemplates(result.templateRenderer); err != nil {
			return nil, err
		}
		verificationDelivery, recoveryDelivery, loginDelivery = recorded, recorded, recorded
		tenantRecorded, _, tenantErr := BuildInvitationNotificationDelivery(control, vault, config.NotificationFingerprintKey, config.NotificationFingerprintKeyVersion)
		if tenantErr != nil {
			return nil, tenantErr
		}
		result.tenantInvitationDelivery = tenantRecorded
	}
	if err = mfaService.EnableEmailLogin(loginDelivery, config.EmailLoginHMACSecret); err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	mfaStore.EnablePlatformEmailLogin()
	if err = mfaService.EnablePlatformEmailLogin(mfaStore, loginDelivery); err != nil {
		return nil, err
	}
	resendLimits, err := loginresendredis.New(redisClient, config.RedisPrefix)
	if err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	if err = mfaService.EnableEmailLoginResend(resendLimits, emailLoginResendKeyAdapter{deriver: deriver}); err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	recoveryAttempts, err := recoveryredis.New(redisClient, config.RedisPrefix)
	if err != nil {
		return nil, err
	}
	recoveryStore, err := identitypersistence.NewPasswordRecoveryStore(control)
	if err != nil {
		return nil, err
	}
	recoveryRequest, err := identityapp.NewPasswordRecoveryRequestService(recoveryStore, recoveryDelivery, identityapp.SecureVerificationTokenGenerator{}, identityIDs{}, systemClock{})
	if err != nil {
		return nil, err
	}
	publicRecovery, err := identityapp.NewPublicPasswordRecoveryService(recoveryRequest, recoveryAttempts, deriver, delivery)
	if err != nil {
		return nil, err
	}
	recoveryReset, err := identityapp.NewPasswordRecoveryResetService(recoveryStore, hasher, identityIDs{}, systemClock{})
	if err != nil {
		return nil, err
	}
	result.recovery, err = httpapi.NewPasswordRecoveryHandler(publicRecovery, recoveryReset, protection, ipResolver, deriver)
	if err != nil {
		return nil, err
	}
	registrationAttempts, err := registrationredis.New(redisClient, config.RedisPrefix)
	if err != nil {
		return nil, err
	}
	registrationCommitter, err := identitypersistence.NewSelfRegistrationCommitter(control)
	if err != nil {
		return nil, err
	}
	registrationService, err := identityapp.NewSelfRegistrationService(hasher, identityIDs{}, systemClock{}, registrationCommitter)
	if err != nil {
		return nil, err
	}
	verificationIssuer, err := identitypersistence.NewEmailVerificationIssuer(control)
	if err != nil {
		return nil, err
	}
	verificationService, err := identityapp.NewEmailVerificationIssueService(verificationIssuer, verificationDelivery, identityapp.SecureVerificationTokenGenerator{}, identityIDs{}, systemClock{})
	if err != nil {
		return nil, err
	}
	openRegistration, err := identityapp.NewOpenRegistrationService(registrationService, registrationAttempts, deriver, verificationService)
	if err != nil {
		return nil, err
	}
	result.registration, err = httpapi.NewRegistrationHandler(openRegistration, protection, ipResolver, deriver)
	if err != nil {
		return nil, err
	}
	verificationCommitter, err := identitypersistence.NewEmailVerificationCommitter(control)
	if err != nil {
		return nil, err
	}
	verificationConsumer, err := identityapp.NewEmailVerificationService(verificationCommitter, identityIDs{}, systemClock{})
	if err != nil {
		return nil, err
	}
	result.verification, err = httpapi.NewEmailVerificationHandler(verificationConsumer, protection)
	if err != nil {
		return nil, err
	}
	verificationResend, err := identityapp.NewEmailVerificationResendService(passwordReader, registrationAttempts, deriver, verificationService)
	if err != nil {
		return nil, err
	}
	result.resend, err = httpapi.NewEmailVerificationResendHandler(verificationResend, protection, ipResolver, deriver)
	if err != nil {
		return nil, err
	}
	result.delivery = delivery
	return result, nil
}

// IdentityRuntime serves identity and control-plane Platform operations without
// requiring tenant document storage, parsers or a message broker. Building is
// lazy; callers must Probe successfully before marking readiness.
type IdentityRuntime struct {
	payments        *commercial.Module
	googleTransport *googleoidc.Transport
	App             *fiber.App
	redis           *attemptredis.ClientRunner
	control         *postgres.Pool
}

func (r *IdentityRuntime) Probe(ctx context.Context) error {
	if ctx == nil || r == nil || r.redis == nil || r.control == nil {
		return errors.New("pilot_dependency_unavailable")
	}
	if err := r.redis.Ping(ctx); err != nil {
		return &dependencyError{cause: err}
	}
	if err := r.control.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		return tenancypersistence.VerifyControlCompatibility(c, tenancysql.New(db))
	}); err != nil {
		return &dependencyError{cause: err}
	}
	return nil
}

func BuildIdentityRuntime(config Config, state *health.State, control *postgres.Pool) (*IdentityRuntime, error) {
	if state == nil || control == nil {
		return nil, errors.New("pilot_identity_runtime_invalid")
	}
	redisClient, err := attemptredis.NewClient(config.Redis)
	if err != nil {
		return nil, errors.New("pilot_identity_runtime_invalid")
	}
	fail := func(err error) (*IdentityRuntime, error) {
		_ = redisClient.Close()
		return nil, err
	}
	composition, err := composeIdentity(config, control, redisClient)
	if err != nil {
		return fail(err)
	}

	passwordReauthentication, err := identityapp.NewGoogleLinkReauthenticationService(composition.authenticator, composition.rehasher)
	if err != nil {
		return fail(err)
	}
	passwordChangeCommitter, err := identitypersistence.NewPasswordChangeCommitter(control)
	if err != nil {
		return fail(err)
	}
	passwordChange, err := identityapp.NewPasswordChangeService(passwordReauthentication, composition.hasher, identityapp.SecureSessionTokenGenerator{}, identityIDs{}, systemClock{}, passwordChangeCommitter)
	if err != nil {
		return fail(err)
	}
	passwordChangeHandler, err := httpapi.NewPasswordChangeHandler(passwordChange, composition.ipResolver, composition.deriver)
	if err != nil {
		return fail(err)
	}

	sessionReader, err := identitypersistence.NewSessionReader(control)
	if err != nil {
		return fail(err)
	}
	if err = sessionReader.EnableSecurityPolicy(currentSecurityPolicy); err != nil {
		return fail(err)
	}
	sessionValidator, err := identityapp.NewSessionValidator(sessionReader, systemClock{})
	if err != nil {
		return fail(err)
	}
	sessionToucher, err := identitypersistence.NewSessionToucher(control)
	if err != nil {
		return fail(err)
	}
	if err = sessionToucher.EnableSecurityPolicy(currentSecurityPolicy); err != nil {
		return fail(err)
	}
	browserMiddleware, err := httpapi.NewBrowserSessionMiddleware(sessionValidator, sessionToucher, composition.protection, systemClock{})
	if err != nil {
		return fail(err)
	}
	bootstrapHandler, err := httpapi.NewSessionBootstrapHandler(composition.protection)
	if err != nil {
		return fail(err)
	}
	sessionInventory, err := identityapp.NewSessionInventoryService(sessionReader, systemClock{})
	if err != nil {
		return fail(err)
	}
	sessionInventoryHandler, err := httpapi.NewSessionInventoryHandler(sessionInventory, composition.protection)
	if err != nil {
		return fail(err)
	}
	sessionAdministrationStore, err := identitypersistence.NewSessionAdministrationStore(control)
	if err != nil {
		return fail(err)
	}
	sessionAdministration, err := identityapp.NewSessionAdministrationService(sessionAdministrationStore, identityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	sessionAdministrationHandler, err := httpapi.NewSessionAdministrationHandler(sessionAdministration)
	if err != nil {
		return fail(err)
	}

	accountTypeStore, err := identitypersistence.NewAccountTypeStore(control)
	if err != nil {
		return fail(err)
	}
	accountType, err := identityapp.NewAccountTypeService(accountTypeStore, accountTypeStore, identityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	membershipReader, err := authzpersistence.NewMembershipDirectoryReader(control)
	if err != nil {
		return fail(err)
	}
	membershipDirectory, err := authzapp.NewMembershipDirectory(membershipReader)
	if err != nil {
		return fail(err)
	}
	tenderpreneurAccess, err := newTenderpreneurPortalAccess(membershipDirectory)
	if err != nil {
		return fail(err)
	}
	if err = accountType.EnableTenderpreneurAccess(tenderpreneurAccess); err != nil {
		return fail(err)
	}
	platformStore, err := platformpersistence.NewStore(control)
	if err != nil {
		return fail(err)
	}
	platformModule, err := platformadmin.NewModuleWithAdministratorChanges(platformStore, platformStore, platformStore, platformStore, platformIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	if composition.delivery != nil {
		delivery, buildErr := newPlatformEmailDelivery(control, composition.delivery, config, composition.templateRenderer)
		if buildErr != nil {
			return fail(buildErr)
		}
		platformModule, err = platformadmin.NewModuleWithAdministration(platformStore, platformStore, platformStore, platformStore, platformStore, platformapp.SecureInvitationTokenGenerator{}, platformIDs{}, systemClock{}, delivery)
		if err != nil {
			return fail(err)
		}
	}
	if err = accountType.EnablePlatformAccess(platformPortalAdapter{platform: platformModule, mfa: composition.mfaStore}); err != nil {
		return fail(err)
	}
	accountTypeHandler, err := httpapi.NewAccountTypeHandler(accountType, composition.protection)
	if err != nil {
		return fail(err)
	}
	membershipHandler, err := httpapi.NewMembershipDirectoryHandler(membershipDirectory)
	if err != nil {
		return fail(err)
	}
	platformAdministrationHandler, err := httpapi.NewPlatformAdministrationHandler(platformModule, systemClock{})
	if err != nil {
		return fail(err)
	}
	platformAuditStore, err := controlintegration.NewPlatformAuditReadStore(control)
	if err != nil {
		return fail(err)
	}
	platformAudit, err := controlapp.NewPlatformAuditService(platformAuditStore, controlAuditIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	platformAuditHandler, err := httpapi.NewPlatformAuditHandler(platformAudit)
	if err != nil {
		return fail(err)
	}
	platformHistoryStore, err := notificationpersistence.NewPlatformHistoryStore(control)
	if err != nil {
		return fail(err)
	}
	platformHistory, err := notificationapp.NewPlatformHistoryService(platformHistoryStore, notificationIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	platformHistoryHandler, err := httpapi.NewPlatformNotificationHistoryHandler(platformHistory)
	if err != nil {
		return fail(err)
	}
	platformSettingsReadStore, err := settingspersistence.NewReadStore(control)
	if err != nil {
		return fail(err)
	}
	platformSettingsReader, err := settingsapp.NewReadService(platformSettingsReadStore, systemClock{})
	if err != nil {
		return fail(err)
	}
	platformSettingsHandler, err := httpapi.NewPlatformIntegrationSettingsHandler(platformSettingsReader, systemClock{})
	if err != nil {
		return fail(err)
	}
	informationStore, err := settingspersistence.NewInformationStore(control)
	if err != nil {
		return fail(err)
	}
	informationService, err := settingsapp.NewInformationService(informationStore, platformSettingIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	policyHandler, err := buildPlatformSessionPolicyHandler(control)
	if err != nil {
		return fail(err)
	}
	informationHandler, err := httpapi.NewPlatformInformationHandler(informationService)
	if err != nil {
		return fail(err)
	}
	var platformSettingsWriteHandler *httpapi.PlatformIntegrationSettingsWriteHandler
	if len(config.SMTPVaultKey) == 32 && config.SMTPVaultKeyVersion > 0 {
		vault, vaultErr := smtpsecret.NewVault(control, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if vaultErr != nil {
			return fail(vaultErr)
		}
		platformSettingsReader.EnableSMTPUsernames(vault)
		platformSettingsWriteHandler, err = BuildPlatformSMTPWriteHandler(control, vault, config.PesaPal)
		if err != nil {
			return fail(err)
		}
	}

	platformTenantStore, err := tenancypersistence.NewPlatformTenantDirectoryStore(control)
	if err != nil {
		return fail(err)
	}
	platformTenants, err := tenancyapp.NewPlatformTenantDirectoryService(platformTenantStore)
	if err != nil {
		return fail(err)
	}
	platformTenantHandler, err := httpapi.NewPlatformTenantDirectoryHandler(platformTenants, systemClock{})
	if err != nil {
		return fail(err)
	}
	commercialStore, err := commercialpersistence.NewStore(control)
	if err != nil {
		return fail(err)
	}
	commercialModule := commercial.NewModule()
	if err = commercialModule.ConfigurePlatformFinancialQueues(commercialStore, commercialIDs{}, systemClock{}); err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigureFinancialReads(commercialStore, commercialIDs{}, systemClock{}); err != nil {
		return fail(err)
	}
	// Mount only read operations: this composition must not activate payment or
	// financial-adjustment effects just because their shared module is present.
	platformFinancialHandler, err := httpapi.NewCommercialPlatformFinancialHandler(commercialModule)
	if err != nil {
		return fail(err)
	}
	catalogueHandler, err := httpapi.NewCommercialCatalogueHandler(commercialModule)
	if err != nil {
		return fail(err)
	}
	dashboardSources, err := dashboardintegration.NewPlatformSources(commercialModule)
	if err != nil {
		return fail(err)
	}
	dashboardModule, err := dashboard.NewModule(systemClock{})
	if err != nil {
		return fail(err)
	}
	if err = dashboardModule.ConfigurePlatform(dashboardSources); err != nil {
		return fail(err)
	}
	dashboardHandler, err := httpapi.NewPlatformDashboardHandler(dashboardModule)
	if err != nil {
		return fail(err)
	}

	mfaHandler, err := httpapi.NewMFAHandler(composition.mfaService, composition.protection)
	if err != nil {
		return fail(err)
	}
	if err = mfaHandler.EnableEmailLoginResend(composition.ipResolver, emailLoginResendKeyAdapter{deriver: composition.deriver}); err != nil {
		return fail(err)
	}
	mfaReadiness, err := identityapp.NewMFAReadinessServiceWithFactors(composition.passwordReader, composition.mfaStore, systemClock{}, true)
	if err != nil {
		return fail(err)
	}
	mfaReadinessHandler, err := httpapi.NewMFAReadinessHandler(mfaReadiness, composition.protection)
	if err != nil {
		return fail(err)
	}
	recoveryPreparation, err := identityapp.NewRecoveryPreparationService(composition.passwordReader, systemClock{}, composition.delivery != nil)
	if err != nil {
		return fail(err)
	}
	recoveryPreparationHandler, err := httpapi.NewRecoveryPreparationHandler(recoveryPreparation, composition.protection)
	if err != nil {
		return fail(err)
	}
	logoutCommitter, err := identitypersistence.NewLogoutCommitter(control)
	if err != nil {
		return fail(err)
	}
	logout, err := identityapp.NewLogoutService(logoutCommitter, identityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	logoutHandler, err := httpapi.NewLogoutHandler(logout)
	if err != nil {
		return fail(err)
	}

	app, err := httpapi.NewIdentityAccountApp(state, httpapi.IdentityAccountRoutes{
		Browser: browserMiddleware, Login: composition.login, PasswordChange: passwordChangeHandler,
		Registration: composition.registration, EmailVerification: composition.verification, EmailVerificationResend: composition.resend, PasswordRecovery: composition.recovery,
		Bootstrap: bootstrapHandler, SessionInventory: sessionInventoryHandler, SessionAdministration: sessionAdministrationHandler,
		AccountType: accountTypeHandler, RecoveryPreparation: recoveryPreparationHandler, MFAReadiness: mfaReadinessHandler,
		MFA: mfaHandler, Memberships: membershipHandler, Logout: logoutHandler,
	})
	if err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformDirectoryRoutes(app, browserMiddleware, platformAdministrationHandler); err != nil {
		return fail(err)
	}
	if composition.delivery != nil {
		if err = httpapi.RegisterPlatformInvitationRoutes(app, browserMiddleware, platformAdministrationHandler); err != nil {
			return fail(err)
		}
	}
	if err = httpapi.RegisterPlatformAdministratorChangeRoutes(app, browserMiddleware, platformAdministrationHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformSessionRevocationRoute(app, browserMiddleware, platformAdministrationHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformTenantDirectoryRoutes(app, browserMiddleware, platformTenantHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterCommercialCatalogueRoutes(app, browserMiddleware, catalogueHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterCommercialPlatformFinancialReadRoutes(app, browserMiddleware, platformFinancialHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformDashboardRoute(app, browserMiddleware, dashboardHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformAuditRoutes(app, browserMiddleware, platformAuditHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformNotificationHistoryRoutes(app, browserMiddleware, platformHistoryHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformIntegrationSettingsReadRoutes(app, browserMiddleware, platformSettingsHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformSessionPolicyRoutes(app, browserMiddleware, policyHandler); err != nil {
		return fail(err)
	}
	if err = registerRegionalRuntime(app, browserMiddleware, control, config); err != nil {
		return nil, err
	}
	if err = registerManagedPesaPal(app, browserMiddleware, control, commercialModule, commercialStore, config); err != nil {
		return nil, err
	}
	if err = httpapi.RegisterPlatformInformationRoutes(app, browserMiddleware, informationHandler); err != nil {
		return fail(err)
	}
	if platformSettingsWriteHandler != nil {
		if err = httpapi.RegisterPlatformIntegrationSettingsWriteRoutes(app, browserMiddleware, platformSettingsWriteHandler); err != nil {
			return fail(err)
		}
	}
	if err = registerManagedSettings(app, browserMiddleware, composition); err != nil {
		return fail(err)
	}
	googleStart, googleCallback, googleLinkStart, googleLinkCallback, err := composeManagedGoogle(config, control, redisClient, composition)
	if err != nil {
		return fail(err)
	}
	if googleStart != nil {
		ready := func(c fiber.Ctx) error {
			if !state.Ready() {
				return errors.New("service_not_ready")
			}
			return c.Next()
		}
		app.Post(httpapi.GoogleStartPath, ready, googleStart.Handle)
		app.Get(httpapi.GoogleCallbackPath, ready, googleCallback.Handle)
		app.Post(httpapi.GoogleLinkStartPath, ready, browserMiddleware.Handle, googleLinkStart.Handle)
		app.Get(httpapi.GoogleLinkCallbackPath, ready, browserMiddleware.Handle, googleLinkCallback.Handle)
	}
	if err = httpapi.InstallHealthFallback(app, state); err != nil {
		return fail(err)
	}
	return &IdentityRuntime{payments: commercialModule, App: app, redis: redisClient, control: control, googleTransport: composition.googleTransport}, nil
}
func (r *IdentityRuntime) Close() error {
	if r != nil && r.googleTransport != nil {
		r.googleTransport.CloseIdleConnections()
	}
	if r == nil || r.redis == nil {
		return nil
	}
	return r.redis.Close()
}
