package integration_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/registrationredis"
	"github.com/google/uuid"
)

func registrationLiveScopes(identitySeed, networkSeed string) attempt.Scopes {
	identityHash := sha256.Sum256([]byte("registration-identity:" + identitySeed))
	networkHash := sha256.Sum256([]byte("registration-network:" + networkSeed))
	return attempt.Scopes{Identity: attempt.Key(identityHash), Network: attempt.Key(networkHash)}
}
func TestLiveRegistrationRedisLimitsAreAtomicAndDoNotClear(t *testing.T) {
	fixture := redisIntegration(t)
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, fixture, prefix)
	runner, err := attemptredis.NewClient(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	controller, err := registrationredis.New(runner, prefix)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	identityScopes := registrationLiveScopes("same", "identity-network")
	for index := 0; index < 3; index++ {
		if err = controller.Allow(ctx, identityScopes); err != nil {
			t.Fatal("identity registration denied early", index, err)
		}
	}
	if err = controller.Allow(ctx, identityScopes); !errors.Is(err, identityapp.ErrRegistrationAttemptDenied) {
		t.Fatal("identity registration limit not enforced", err)
	}
	networkSeed := "shared-network"
	for index := 0; index < 10; index++ {
		if err = controller.Allow(ctx, registrationLiveScopes(uuid.NewString(), networkSeed)); err != nil {
			t.Fatal("network registration denied early", index, err)
		}
	}
	if err = controller.Allow(ctx, registrationLiveScopes(uuid.NewString(), networkSeed)); !errors.Is(err, identityapp.ErrRegistrationAttemptDenied) {
		t.Fatal("network registration limit not enforced", err)
	}
}
