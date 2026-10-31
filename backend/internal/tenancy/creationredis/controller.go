// Package creationredis implements the approved fail-closed organization-creation limits.
// Redis stores only HMAC-derived identity and network scopes.
package creationredis

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"bidora/backend/internal/identity/attemptredis"
)

const script = `
if redis.call('EXISTS', KEYS[2]) == 1 or redis.call('EXISTS', KEYS[4]) == 1 then return 0 end
local identity = redis.call('INCR', KEYS[1])
if identity == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
local network = redis.call('INCR', KEYS[3])
if network == 1 then redis.call('PEXPIRE', KEYS[3], ARGV[1]) end
if identity > tonumber(ARGV[2]) then redis.call('SET', KEYS[2], '1', 'PX', ARGV[4]); return 0 end
if network > tonumber(ARGV[3]) then redis.call('SET', KEYS[4], '1', 'PX', ARGV[4]); return 0 end
return 1
`

const (
	window        = 24 * time.Hour
	deny          = 15 * time.Minute
	identityLimit = 3
	networkLimit  = 10
)

type Controller struct {
	runner attemptredis.ScriptRunner
	prefix string
}

func New(runner attemptredis.ScriptRunner, prefix string) (*Controller, error) {
	if runner == nil || !validPrefix(prefix) {
		return nil, errors.New("organization_creation_limiter_config_invalid")
	}
	return &Controller{runner: runner, prefix: prefix}, nil
}

func (c *Controller) AllowOrganizationCreation(ctx context.Context, identityScope, networkScope [32]byte, now time.Time) (bool, error) {
	if ctx == nil || identityScope == [32]byte{} || networkScope == [32]byte{} || now.IsZero() {
		return false, errors.New("organization_creation_limiter_scope_invalid")
	}
	if c == nil || c.runner == nil || !validPrefix(c.prefix) {
		return false, errors.New("organization_creation_limiter_unavailable")
	}
	keys := []string{c.key("identity", identityScope), c.key("identity-block", identityScope), c.key("network", networkScope), c.key("network-block", networkScope)}
	value, err := c.runner.Eval(ctx, script, keys, window.Milliseconds(), identityLimit, networkLimit, deny.Milliseconds())
	if err != nil {
		return false, &controllerError{cause: err}
	}
	number, ok := value.(int64)
	if !ok {
		if integer, integerOK := value.(int); integerOK {
			number, ok = int64(integer), true
		}
	}
	if !ok || (number != 0 && number != 1) {
		return false, errors.New("organization_creation_limiter_result_invalid")
	}
	return number == 1, nil
}

func (c *Controller) key(kind string, scope [32]byte) string {
	return c.prefix + ":{" + c.prefix + "}:organization-creation:" + kind + ":" + hex.EncodeToString(scope[:])
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

type controllerError struct{ cause error }

func (e *controllerError) Error() string                 { return "organization_creation_limiter_unavailable" }
func (e *controllerError) Unwrap() error                 { return e.cause }
func (e *controllerError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
