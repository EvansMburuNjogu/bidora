// Package googleoidc validates security-relevant Google claims after signature
// verification. It does not fetch keys, exchange codes or perform HTTP redirects.
package googleoidc

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/identity/email"
)

const maxTokenLifetime = 2 * time.Hour
const clockSkew = 2 * time.Minute

type Config struct{ ClientID string }

func (Config) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED Google OIDC config]") }

type Claims struct {
	Issuer, Audience, Subject, Email, Nonce string
	EmailVerified                           bool
	IssuedAt, ExpiresAt                     time.Time
}

func (Claims) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED Google OIDC claims]") }

type Identity struct {
	Subject             string
	Email               email.Address
	IssuedAt, ExpiresAt time.Time
}

func (Identity) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED Google OIDC identity]") }

type Validator struct{ clientID string }

func NewValidator(config Config) (*Validator, error) {
	if !bounded(config.ClientID, 1, 255) {
		return nil, errors.New("google_oidc_config_invalid")
	}
	return &Validator{config.ClientID}, nil
}
func (v *Validator) Validate(claims Claims, expectedNonce string, now time.Time) (Identity, error) {
	if v == nil || v.clientID == "" || now.IsZero() || !bounded(expectedNonce, 32, 512) {
		return Identity{}, errors.New("google_oidc_invalid")
	}
	if claims.Issuer != "https://accounts.google.com" && claims.Issuer != "accounts.google.com" {
		return Identity{}, errors.New("google_oidc_invalid")
	}
	if !constantEqual(claims.Audience, v.clientID) || !constantEqual(claims.Nonce, expectedNonce) || !claims.EmailVerified || !bounded(claims.Subject, 1, 255) {
		return Identity{}, errors.New("google_oidc_invalid")
	}
	for _, value := range []byte(claims.Subject) {
		if value < 0x21 || value > 0x7e {
			return Identity{}, errors.New("google_oidc_invalid")
		}
	}
	address, err := email.Parse(claims.Email)
	if err != nil {
		return Identity{}, errors.New("google_oidc_invalid")
	}
	issuedAt, expiresAt, current := claims.IssuedAt.UTC(), claims.ExpiresAt.UTC(), now.UTC()
	if issuedAt.IsZero() || expiresAt.IsZero() || issuedAt.After(current.Add(clockSkew)) || !expiresAt.After(current.Add(-clockSkew)) || !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > maxTokenLifetime {
		return Identity{}, errors.New("google_oidc_invalid")
	}
	return Identity{Subject: claims.Subject, Email: address, IssuedAt: issuedAt, ExpiresAt: expiresAt}, nil
}
func constantEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
func bounded(value string, min, max int) bool { return len(value) >= min && len(value) <= max }
