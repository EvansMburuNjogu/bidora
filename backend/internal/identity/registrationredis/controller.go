// Package registrationredis implements the approved atomic open-registration
// identity/network limits. It stores only HMAC-derived scopes.
package registrationredis

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
)

const script = `
local function remaining()
  local wait = math.max(0, redis.call('PTTL', KEYS[2]), redis.call('PTTL', KEYS[4]))
  if tonumber(redis.call('GET', KEYS[1]) or '0') >= tonumber(ARGV[3]) then wait = math.max(wait, redis.call('PTTL', KEYS[1])) end
  if tonumber(redis.call('GET', KEYS[3]) or '0') >= tonumber(ARGV[4]) then wait = math.max(wait, redis.call('PTTL', KEYS[3])) end
  return math.max(1, wait)
end
if redis.call('EXISTS', KEYS[2]) == 1 or redis.call('EXISTS', KEYS[4]) == 1 then return -remaining() end
local identity = redis.call('INCR', KEYS[1])
if identity == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local network = redis.call('INCR', KEYS[3])
if network == 1 then redis.call('PEXPIRE', KEYS[3], ARGV[2]) end
if identity > tonumber(ARGV[3]) then
  redis.call('SET', KEYS[2], '1', 'PX', math.max(tonumber(ARGV[5]), redis.call('PTTL', KEYS[1])))
  return -remaining()
end
if network > tonumber(ARGV[4]) then
  redis.call('SET', KEYS[4], '1', 'PX', math.max(tonumber(ARGV[5]), redis.call('PTTL', KEYS[3])))
  return -remaining()
end
return 1
`
const (
	identityLimit = 3
	networkLimit  = 10
	window        = time.Hour
	deny          = 15 * time.Minute
)

type Controller struct {
	runner attemptredis.ScriptRunner
	prefix string
}

func New(runner attemptredis.ScriptRunner, prefix string) (*Controller, error) {
	if runner == nil || !validPrefix(prefix) {
		return nil, errors.New("registration_attempt_config_invalid")
	}
	return &Controller{runner, prefix}, nil
}
func (c *Controller) Allow(ctx context.Context, scopes attempt.Scopes) error {
	if ctx == nil || !scopes.Valid() {
		return errors.New("registration_attempt_scope_invalid")
	}
	if c == nil || c.runner == nil || !validPrefix(c.prefix) {
		return errors.New("registration_attempt_unavailable")
	}
	keys := []string{c.key("identity", scopes.Identity), c.key("identity-block", scopes.Identity), c.key("network", scopes.Network), c.key("network-block", scopes.Network)}
	value, err := c.runner.Eval(ctx, script, keys, window.Milliseconds(), window.Milliseconds(), identityLimit, networkLimit, deny.Milliseconds())
	if err != nil {
		return &controllerError{cause: err}
	}
	number, ok := integer(value)
	if !ok {
		return errors.New("registration_attempt_result_invalid")
	}
	if number <= 0 && number >= -window.Milliseconds() {
		wait := deny
		if number < 0 {
			wait = time.Duration(-number) * time.Millisecond
		}
		return &retryError{wait: wait}
	}
	if number != 1 {
		return errors.New("registration_attempt_result_invalid")
	}
	return nil
}
func (c *Controller) key(kind string, key attempt.Key) string {
	return c.prefix + ":{" + c.prefix + "}:registration:" + kind + ":" + hex.EncodeToString(key[:])
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

type retryError struct{ wait time.Duration }

func (e *retryError) Error() string          { return identityapp.ErrRegistrationAttemptDenied.Error() }
func (e *retryError) Unwrap() error          { return identityapp.ErrRegistrationAttemptDenied }
func (e *retryError) RetryAfterSeconds() int { return int((e.wait + time.Second - 1) / time.Second) }

type controllerError struct{ cause error }

func (e *controllerError) Error() string                 { return "registration_attempt_unavailable" }
func (e *controllerError) Unwrap() error                 { return e.cause }
func (e *controllerError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
