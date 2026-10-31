// Package attempt defines the approved login-attempt policy and privacy-safe
// scope keys. Counter storage and trusted-proxy address extraction are adapters.
package attempt

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"time"
)

type Policy struct {
	IdentityFailureLimit         int
	IdentityWindow, IdentityDeny time.Duration
	NetworkMinuteLimit           int
	NetworkMinuteWindow          time.Duration
	NetworkWindowLimit           int
	NetworkWindow, NetworkDeny   time.Duration
}

func PilotPolicy() Policy {
	return Policy{5, 15 * time.Minute, 15 * time.Minute, 30, time.Minute, 100, 15 * time.Minute, 15 * time.Minute}
}

type Key [32]byte

func (k Key) IsZero() bool                { return k == Key{} }
func (Key) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED attempt scope]") }

type Scopes struct{ Identity, Network Key }

func (s Scopes) Valid() bool { return !s.Identity.IsZero() && !s.Network.IsZero() }

type Deriver struct{ secret []byte }

func NewDeriver(secret []byte) (*Deriver, error) {
	if len(secret) < 32 {
		return nil, errors.New("attempt_hmac_secret_invalid")
	}
	owned := append([]byte(nil), secret...)
	return &Deriver{owned}, nil
}
func (d *Deriver) Identity(emailLookup string) (Key, error) {
	if d == nil || len(d.secret) < 32 || emailLookup == "" {
		return Key{}, errors.New("attempt_identity_scope_invalid")
	}
	return d.derive("bidora.login.identity.v1\x00", emailLookup), nil
}
func (d *Deriver) Network(address netip.Addr) (Key, error) {
	if d == nil || len(d.secret) < 32 || !address.IsValid() {
		return Key{}, errors.New("attempt_network_scope_invalid")
	}
	return d.derive("bidora.login.network.v1\x00", address.Unmap().String()), nil
}
func (d *Deriver) RegistrationIdentity(emailLookup string) (Key, error) {
	if d == nil || len(d.secret) < 32 || emailLookup == "" {
		return Key{}, errors.New("attempt_identity_scope_invalid")
	}
	return d.derive("bidora.registration.identity.v1\x00", emailLookup), nil
}
func (d *Deriver) RegistrationNetwork(address netip.Addr) (Key, error) {
	if d == nil || len(d.secret) < 32 || !address.IsValid() {
		return Key{}, errors.New("attempt_network_scope_invalid")
	}
	return d.derive("bidora.registration.network.v1\x00", address.Unmap().String()), nil
}
func (d *Deriver) RecoveryIdentity(emailLookup string) (Key, error) {
	if d == nil || len(d.secret) < 32 || emailLookup == "" {
		return Key{}, errors.New("attempt_identity_scope_invalid")
	}
	return d.derive("bidora.recovery.identity.v1\x00", emailLookup), nil
}
func (d *Deriver) EmailLoginResendIdentity(user [16]byte) (Key, error) {
	if d == nil || len(d.secret) < 32 || user == ([16]byte{}) {
		return Key{}, errors.New("attempt_identity_scope_invalid")
	}
	mac := hmac.New(sha256.New, d.secret)
	_, _ = mac.Write([]byte("bidora.email-login-resend.identity.v1\x00"))
	_, _ = mac.Write(user[:])
	var key Key
	copy(key[:], mac.Sum(nil))
	return key, nil
}
func (d *Deriver) EmailLoginResendNetwork(address netip.Addr) (Key, error) {
	if d == nil || len(d.secret) < 32 || !address.IsValid() {
		return Key{}, errors.New("attempt_network_scope_invalid")
	}
	return d.derive("bidora.email-login-resend.network.v1\x00", address.Unmap().String()), nil
}
func (d *Deriver) MFA(purpose string, user, target [16]byte) (Key, error) {
	if d == nil || len(d.secret) < 32 || user == ([16]byte{}) || target == ([16]byte{}) {
		return Key{}, errors.New("attempt_mfa_scope_invalid")
	}
	switch purpose {
	case "enrollment", "login", "email-login", "step-up", "replacement":
	default:
		return Key{}, errors.New("attempt_mfa_scope_invalid")
	}
	mac := hmac.New(sha256.New, d.secret)
	_, _ = mac.Write([]byte("bidora.mfa.attempt.v1\x00" + purpose + "\x00"))
	_, _ = mac.Write(user[:])
	_, _ = mac.Write(target[:])
	var key Key
	copy(key[:], mac.Sum(nil))
	return key, nil
}
func (d *Deriver) OrganizationCreationIdentity(user [16]byte) (Key, error) {
	if d == nil || len(d.secret) < 32 || user == ([16]byte{}) {
		return Key{}, errors.New("attempt_identity_scope_invalid")
	}
	mac := hmac.New(sha256.New, d.secret)
	_, _ = mac.Write([]byte("bidora.organization-creation.identity.v1\x00"))
	_, _ = mac.Write(user[:])
	var key Key
	copy(key[:], mac.Sum(nil))
	return key, nil
}
func (d *Deriver) OrganizationCreationNetwork(address netip.Addr) (Key, error) {
	if d == nil || len(d.secret) < 32 || !address.IsValid() {
		return Key{}, errors.New("attempt_network_scope_invalid")
	}
	return d.derive("bidora.organization-creation.network.v1\x00", address.Unmap().String()), nil
}
func (d *Deriver) RecoveryNetwork(address netip.Addr) (Key, error) {
	if d == nil || len(d.secret) < 32 || !address.IsValid() {
		return Key{}, errors.New("attempt_network_scope_invalid")
	}
	return d.derive("bidora.recovery.network.v1\x00", address.Unmap().String()), nil
}
func (d *Deriver) derive(domain, value string) Key {
	mac := hmac.New(sha256.New, d.secret)
	_, _ = mac.Write([]byte(domain))
	_, _ = mac.Write([]byte(value))
	var key Key
	copy(key[:], mac.Sum(nil))
	return key
}
