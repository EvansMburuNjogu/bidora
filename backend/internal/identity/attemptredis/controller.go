// Package attemptredis adapts the approved login-attempt policy to atomic Redis
// scripts. Redis remains ephemeral control state, never identity/business truth.
package attemptredis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/securitypolicy"
	redis "github.com/redis/go-redis/v9"
)

const reservationLifetime = 5 * time.Minute
const beginScript = `
if redis.call('EXISTS', KEYS[5]) == 1 then return 0 end
local minute = redis.call('INCR', KEYS[3])
if minute == 1 then redis.call('PEXPIRE', KEYS[3], ARGV[1]) end
local window = redis.call('INCR', KEYS[4])
if window == 1 then redis.call('PEXPIRE', KEYS[4], ARGV[2]) end
if minute > tonumber(ARGV[3]) or window > tonumber(ARGV[4]) then
  redis.call('SET', KEYS[5], '1', 'PX', ARGV[5])
  return 0
end
if redis.call('EXISTS', KEYS[2]) == 1 then return 0 end
if not redis.call('SET', KEYS[6], '1', 'PX', ARGV[6], 'NX') then return -1 end
return 1
`
const finishScript = `
if redis.call('EXISTS', KEYS[3]) == 0 then return 0 end
redis.call('DEL', KEYS[3])
if ARGV[1] == '1' then
  redis.call('DEL', KEYS[1], KEYS[2])
  return 1
end
local failures = redis.call('INCR', KEYS[1])
if failures == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[2]) end
if failures >= tonumber(ARGV[3]) then redis.call('SET', KEYS[2], '1', 'PX', ARGV[4]) end
return 1
`

type ScriptRunner interface {
	Eval(context.Context, string, []string, ...any) (any, error)
}
type ClientRunner struct {
	client    redis.UniversalClient
	closeOnce sync.Once
	closeErr  error
}

func NewClientRunner(client redis.UniversalClient) (*ClientRunner, error) {
	if client == nil {
		return nil, errors.New("redis_attempt_client_invalid")
	}
	return &ClientRunner{client: client}, nil
}
func (r *ClientRunner) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("redis_attempt_client_invalid")
	}
	return r.client.Eval(ctx, script, keys, args...).Result()
}

type Controller struct {
	runner ScriptRunner
	random io.Reader
	policy attempt.Policy
	prefix string
}

func NewSecureController(runner ScriptRunner, prefix string) (*Controller, error) {
	return NewController(runner, rand.Reader, prefix)
}
func NewController(runner ScriptRunner, random io.Reader, prefix string) (*Controller, error) {
	if runner == nil || random == nil || !validPrefix(prefix) {
		return nil, errors.New("redis_attempt_config_invalid")
	}
	return &Controller{runner, random, attempt.PilotPolicy(), prefix}, nil
}
func (c *Controller) Begin(ctx context.Context, scopes attempt.Scopes) (identityapp.AttemptReservation, error) {
	if ctx == nil || !scopes.Valid() {
		return nil, errors.New("redis_attempt_scope_invalid")
	}
	if c == nil || c.runner == nil || c.random == nil {
		return nil, errors.New("redis_attempt_unavailable")
	}
	reservationRaw := make([]byte, 16)
	if _, err := io.ReadFull(c.random, reservationRaw); err != nil {
		return nil, errors.New("redis_attempt_random_failed")
	}
	reservationID := hex.EncodeToString(reservationRaw)
	identityKey := hex.EncodeToString(scopes.Identity[:])
	networkKey := hex.EncodeToString(scopes.Network[:])
	base := c.prefix + ":{login-attempt}"
	keys := []string{base + ":i:fail:" + identityKey, base + ":i:block:" + identityKey, base + ":n:minute:" + networkKey, base + ":n:window:" + networkKey, base + ":n:block:" + networkKey, base + ":r:" + reservationID}
	result, err := c.runner.Eval(ctx, beginScript, keys, millis(c.policy.NetworkMinuteWindow), millis(c.policy.NetworkWindow), c.policy.NetworkMinuteLimit, c.policy.NetworkWindowLimit, millis(c.policy.NetworkDeny), millis(reservationLifetime))
	if err != nil {
		return nil, err
	}
	value, ok := integer(result)
	if !ok || value < 0 {
		return nil, errors.New("redis_attempt_result_invalid")
	}
	if value == 0 {
		return nil, identityapp.ErrAttemptDenied
	}
	if value != 1 {
		return nil, errors.New("redis_attempt_result_invalid")
	}
	p := c.policy
	configured := securitypolicy.From(ctx)
	p.IdentityFailureLimit = int(configured.FailedLoginAttempts)
	p.IdentityWindow = time.Duration(configured.FailureWindowSeconds) * time.Second
	p.IdentityDeny = time.Duration(configured.LockoutSeconds) * time.Second
	return &reservation{policy: p, controller: c, identityFailureKey: keys[0], identityBlockKey: keys[1], reservationKey: keys[5]}, nil
}

// ClearRecoveredIdentity removes only this identity's password failures and
// lock. Network budgets, network blocks and email-code controls are untouched.
func (c *Controller) ClearRecoveredIdentity(ctx context.Context, key attempt.Key) error {
	if ctx == nil || key.IsZero() || c == nil || c.runner == nil {
		return errors.New("redis_attempt_scope_invalid")
	}
	base := c.prefix + ":{login-attempt}:i:"
	suffix := hex.EncodeToString(key[:])
	result, err := c.runner.Eval(ctx, `return redis.call("DEL", KEYS[1], KEYS[2])`, []string{base + "fail:" + suffix, base + "block:" + suffix})
	if err != nil {
		return err
	}
	value, ok := integer(result)
	if !ok || value < 0 || value > 2 {
		return errors.New("redis_attempt_result_invalid")
	}
	return nil
}

type reservation struct {
	policy                                               attempt.Policy
	controller                                           *Controller
	identityFailureKey, identityBlockKey, reservationKey string
}

func (r *reservation) Finish(ctx context.Context, success bool) error {
	if ctx == nil || r == nil || r.controller == nil {
		return errors.New("redis_attempt_reservation_invalid")
	}
	successArg := "0"
	if success {
		successArg = "1"
	}
	p := r.policy
	result, err := r.controller.runner.Eval(ctx, finishScript, []string{r.identityFailureKey, r.identityBlockKey, r.reservationKey}, successArg, millis(p.IdentityWindow), p.IdentityFailureLimit, millis(p.IdentityDeny))
	if err != nil {
		return err
	}
	value, ok := integer(result)
	if !ok || value != 1 {
		return errors.New("redis_attempt_reservation_invalid")
	}
	return nil
}
func integer(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	default:
		return 0, false
	}
}
func millis(value time.Duration) int64 { return value.Milliseconds() }
func validPrefix(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}
func (*Controller) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED redis attempt controller]")
}
