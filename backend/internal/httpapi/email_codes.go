package httpapi

import (
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/invitationcode"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"math/big"
	"net/netip"
)

const EmailCodeChallengeHeader = "X-Bidora-Code-Challenge"

type EmailCodes struct {
	secret   []byte
	attempts app.AttemptController
	keys     *attempt.Deriver
	ips      LoginClientIPResolver
}

func (*EmailCodes) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED email code support]") }
func invalidEmailCode(err error) bool {
	return errors.Is(err, app.ErrAttemptDenied) || err.Error() == "email_code_invalid"
}

func NewEmailCodes(secret []byte, attempts app.AttemptController, keys *attempt.Deriver, ips LoginClientIPResolver) (*EmailCodes, error) {
	if len(secret) != 32 || attempts == nil || keys == nil || ips == nil {
		return nil, errors.New("email_codes_config_invalid")
	}
	return &EmailCodes{append([]byte(nil), secret...), attempts, keys, ips}, nil
}
func (s *EmailCodes) issue(ctx context.Context, purpose string) (context.Context, string, error) {
	ref, err := uuid.NewRandom()
	if err != nil {
		return ctx, "", err
	}
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return ctx, "", err
	}
	token, err := verification.BoundCode(s.secret, purpose, ref.String(), fmt.Sprintf("%06d", number.Int64()))
	if err != nil {
		return ctx, "", err
	}
	return verification.WithDeliveryToken(ctx, token), ref.String(), nil
}
func (s *EmailCodes) redeem(c fiber.Ctx, purpose, reference, code string) (string, app.AttemptReservation, error) {
	token, err := verification.BoundCode(s.secret, purpose, reference, code)
	if err != nil {
		return "", nil, err
	}
	reservation, err := s.beginScope(c, "email-code:"+purpose+":"+token.ChallengeRef())
	return token.Authority(), reservation, err
}
func (s *EmailCodes) invitationAttempt(c fiber.Ctx, kind, wire string) (app.AttemptReservation, error) {
	nonce, _, ok := invitationcode.Split(wire)
	if !ok {
		return nil, nil
	}
	return s.beginScope(c, "invitation:"+kind+":"+nonce)
}
func (s *EmailCodes) beginScope(c fiber.Ctx, scope string) (app.AttemptReservation, error) {
	remote := c.RemoteAddr()
	if remote == nil {
		return nil, errors.New("email_code_network_required")
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return nil, err
	}
	ip, err := s.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return nil, err
	}
	identity, err := s.keys.Identity(scope)
	if err != nil {
		return nil, err
	}
	network, err := s.keys.Network(ip)
	if err != nil {
		return nil, err
	}
	return s.attempts.Begin(c.Context(), attempt.Scopes{Identity: identity, Network: network})
}
