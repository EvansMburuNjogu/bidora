package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/registrationredis"
)

func registrationScopes() attempt.Scopes {
	var scopes attempt.Scopes
	scopes.Identity[31] = 1
	scopes.Network[31] = 2
	return scopes
}
func TestRegistrationRedisControllerUsesAtomicPrivacySafeLimits(t *testing.T) {
	runner := &scriptRunnerFake{results: []scriptResult{{value: int64(1)}}}
	controller, err := registrationredis.New(runner, "bidora:test")
	if err != nil {
		t.Fatal(err)
	}
	if err = controller.Allow(context.Background(), registrationScopes()); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || len(runner.calls[0].keys) != 4 || len(runner.calls[0].args) != 5 {
		t.Fatal("registration limiter command drifted")
	}
	for _, key := range runner.calls[0].keys {
		if !strings.Contains(key, "{bidora:test}") || strings.Contains(key, "email") || strings.Contains(key, "127.0.0.1") {
			t.Fatal("unsafe registration limiter key", key)
		}
	}
}
func TestRegistrationRedisControllerDeniesAndFailsClosed(t *testing.T) {
	cause := errors.New("secret-canary Redis")
	for _, tc := range []struct {
		value any
		cause error
		want  string
	}{{int64(0), nil, "registration_attempt_denied"}, {int64(2), nil, "registration_attempt_result_invalid"}, {"1", nil, "registration_attempt_result_invalid"}, {nil, cause, "registration_attempt_unavailable"}} {
		runner := &scriptRunnerFake{results: []scriptResult{{value: tc.value, err: tc.cause}}}
		controller, _ := registrationredis.New(runner, "bidora")
		err := controller.Allow(context.Background(), registrationScopes())
		if err == nil || err.Error() != tc.want || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
			t.Fatal("registration limiter failure drifted", err)
		}
		if tc.want == "registration_attempt_denied" && !errors.Is(err, identityapp.ErrRegistrationAttemptDenied) {
			t.Fatal("denial sentinel lost")
		}
	}
	if controller, err := registrationredis.New(nil, "bidora"); err == nil || controller != nil {
		t.Fatal("nil runner accepted")
	}
	for _, prefix := range []string{"", "has space", "has{brace}", strings.Repeat("x", 65)} {
		if controller, err := registrationredis.New(&scriptRunnerFake{}, prefix); err == nil || controller != nil {
			t.Fatal("invalid prefix accepted")
		}
	}
	var zero *registrationredis.Controller
	if err := zero.Allow(context.Background(), registrationScopes()); err == nil {
		t.Fatal("zero limiter accepted")
	}
}
