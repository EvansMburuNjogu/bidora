// Package invitation implements the approved opaque tenant-invitation token.
// It does not grant authority or persist invitation state.
package invitation

import (
	"bidora/backend/internal/invitationcode"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"
)

const TokenBytes = 32
const Lifetime = 72 * time.Hour

type Token struct {
	raw  string
	hash [32]byte
}

func (Token) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED invitation token]") }
func (t Token) RevealForDelivery() string   { return t.raw }
func (t Token) Hash() [32]byte              { return t.hash }
func (t Token) IsZero() bool                { return t.raw == "" || t.hash == [32]byte{} }
func Generate() (Token, error) {
	raw, hash, err := invitationcode.Generate(rand.Reader, "tenant")
	if err != nil {
		return Token{}, err
	}
	return Token{raw: raw, hash: hash}, nil
}
func GenerateWithRandom(random io.Reader) (Token, error) {
	if random == nil {
		return Token{}, errors.New("invitation_random_invalid")
	}
	raw := make([]byte, TokenBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return Token{}, errors.New("invitation_random_failed")
	}
	return Token{raw: base64.RawURLEncoding.EncodeToString(raw), hash: sha256.Sum256(raw)}, nil
}
func HashPresented(encoded string) ([32]byte, error) {
	if _, _, ok := invitationcode.Split(encoded); ok {
		return invitationcode.Hash("tenant", encoded)
	}
	if len(encoded) != base64.RawURLEncoding.EncodedLen(TokenBytes) {
		return [32]byte{}, errors.New("invitation_token_invalid")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != TokenBytes {
		return [32]byte{}, errors.New("invitation_token_invalid")
	}
	return sha256.Sum256(raw), nil
}
func ExpiresAt(createdAt time.Time) (time.Time, error) {
	if createdAt.IsZero() {
		return time.Time{}, errors.New("invitation_time_invalid")
	}
	return createdAt.UTC().Add(Lifetime), nil
}
