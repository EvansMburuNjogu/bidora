package httpapi

import (
	"errors"

	"bidora/backend/internal/health"
	"github.com/gofiber/fiber/v3"
)

// IdentityAccountRoutes contains the control-plane identity operations that do
// not require a tenant database or document/provider dependency.
type IdentityAccountRoutes struct {
	Browser                 *BrowserSessionMiddleware
	Login                   *LoginHandler
	PasswordChange          *PasswordChangeHandler
	Registration            *RegistrationHandler
	EmailVerification       *EmailVerificationHandler
	EmailVerificationResend *EmailVerificationResendHandler
	PasswordRecovery        *PasswordRecoveryHandler
	Bootstrap               *SessionBootstrapHandler
	SessionInventory        *SessionInventoryHandler
	SessionAdministration   *SessionAdministrationHandler
	AccountType             *AccountTypeHandler
	RecoveryPreparation     *RecoveryPreparationHandler
	MFAReadiness            *MFAReadinessHandler
	MFA                     *MFAHandler
	Memberships             *MembershipDirectoryHandler
	Logout                  *LogoutHandler
}

// NewIdentityAccountApp mounts the bounded identity/account module without
// tenant-private or external document-provider operations. The composing
// caller must register any owner-module routes and then call
// InstallHealthFallback before serving the app.
func NewIdentityAccountApp(state *health.State, routes IdentityAccountRoutes) (*fiber.App, error) {
	if state == nil || routes.Browser == nil || routes.Login == nil || routes.PasswordChange == nil ||
		routes.Bootstrap == nil || routes.SessionInventory == nil || routes.SessionAdministration == nil ||
		routes.AccountType == nil || routes.RecoveryPreparation == nil || routes.MFAReadiness == nil ||
		routes.MFA == nil || routes.Memberships == nil || routes.Logout == nil {
		return nil, errors.New("identity_account_routes_invalid")
	}
	if (routes.Registration == nil) != (routes.EmailVerification == nil) ||
		(routes.Registration == nil) != (routes.EmailVerificationResend == nil) ||
		(routes.Registration == nil) != (routes.PasswordRecovery == nil) {
		return nil, errors.New("identity_account_optional_routes_invalid")
	}

	// Configuration payloads have their own 16 KiB bound; existing identity
	// binders retain their strict per-operation 4 KiB limits.
	app := newBase(32 * 1024)
	app.Use("/v1", routes.Browser.CaptureAuditContext)
	ready := func(c fiber.Ctx) error {
		if !state.Ready() {
			return fail(c, 503, "SERVICE_NOT_READY", "The service is not ready to accept requests.", true)
		}
		return c.Next()
	}

	app.Post(LoginPath, ready, routes.Login.Handle)
	app.Post(PasswordChangePath, ready, routes.Browser.Handle, routes.PasswordChange.Handle)
	if routes.Registration != nil {
		app.Post(RegistrationPath, ready, routes.Registration.Handle)
		app.Post(InvitationRegistrationPath, ready, routes.Registration.HandleInvitation)
		app.Post(InvitationDetailsPath, ready, routes.Registration.InvitationDetails)
		app.Post(InvitationActivationPath, ready, routes.Registration.ActivateInvitation)
		app.Post(EmailVerificationPath, ready, routes.EmailVerification.Handle)
		app.Post(EmailVerificationResendPath, ready, routes.EmailVerificationResend.Handle)
		app.Post(PasswordRecoveryRequestPath, ready, routes.PasswordRecovery.Request)
		app.Post(PasswordRecoveryResetPath, ready, routes.PasswordRecovery.Reset)
	}
	app.Get(SessionBootstrapPath, ready, routes.Browser.Bootstrap, routes.Bootstrap.Handle)
	app.Get(SessionInventoryPath, ready, routes.Browser.NoTouch, routes.SessionInventory.Get)
	app.Delete(SessionRevokePath, ready, routes.Browser.NoTouch, routes.SessionAdministration.RevokeOther)
	app.Post(SessionRevokeAllPath, ready, routes.Browser.NoTouch, routes.SessionAdministration.RevokeAll)
	app.Get(MePortalPath, ready, routes.Browser.NoTouch, routes.AccountType.Get)
	app.Post(SessionRenewPath, ready, routes.Browser.Renew, routes.Browser.RenewResult)
	app.Post(AccountTypeSelectionPath, ready, routes.Browser.Handle, routes.AccountType.Select)
	app.Get(RecoveryPreparationPath, ready, routes.Browser.Handle, routes.RecoveryPreparation.Get)
	app.Get(MFAReadinessPath, ready, routes.Browser.Handle, routes.MFAReadiness.Get)
	app.Post(MFAChallengePath, ready, routes.MFA.Challenge)
	app.Post(EmailLoginResendPath, ready, routes.MFA.ResendLoginCode)
	app.Post(EmailLoginCancelPath, ready, routes.MFA.CancelLoginCode)
	if !routes.MFA.emailOnly {
		app.Post(MFAEnrollPath, ready, routes.Browser.NoTouch, routes.MFA.Enroll)
		app.Post(MFAConfirmPath, ready, routes.Browser.NoTouch, routes.MFA.Confirm)
		app.Post(MFAStepUpPath, ready, routes.Browser.NoTouch, routes.MFA.StepUp)
		app.Post(MFARecoveryRegeneratePath, ready, routes.Browser.NoTouch, routes.MFA.Regenerate)
		app.Post(MFAReplacePath, ready, routes.Browser.NoTouch, routes.MFA.Replace)
	}
	app.Get(MembershipDirectoryPath, ready, routes.Browser.Handle, routes.Memberships.Handle)
	app.Post(LogoutPath, ready, routes.Browser.Handle, routes.Logout.Handle)
	return app, nil
}
