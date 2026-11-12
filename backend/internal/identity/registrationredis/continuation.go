package registrationredis

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attemptredis"
)

// ContinuationStore is ephemeral replay control, not proof of identity or a
// session. Its opaque browser secret is valid only for a newly created user.
type ContinuationStore struct {
	runner attemptredis.ScriptRunner
	prefix string
}

func NewContinuationStore(runner attemptredis.ScriptRunner, prefix string) (*ContinuationStore, error) {
	if runner == nil || !validPrefix(prefix) {
		return nil, errors.New("registration_continuation_config_invalid")
	}
	return &ContinuationStore{runner: runner, prefix: prefix + "registration-continuation:"}, nil
}

func (s *ContinuationStore) key(secret string) (string, error) {
	if s == nil || s.runner == nil {
		return "", errors.New("registration_continuation_unavailable")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != secret {
		return "", errors.New("registration_continuation_invalid")
	}
	hash := sha256.Sum256(decoded)
	return s.prefix + hex.EncodeToString(hash[:]), nil
}

// Save associates a browser secret with a newly created user. Callers must
// establish that the user was created by the same registration request.
func (s *ContinuationStore) Save(ctx context.Context, secret string, user identityapp.ID) error {
	if ctx == nil || user.IsZero() {
		return errors.New("registration_continuation_invalid")
	}
	key, err := s.key(secret)
	if err != nil {
		return err
	}
	result, err := s.runner.Eval(ctx, `if redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2], 'NX') then return 1 end return 0`, []string{key}, hex.EncodeToString(user[:]), identityapp.RegistrationContinuationLifetime.Milliseconds())
	if err != nil || result != int64(1) {
		return errors.New("registration_continuation_unavailable")
	}
	return nil
}

// Consume matches the verified user and removes the binding atomically. A
// missing, expired, mismatched or already-used binding never grants authority.
func (s *ContinuationStore) Consume(ctx context.Context, secret string, verifiedUser identityapp.ID) (bool, error) {
	if ctx == nil || verifiedUser.IsZero() {
		return false, errors.New("registration_continuation_invalid")
	}
	key, err := s.key(secret)
	if err != nil {
		return false, err
	}
	result, err := s.runner.Eval(ctx, `local value=redis.call('GET',KEYS[1]); if not value or value~=ARGV[1] then return 0 end; redis.call('DEL',KEYS[1]); return 1`, []string{key}, hex.EncodeToString(verifiedUser[:]))
	if err != nil {
		return false, errors.New("registration_continuation_unavailable")
	}
	switch result {
	case int64(1):
		return true, nil
	case int64(0):
		return false, nil
	default:
		return false, errors.New("registration_continuation_unavailable")
	}
}
