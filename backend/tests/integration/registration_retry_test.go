package integration_test

import (
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/registrationredis"
	"context"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"os"
	"testing"
	"time"
)

func TestLiveRegistrationResendWaitsForActualQuotaWithoutRenewing(t *testing.T) {
	fixture := redisIntegration(t)
	base := os.Getenv("BIDORA_TEST_RECOVERY_REDIS_PREFIX")
	if base == "" {
		base = "bidora:test"
	}
	prefix := base + ":reg:" + uuid.NewString()[:8]
	runner, err := attemptredis.NewClient(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	limiter, _ := registrationredis.New(runner, prefix)
	scopes := liveScopes(uuid.NewString())
	p := prefix + ":{" + prefix + "}:registration:"
	i, n := hex.EncodeToString(scopes.Identity[:]), hex.EncodeToString(scopes.Network[:])
	keys := []string{p + "identity:" + i, p + "identity-block:" + i, p + "network:" + n, p + "network-block:" + n}
	ctx := context.Background()
	defer fixture.direct.Del(ctx, keys...)
	if err = fixture.direct.Set(ctx, keys[0], 4, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err = fixture.direct.Set(ctx, keys[1], 1, 30*time.Second).Err(); err != nil {
		t.Fatal(err)
	}
	before := fixture.direct.PTTL(ctx, keys[1]).Val()
	err = limiter.Allow(ctx, scopes)
	var retry interface{ RetryAfterSeconds() int }
	if !errors.Is(err, app.ErrRegistrationAttemptDenied) || !errors.As(err, &retry) || retry.RetryAfterSeconds() < 59 || retry.RetryAfterSeconds() > 60 {
		t.Fatal("incorrect signup/resend wait")
	}
	if fixture.direct.PTTL(ctx, keys[1]).Val() > before || fixture.direct.Get(ctx, keys[0]).Val() != "4" {
		t.Fatal("blocked request renewed quota")
	}
	fixture.direct.PExpire(ctx, keys[0], 20*time.Millisecond)
	fixture.direct.PExpire(ctx, keys[1], 20*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	for range 3 {
		if err = limiter.Allow(ctx, scopes); err != nil {
			t.Fatal("request denied after actual expiry", err)
		}
	}
	if err = limiter.Allow(ctx, scopes); !errors.Is(err, app.ErrRegistrationAttemptDenied) {
		t.Fatal("three-per-hour quota bypassed")
	}
}
