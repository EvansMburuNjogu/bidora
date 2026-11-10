package verification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"regexp"
)

var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

type deliveryTokenKey struct{}

func WithDeliveryToken(ctx context.Context, token Token) context.Context {
	return context.WithValue(ctx, deliveryTokenKey{}, token)
}
func GenerateForContext(ctx context.Context, fallback func() (Token, error)) (Token, error) {
	if token, ok := ctx.Value(deliveryTokenKey{}).(Token); ok && !token.IsZero() {
		return token, nil
	}
	return fallback()
}

// A short code is useful only with its random per-request reference. Its
// persisted authority is a purpose-separated, server-keyed 256-bit token.
func BoundCode(secret []byte, purpose, reference, code string) (Token, error) {
	ref, err := uuid.Parse(reference)
	if len(secret) != 32 || err != nil || ref == uuid.Nil || !sixDigits.MatchString(code) || (purpose != "verify" && purpose != "recovery") {
		return Token{}, errors.New("email_code_invalid")
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("bidora.email-code.v1\x00" + purpose + "\x00" + ref.String() + "\x00" + code))
	raw := mac.Sum(nil)
	return Token{raw: base64.RawURLEncoding.EncodeToString(raw), hash: sha256.Sum256(raw), display: code, reference: ref.String()}, nil
}

// Authority is only for the existing server-side single-use consumer,
// never an email, browser response or log.
func (t Token) Authority() string { return t.raw }
