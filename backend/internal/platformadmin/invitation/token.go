// Package invitation owns opaque Platform Administrator invitation tokens.
// Raw values may be revealed only to an authorized delivery adapter.
package invitation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	TokenBytes = 32
	Lifetime   = 72 * time.Hour
)

type Token struct {
	raw  string
	hash [32]byte
}

func (Token) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED platform invitation token]") }
func (t Token) RevealForDelivery() string   { return t.raw }
func (t Token) Hash() [32]byte              { return t.hash }
func (t Token) IsZero() bool                { return t.raw == "" || t.hash == [32]byte{} }
func Generate() (Token, error)              { return GenerateWithRandom(rand.Reader) }
func GenerateWithRandom(random io.Reader) (Token, error) {
	if random == nil {
		return Token{}, errors.New("platform_invitation_random_invalid")
	}
	raw := make([]byte, TokenBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return Token{}, errors.New("platform_invitation_random_failed")
	}
	return Token{raw: base64.RawURLEncoding.EncodeToString(raw), hash: sha256.Sum256(raw)}, nil
}
func HashPresented(encoded string) ([32]byte, error) {
	if len(encoded) != base64.RawURLEncoding.EncodedLen(TokenBytes) {
		return [32]byte{}, errors.New("platform_invitation_token_invalid")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != TokenBytes {
		return [32]byte{}, errors.New("platform_invitation_token_invalid")
	}
	return sha256.Sum256(raw), nil
}
func ExpiresAt(activatedAt time.Time) (time.Time, error) {
	if activatedAt.IsZero() {
		return time.Time{}, errors.New("platform_invitation_time_invalid")
	}
	return activatedAt.UTC().Add(Lifetime), nil
}
