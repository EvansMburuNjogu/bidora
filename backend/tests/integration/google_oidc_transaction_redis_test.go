package integration_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/oidcredis"
	"github.com/google/uuid"
)

func TestLiveGoogleOIDCRedisTransactionIsSingleUse(t *testing.T) {
	fixture := redisIntegration(t)
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, fixture, prefix)
	runner, err := attemptredis.NewClient(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	store, err := oidcredis.New(runner, prefix, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := googleoidc.GenerateTransaction()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Create(context.Background(), transaction.StateForAuthorization(), transaction.MaterialForStore()); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int64
	var group sync.WaitGroup
	for index := 0; index < 24; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			material, consumeErr := store.Consume(context.Background(), transaction.StateForAuthorization())
			if consumeErr == nil {
				if material != transaction.MaterialForStore() {
					t.Error("consumed Google transaction changed")
				}
				successes.Add(1)
				return
			}
			if consumeErr.Error() != "google_oidc_transaction_invalid" {
				t.Error("unexpected consume failure", consumeErr)
			}
		}()
	}
	group.Wait()
	if successes.Load() != 1 {
		t.Fatal("Google transaction was not consumed exactly once", successes.Load())
	}
	if err = store.Create(context.Background(), transaction.StateForAuthorization(), transaction.MaterialForStore()); err != nil {
		t.Fatal("consumed state key was not removed", err)
	}
}
