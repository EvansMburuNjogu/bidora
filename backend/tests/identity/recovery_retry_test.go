package identity_test

import (
	"context"
	"errors"
	"testing"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/recoveryredis"
)

func TestRecoveryLimiterReportsActualWaitAndPreservesItThroughService(t *testing.T) {
	for _, tc := range []struct {
		value   int64
		seconds int
	}{{-2400000, 2400}, {-60001, 61}, {0, 900}} {
		runner := &scriptRunnerFake{results: []scriptResult{{value: tc.value}}}
		limiter, _ := recoveryredis.New(runner, "test")
		err := limiter.Allow(context.Background(), attemptScopes())
		var retry interface{ RetryAfterSeconds() int }
		if !errors.Is(err, app.ErrRecoveryAttemptDenied) || !errors.As(err, &retry) || retry.RetryAfterSeconds() != tc.seconds {
			t.Fatal("incorrect effective recovery wait")
		}
		attempts := &recoveryAttemptsFake{err: err}
		service, _ := app.NewPublicPasswordRecoveryService(&recoveryRequesterFake{}, attempts, recoveryKeyFake{key: attemptScopes().Identity})
		wrapped := service.Request(context.Background(), app.PublicPasswordRecoveryInput{Email: "unknown@example.test", CorrelationID: identityID(1), NetworkScope: attemptScopes().Network})
		if wrapped == nil || wrapped.Error() != "password_recovery_attempt_denied" || !errors.As(wrapped, &retry) || retry.RetryAfterSeconds() != tc.seconds {
			t.Fatal("retry metadata discarded")
		}
	}
}
