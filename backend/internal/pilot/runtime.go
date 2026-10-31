// Package pilot composes the approved External Tender Pilot backend from
// explicit server-owned dependencies. It performs no migration or service start.
package pilot

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	authzapp "bidora/backend/internal/authorization/application"
	"bidora/backend/internal/authorization/invitation"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	commercial "bidora/backend/internal/commercial"
	commercialdomain "bidora/backend/internal/commercial/domain"
	commercialpersistence "bidora/backend/internal/commercial/persistence"
	controlapp "bidora/backend/internal/controlplane/application"
	controlintegration "bidora/backend/internal/controlplane/integration"
	controlpersistence "bidora/backend/internal/controlplane/persistence"
	dashboard "bidora/backend/internal/dashboard"
	dashboardintegration "bidora/backend/internal/dashboard/integration"
	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	entitlementpersistence "bidora/backend/internal/entitlementcheckpoint/persistence"
	dispatchapp "bidora/backend/internal/entitlementdispatch/application"
	"bidora/backend/internal/eventing"
	"bidora/backend/internal/health"
	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/mfaredis"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	notificationapp "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/emailtemplate"
	notificationpersistence "bidora/backend/internal/notification/persistence"
	opportunityapp "bidora/backend/internal/opportunity/application"
	"bidora/backend/internal/opportunity/extractopenai"
	"bidora/backend/internal/opportunity/integration"
	"bidora/backend/internal/opportunity/measurementprofile"
	"bidora/backend/internal/opportunity/measureprocess"
	opportunitypersistence "bidora/backend/internal/opportunity/persistence"
	opportunitysql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/opportunity/proposalexport"
	"bidora/backend/internal/opportunity/scanclamav"
	"bidora/backend/internal/opportunity/storageminio"
	organization "bidora/backend/internal/organization"
	organizationapp "bidora/backend/internal/organization/application"
	organizationintegration "bidora/backend/internal/organization/integration"
	organizationpersistence "bidora/backend/internal/organization/persistence"
	platformadmin "bidora/backend/internal/platformadmin"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	settingsapp "bidora/backend/internal/platformsettings/application"
	settingspersistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/platformsettings/pesapalruntime"
	"bidora/backend/internal/platformsettings/smtpsecret"
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	tenancypersistence "bidora/backend/internal/tenancy/persistence"
	tenancysql "bidora/backend/internal/tenancy/persistence/sqlc"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type GoogleConfig struct {
	// Deprecated credential fields are ignored; all real roots use managed storage.
	ClientID, ClientSecret                                    string
	CallbackURL, SuccessRedirect, FailureRedirect             string
	LinkCallbackURL, LinkSuccessRedirect, LinkFailureRedirect string
	TransactionTTL                                            time.Duration
	Transport                                                 googleoidc.TransportConfig
}

func (GoogleConfig) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED Google pilot config]") }

type Config struct {
	PesaPal                           pesapalruntime.Options
	PasswordPolicy                    password.Policy
	DummyPasswordPHC                  string
	AttemptHMACSecret                 []byte
	EmailLoginHMACSecret              []byte
	Redis                             attemptredis.Config
	RedisPrefix                       string
	AllowedOrigins                    []string
	TrustedProxies                    []netip.Prefix
	Google                            *GoogleConfig
	SMTPVaultKey                      []byte
	SMTPVaultKeyVersion               int32
	NotificationFingerprintKey        []byte
	NotificationFingerprintKeyVersion int32
	MinIO                             *storageminio.Config
	OpenAI                            *extractopenai.Config // Deprecated and ignored: active settings own credentials/model/limits.
	ClamAV                            *scanclamav.Config
	NATS                              *eventing.NATSConfig
	DocumentMeasurerPath              string
	PDFToTextPath                     string
	MFAKeyRing                        *mfa.KeyRing
	DevelopmentLoopbackHTTP           bool
}

func (Config) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED pilot config]") }

type TenantPool struct {
	Assignment tenancyapp.Assignment
	Pool       *postgres.Pool
}
type Dependencies struct {
	State   *health.State
	Control *postgres.Pool
	Tenants []TenantPool
}
type Runtime struct {
	payments             *commercial.Module
	App                  *fiber.App
	redis                *attemptredis.ClientRunner
	control              *postgres.Pool
	tenants              []*postgres.Pool
	assignments          []tenancyapp.Assignment
	googleTransport      *googleoidc.Transport
	objectStorage        *storageminio.Storage
	processors           []*opportunityapp.DocumentProcessor
	responseDraftWorkers []*opportunityapp.ResponseDraftWorker
	proposalDraftWorkers []*opportunityapp.ProposalDraftWorker
	measurementWorkers   []*opportunityapp.DocumentMeasurementCoordinator
	completionWorkers    []*opportunityapp.DocumentCompletionCoordinator
	entitlementBridge    *eventing.Bridge
	ackDispatcher        *dispatchapp.Dispatcher
	ackPreviousTenant    dispatchapp.ID
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.googleTransport != nil {
		r.googleTransport.CloseIdleConnections()
	}
	var bridgeErr error
	if r.entitlementBridge != nil {
		bridgeErr = r.entitlementBridge.Close()
	}
	if r.redis == nil {
		return bridgeErr
	}
	return errors.Join(bridgeErr, r.redis.Close())
}
func (r *Runtime) ProcessNextDocumentJobs(ctx context.Context) (bool, error) {
	if ctx == nil || r == nil || len(r.processors) == 0 {
		return false, errors.New("pilot_processing_unavailable")
	}
	if r.entitlementBridge != nil {
		processed, err := r.entitlementBridge.ProcessNext(ctx)
		if err != nil || processed {
			return processed, err
		}
	}
	if r.ackDispatcher != nil {
		result, err := r.ackDispatcher.ProcessNext(ctx, r.ackPreviousTenant)
		if result.Processed {
			r.ackPreviousTenant = result.TenantID
		}
		if result.Processed {
			return true, nil
		}
		if err != nil {
			return false, err
		}
	}
	for _, processor := range r.processors {
		processed, err := processor.ProcessNextScan(ctx)
		if err != nil || processed {
			return processed, err
		}
		processed, err = processor.ProcessNextAnalysis(ctx)
		if err != nil || processed {
			return processed, err
		}
	}
	for _, worker := range r.measurementWorkers {
		processed, err := worker.ProcessNext(ctx)
		if err != nil || processed {
			return processed, err
		}
	}
	for _, worker := range r.completionWorkers {
		processed, err := worker.ProcessNext(ctx)
		if err != nil || processed {
			return processed, err
		}
	}
	for _, worker := range r.responseDraftWorkers {
		processed, err := worker.ProcessNext(ctx)
		if err != nil || processed {
			return processed, err
		}
	}
	for _, worker := range r.proposalDraftWorkers {
		processed, err := worker.ProcessNext(ctx)
		if err != nil || processed {
			return processed, err
		}
	}
	return false, nil
}
func (r *Runtime) Probe(ctx context.Context) error {
	if ctx == nil || r == nil || r.redis == nil || r.control == nil || r.objectStorage == nil || len(r.tenants) == 0 {
		return errors.New("pilot_dependency_unavailable")
	}
	if err := r.redis.Ping(ctx); err != nil {
		return &dependencyError{cause: err}
	}
	if err := r.control.Ping(ctx); err != nil {
		return &dependencyError{cause: err}
	}
	if err := r.objectStorage.Verify(ctx); err != nil {
		return &dependencyError{cause: err}
	}
	if err := r.control.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		return tenancypersistence.VerifyControlCompatibility(txCtx, tenancysql.New(db))
	}); err != nil {
		return &dependencyError{cause: err}
	}
	assignmentReader, err := tenancypersistence.NewAssignmentReader(r.control)
	if err != nil {
		return &dependencyError{cause: err}
	}
	assignmentResolver, err := tenancyapp.NewAssignmentResolver(assignmentReader)
	if err != nil {
		return &dependencyError{cause: err}
	}
	for _, assignment := range r.assignments {
		if _, err := assignmentResolver.ResolveCaptureAssignment(ctx, assignment.TenantID, assignment.Generation); err != nil {
			return &dependencyError{cause: err}
		}
	}
	for _, tenant := range r.tenants {
		if tenant == nil {
			return errors.New("pilot_dependency_unavailable")
		}
		if err := tenant.Ping(ctx); err != nil {
			return &dependencyError{cause: err}
		}
		if err := tenant.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
			return opportunitypersistence.VerifyCaptureCompatibility(txCtx, opportunitysql.New(db))
		}); err != nil {
			return &dependencyError{cause: err}
		}
	}
	return nil
}

type dependencyError struct{ cause error }

func (e *dependencyError) Error() string                 { return "pilot_dependency_unavailable" }
func (e *dependencyError) Unwrap() error                 { return e.cause }
func (e *dependencyError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
func Build(config Config, deps Dependencies) (*Runtime, error) {
	if deps.State == nil || deps.Control == nil || len(deps.Tenants) == 0 || config.DummyPasswordPHC == "" || config.MFAKeyRing == nil || config.DevelopmentLoopbackHTTP {
		return nil, errors.New("pilot_runtime_invalid")
	}
	redisClient, err := attemptredis.NewClient(config.Redis)
	if err != nil {
		return nil, errors.New("pilot_runtime_invalid")
	}
	var googleTransport *googleoidc.Transport
	fail := func(err error) (*Runtime, error) {
		if googleTransport != nil {
			googleTransport.CloseIdleConnections()
		}
		_ = redisClient.Close()
		return nil, err
	}
	identity, err := composeIdentity(config, deps.Control, redisClient)
	if err != nil {
		return fail(err)
	}
	hasher, deriver := identity.hasher, identity.deriver
	protection, ipResolver := identity.protection, identity.ipResolver
	passwordReader := identity.passwordReader
	authenticator, rehasher := identity.authenticator, identity.rehasher
	mfaStore, mfaService := identity.mfaStore, identity.mfaService
	loginHandler := identity.login
	passwordReauthentication, err := identityapp.NewGoogleLinkReauthenticationService(authenticator, rehasher)
	if err != nil {
		return fail(err)
	}
	passwordChangeCommitter, err := identitypersistence.NewPasswordChangeCommitter(deps.Control)
	if err != nil {
		return fail(err)
	}
	passwordChange, err := identityapp.NewPasswordChangeService(passwordReauthentication, hasher, identityapp.SecureSessionTokenGenerator{}, identityIDs{}, systemClock{}, passwordChangeCommitter)
	if err != nil {
		return fail(err)
	}
	passwordChangeHandler, err := httpapi.NewPasswordChangeHandler(passwordChange, ipResolver, deriver)
	if err != nil {
		return fail(err)
	}
	registrationHandler := identity.registration
	emailVerificationHandler := identity.verification
	emailVerificationResendHandler := identity.resend
	passwordRecoveryHandler := identity.recovery
	var invitationIssueHandler *httpapi.InvitationIssueHandler
	var invitationAcceptHandler *httpapi.InvitationAcceptHandler
	var invitationDirectoryHandler *httpapi.InvitationDirectoryHandler
	var invitationCancellationHandler *httpapi.InvitationCancellationHandler
	if identity.delivery != nil {
		delivery := identity.delivery
		invitationIssuer, buildErr := authzpersistence.NewInvitationIssuer(deps.Control)
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationIssue, buildErr := authzapp.NewInvitationIssueService(invitationIssuer, invitationDeliveryAdapter{smtp: delivery}, authzapp.SecureInvitationTokenGenerator{}, authorizationIDs{}, systemClock{})
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationIssueHandler, buildErr = httpapi.NewInvitationIssueHandler(invitationIssue)
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationCommitter, buildErr := authzpersistence.NewInvitationAcceptanceCommitter(deps.Control)
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationAcceptance, buildErr := authzapp.NewInvitationAcceptanceService(invitationCommitter, authorizationIDs{}, systemClock{})
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationAcceptHandler, buildErr = httpapi.NewInvitationAcceptHandler(invitationAcceptance)
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationDirectoryReader, buildErr := authzpersistence.NewInvitationDirectoryReader(deps.Control)
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationDirectory, buildErr := authzapp.NewInvitationDirectory(invitationDirectoryReader, systemClock{})
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationDirectoryHandler, buildErr = httpapi.NewInvitationDirectoryHandler(invitationDirectory)
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationCancellationCommitter, buildErr := authzpersistence.NewInvitationCancellationCommitter(deps.Control)
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationCancellation, buildErr := authzapp.NewInvitationCancellationService(invitationCancellationCommitter, authorizationIDs{}, systemClock{})
		if buildErr != nil {
			return fail(buildErr)
		}
		invitationCancellationHandler, buildErr = httpapi.NewInvitationCancellationHandler(invitationCancellation)
		if buildErr != nil {
			return fail(buildErr)
		}
	}
	googleTransport = identity.googleTransport
	googleStartHandler, googleCallbackHandler, googleLinkStartHandler, googleLinkCallbackHandler, err := composeManagedGoogle(config, deps.Control, redisClient, identity)
	if err != nil {
		return fail(err)
	}
	sessionReader, err := identitypersistence.NewSessionReader(deps.Control)
	if err != nil {
		return fail(err)
	}
	if err = sessionReader.EnableSessionPolicy(currentSessionLimits); err != nil {
		return fail(err)
	}
	sessionValidator, err := identityapp.NewSessionValidator(sessionReader, systemClock{})
	if err != nil {
		return fail(err)
	}
	sessionToucher, err := identitypersistence.NewSessionToucher(deps.Control)
	if err != nil {
		return fail(err)
	}
	if err = sessionToucher.EnableSessionPolicy(currentSessionLimits); err != nil {
		return fail(err)
	}
	browserMiddleware, err := httpapi.NewBrowserSessionMiddleware(sessionValidator, sessionToucher, protection, systemClock{})
	if err != nil {
		return fail(err)
	}
	bootstrapHandler, err := httpapi.NewSessionBootstrapHandler(protection)
	if err != nil {
		return fail(err)
	}
	sessionInventoryService, err := identityapp.NewSessionInventoryService(sessionReader, systemClock{})
	if err != nil {
		return fail(err)
	}
	sessionInventoryHandler, err := httpapi.NewSessionInventoryHandler(sessionInventoryService, protection)
	if err != nil {
		return fail(err)
	}
	sessionAdministrationStore, err := identitypersistence.NewSessionAdministrationStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	sessionAdministrationService, err := identityapp.NewSessionAdministrationService(sessionAdministrationStore, identityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	sessionAdministrationHandler, err := httpapi.NewSessionAdministrationHandler(sessionAdministrationService)
	if err != nil {
		return fail(err)
	}
	accountTypeStore, err := identitypersistence.NewAccountTypeStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	accountTypeService, err := identityapp.NewAccountTypeService(accountTypeStore, accountTypeStore, identityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	platformStore, err := platformpersistence.NewStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	platformModule, err := platformadmin.NewModuleWithAdministratorChanges(platformStore, platformStore, platformStore, platformStore, platformIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	if identity.delivery != nil {
		platformDelivery, buildErr := newPlatformEmailDelivery(deps.Control, identity.delivery, config, identity.templateRenderer)
		if buildErr != nil {
			return fail(buildErr)
		}
		platformModule, buildErr = platformadmin.NewModuleWithAdministration(platformStore, platformStore, platformStore, platformStore, platformStore, platformapp.SecureInvitationTokenGenerator{}, platformIDs{}, systemClock{}, platformDelivery)
		if buildErr != nil {
			return fail(buildErr)
		}
	}
	platformAdministrationHandler, err := httpapi.NewPlatformAdministrationHandler(platformModule, systemClock{})
	if err != nil {
		return fail(err)
	}
	platformTenantDirectoryStore, err := tenancypersistence.NewPlatformTenantDirectoryStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	platformTenantDirectory, err := tenancyapp.NewPlatformTenantDirectoryService(platformTenantDirectoryStore)
	if err != nil {
		return fail(err)
	}
	platformTenantDirectoryHandler, err := httpapi.NewPlatformTenantDirectoryHandler(platformTenantDirectory, systemClock{})
	if err != nil {
		return fail(err)
	}
	platformAuditStore, err := controlintegration.NewPlatformAuditReadStore(deps.Control)
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
	platformHistoryStore, err := notificationpersistence.NewPlatformHistoryStore(deps.Control)
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
	platformSettingsReadStore, err := settingspersistence.NewReadStore(deps.Control)
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
	informationStore, err := settingspersistence.NewInformationStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	informationService, err := settingsapp.NewInformationService(informationStore, platformSettingIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	policyHandler, err := buildPlatformSessionPolicyHandler(deps.Control)
	if err != nil {
		return fail(err)
	}
	informationHandler, err := httpapi.NewPlatformInformationHandler(informationService)
	if err != nil {
		return fail(err)
	}
	var platformSettingsWriteHandler *httpapi.PlatformIntegrationSettingsWriteHandler
	if len(config.SMTPVaultKey) == 32 && config.SMTPVaultKeyVersion > 0 {
		vault, vaultErr := smtpsecret.NewVault(deps.Control, config.SMTPVaultKey, config.SMTPVaultKeyVersion)
		if vaultErr != nil {
			return fail(vaultErr)
		}
		platformSettingsWriteHandler, err = BuildPlatformSMTPWriteHandler(deps.Control, vault, config.PesaPal)
		if err != nil {
			return fail(err)
		}
	}
	commercialStore, err := commercialpersistence.NewStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	commercialModule := commercial.NewModule()
	if err = commercialModule.ConfigureSubscriptionLifecycle(commercialStore, commercialIDs{}, systemClock{}); err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigureTrialStart(commercialStore, commercialIDs{}, systemClock{}); err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigureAIUsage(commercialStore, commercialIDs{}, systemClock{}); err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigurePaidWriteAccess(commercialStore, systemClock{}); err != nil {
		return fail(err)
	}
	commercialCatalogueHandler, err := httpapi.NewCommercialCatalogueHandler(commercialModule)
	if err != nil {
		return fail(err)
	}
	commercialSubscriptionHandler, err := httpapi.NewCommercialSubscriptionHandlerWithTrial(commercialModule, commercialModule)
	if err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigureFinancialLists(commercialStore); err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigureInvoiceReads(commercialStore); err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigurePlatformFinancialQueues(commercialStore, commercialIDs{}, systemClock{}); err != nil {
		return fail(err)
	}
	if err = commercialModule.ConfigureFinancialReads(commercialStore, commercialIDs{}, systemClock{}); err != nil {
		return fail(err)
	}
	commercialFinancialHandler, err := httpapi.NewCommercialFinancialHandler(commercialModule)
	if err != nil {
		return fail(err)
	}
	platformFinancialHandler, err := httpapi.NewCommercialPlatformFinancialHandler(commercialModule)
	if err != nil {
		return fail(err)
	}
	platformDashboardSources, err := dashboardintegration.NewPlatformSources(commercialModule)
	if err != nil {
		return fail(err)
	}
	dashboardModule, err := dashboard.NewModule(systemClock{})
	if err != nil {
		return fail(err)
	}
	if err = dashboardModule.ConfigurePlatform(platformDashboardSources); err != nil {
		return fail(err)
	}
	platformDashboardHandler, err := httpapi.NewPlatformDashboardHandler(dashboardModule)
	if err != nil {
		return fail(err)
	}
	if err = accountTypeService.EnablePlatformAccess(platformPortalAdapter{platform: platformModule, mfa: mfaStore}); err != nil {
		return fail(err)
	}
	accountTypeHandler, err := httpapi.NewAccountTypeHandler(accountTypeService, protection)
	if err != nil {
		return fail(err)
	}
	mfaHandler, err := httpapi.NewMFAHandler(mfaService, protection)
	if err != nil {
		return fail(err)
	}
	if err = mfaHandler.EnableEmailLoginResend(ipResolver, emailLoginResendKeyAdapter{deriver: deriver}); err != nil {
		return fail(err)
	}
	recoveryPreparationService, err := identityapp.NewRecoveryPreparationService(passwordReader, systemClock{}, identity.delivery != nil)
	if err != nil {
		return fail(err)
	}
	recoveryPreparationHandler, err := httpapi.NewRecoveryPreparationHandler(recoveryPreparationService, protection)
	if err != nil {
		return fail(err)
	}
	mfaReadinessService, err := identityapp.NewMFAReadinessServiceWithFactors(passwordReader, mfaStore, systemClock{})
	if err != nil {
		return fail(err)
	}
	mfaReadinessHandler, err := httpapi.NewMFAReadinessHandler(mfaReadinessService, protection)
	if err != nil {
		return fail(err)
	}
	membershipReader, err := authzpersistence.NewMembershipDirectoryReader(deps.Control)
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
	if err = accountTypeService.EnableTenderpreneurAccess(tenderpreneurAccess); err != nil {
		return fail(err)
	}
	membershipHandler, err := httpapi.NewMembershipDirectoryHandler(membershipDirectory)
	if err != nil {
		return fail(err)
	}
	membershipRevocationCommitter, err := authzpersistence.NewMembershipRevocationCommitter(deps.Control)
	if err != nil {
		return fail(err)
	}
	membershipRevocation, err := authzapp.NewMembershipRevocationService(membershipRevocationCommitter, authorizationIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	membershipRevocationHandler, err := httpapi.NewMembershipRevocationHandler(membershipRevocation)
	if err != nil {
		return fail(err)
	}
	logoutCommitter, err := identitypersistence.NewLogoutCommitter(deps.Control)
	if err != nil {
		return fail(err)
	}
	logoutService, err := identityapp.NewLogoutService(logoutCommitter, identityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	logoutHandler, err := httpapi.NewLogoutHandler(logoutService)
	if err != nil {
		return fail(err)
	}
	grantReader, err := authzpersistence.NewCaptureGrantReader(deps.Control)
	if err != nil {
		return fail(err)
	}
	controlAuthorizer, err := authzapp.NewCaptureAuthorizer(grantReader)
	if err != nil {
		return fail(err)
	}
	authorizationAdapter, err := integration.NewPaidCaptureAuthorizationAdapter(controlAuthorizer, commercialModule)
	if err != nil {
		return fail(err)
	}
	workflowGrantReader, err := authzpersistence.NewOpportunityWorkflowGrantReader(deps.Control)
	if err != nil {
		return fail(err)
	}
	workflowControlAuthorizer, err := authzapp.NewOpportunityWorkflowAuthorizer(workflowGrantReader)
	if err != nil {
		return fail(err)
	}
	workflowAuthorization, err := integration.NewPaidWorkflowAuthorizationAdapter(workflowControlAuthorizer, commercialModule)
	if err != nil {
		return fail(err)
	}
	organizationControlAuthorizer, err := authzapp.NewOrganizationAuthorizer(workflowGrantReader)
	if err != nil {
		return fail(err)
	}
	organizationAuthorization, err := organizationintegration.NewAuthorizationAdapter(organizationControlAuthorizer)
	if err != nil {
		return fail(err)
	}
	assignmentReader, err := tenancypersistence.NewAssignmentReader(deps.Control)
	if err != nil {
		return fail(err)
	}
	assignmentResolver, err := tenancyapp.NewAssignmentResolver(assignmentReader)
	if err != nil {
		return fail(err)
	}
	ackAssignmentResolver, err := tenancyapp.NewAIEntitlementAcknowledgementAssignmentResolver(assignmentReader)
	if err != nil {
		return fail(err)
	}
	completionAssignmentResolver, err := tenancyapp.NewDocumentCompletionAssignmentResolver(assignmentReader)
	if err != nil {
		return fail(err)
	}
	registrations := make([]integration.CaptureStoreRegistration, 0, len(deps.Tenants))
	workflowRegistrations := make([]integration.WorkflowStoreRegistration, 0, len(deps.Tenants))
	organizationRegistrations := make([]organizationintegration.StoreRegistration, 0, len(deps.Tenants))
	workflowStores := make([]*opportunitypersistence.WorkflowStore, 0, len(deps.Tenants))
	for _, tenant := range deps.Tenants {
		if tenant.Pool == nil || !tenant.Assignment.Valid() {
			return fail(errors.New("pilot_runtime_invalid"))
		}
		store, err := opportunitypersistence.NewCaptureStore(tenant.Pool, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation)
		if err != nil {
			return fail(err)
		}
		registrations = append(registrations, integration.CaptureStoreRegistration{Assignment: tenant.Assignment, Store: store})
		workflowStore, err := opportunitypersistence.NewWorkflowStore(tenant.Pool, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation)
		if err != nil {
			return fail(err)
		}
		workflowRegistrations = append(workflowRegistrations, integration.WorkflowStoreRegistration{Assignment: tenant.Assignment, Store: workflowStore})
		workflowStores = append(workflowStores, workflowStore)
		organizationStore, err := organizationpersistence.New(tenant.Pool)
		if err != nil {
			return fail(err)
		}
		organizationRegistrations = append(organizationRegistrations, organizationintegration.StoreRegistration{Assignment: tenant.Assignment, Store: organizationStore})
	}
	catalog, err := integration.NewCaptureStoreCatalog(registrations)
	if err != nil {
		return fail(err)
	}
	routed, err := integration.NewRoutedCaptureUnitOfWork(assignmentResolver, catalog)
	if err != nil {
		return fail(err)
	}
	captureExecutor := &opportunityapp.CaptureExecutor{Authorizer: authorizationAdapter, Documents: blockedDocuments{}, UOW: routed, IDs: opportunityIDs{}, Clock: systemClock{}}
	captureHandler, err := httpapi.NewManualCaptureHandler(captureExecutor)
	if err != nil {
		return fail(err)
	}
	workflowRouter, err := integration.NewRoutedWorkflowStore(assignmentResolver, workflowRegistrations)
	if err != nil {
		return fail(err)
	}
	organizationRouter, err := organizationintegration.NewRoutedStore(assignmentResolver, organizationRegistrations)
	if err != nil {
		return fail(err)
	}
	organizationModule, err := organization.NewModule(organizationAuthorization, organizationRouter, organizationIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	organizationHandler, err := httpapi.NewOrganizationHandler(organizationModule)
	if err != nil {
		return fail(err)
	}
	workflowService, err := opportunityapp.NewWorkflowService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	workflowHandler, err := httpapi.NewOpportunityWorkflowHandler(workflowService)
	if err != nil {
		return fail(err)
	}
	workflowReadService, err := opportunityapp.NewWorkflowReadService(workflowAuthorization, workflowRouter)
	if err != nil {
		return fail(err)
	}
	workflowReadHandler, err := httpapi.NewOpportunityReadHandler(workflowReadService)
	if err != nil {
		return fail(err)
	}
	tenderpreneurDashboardSources, err := dashboardintegration.NewTenderSources(commercialModule, workflowReadService)
	if err != nil {
		return fail(err)
	}
	if err = dashboardModule.ConfigureTender(tenderpreneurDashboardSources, tenderpreneurDashboardSources, tenderpreneurDashboardSources); err != nil {
		return fail(err)
	}
	tenderpreneurDashboardHandler, err := httpapi.NewTenderpreneurDashboardHandler(dashboardModule)
	if err != nil {
		return fail(err)
	}
	requirementService, err := opportunityapp.NewRequirementService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	requirementHandler, err := httpapi.NewRequirementHandler(requirementService)
	if err != nil {
		return fail(err)
	}
	requirementCorrectionService, err := opportunityapp.NewRequirementCorrectionService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	requirementCorrectionHandler, err := httpapi.NewRequirementCorrectionHandler(requirementCorrectionService)
	if err != nil {
		return fail(err)
	}
	reviewThreadService, err := opportunityapp.NewReviewThreadService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	reviewThreadHandler, err := httpapi.NewReviewThreadHandler(reviewThreadService)
	if err != nil {
		return fail(err)
	}
	internalReviewService, err := opportunityapp.NewInternalReviewDecisionService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	internalReviewHandler, err := httpapi.NewInternalReviewDecisionHandler(internalReviewService)
	if err != nil {
		return fail(err)
	}
	collaborationTimelineService, err := opportunityapp.NewCollaborationTimelineService(workflowAuthorization, workflowRouter)
	if err != nil {
		return fail(err)
	}
	collaborationTimelineHandler, err := httpapi.NewCollaborationTimelineHandler(collaborationTimelineService)
	if err != nil {
		return fail(err)
	}
	aiReviewService, err := opportunityapp.NewAIRequirementReviewService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	aiReviewHandler, err := httpapi.NewAIRequirementReviewHandler(aiReviewService)
	if err != nil {
		return fail(err)
	}
	if config.MinIO == nil {
		return fail(errors.New("pilot_minio_config_invalid"))
	}
	objectStorage, err := storageminio.New(*config.MinIO)
	if err != nil {
		return fail(err)
	}
	documentUploadService, err := opportunityapp.NewDocumentUploadService(workflowAuthorization, workflowRouter, objectStorage, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	operationalStore, err := settingspersistence.NewRegionalPolicyStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	if err = documentUploadService.EnableOperationalPolicy(operationalStore.UploadLimit); err != nil {
		return fail(err)
	}
	documentUploadHandler, err := httpapi.NewDocumentUploadHandler(documentUploadService)
	if err != nil {
		return fail(err)
	}
	evidenceService, err := opportunityapp.NewRequirementEvidenceService(workflowAuthorization, workflowRouter, objectStorage, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	evidenceHandler, err := httpapi.NewRequirementEvidenceHandler(evidenceService)
	if err != nil {
		return fail(err)
	}
	technicalResponseService, err := opportunityapp.NewTechnicalResponseService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	technicalResponseHandler, err := httpapi.NewTechnicalResponseHandler(technicalResponseService)
	if err != nil {
		return fail(err)
	}
	proposalService, err := opportunityapp.NewProposalService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	proposalHandler, err := httpapi.NewProposalHandler(proposalService)
	if err != nil {
		return fail(err)
	}
	proposalExportService, err := opportunityapp.NewProposalExportService(workflowAuthorization, workflowRouter, proposalexport.New())
	if err != nil {
		return fail(err)
	}
	proposalExportHandler, err := httpapi.NewProposalExportHandler(proposalExportService)
	if err != nil {
		return fail(err)
	}
	proposalPackageService, err := opportunityapp.NewProposalPackageService(workflowAuthorization, workflowRouter, workflowRouter, proposalexport.New(), opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	proposalPackageHandler, err := httpapi.NewProposalPackageHandler(proposalPackageService)
	if err != nil {
		return fail(err)
	}
	proposalApprovalService, err := opportunityapp.NewProposalApprovalService(workflowAuthorization, workflowRouter, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	proposalApprovalHandler, err := httpapi.NewProposalApprovalHandler(proposalApprovalService)
	if err != nil {
		return fail(err)
	}
	externalEvidenceService, err := opportunityapp.NewExternalActionEvidenceService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	externalEvidenceHandler, err := httpapi.NewExternalActionEvidenceHandler(externalEvidenceService)
	if err != nil {
		return fail(err)
	}
	submissionService, err := opportunityapp.NewSubmissionService(workflowAuthorization, workflowRouter, workflowRouter, objectStorage, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	submissionHandler, err := httpapi.NewSubmissionHandler(submissionService)
	if err != nil {
		return fail(err)
	}
	reportedOutcomeService, err := opportunityapp.NewReportedOutcomeService(workflowAuthorization, workflowRouter, objectStorage, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	reportedOutcomeHandler, err := httpapi.NewReportedOutcomeHandler(reportedOutcomeService)
	if err != nil {
		return fail(err)
	}
	archivePreparationService, err := opportunityapp.NewArchivePreparationService(workflowAuthorization, workflowRouter, systemClock{})
	if err != nil {
		return fail(err)
	}
	archivePreparationHandler, err := httpapi.NewArchivePreparationHandler(archivePreparationService)
	if err != nil {
		return fail(err)
	}
	bidArchiveService, err := opportunityapp.NewBidArchiveService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	bidArchiveHandler, err := httpapi.NewBidArchiveHandler(bidArchiveService)
	if err != nil {
		return fail(err)
	}
	ownershipContinuityService, err := opportunityapp.NewOwnershipContinuityService(workflowAuthorization, workflowRouter, systemClock{})
	if err != nil {
		return fail(err)
	}
	ownershipContinuityHandler, err := httpapi.NewOwnershipContinuityHandler(ownershipContinuityService)
	if err != nil {
		return fail(err)
	}
	membershipRevocationHandler, err = httpapi.NewMembershipRevocationHandlerWithContinuity(membershipRevocation, ownershipContinuityService)
	if err != nil {
		return fail(err)
	}
	membershipOffboardingStore, err := authzpersistence.NewMembershipOffboardingStore(deps.Control)
	if err != nil {
		return fail(err)
	}
	membershipOffboardingService, err := authzapp.NewMembershipOffboardingService(membershipOffboardingStore, workflowRouter, authorizationIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	membershipOffboardingHandler, err := httpapi.NewMembershipOffboardingHandler(membershipOffboardingService)
	if err != nil {
		return fail(err)
	}
	if config.ClamAV == nil {
		return fail(errors.New("pilot_processing_config_invalid"))
	}
	extractor, err := newManagedOpenAI(deps.Control, config)
	if err != nil {
		return fail(err)
	}
	limits := opportunityapp.MeasurementLimits{MaxPDFPages: 2000, MaxDOCXCharacters: 5000000, MaxArchiveEntries: 10000, MaxExpandedBytes: 100 << 20, MaxMemoryBytes: 512 << 20, MaxCPUMilliseconds: 120000, MaxXMLDepth: 256, MaxArchiveDepth: 1, WallTimeout: 180 * time.Second}
	measurer, err := measureprocess.New(measureprocess.Config{ExecutablePath: config.DocumentMeasurerPath})
	if err != nil {
		return fail(err)
	}
	textCounter, err := measureprocess.NewTextCounter(config.PDFToTextPath, measurer)
	if err != nil {
		return fail(err)
	}
	creditCalculator, err := integration.NewDocumentCreditCalculator(commercialModule)
	if err != nil {
		return fail(err)
	}
	aiDraftService, err := opportunityapp.NewResponseDraftService(workflowAuthorization, workflowRouter, objectStorage, textCounter, creditCalculator, limits, extractor, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	aiDraftHandler, err := httpapi.NewAIResponseDraftHandler(aiDraftService, true)
	if err != nil {
		return fail(err)
	}
	proposalDraftService, err := opportunityapp.NewProposalDraftService(workflowAuthorization, workflowRouter, objectStorage, textCounter, creditCalculator, limits, extractor, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	proposalDraftHandler, err := httpapi.NewAIProposalDraftHandler(proposalDraftService)
	if err != nil {
		return fail(err)
	}
	proposalApplicationService, err := opportunityapp.NewProposalApplicationService(workflowAuthorization, workflowRouter, opportunityIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	proposalApplicationHandler, err := httpapi.NewProposalApplicationHandler(proposalApplicationService)
	if err != nil {
		return fail(err)
	}
	scanner, err := scanclamav.New(*config.ClamAV)
	if err != nil {
		return fail(err)
	}
	if config.NATS == nil {
		return fail(errors.New("pilot_nats_config_invalid"))
	}
	commercialAccounting, err := integration.NewDocumentCommercialAccounting(commercialModule)
	if err != nil {
		return fail(err)
	}
	completionPlacement, err := integration.NewDocumentCompletionPlacement(completionAssignmentResolver)
	if err != nil {
		return fail(err)
	}
	processors := make([]*opportunityapp.DocumentProcessor, 0, len(deps.Tenants))
	measurementWorkers := make([]*opportunityapp.DocumentMeasurementCoordinator, 0, len(deps.Tenants))
	completionWorkers := make([]*opportunityapp.DocumentCompletionCoordinator, 0, len(deps.Tenants))
	bridgeOutboxes := make([]*eventing.Outbox, 0, len(deps.Tenants))
	ackEndpoints := make([]integration.AIEntitlementAcknowledgementEndpointRegistration, 0, len(deps.Tenants))
	responseDraftWorkers := make([]*opportunityapp.ResponseDraftWorker, 0, len(deps.Tenants))
	proposalDraftWorkers := make([]*opportunityapp.ProposalDraftWorker, 0, len(deps.Tenants))
	for tenantIndex, tenant := range deps.Tenants {
		store, storeErr := opportunitypersistence.NewDocumentProcessingStore(tenant.Pool, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation)
		if storeErr != nil {
			return fail(storeErr)
		}
		processor, processorErr := opportunityapp.NewEntitledDocumentProcessor(store, objectStorage, scanner, extractor, workflowAuthorization, commercialAccounting, opportunityIDs{}, systemClock{})
		if processorErr != nil {
			return fail(processorErr)
		}
		processors = append(processors, processor)
		entitlementStore, workerErr := entitlementpersistence.NewStore(tenant.Pool)
		if workerErr != nil {
			return fail(workerErr)
		}
		entitlementService, workerErr := entitlementapp.NewService(entitlementStore, entitlementIDs{}, systemClock{})
		if workerErr != nil {
			return fail(workerErr)
		}
		entitlementRequester, workerErr := integration.NewDocumentEntitlementRequester(entitlementService)
		if workerErr != nil {
			return fail(workerErr)
		}
		measurementStore, workerErr := opportunitypersistence.NewDocumentMeasurementStore(tenant.Pool, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation)
		if workerErr != nil {
			return fail(workerErr)
		}
		measurementWorker, workerErr := opportunityapp.NewDocumentMeasurementCoordinator(measurementStore, objectStorage, measurer, workflowAuthorization, creditCalculator, entitlementRequester, opportunityIDs{}, systemClock{}, limits, measurementprofile.AttemptIdentity())
		if workerErr != nil {
			return fail(workerErr)
		}
		measurementWorkers = append(measurementWorkers, measurementWorker)
		completionStore, workerErr := opportunitypersistence.NewDocumentCompletionStore(tenant.Pool)
		if workerErr != nil {
			return fail(workerErr)
		}
		completionWorker, workerErr := opportunityapp.NewDocumentCompletionCoordinator(completionStore, commercialAccounting, completionPlacement, opportunityIDs{}, systemClock{}, completionObserver{}, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation)
		if workerErr != nil {
			return fail(workerErr)
		}
		completionWorkers = append(completionWorkers, completionWorker)
		outbox, workerErr := eventing.NewOutbox(tenant.Pool)
		if workerErr != nil {
			return fail(workerErr)
		}
		bridgeOutboxes = append(bridgeOutboxes, outbox)
		ackStore, workerErr := opportunitypersistence.NewDocumentEntitlementAcknowledgementStore(tenant.Pool)
		if workerErr != nil {
			return fail(workerErr)
		}
		ackService, workerErr := opportunityapp.NewDocumentEntitlementAcknowledgementService(ackStore, opportunityIDs{}, systemClock{}, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation)
		if workerErr != nil {
			return fail(workerErr)
		}
		ackEndpoints = append(ackEndpoints, integration.AIEntitlementAcknowledgementEndpointRegistration{Assignment: tenant.Assignment, Endpoint: ackService})
		worker, workerErr := opportunityapp.NewResponseDraftWorker(workflowStores[tenantIndex], objectStorage, extractor, workflowAuthorization, commercialAccounting, creditCalculator, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation, opportunityIDs{}, systemClock{})
		if workerErr != nil {
			return fail(workerErr)
		}
		responseDraftWorkers = append(responseDraftWorkers, worker)
		proposalWorker, workerErr := opportunityapp.NewProposalDraftWorker(workflowStores[tenantIndex], objectStorage, extractor, workflowAuthorization, commercialAccounting, creditCalculator, opportunityapp.ID(tenant.Assignment.TenantID), tenant.Assignment.Generation, opportunityIDs{}, systemClock{})
		if workerErr != nil {
			return fail(workerErr)
		}
		proposalDraftWorkers = append(proposalDraftWorkers, proposalWorker)
	}
	entitlementBridge, err := eventing.NewBridge(*config.NATS, bridgeOutboxes, commercialModule, eventingIDs{}, systemClock{})
	if err != nil {
		return fail(err)
	}
	ackOutbox, err := controlpersistence.NewAIEntitlementAcknowledgementOutbox(deps.Control)
	if err != nil {
		return fail(err)
	}
	ackCatalog, err := integration.NewAIEntitlementAcknowledgementEndpointCatalog(ackEndpoints)
	if err != nil {
		return fail(err)
	}
	ackDispatcher, err := dispatchapp.NewDispatcher(ackOutbox, ackAssignmentResolver, ackCatalog, systemClock{}, dispatchObserver{})
	if err != nil {
		return fail(err)
	}
	app, err := httpapi.NewPilot(deps.State, httpapi.PilotRoutes{Browser: browserMiddleware, Login: loginHandler, PasswordChange: passwordChangeHandler, Registration: registrationHandler, EmailVerification: emailVerificationHandler, EmailVerificationResend: emailVerificationResendHandler, InvitationIssue: invitationIssueHandler, InvitationAccept: invitationAcceptHandler, InvitationDirectory: invitationDirectoryHandler, InvitationCancel: invitationCancellationHandler, Memberships: membershipHandler, MembershipRevoke: membershipRevocationHandler, MembershipOffboarding: membershipOffboardingHandler, Logout: logoutHandler, Bootstrap: bootstrapHandler, SessionInventory: sessionInventoryHandler, SessionAdministration: sessionAdministrationHandler, AccountType: accountTypeHandler, MFA: mfaHandler, RecoveryPreparation: recoveryPreparationHandler, PasswordRecovery: passwordRecoveryHandler, MFAReadiness: mfaReadinessHandler, Capture: captureHandler, OpportunityWorkflow: workflowHandler, OpportunityRead: workflowReadHandler, Requirements: requirementHandler, DocumentUploads: documentUploadHandler, AIRequirementReviews: aiReviewHandler, RequirementEvidence: evidenceHandler, TechnicalResponses: technicalResponseHandler, AIResponseDrafts: aiDraftHandler, Proposals: proposalHandler, ProposalExports: proposalExportHandler, AIProposalDrafts: proposalDraftHandler, ProposalApplication: proposalApplicationHandler, RequirementCorrections: requirementCorrectionHandler, ReviewThreads: reviewThreadHandler, InternalReviews: internalReviewHandler, CollaborationTimeline: collaborationTimelineHandler, ProposalPackages: proposalPackageHandler, ProposalApprovals: proposalApprovalHandler, Submissions: submissionHandler, ReportedOutcomes: reportedOutcomeHandler, ExternalActionEvidence: externalEvidenceHandler, ArchivePreparation: archivePreparationHandler, BidArchive: bidArchiveHandler, OwnershipContinuity: ownershipContinuityHandler, GoogleStart: googleStartHandler, GoogleCallback: googleCallbackHandler, GoogleLinkStart: googleLinkStartHandler, GoogleLinkCallback: googleLinkCallbackHandler})
	if err != nil {
		return fail(err)
	}
	if err = registerManagedSettings(app, browserMiddleware, identity); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformDirectoryRoutes(app, browserMiddleware, platformAdministrationHandler); err != nil {
		return fail(err)
	}
	if identity.delivery != nil {
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
	if err = httpapi.RegisterPlatformTenantDirectoryRoutes(app, browserMiddleware, platformTenantDirectoryHandler); err != nil {
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
	if err = registerRegionalRuntime(app, browserMiddleware, deps.Control, config); err != nil {
		return nil, err
	}
	if err = registerManagedPesaPal(app, browserMiddleware, deps.Control, commercialModule, commercialStore, config); err != nil {
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
	if err = httpapi.RegisterCommercialCatalogueRoutes(app, browserMiddleware, commercialCatalogueHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterCommercialSubscriptionRoutes(app, browserMiddleware, commercialSubscriptionHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterCommercialFinancialReadRoutes(app, browserMiddleware, commercialFinancialHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterCommercialPlatformFinancialReadRoutes(app, browserMiddleware, platformFinancialHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterPlatformDashboardRoute(app, browserMiddleware, platformDashboardHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterTenderpreneurDashboardRoute(app, browserMiddleware, tenderpreneurDashboardHandler); err != nil {
		return fail(err)
	}
	if err = httpapi.RegisterOrganizationRoutes(app, browserMiddleware, organizationHandler); err != nil {
		return fail(err)
	}
	tenantPools := make([]*postgres.Pool, len(deps.Tenants))
	assignments := make([]tenancyapp.Assignment, len(deps.Tenants))
	for index := range deps.Tenants {
		tenantPools[index] = deps.Tenants[index].Pool
		assignments[index] = deps.Tenants[index].Assignment
	}
	return &Runtime{payments: commercialModule, App: app, redis: redisClient, control: deps.Control, tenants: tenantPools, assignments: assignments, googleTransport: googleTransport, objectStorage: objectStorage, processors: processors, measurementWorkers: measurementWorkers, completionWorkers: completionWorkers, entitlementBridge: entitlementBridge, ackDispatcher: ackDispatcher, responseDraftWorkers: responseDraftWorkers, proposalDraftWorkers: proposalDraftWorkers}, nil
}

type completionObserver struct{}

func (completionObserver) ObserveDocumentCompletion(opportunityapp.DocumentCompletionObservation) {}

type dispatchObserver struct{}

func (dispatchObserver) Observe(dispatchapp.Observation) {}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type commercialIDs struct{}

func (commercialIDs) NewID() (commercialdomain.ID, error) {
	value, err := uuid.NewRandom()
	return commercialdomain.ID(value), err
}

type organizationIDs struct{}

func (organizationIDs) NewID() (organizationapp.ID, error) {
	value, err := uuid.NewRandom()
	return organizationapp.ID(value), err
}

type identityIDs struct{}

func (identityIDs) NewID() (identityapp.ID, error) {
	value, err := uuid.NewRandom()
	return identityapp.ID(value), err
}

type controlAuditIDs struct{}

func (controlAuditIDs) NewID() (controlapp.ID, error) {
	value, err := uuid.NewRandom()
	return controlapp.ID(value), err
}

type platformIDs struct{}

func (platformIDs) NewID() (platformapp.ID, error) {
	value, err := uuid.NewRandom()
	return platformapp.ID(value), err
}

type platformSMTPSender struct{ delivery smtpDelivery }

func (s platformSMTPSender) Send(ctx context.Context, recipient email.Address, message emailtemplate.Message) error {
	if s.delivery == nil {
		return errors.New("platform_email_delivery_unavailable")
	}
	return s.delivery.DeliverRendered(ctx, recipient, message.Subject, message.Text, message.HTML)
}

type mfaAttemptAdapter struct{ controller *mfaredis.Controller }
type mfaScopeAdapter struct{ deriver *attempt.Deriver }
type emailLoginResendKeyAdapter struct{ deriver *attempt.Deriver }

func (a emailLoginResendKeyAdapter) EmailLoginResendIdentity(user identityapp.ID) ([32]byte, error) {
	key, err := a.deriver.EmailLoginResendIdentity([16]byte(user))
	return [32]byte(key), err
}
func (a emailLoginResendKeyAdapter) EmailLoginResendNetwork(address netip.Addr) ([32]byte, error) {
	key, err := a.deriver.EmailLoginResendNetwork(address)
	return [32]byte(key), err
}

func (a mfaScopeAdapter) DeriveMFAScope(p string, u, t identityapp.ID) ([32]byte, error) {
	k, e := a.deriver.MFA(p, [16]byte(u), [16]byte(t))
	return [32]byte(k), e
}

func (a mfaAttemptAdapter) BeginMFA(ctx context.Context, scope [32]byte) (identityapp.MFAAttemptReservation, error) {
	r, e := a.controller.Begin(ctx, scope)
	if errors.Is(e, mfaredis.ErrDenied) {
		return nil, identityapp.ErrMFAAttemptDenied
	}
	if e != nil {
		return nil, e
	}
	return mfaReservationAdapter{r}, nil
}

type mfaReservationAdapter struct{ inner mfaredis.Reservation }

func (a mfaReservationAdapter) Finish(ctx context.Context, success bool) error {
	e := a.inner.Finish(ctx, success)
	if errors.Is(e, mfaredis.ErrDenied) {
		return identityapp.ErrMFAAttemptDenied
	}
	return e
}

type authorizationIDs struct{}

func (authorizationIDs) NewID() (authzapp.ID, error) {
	value, err := uuid.NewRandom()
	return authzapp.ID(value), err
}

type invitationDeliveryAdapter struct{ smtp smtpDelivery }

func (d invitationDeliveryAdapter) DeliverTenantInvitation(context.Context, string, invitation.Token, time.Time) error {
	return errors.New("tenant_role_required")
}

func (d invitationDeliveryAdapter) DeliverTenantRoleInvitation(ctx context.Context, recipient string, role authzapp.TenderpreneurRole, token invitation.Token, expiresAt time.Time) error {
	address, err := email.Parse(recipient)
	if err != nil || d.smtp == nil {
		return errors.New("invitation_delivery_unavailable")
	}
	return d.smtp.DeliverTenantRoleInvitation(ctx, address, string(role), token.RevealForDelivery(), expiresAt)
}

type eventingIDs struct{}

func (eventingIDs) NewID() (eventing.ID, error) {
	value, err := uuid.NewRandom()
	return eventing.ID(value), err
}

type entitlementIDs struct{}

func (entitlementIDs) NewID() (entitlementapp.ID, error) {
	value, err := uuid.NewRandom()
	return entitlementapp.ID(value), err
}

type opportunityIDs struct{}

func (opportunityIDs) NewID() (opportunityapp.ID, error) {
	value, err := uuid.NewRandom()
	return opportunityapp.ID(value), err
}

type blockedDocuments struct{}

func (blockedDocuments) CheckCaptureSource(context.Context, opportunityapp.CaptureAuthority, string) error {
	return errors.New("capture_source_unavailable")
}
