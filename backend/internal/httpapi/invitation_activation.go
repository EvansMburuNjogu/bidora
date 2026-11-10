package httpapi

import (
	"bidora/backend/internal/identity/attempt"
	platformapp "bidora/backend/internal/platformadmin/application"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"net/netip"
)

const InvitationActivationPath = "/v1/auth/invitation-activate"

type InvitationActivationUseCase interface {
	ActivateInvitation(context.Context, platformapp.ActivateInvitationInput) error
}

func (h *RegistrationHandler) EnableInvitationActivation(usecase InvitationActivationUseCase) {
	h.activation = usecase
}
func (h *RegistrationHandler) ActivateInvitation(c fiber.Ctx) error {
	if c == nil {
		return errors.New("invitation_context_required")
	}
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if h == nil || h.activation == nil || h.invitation == nil || h.ips == nil || h.keys == nil || h.protection == nil || h.codes == nil {
		return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
	}
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := bindWorkflow(c, &body); err != nil {
		return err
	}
	if !requireWorkflowFields(c.Body(), []string{"token", "password"}) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	rawKey, ok := singleHeader(c, IdempotencyKeyHeader)
	key, err := uuid.Parse(rawKey)
	if !ok || err != nil || key == uuid.Nil {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "Activation could not be started.", true)
	}
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
	}
	client, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
	}
	network, err := h.keys.RegistrationNetwork(client)
	if err != nil {
		return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
	}
	proof, err := h.codes.invitationAttempt(c, "platform", body.Token)
	if err != nil {
		return fail(c, 429, "INVITATION_LIMITED", "Please wait or use a fresh invitation.", true)
	}
	saved, lookupErr := h.invitation.RegistrationIdentity(c.Context(), body.Token)
	if proof != nil {
		if err = proof.Finish(c.Context(), lookupErr == nil); err != nil {
			return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
		}
	}
	if lookupErr != nil && !errors.Is(lookupErr, platformapp.ErrInvitationInvalid) {
		return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
	}
	limiter, ok := h.registration.(interface {
		AllowInvitationEnrollment(context.Context, string, attempt.Key) error
	})
	if !ok {
		return fail(c, 503, "ACTIVATION_UNAVAILABLE", "Activation is temporarily unavailable.", true)
	}
	subject := saved.Email
	if lookupErr != nil {
		digest := sha256.Sum256([]byte(body.Token))
		subject = "invitation:" + hex.EncodeToString(digest[:])
	}
	if err = limiter.AllowInvitationEnrollment(c.Context(), subject, network); err != nil {
		return registrationFailure(c, err)
	}
	password := []byte(body.Password)
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	err = h.activation.ActivateInvitation(c.Context(), platformapp.ActivateInvitationInput{PresentedToken: body.Token, Password: password, CorrelationID: platformapp.ID(correlation), IdempotencyKey: platformapp.ID(key)})
	if err != nil {
		if err.Error() == "invitation_password_invalid" {
			return fail(c, 400, "PASSWORD_INVALID", "Choose a password that meets the requirements.", false)
		}
		if errors.Is(err, platformapp.ErrInvitationInvalid) || errors.Is(err, platformapp.ErrInvitationConflict) || errors.Is(err, platformapp.ErrConcurrentChange) {
			return fail(c, 409, "INVITATION_UNAVAILABLE", "This invitation cannot activate this account. It may already be used or belong to an established account. Sign in normally or request a fresh invitation.", false)
		}
		return fail(c, 503, "ACTIVATION_UNCONFIRMED", "Activation could not be confirmed. Try signing in with your chosen password before retrying.", true)
	}
	// Enrollment never creates a browser session or inherits the inviter's session.
	return c.SendStatus(204)
}
