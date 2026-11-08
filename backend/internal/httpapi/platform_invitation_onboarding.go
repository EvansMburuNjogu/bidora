package httpapi

import (
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"io"
	"net/netip"
)

type PlatformInvitationOnboarding interface {
	Preview(context.Context, string) (platformapp.InvitationPreview, error)
	SetUpNewAccount(context.Context, string, []byte, string, string, platformapp.ID, identityapp.RegistrationPassword) (string, error)
}
type PlatformInvitationOnboardingHandler struct {
	service  PlatformInvitationOnboarding
	login    *LoginHandler
	hasher   identityapp.RegistrationPassword
	attempts identityapp.AttemptController
	keys     *attempt.Deriver
}

func RegisterPlatformInvitationOnboarding(app *fiber.App, service PlatformInvitationOnboarding, login *LoginHandler, hasher identityapp.RegistrationPassword, attempts identityapp.AttemptController, keys *attempt.Deriver) error {
	if app == nil || service == nil || login == nil || hasher == nil || attempts == nil || keys == nil {
		return errors.New("invitation_onboarding_invalid")
	}
	h := &PlatformInvitationOnboardingHandler{service, login, hasher, attempts, keys}
	app.Post("/v1/auth/administrator-invitation/preview", h.preview)
	app.Post("/v1/auth/administrator-invitation/setup", h.setup)
	return nil
}

type invitationSetupBody struct {
	Token     string `json:"token"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (h *PlatformInvitationOnboardingHandler) input(c fiber.Ctx) (invitationSetupBody, identityapp.AttemptReservation, attempt.Key, bool) {
	c.Set("Cache-Control", "private, no-store")
	c.Set("Referrer-Policy", "no-referrer")
	c.Set("X-Content-Type-Options", "nosniff")
	if h.login.protection.ValidateOrigin(c.Get("Origin")) != nil {
		_ = fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
		return invitationSetupBody{}, nil, attempt.Key{}, false
	}
	if len(c.Request().URI().QueryString()) != 0 || len(c.Body()) < 2 || len(c.Body()) > 4096 || c.Get("Content-Type") != "application/json" {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return invitationSetupBody{}, nil, attempt.Key{}, false
	}
	var body invitationSetupBody
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if dec.Decode(&body) != nil || dec.Decode(new(any)) != io.EOF || len(body.Token) != 43 || len(body.Password) > 1024 || len(body.FirstName) > 100 || len(body.LastName) > 100 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return body, nil, attempt.Key{}, false
	}
	if c.RemoteAddr() == nil {
		_ = fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		return body, nil, attempt.Key{}, false
	}
	peer, e := netip.ParseAddrPort(c.RemoteAddr().String())
	if e != nil {
		_ = fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		return body, nil, attempt.Key{}, false
	}
	ip, e := h.login.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if e != nil {
		_ = fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		return body, nil, attempt.Key{}, false
	}
	identity, e := h.keys.RegistrationIdentity("administrator-invitation:" + body.Token)
	if e != nil {
		_ = fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		return body, nil, attempt.Key{}, false
	}
	network, e := h.keys.RegistrationNetwork(ip)
	if e != nil {
		_ = fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		return body, nil, attempt.Key{}, false
	}
	r, e := h.attempts.Begin(c.Context(), attempt.Scopes{Identity: identity, Network: network})
	if e != nil || r == nil {
		if errors.Is(e, identityapp.ErrAttemptDenied) {
			_ = fail(c, 429, "TOO_MANY_ATTEMPTS", "Please wait before trying again.", true)
		} else {
			_ = fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		}
		return body, nil, attempt.Key{}, false
	}
	loginNetwork, e := h.keys.Network(ip)
	if e != nil {
		_ = r.Finish(c.Context(), false)
		_ = fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
		return body, nil, attempt.Key{}, false
	}
	return body, r, loginNetwork, true
}
func (h *PlatformInvitationOnboardingHandler) preview(c fiber.Ctx) error {
	body, reservation, _, ok := h.input(c)
	if !ok {
		return nil
	}
	result, e := h.service.Preview(c.Context(), body.Token)
	if finish := reservation.Finish(c.Context(), e == nil); finish != nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if e != nil {
		return onboardingFailure(c, e)
	}
	return c.JSON(fiber.Map{"data": result})
}
func (h *PlatformInvitationOnboardingHandler) setup(c fiber.Ctx) error {
	body, reservation, network, ok := h.input(c)
	if !ok {
		return nil
	}
	correlation, valid := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !valid {
		_ = reservation.Finish(c.Context(), false)
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	password := []byte(body.Password)
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	address, e := h.service.SetUpNewAccount(c.Context(), body.Token, password, body.FirstName, body.LastName, platformapp.ID(correlation), h.hasher)
	if finish := reservation.Finish(c.Context(), e == nil); finish != nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Account setup could not be confirmed. Try signing in with your chosen password.", true)
	}
	if e != nil {
		return onboardingFailure(c, e)
	}
	result, e := h.login.login.Login(c.Context(), identityapp.LoginInput{Credentials: identityapp.AuthenticateInput{Email: address, Password: password, NetworkScope: network}, CorrelationID: correlation})
	if e != nil {
		return loginFailure(c, e)
	}
	// New administrators must finish the canonical email-OTP MFA before access.
	if !result.MFARequired {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Sign in to finish administrator verification.", true)
	}
	return writeLoginResult(c, result)
}
func onboardingFailure(c fiber.Ctx, e error) error {
	if errors.Is(e, platformapp.ErrInvitedAccountExists) {
		return fail(c, 409, "INVITED_ACCOUNT_EXISTS", "This account already exists. Sign in with its existing password; the invitation cannot reset it.", false)
	}
	if errors.Is(e, platformapp.ErrInvitationInvalid) {
		return fail(c, 400, "INVITATION_INVALID", "The invitation or account details are invalid. The invitation may have expired, been cancelled or already been accepted.", false)
	}
	return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Invitation setup is temporarily unavailable.", true)
}
