package identity_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
)

type scriptCall struct {
	script string
	keys   []string
	args   []any
}
type scriptResult struct {
	value any
	err   error
}
type scriptRunnerFake struct {
	results []scriptResult
	calls   []scriptCall
}

func (r *scriptRunnerFake) Eval(_ context.Context, script string, keys []string, args ...any) (any, error) {
	r.calls = append(r.calls, scriptCall{script, append([]string(nil), keys...), append([]any(nil), args...)})
	if len(r.results) == 0 {
		return nil, errors.New("unexpected script")
	}
	result := r.results[0]
	r.results = r.results[1:]
	return result.value, result.err
}

type redisRandom byte

func (r redisRandom) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

type redisRandomFailure struct{}

func (redisRandomFailure) Read([]byte) (int, error) {
	return 0, errors.New("secret-canary random failure")
}
func attemptScopes() attempt.Scopes {
	var scopes attempt.Scopes
	scopes.Identity[31] = 1
	scopes.Network[31] = 2
	return scopes
}

func TestRedisAttemptControllerReservesAndFinishesAtomically(t *testing.T) {
	runner := &scriptRunnerFake{results: []scriptResult{{value: int64(1)}, {value: int64(1)}}}
	controller, err := attemptredis.NewController(runner, redisRandom(3), "bidora")
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := controller.Begin(context.Background(), attemptScopes())
	if err != nil || reservation == nil {
		t.Fatal("reservation failed", err)
	}
	if err := reservation.Finish(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 || !strings.Contains(runner.calls[0].script, "INCR") || !strings.Contains(runner.calls[0].script, "PEXPIRE") || !strings.Contains(runner.calls[0].script, "NX") || !strings.Contains(runner.calls[1].script, "DEL") || !strings.Contains(runner.calls[1].script, "failures >=") {
		t.Fatal("atomic scripts drifted")
	}
	if len(runner.calls[0].keys) != 6 || len(runner.calls[1].keys) != 3 {
		t.Fatal("script key scope drifted")
	}
	for _, key := range runner.calls[0].keys {
		if !strings.Contains(key, "bidora:{login-attempt}:") {
			t.Fatal("cluster hash slot missing", key)
		}
		if strings.Contains(key, "email") || strings.Contains(key, "192.") {
			t.Fatal("raw identity/network key leaked", key)
		}
	}
	wantArgs := []any{int64(60000), int64(900000), 30, 100, int64(900000), int64(300000)}
	if fmt.Sprint(runner.calls[0].args) != fmt.Sprint(wantArgs) {
		t.Fatal("approved begin limits drifted", runner.calls[0].args)
	}
	wantFinish := []any{"0", int64(900000), 5, int64(900000)}
	if fmt.Sprint(runner.calls[1].args) != fmt.Sprint(wantFinish) {
		t.Fatal("approved finish limits drifted", runner.calls[1].args)
	}
}

func TestRedisAttemptControllerSuccessClearsIdentityOnly(t *testing.T) {
	runner := &scriptRunnerFake{results: []scriptResult{{value: 1}, {value: 1}}}
	controller, _ := attemptredis.NewController(runner, redisRandom(4), "bidora")
	reservation, err := controller.Begin(context.Background(), attemptScopes())
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Finish(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if runner.calls[1].args[0] != "1" || strings.Contains(strings.Join(runner.calls[1].keys, ","), ":n:") {
		t.Fatal("success altered network counters")
	}
}

func TestRedisAttemptControllerDenialsAndFailuresFailClosed(t *testing.T) {
	cause := errors.New("secret-canary redis diagnostic")
	for _, tc := range []struct {
		name   string
		result scriptResult
		want   string
		denied bool
	}{{"denied", scriptResult{value: int64(0)}, "attempt_denied", true}, {"script collision", scriptResult{value: int64(-1)}, "redis_attempt_result_invalid", false}, {"unexpected positive", scriptResult{value: int64(2)}, "redis_attempt_result_invalid", false}, {"wrong type", scriptResult{value: "1"}, "redis_attempt_result_invalid", false}, {"dependency", scriptResult{err: cause}, "secret-canary redis diagnostic", false}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptRunnerFake{results: []scriptResult{tc.result}}
			controller, _ := attemptredis.NewController(runner, redisRandom(1), "bidora")
			reservation, err := controller.Begin(context.Background(), attemptScopes())
			if err == nil || reservation != nil || err.Error() != tc.want {
				t.Fatal("unsafe Redis result", reservation, err)
			}
			if errors.Is(err, identityapp.ErrAttemptDenied) != tc.denied {
				t.Fatal("denial mapping drifted")
			}
		})
	}
	runner := &scriptRunnerFake{}
	controller, _ := attemptredis.NewController(runner, redisRandomFailure{}, "bidora")
	if reservation, err := controller.Begin(context.Background(), attemptScopes()); err == nil || err.Error() != "redis_attempt_random_failed" || reservation != nil || len(runner.calls) != 0 || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("random failure unsafe", err)
	}
	controller, _ = attemptredis.NewController(runner, redisRandom(1), "bidora")
	if _, err := controller.Begin(nil, attemptScopes()); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := controller.Begin(context.Background(), attempt.Scopes{}); err == nil {
		t.Fatal("zero scopes accepted")
	}
}

func TestRedisAttemptClientConfigurationRequiresBoundedVerifiedTLS(t *testing.T) {
	valid := attemptredis.Config{Address: "redis.example.test:6380", Username: "bidora", Password: "secret-canary-password", TLS: &tls.Config{ServerName: "redis.example.test", MinVersion: tls.VersionTLS12}, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, PoolSize: 4}
	runner, err := attemptredis.NewClient(valid)
	if err != nil {
		t.Fatal(err)
	}
	if formatted := fmt.Sprintf("%#v", valid); strings.Contains(formatted, "secret-canary") || strings.Contains(formatted, "redis.example") {
		t.Fatal("Redis config formatting leaked")
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runner.Close(); err != nil {
		t.Fatal("Redis close was not idempotent", err)
	}
	cases := []func(*attemptredis.Config){func(c *attemptredis.Config) { c.Address = "ambient" }, func(c *attemptredis.Config) { c.Address = " bad:6380" }, func(c *attemptredis.Config) { c.Password = "" }, func(c *attemptredis.Config) { c.TLS = nil }, func(c *attemptredis.Config) {
		c.TLS = &tls.Config{ServerName: "redis.example.test", InsecureSkipVerify: true}
	}, func(c *attemptredis.Config) {
		c.TLS = &tls.Config{ServerName: "redis.example.test", MinVersion: tls.VersionTLS11}
	}, func(c *attemptredis.Config) {
		c.TLS = &tls.Config{ServerName: "redis.example.test", MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS12}
	}, func(c *attemptredis.Config) { c.DialTimeout = 0 }, func(c *attemptredis.Config) { c.ReadTimeout = 11 * time.Second }, func(c *attemptredis.Config) { c.PoolSize = 0 }, func(c *attemptredis.Config) { c.PoolSize = 101 }}
	for i, mutate := range cases {
		copy := valid
		tlsCopy := valid.TLS.Clone()
		copy.TLS = tlsCopy
		mutate(&copy)
		if runner, err := attemptredis.NewClient(copy); err == nil || err.Error() != "redis_attempt_config_invalid" || runner != nil {
			t.Fatal("unsafe Redis config accepted", i)
		}
	}
	var zero *attemptredis.ClientRunner
	if err := zero.Close(); err != nil {
		t.Fatal("zero Redis runner close failed")
	}
}

func TestRedisClientRunnerProbeFailsClosed(t *testing.T) {
	var runner *attemptredis.ClientRunner
	if err := runner.Ping(context.Background()); err == nil || err.Error() != "redis_attempt_unavailable" {
		t.Fatal("zero Redis runner probe accepted", err)
	}
	if err := runner.Ping(nil); err == nil || err.Error() != "redis_attempt_unavailable" {
		t.Fatal("nil Redis probe context accepted", err)
	}
}

func TestRedisAttemptReservationIsSingleUseAndConfigurationExplicit(t *testing.T) {
	runner := &scriptRunnerFake{results: []scriptResult{{value: int64(1)}, {value: int64(1)}, {value: int64(0)}}}
	controller, _ := attemptredis.NewController(runner, redisRandom(1), "bidora")
	reservation, _ := controller.Begin(context.Background(), attemptScopes())
	if err := reservation.Finish(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if err := reservation.Finish(context.Background(), false); err == nil || err.Error() != "redis_attempt_reservation_invalid" {
		t.Fatal("reservation replay accepted", err)
	}
	for _, prefix := range []string{"", "has space", "has{brace}", strings.Repeat("x", 65)} {
		if controller, err := attemptredis.NewController(runner, redisRandom(1), prefix); err == nil || err.Error() != "redis_attempt_config_invalid" || controller != nil {
			t.Fatal("invalid prefix accepted", prefix)
		}
	}
	if controller, err := attemptredis.NewController(nil, redisRandom(1), "bidora"); err == nil || controller != nil {
		t.Fatal("nil runner accepted")
	}
	if client, err := attemptredis.NewClientRunner(nil); err == nil || client != nil {
		t.Fatal("nil Redis client accepted")
	}
	if controller, err := attemptredis.NewSecureController(runner, "bidora"); err != nil || controller == nil {
		t.Fatal("secure controller construction failed", err)
	}
}
