package integration_test

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/recoveryredis"
	"github.com/google/uuid"
)

func TestLiveRecoveryRetryTracksQuotaWithoutRenewingBlocks(t *testing.T) {
	fixture := redisIntegration(t)
	prefix := os.Getenv("BIDORA_TEST_RECOVERY_REDIS_PREFIX")
	if prefix == "" {
		prefix = "bidora:test:recovery"
	}
	prefix += ":" + uuid.NewString()[:8]
	runner, err := attemptredis.NewClient(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	limiter, err := recoveryredis.New(runner, prefix)
	if err != nil {
		t.Fatal(err)
	}
	scopes := liveScopes(uuid.NewString())
	base := prefix + ":{" + prefix + "}:recovery:"
	i, n := hex.EncodeToString(scopes.Identity[:]), hex.EncodeToString(scopes.Network[:])
	keys := []string{base + "identity:" + i, base + "identity-block:" + i, base + "network:" + n, base + "network-block:" + n}
	ctx := context.Background()
	defer fixture.direct.Del(ctx, keys...)
	// Existing pre-fix block is short, but its hourly quota remains exhausted.
	if err = fixture.direct.Set(ctx, keys[0], 4, 40*time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err = fixture.direct.Set(ctx, keys[1], 1, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	before := fixture.direct.PTTL(ctx, keys[1]).Val()
	for range 2 {
		err = limiter.Allow(ctx, scopes)
		var retry interface{ RetryAfterSeconds() int }
		if !errors.Is(err, app.ErrRecoveryAttemptDenied) || !errors.As(err, &retry) || retry.RetryAfterSeconds() < 2398 || retry.RetryAfterSeconds() > 2400 {
			t.Fatal("quota countdown incorrect")
		}
	}
	if fixture.direct.PTTL(ctx, keys[1]).Val() > before {
		t.Fatal("blocked retry extended timer")
	}
	if fixture.direct.Get(ctx, keys[0]).Val() != "4" {
		t.Fatal("blocked retry increased quota")
	}
	// Both windows expire: the next request must be accepted, not blocked again.
	if err = fixture.direct.PExpire(ctx, keys[0], 30*time.Millisecond).Err(); err != nil {
		t.Fatal(err)
	}
	if err = fixture.direct.PExpire(ctx, keys[1], 30*time.Millisecond).Err(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	if err = limiter.Allow(ctx, scopes); err != nil {
		t.Fatal("request still blocked after effective expiry", err)
	}
	// At an exhausted second scope, countdown must include that scope too.
	if err = fixture.direct.Set(ctx, keys[2], 10, 30*time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err = fixture.direct.Set(ctx, keys[1], 1, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	err = limiter.Allow(ctx, scopes)
	var retry interface{ RetryAfterSeconds() int }
	if !errors.As(err, &retry) || retry.RetryAfterSeconds() < 1798 {
		t.Fatal("network quota omitted from countdown")
	}
}
