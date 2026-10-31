// Package session implements opaque browser-session and CSRF token values. It
// does not set cookies, authorize tenant operations or persist sessions.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"
)

const TokenBytes = 32
const IdleLifetime = 30 * time.Minute
const AbsoluteLifetime = 12 * time.Hour
const PlatformIdleLifetime = 7 * 24 * time.Hour
const PlatformAbsoluteLifetime = 7 * 24 * time.Hour
const CookieName = "__Host-bidora_session"
const CSRFCookieName = "__Host-bidora_csrf"
const CSRFHeader = "X-CSRF-Token"

type CookiePolicy struct {
	Name, Path, SameSite       string
	Secure, HttpOnly, HostOnly bool
	Idle, Absolute             time.Duration
}

func PilotCookiePolicy() CookiePolicy {
	return CookiePolicy{Name: CookieName, Path: "/", SameSite: "Lax", Secure: true, HttpOnly: true, HostOnly: true, Idle: IdleLifetime, Absolute: AbsoluteLifetime}
}
func PilotCSRFCookiePolicy() CookiePolicy {
	return CookiePolicy{Name: CSRFCookieName, Path: "/", SameSite: "Lax", Secure: true, HttpOnly: false, HostOnly: true, Idle: IdleLifetime, Absolute: AbsoluteLifetime}
}

type SessionToken struct {
	raw  string
	hash [32]byte
}
type CSRFToken struct {
	raw  string
	hash [32]byte
}
type Pair struct {
	Session SessionToken
	CSRF    CSRFToken
}

func (SessionToken) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED session token]") }
func (CSRFToken) Format(s fmt.State, verb rune)    { fmt.Fprint(s, "[REDACTED csrf token]") }
func (t SessionToken) RevealForCookie() string     { return t.raw }
func (t CSRFToken) RevealForBootstrap() string     { return t.raw }
func (t SessionToken) Hash() [32]byte              { return t.hash }
func (t CSRFToken) Hash() [32]byte                 { return t.hash }
func (t SessionToken) IsZero() bool                { return t.raw == "" || t.hash == [32]byte{} }
func (t CSRFToken) IsZero() bool                   { return t.raw == "" || t.hash == [32]byte{} }

func GeneratePair() (Pair, error) { return GeneratePairWithRandom(rand.Reader) }
func GeneratePairWithRandom(random io.Reader) (Pair, error) {
	if random == nil {
		return Pair{}, errors.New("session_random_invalid")
	}
	sessionRaw := make([]byte, TokenBytes)
	csrfRaw := make([]byte, TokenBytes)
	if _, err := io.ReadFull(random, sessionRaw); err != nil {
		return Pair{}, errors.New("session_random_failed")
	}
	if _, err := io.ReadFull(random, csrfRaw); err != nil {
		return Pair{}, errors.New("session_random_failed")
	}
	return Pair{Session: newSessionToken(sessionRaw), CSRF: newCSRFToken(csrfRaw)}, nil
}
func HashPresentedSession(encoded string) ([32]byte, error) {
	return hashPresented("bidora.session.v1\x00", encoded)
}
func HashPresentedCSRF(encoded string) ([32]byte, error) {
	return hashPresented("bidora.csrf.v1\x00", encoded)
}
func MatchesCSRF(encoded string, expected [32]byte) (bool, error) {
	actual, err := HashPresentedCSRF(encoded)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual[:], expected[:]) == 1, nil
}
func PlatformExpiries(createdAt time.Time) (idle, absolute time.Time, err error) {
	if createdAt.IsZero() {
		return time.Time{}, time.Time{}, errors.New("session_time_invalid")
	}
	createdAt = createdAt.UTC()
	return createdAt.Add(PlatformIdleLifetime), createdAt.Add(PlatformAbsoluteLifetime), nil
}
func Expiries(createdAt time.Time) (idle, absolute time.Time, err error) {
	if createdAt.IsZero() {
		return time.Time{}, time.Time{}, errors.New("session_time_invalid")
	}
	createdAt = createdAt.UTC()
	return createdAt.Add(IdleLifetime), createdAt.Add(AbsoluteLifetime), nil
}

func newSessionToken(raw []byte) SessionToken {
	return SessionToken{raw: base64.RawURLEncoding.EncodeToString(raw), hash: domainHash("bidora.session.v1\x00", raw)}
}
func newCSRFToken(raw []byte) CSRFToken {
	return CSRFToken{raw: base64.RawURLEncoding.EncodeToString(raw), hash: domainHash("bidora.csrf.v1\x00", raw)}
}
func hashPresented(domain, encoded string) ([32]byte, error) {
	if len(encoded) != base64.RawURLEncoding.EncodedLen(TokenBytes) {
		return [32]byte{}, errors.New("session_token_invalid")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != TokenBytes {
		return [32]byte{}, errors.New("session_token_invalid")
	}
	return domainHash(domain, raw), nil
}
func domainHash(domain string, raw []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write(raw)
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}
