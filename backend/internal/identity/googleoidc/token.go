package googleoidc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const maxIDTokenBytes = 16 * 1024

var ErrKeyNotFound = errors.New("google_oidc_key_not_found")

type KeySource interface {
	Key(context.Context, string) (*rsa.PublicKey, error)
}
type Clock interface{ Now() time.Time }
type TokenVerifier struct {
	keys      KeySource
	validator *Validator
	clock     Clock
}
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }
func NewTokenVerifier(clientID string, keys KeySource) (*TokenVerifier, error) {
	return NewTokenVerifierWithClock(clientID, keys, systemClock{})
}
func NewTokenVerifierWithClock(clientID string, keys KeySource, clock Clock) (*TokenVerifier, error) {
	validator, err := NewValidator(Config{ClientID: clientID})
	if err != nil || keys == nil || clock == nil {
		return nil, errors.New("google_oidc_verifier_config_invalid")
	}
	return &TokenVerifier{keys: keys, validator: validator, clock: clock}, nil
}
func (v *TokenVerifier) Verify(ctx context.Context, rawToken, expectedNonce string) (Identity, error) {
	if v == nil || v.keys == nil || v.validator == nil || v.clock == nil || ctx == nil || len(rawToken) < 1 || len(rawToken) > maxIDTokenBytes || strings.TrimSpace(rawToken) != rawToken {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	headerBytes, err := decodeSegment(parts[0], 2048)
	if err != nil || uniqueJSON(headerBytes) != nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "RS256" || !boundedASCII(header.KeyID, 1, 255) || (header.Type != "" && header.Type != "JWT") {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	key, err := v.keys.Key(ctx, header.KeyID)
	if errors.Is(err, ErrKeyNotFound) || err == nil && key == nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	if err != nil {
		return Identity{}, &tokenError{cause: err}
	}
	signature, err := decodeSegment(parts[2], 1024)
	if err != nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	payloadBytes, err := decodeSegment(parts[1], maxIDTokenBytes)
	if err != nil || uniqueJSON(payloadBytes) != nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	var payload struct {
		Issuer        string      `json:"iss"`
		Audience      string      `json:"aud"`
		Subject       string      `json:"sub"`
		Email         string      `json:"email"`
		EmailVerified bool        `json:"email_verified"`
		Nonce         string      `json:"nonce"`
		IssuedAt      json.Number `json:"iat"`
		ExpiresAt     json.Number `json:"exp"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(payloadBytes)))
	decoder.UseNumber()
	if decoder.Decode(&payload) != nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	issuedUnix, err := payload.IssuedAt.Int64()
	if err != nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	expiresUnix, err := payload.ExpiresAt.Int64()
	if err != nil {
		return Identity{}, errors.New("google_oidc_token_invalid")
	}
	return v.validator.Validate(Claims{Issuer: payload.Issuer, Audience: payload.Audience, Subject: payload.Subject, Email: payload.Email, EmailVerified: payload.EmailVerified, Nonce: payload.Nonce, IssuedAt: time.Unix(issuedUnix, 0).UTC(), ExpiresAt: time.Unix(expiresUnix, 0).UTC()}, expectedNonce, v.clock.Now())
}
func decodeSegment(value string, max int) ([]byte, error) {
	if value == "" || len(value) > base64.RawURLEncoding.EncodedLen(max) {
		return nil, errors.New("google_oidc_token_invalid")
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(decoded) > max {
		return nil, errors.New("google_oidc_token_invalid")
	}
	return decoded, nil
}
func boundedASCII(value string, min, max int) bool {
	if len(value) < min || len(value) > max {
		return false
	}
	for _, b := range []byte(value) {
		if b < 0x21 || b > 0x7e {
			return false
		}
	}
	return true
}
func uniqueJSON(body []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if err := walkJSON(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("google_oidc_json_invalid")
	}
	return nil
}
func walkJSON(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("google_oidc_json_invalid")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("google_oidc_json_invalid")
			}
			if _, exists := seen[key]; exists {
				return errors.New("google_oidc_json_invalid")
			}
			seen[key] = struct{}{}
			if err = walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("google_oidc_json_invalid")
		}
	case '[':
		for decoder.More() {
			if err = walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("google_oidc_json_invalid")
		}
	default:
		return errors.New("google_oidc_json_invalid")
	}
	return nil
}

type tokenError struct{ cause error }

func (e *tokenError) Error() string                 { return "google_oidc_verification_unavailable" }
func (e *tokenError) Unwrap() error                 { return e.cause }
func (e *tokenError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
