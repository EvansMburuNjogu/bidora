// Package oidcredis stores short-lived Google OIDC transaction material in
// Redis. It is ephemeral replay control, never identity or session truth.
package oidcredis

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/googleoidc"
	redis "github.com/redis/go-redis/v9"
)

const createScript = `
if redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2], 'NX') then return 1 end
return 0
`
const consumeScript = `
local value = redis.call('GET', KEYS[1])
if not value then return false end
redis.call('DEL', KEYS[1])
return value
`

type Store struct {
	runner attemptredis.ScriptRunner
	prefix string
	ttl    time.Duration
}

func New(runner attemptredis.ScriptRunner, prefix string, ttl time.Duration) (*Store, error) {
	if runner == nil || !validPrefix(prefix) || ttl < time.Minute || ttl > 15*time.Minute {
		return nil, errors.New("google_oidc_transaction_config_invalid")
	}
	return &Store{runner: runner, prefix: prefix, ttl: ttl}, nil
}
func (s *Store) Create(ctx context.Context, state string, material googleoidc.TransactionMaterial) error {
	if ctx == nil || material.IsZero() {
		return errors.New("google_oidc_transaction_invalid")
	}
	key, err := s.key(state)
	if err != nil {
		return errors.New("google_oidc_transaction_invalid")
	}
	if s == nil || s.runner == nil {
		return errors.New("google_oidc_transaction_unavailable")
	}
	value := material.Nonce + "." + material.Verifier + "." + hex.EncodeToString(material.BrowserBindingHash[:]) + "." + string(material.Purpose) + "." + hex.EncodeToString(material.PrincipalBindingHash[:]) + "." + strconv.FormatInt(material.CredentialVersion, 10)
	result, err := s.runner.Eval(ctx, createScript, []string{key}, value, s.ttl.Milliseconds())
	if err != nil {
		return &storeError{cause: err}
	}
	number, ok := integer(result)
	if !ok || number != 1 {
		return errors.New("google_oidc_transaction_unavailable")
	}
	return nil
}
func (s *Store) Consume(ctx context.Context, state string) (googleoidc.TransactionMaterial, error) {
	if ctx == nil {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	key, err := s.key(state)
	if err != nil {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	if s == nil || s.runner == nil {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_unavailable")
	}
	result, err := s.runner.Eval(ctx, consumeScript, []string{key})
	if errors.Is(err, redis.Nil) {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	if err != nil {
		return googleoidc.TransactionMaterial{}, &storeError{cause: err}
	}
	value, ok := result.(string)
	if !ok {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	parts := strings.Split(value, ".")
	if len(parts) != 6 {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	decodedBindingHash, bindingErr := hex.DecodeString(parts[2])
	decodedPrincipalHash, principalErr := hex.DecodeString(parts[4])
	if bindingErr != nil || principalErr != nil || len(decodedBindingHash) != 32 || len(decodedPrincipalHash) != 32 {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	var browserBindingHash, principalBindingHash [32]byte
	copy(browserBindingHash[:], decodedBindingHash)
	copy(principalBindingHash[:], decodedPrincipalHash)
	credentialVersion, versionErr := strconv.ParseInt(parts[5], 10, 64)
	if versionErr != nil {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	material, err := googleoidc.ParseTransactionMaterial(parts[0], parts[1], browserBindingHash, googleoidc.TransactionPurpose(parts[3]), principalBindingHash, credentialVersion)
	if err != nil {
		return googleoidc.TransactionMaterial{}, errors.New("google_oidc_transaction_invalid")
	}
	return material, nil
}
func (s *Store) key(state string) (string, error) {
	if s == nil || !validPrefix(s.prefix) {
		return "", errors.New("google_oidc_transaction_unavailable")
	}
	hash, err := googleoidc.HashPresentedState(state)
	if err != nil {
		return "", err
	}
	return s.prefix + ":{" + s.prefix + "}:google-oidc:transaction:" + hex.EncodeToString(hash[:]), nil
}
func validPrefix(value string) bool {
	if len(value) < 1 || len(value) > 64 || strings.TrimSpace(value) != value || strings.ContainsAny(value, " {}\t\r\n") {
		return false
	}
	for _, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune(":_-.", rune(character)) {
			continue
		}
		return false
	}
	return true
}
func integer(value any) (int64, bool) {
	switch number := value.(type) {
	case int64:
		return number, true
	case int:
		return int64(number), true
	default:
		return 0, false
	}
}

type storeError struct{ cause error }

func (e *storeError) Error() string                 { return "google_oidc_transaction_unavailable" }
func (e *storeError) Unwrap() error                 { return e.cause }
func (e *storeError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
