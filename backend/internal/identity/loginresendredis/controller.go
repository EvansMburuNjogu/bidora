// Package loginresendredis enforces privacy-safe email-login resend limits.
// Redis holds only HMAC-derived scopes and ephemeral counters; it is never
// authentication or delivery truth.
package loginresendredis

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"bidora/backend/internal/identity/attemptredis"
)

const allowScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end
local identity = tonumber(redis.call('GET', KEYS[2]) or '0')
local network = tonumber(redis.call('GET', KEYS[3]) or '0')
if identity >= tonumber(ARGV[3]) or network >= tonumber(ARGV[4]) then return 0 end
if not redis.call('SET', KEYS[1], '1', 'PX', ARGV[1], 'NX') then return 0 end
identity = redis.call('INCR', KEYS[2])
if identity == 1 then redis.call('PEXPIRE', KEYS[2], ARGV[2]) end
network = redis.call('INCR', KEYS[3])
if network == 1 then redis.call('PEXPIRE', KEYS[3], ARGV[2]) end
return 1
`

const (
	cooldown      = 30 * time.Second
	window        = 15 * time.Minute
	identityLimit = 3
	networkLimit  = 10
)

var ErrDenied = errors.New("email_login_resend_denied")

type Controller struct {
	runner attemptredis.ScriptRunner
	prefix string
}

func New(runner attemptredis.ScriptRunner, prefix string) (*Controller, error) {
	if runner == nil || !validPrefix(prefix) {
		return nil, errors.New("email_login_resend_config_invalid")
	}
	return &Controller{runner: runner, prefix: prefix}, nil
}

func (c *Controller) Allow(ctx context.Context, identity, network [32]byte) error {
	if ctx == nil || c == nil || c.runner == nil || identity == ([32]byte{}) || network == ([32]byte{}) {
		return errors.New("email_login_resend_unavailable")
	}
	base := c.prefix + ":{" + c.prefix + ":email-login-resend}:"
	keys := []string{
		base + "cooldown:" + hex.EncodeToString(identity[:]),
		base + "identity:" + hex.EncodeToString(identity[:]),
		base + "network:" + hex.EncodeToString(network[:]),
	}
	value, err := c.runner.Eval(ctx, allowScript, keys, cooldown.Milliseconds(), window.Milliseconds(), identityLimit, networkLimit)
	if err != nil {
		return &availabilityError{cause: err}
	}
	number, ok := integer(value)
	if !ok {
		return errors.New("email_login_resend_result_invalid")
	}
	if number == 0 {
		return ErrDenied
	}
	if number != 1 {
		return errors.New("email_login_resend_result_invalid")
	}
	return nil
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

type availabilityError struct{ cause error }

func (e *availabilityError) Error() string                 { return "email_login_resend_unavailable" }
func (e *availabilityError) Unwrap() error                 { return e.cause }
func (e *availabilityError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
