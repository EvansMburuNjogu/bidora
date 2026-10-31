// Package mfaredis provides fail-closed, hash-scoped MFA challenge limiting.
// Redis is ephemeral abuse-control state and never factor or recovery-code truth.
package mfaredis

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"bidora/backend/internal/identity/attemptredis"
)

const reservationLifetime = time.Minute
const beginScript = `
if redis.call('EXISTS', KEYS[2]) == 1 then return 0 end
if not redis.call('SET', KEYS[3], '1', 'PX', ARGV[1], 'NX') then return -1 end
return 1
`
const finishScript = `
if redis.call('EXISTS', KEYS[3]) == 0 then return 0 end
redis.call('DEL', KEYS[3])
if ARGV[1] == '1' then redis.call('DEL', KEYS[1], KEYS[2]); return 1 end
local failures = redis.call('INCR', KEYS[1])
if failures == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[2]) end
if failures >= tonumber(ARGV[3]) then redis.call('SET', KEYS[2], '1', 'PX', ARGV[2]); return 2 end
return 1
`

var ErrDenied = errors.New("mfa_attempt_denied")

type Controller struct {
	runner attemptredis.ScriptRunner
	prefix string
}

type Reservation interface {
	Finish(context.Context, bool) error
}

func New(runner attemptredis.ScriptRunner, prefix string) (*Controller, error) {
	if runner == nil || !validPrefix(prefix) {
		return nil, errors.New("mfa_attempt_config_invalid")
	}
	return &Controller{runner: runner, prefix: prefix}, nil
}

func (c *Controller) Begin(ctx context.Context, scope [32]byte) (Reservation, error) {
	if ctx == nil || c == nil || c.runner == nil || scope == ([32]byte{}) {
		return nil, errors.New("mfa_attempt_unavailable")
	}
	base := c.prefix + ":{" + c.prefix + ":mfa}:" + hex.EncodeToString(scope[:])
	keys := []string{base + ":fail", base + ":block", base + ":reservation"}
	value, err := c.runner.Eval(ctx, beginScript, keys, reservationLifetime.Milliseconds())
	if err != nil {
		return nil, &availabilityError{err}
	}
	n, ok := integer(value)
	if !ok || n < 0 {
		return nil, errors.New("mfa_attempt_result_invalid")
	}
	if n == 0 {
		return nil, ErrDenied
	}
	if n != 1 {
		return nil, errors.New("mfa_attempt_result_invalid")
	}
	return &reservation{c: c, failures: keys[0], block: keys[1], key: keys[2]}, nil
}

type reservation struct {
	c                    *Controller
	failures, block, key string
}

func (r *reservation) Finish(ctx context.Context, success bool) error {
	if ctx == nil || r == nil || r.c == nil {
		return errors.New("mfa_attempt_unavailable")
	}
	v := "0"
	if success {
		v = "1"
	}
	value, err := r.c.runner.Eval(ctx, finishScript, []string{r.failures, r.block, r.key}, v, (10 * time.Minute).Milliseconds(), 5)
	if err != nil {
		return &availabilityError{err}
	}
	n, ok := integer(value)
	if !ok {
		return errors.New("mfa_attempt_result_invalid")
	}
	if n == 2 {
		return ErrDenied
	}
	if n != 1 {
		return errors.New("mfa_attempt_result_invalid")
	}
	return nil
}
func validPrefix(v string) bool {
	if len(v) < 1 || len(v) > 64 || strings.TrimSpace(v) != v {
		return false
	}
	for i := range len(v) {
		c := v[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune(":_-.", rune(c))) {
			return false
		}
	}
	return true
}
func integer(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

type availabilityError struct{ cause error }

func (e *availabilityError) Error() string                 { return "mfa_attempt_unavailable" }
func (e *availabilityError) Unwrap() error                 { return e.cause }
func (e *availabilityError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
