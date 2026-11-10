// Package verification implements the approved email-ownership token value. It
// does not deliver tokens, authorize requests or persist challenge transitions.
package verification

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"
)

const TokenBytes = 32
const Lifetime = 30 * time.Minute

type Token struct {
	raw                string
	hash               [32]byte
	display, reference string
}

func (Token) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED verification token]") }
func (t Token) RevealForDelivery() string {
	if t.display != "" {
		return t.display
	}
	return t.raw
}
func (t Token) ChallengeRef() string { return t.reference }
func (t Token) Hash() [32]byte       { return t.hash }
func (t Token) IsZero() bool         { return t.raw == "" || t.hash == [32]byte{} }

func Generate() (Token, error) { return GenerateWithRandom(rand.Reader) }
func GenerateWithRandom(random io.Reader) (Token, error) {
	if random == nil {
		return Token{}, errors.New("verification_random_invalid")
	}
	raw := make([]byte, TokenBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return Token{}, errors.New("verification_random_failed")
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return Token{raw: encoded, hash: sha256.Sum256(raw)}, nil
}
func HashPresented(encoded string) ([32]byte, error) {
	if len(encoded) != base64.RawURLEncoding.EncodedLen(TokenBytes) {
		return [32]byte{}, errors.New("verification_token_invalid")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != TokenBytes {
		return [32]byte{}, errors.New("verification_token_invalid")
	}
	return sha256.Sum256(raw), nil
}
func ExpiresAt(createdAt time.Time) (time.Time, error) {
	if createdAt.IsZero() {
		return time.Time{}, errors.New("verification_time_invalid")
	}
	return createdAt.UTC().Add(Lifetime), nil
}
