package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/netip"
	"strings"
	"unicode/utf8"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	identityemail "bidora/backend/internal/identity/email"
	platformapp "bidora/backend/internal/platformadmin/application"
	"github.com/gofiber/fiber/v3"
)

type OpenRegistrationUseCase interface {
	Register(context.Context, identityapp.OpenRegistrationInput) (identityapp.OpenRegistrationResult, error)
}
type RegistrationNetworkKeyer interface {
	RegistrationNetwork(netip.Addr) (attempt.Key, error)
}

const InvitationRegistrationPath = "/v1/auth/invitation-register"
const InvitationDetailsPath = "/v1/auth/invitation-details"

type InvitationRegistrationResolver interface {
	RegistrationIdentity(context.Context, string) (platformapp.InvitationRegistrationIdentity, error)
}

type InvitationIdentityReader interface {
	LookupInvitationIdentity(context.Context, string) (identityapp.RecoveryAccountFacts, error)
}

type RegistrationHandler struct {
	activation   InvitationActivationUseCase
	codes        *EmailCodes
	credentials  InvitationIdentityReader
	resend       EmailVerificationResendUseCase
	invitation   InvitationRegistrationResolver
	registration OpenRegistrationUseCase
	protection   *browser.Protection
	ips          LoginClientIPResolver
	keys         RegistrationNetworkKeyer
}

func NewRegistrationHandler(registration OpenRegistrationUseCase, protection *browser.Protection, ips LoginClientIPResolver, keys RegistrationNetworkKeyer) (*RegistrationHandler, error) {
	if registration == nil || protection == nil || ips == nil || keys == nil {
		return nil, errors.New("registration_handler_invalid")
	}
	return &RegistrationHandler{registration: registration, protection: protection, ips: ips, keys: keys}, nil
}
func (h *RegistrationHandler) EnableInvitationVerification(reader InvitationIdentityReader, resend EmailVerificationResendUseCase) {
	h.credentials = reader
	h.resend = resend
}
func (h *RegistrationHandler) EnableShortCodes(codes *EmailCodes) { h.codes = codes }
func (h *RegistrationHandler) EnableInvitationRegistration(resolver InvitationRegistrationResolver) {
	h.invitation = resolver
}
func (h *RegistrationHandler) Handle(c fiber.Ctx) error            { return h.handle(c, false, false) }
func (h *RegistrationHandler) HandleInvitation(c fiber.Ctx) error  { return h.handle(c, true, false) }
func (h *RegistrationHandler) InvitationDetails(c fiber.Ctx) error { return h.handle(c, true, true) }
func (h *RegistrationHandler) handle(c fiber.Ctx, invited, details bool) error {
	if c == nil {
		return errors.New("registration_http_context_required")
	}
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.registration == nil || h.protection == nil || h.ips == nil || h.keys == nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok || !validJSONMediaType(contentType) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	var firstName, lastName, emailValue, passwordValue, invitationToken string
	var err error
	if invited {
		var body struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if len(c.Body()) > LoginBodyLimit {
			return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
		}
		if !utf8.Valid(c.Body()) || !validLoginSurrogates(c.Body()) || hasDuplicateWorkflowField(c.Body()) {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		decoder := json.NewDecoder(bytes.NewReader(c.Body()))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		if details && !requireWorkflowFields(c.Body(), []string{"token"}) {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		invitationToken, passwordValue = body.Token, body.Password
	} else {
		firstName, lastName, emailValue, passwordValue, err = decodeRegistration(c.Body())
	}
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	passwordBytes := []byte(passwordValue)
	defer func() {
		for index := range passwordBytes {
			passwordBytes[index] = 0
		}
	}()
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	networkScope, err := h.keys.RegistrationNetwork(clientIP)
	if err != nil || networkScope.IsZero() {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	correlationID, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	setupState := "CREATE"
	if invited {
		if h.invitation == nil {
			return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
		}
		var proof identityapp.AttemptReservation
		if h.codes != nil {
			var guardErr error
			proof, guardErr = h.codes.invitationAttempt(c, "platform", invitationToken)
			if guardErr != nil {
				return fail(c, 429, "INVITATION_LIMITED", "Please wait or use a fresh invitation.", true)
			}
		}
		identity, resolveErr := h.invitation.RegistrationIdentity(c.Context(), invitationToken)
		if proof != nil {
			if e := proof.Finish(c.Context(), resolveErr == nil); e != nil {
				return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
			}
		}
		if resolveErr != nil {
			if errors.Is(resolveErr, platformapp.ErrInvitationInvalid) {
				return fail(c, 400, "INVITATION_INVALID", "This invitation cannot be used.", false)
			}
			return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
		}
		if h.credentials != nil {
			address, parseErr := identityemail.Parse(identity.Email)
			if parseErr != nil {
				return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
			}
			record, lookupErr := h.credentials.LookupInvitationIdentity(c.Context(), address.Lookup())
			if lookupErr == nil {
				if record.AccountDisabled {
					return fail(c, 400, "INVITATION_INVALID", "This invitation cannot be used.", false)
				}
				if record.EmailVerified {
					setupState = "SIGN_IN"
					if !record.PasswordCredentialPresent && record.GoogleIdentityPresent {
						setupState = "GOOGLE_SIGN_IN"
					}
				} else {
					setupState = "VERIFY"
				}
			} else if !errors.Is(lookupErr, identityapp.ErrCredentialNotFound) {
				return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
			}
		}
		if details && h.activation != nil && setupState != "GOOGLE_SIGN_IN" {
			reader, ok := h.credentials.(interface {
				InvitationPasswordEnrollmentAllowed(context.Context, string) (bool, error)
			})
			if !ok {
				return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Account setup is temporarily unavailable.", true)
			}
			address, _ := identityemail.Parse(identity.Email)
			allowed, stateErr := reader.InvitationPasswordEnrollmentAllowed(c.Context(), address.Lookup())
			if stateErr != nil {
				return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Account setup is temporarily unavailable.", true)
			}
			if allowed {
				setupState = "SET_PASSWORD"
			}
		}
		if details {
			return c.JSON(fiber.Map{"data": fiber.Map{"email": maskInvitationEmail(identity.Email), "role_key": string(identity.Role), "setup_state": setupState}, "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID")}})
		}
		firstName, lastName, emailValue = identity.FirstName, identity.LastName, identity.Email
	}
	if invited && (setupState == "SIGN_IN" || setupState == "GOOGLE_SIGN_IN") {
		return fail(c, 409, "INVITATION_SIGN_IN_REQUIRED", "This account is already verified. Sign in with your existing password.", false)
	}
	ctx := c.Context()
	reference := ""
	if h.codes != nil {
		ctx, reference, err = h.codes.issue(ctx, "verify")
		if err != nil {
			return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
		}
	}
	if invited && setupState == "VERIFY" {
		if h.resend == nil {
			return fail(c, 503, "VERIFICATION_DELIVERY_UNCONFIRMED", "Unable to confirm verification email delivery.", true)
		}
		sent, resendErr := h.resend.Resend(ctx, identityapp.EmailVerificationResendInput{Email: emailValue, CorrelationID: correlationID, NetworkScope: networkScope})
		if resendErr != nil && resendErr.Error() == "verification_resend_attempt_denied" {
			setCodeRetryAfter(c, resendErr)
			return fail(c, 429, "REGISTRATION_LIMITED", "Please wait before requesting another code.", true)
		}
		if resendErr != nil || !sent.Accepted || !sent.VerificationIssued {
			return fail(c, 503, "VERIFICATION_DELIVERY_UNCONFIRMED", "Unable to confirm verification email delivery.", true)
		}
		if reference != "" {
			c.Set(EmailCodeChallengeHeader, reference)
		}
		return c.SendStatus(fiber.StatusAccepted)
	}
	result, err := h.registration.Register(ctx, identityapp.OpenRegistrationInput{Registration: identityapp.SelfRegistrationInput{FirstName: firstName, LastName: lastName, Email: emailValue, Password: passwordBytes, CorrelationID: correlationID}, NetworkScope: networkScope})
	if err != nil {
		return registrationFailure(c, err)
	}
	if !result.Accepted {
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
	if invited && h.credentials != nil && !result.VerificationIssued {
		if result.Created {
			return fail(c, 503, "VERIFICATION_DELIVERY_UNCONFIRMED", "Password saved, but verification email delivery was not confirmed.", true)
		}
		return fail(c, 409, "INVITATION_SETUP_CHANGED", "This account is already set up. Reopen your invitation or sign in.", false)
	}
	if reference != "" {
		c.Set(EmailCodeChallengeHeader, reference)
	}
	return c.SendStatus(fiber.StatusAccepted)
}
func maskInvitationEmail(value string) string {
	local, domain, ok := strings.Cut(value, "@")
	if !ok || local == "" || domain == "" {
		return "***"
	}
	suffix := ""
	if dot := strings.LastIndex(domain, "."); dot >= 0 {
		suffix = domain[dot:]
	}
	return local[:1] + "***@" + domain[:1] + "***" + suffix
}

func decodeRegistration(body []byte) (string, string, string, string, error) {
	if len(body) > LoginBodyLimit || !utf8.Valid(body) || !validLoginSurrogates(body) {
		code := "INVALID_REQUEST"
		if len(body) > LoginBodyLimit {
			code = "REQUEST_TOO_LARGE"
		}
		return "", "", "", "", errors.New(code)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", "", "", "", errors.New("INVALID_REQUEST")
	}
	seen := map[string]bool{}
	values := map[string]string{}
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
		seen[key] = true
		switch key {
		case "first_name", "last_name", "email", "password":
		default:
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
		var value string
		if err = decoder.Decode(&value); err != nil {
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
		values[key] = value
	}
	if _, err = decoder.Token(); err != nil {
		return "", "", "", "", errors.New("INVALID_REQUEST")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return "", "", "", "", errors.New("INVALID_REQUEST")
	}
	for _, key := range []string{"first_name", "last_name", "email", "password"} {
		if !seen[key] {
			return "", "", "", "", errors.New("INVALID_REQUEST")
		}
	}
	return values["first_name"], values["last_name"], values["email"], values["password"], nil
}

func registrationFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "registration_invalid":
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case "registration_attempt_denied":
		setCodeRetryAfter(c, err)
		return fail(c, 429, "REGISTRATION_LIMITED", "Registration is temporarily unavailable.", true)
	case "registration_context_required", "registration_unavailable":
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	default:
		return fail(c, 503, "REGISTRATION_UNAVAILABLE", "Registration is temporarily unavailable.", true)
	}
}
func validJSONMediaType(value string) bool {
	mediaType, params, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json" && (len(params) == 0 || len(params) == 1 && strings.EqualFold(params["charset"], "utf-8"))
}
