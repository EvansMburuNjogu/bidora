package integration_test

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"github.com/google/uuid"
	redis "github.com/redis/go-redis/v9"
)

type liveRedis struct {
	config attemptredis.Config
	direct *redis.Client
}

func redisIntegration(t *testing.T) *liveRedis {
	t.Helper()
	address, user, password, caPath := os.Getenv("BIDORA_TEST_REDIS_ADDR"), os.Getenv("BIDORA_TEST_REDIS_USERNAME"), os.Getenv("BIDORA_TEST_REDIS_PASSWORD"), os.Getenv("BIDORA_TEST_REDIS_CA_FILE")
	if address == "" || password == "" || caPath == "" {
		t.Skip("live Redis integration environment is not configured")
	}
	pem, err := os.ReadFile(caPath)
	if err != nil {
		t.Fatal("integration Redis CA unavailable")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		t.Fatal("integration Redis CA invalid")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: roots}
	config := attemptredis.Config{Address: address, Username: user, Password: password, TLS: tlsConfig, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, PoolSize: 16}
	direct := redis.NewClient(&redis.Options{Addr: address, Username: user, Password: password, TLSConfig: tlsConfig.Clone(), MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = direct.Ping(ctx).Err(); err != nil {
		t.Fatal("live Redis unavailable")
	}
	t.Cleanup(func() { _ = direct.Close() })
	return &liveRedis{config, direct}
}
func liveScopes(seed string) attempt.Scopes {
	identityHash := sha256.Sum256([]byte("identity:" + seed))
	networkHash := sha256.Sum256([]byte("network:" + seed))
	return attempt.Scopes{Identity: attempt.Key(identityHash), Network: attempt.Key(networkHash)}
}
func liveController(t *testing.T, fixture *liveRedis, prefix string) (*attemptredis.Controller, *attemptredis.ClientRunner) {
	t.Helper()
	runner, err := attemptredis.NewClient(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Error(err)
		}
	})
	controller, err := attemptredis.NewSecureController(runner, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return controller, runner
}
func cleanupPrefix(t *testing.T, fixture *liveRedis, prefix string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var cursor uint64
	for {
		keys, next, err := fixture.direct.Scan(ctx, cursor, prefix+":*", 100).Result()
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) > 0 {
			if err = fixture.direct.Del(ctx, keys...).Err(); err != nil {
				t.Fatal(err)
			}
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}

func TestLiveRedisAttemptIdentityLifecycleAndExpiry(t *testing.T) {
	fixture := redisIntegration(t)
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, fixture, prefix)
	controller, _ := liveController(t, fixture, prefix)
	scopes := liveScopes(uuid.NewString())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 4; i++ {
		reservation, err := controller.Begin(ctx, scopes)
		if err != nil {
			t.Fatal(err)
		}
		if err = reservation.Finish(ctx, false); err != nil {
			t.Fatal(err)
		}
	}
	identityHex := hex.EncodeToString(scopes.Identity[:])
	failureKey := prefix + ":{login-attempt}:i:fail:" + identityHex
	ttl, err := fixture.direct.PTTL(ctx, failureKey).Result()
	if err != nil || ttl <= 0 || ttl > 15*time.Minute {
		t.Fatal("identity failure TTL drifted", ttl, err)
	}
	reservation, err := controller.Begin(ctx, scopes)
	if err != nil {
		t.Fatal(err)
	}
	if err = reservation.Finish(ctx, true); err != nil {
		t.Fatal(err)
	}
	if exists := fixture.direct.Exists(ctx, failureKey, prefix+":{login-attempt}:i:block:"+identityHex).Val(); exists != 0 {
		t.Fatal("successful login did not clear identity controls")
	}
	for i := 0; i < 5; i++ {
		reservation, err = controller.Begin(ctx, scopes)
		if err != nil {
			t.Fatal(err)
		}
		if err = reservation.Finish(ctx, false); err != nil {
			t.Fatal(err)
		}
	}
	if reservation, err = controller.Begin(ctx, scopes); !errors.Is(err, identityapp.ErrAttemptDenied) || reservation != nil {
		t.Fatal("identity denial window not enforced", err)
	}
}
func TestLiveRedisAttemptConcurrentNetworkLimitAndSingleUse(t *testing.T) {
	fixture := redisIntegration(t)
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, fixture, prefix)
	controller, _ := liveController(t, fixture, prefix)
	sharedNetwork := liveScopes("shared-network").Network
	type outcome struct {
		reservation identityapp.AttemptReservation
		err         error
	}
	outcomes := make(chan outcome, 40)
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(40)
	for i := 0; i < 40; i++ {
		scope := liveScopes(uuid.NewString())
		scope.Network = sharedNetwork
		go func(scope attempt.Scopes) {
			ready.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			reservation, err := controller.Begin(ctx, scope)
			outcomes <- outcome{reservation, err}
		}(scope)
	}
	ready.Wait()
	close(start)
	accepted, denied := 0, 0
	var acceptedReservations []identityapp.AttemptReservation
	for i := 0; i < 40; i++ {
		result := <-outcomes
		if result.err == nil {
			accepted++
			acceptedReservations = append(acceptedReservations, result.reservation)
		} else if errors.Is(result.err, identityapp.ErrAttemptDenied) {
			denied++
		} else {
			t.Fatal("unexpected concurrent Redis result", result.err)
		}
	}
	if accepted != 30 || denied != 10 {
		t.Fatal("network concurrency limit drifted", accepted, denied)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, reservation := range acceptedReservations {
		if err := reservation.Finish(ctx, true); err != nil {
			t.Fatal(err)
		}
	}
	networkHex := hex.EncodeToString(sharedNetwork[:])
	if exists := fixture.direct.Exists(ctx, prefix+":{login-attempt}:n:minute:"+networkHex, prefix+":{login-attempt}:n:window:"+networkHex, prefix+":{login-attempt}:n:block:"+networkHex).Val(); exists != 3 {
		t.Fatal("success incorrectly cleared network controls", exists)
	}
	singlePrefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, fixture, singlePrefix)
	single, _ := liveController(t, fixture, singlePrefix)
	reservation, err := single.Begin(ctx, liveScopes(uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	if err = reservation.Finish(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err = reservation.Finish(ctx, false); err == nil || err.Error() != "redis_attempt_reservation_invalid" {
		t.Fatal("reservation replay accepted", err)
	}
}
func TestLiveRedisAttemptACLAndOutageFailClosed(t *testing.T) {
	fixture := redisIntegration(t)
	outside, _ := liveController(t, fixture, "outside")
	if reservation, err := outside.Begin(context.Background(), liveScopes(uuid.NewString())); err == nil || reservation != nil {
		t.Fatal("Redis ACL allowed out-of-namespace attempt state")
	}
	bad := fixture.config
	bad.Address = "localhost:56378"
	bad.DialTimeout = 100 * time.Millisecond
	bad.ReadTimeout = 100 * time.Millisecond
	bad.WriteTimeout = 100 * time.Millisecond
	runner, err := attemptredis.NewClient(bad)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	controller, err := attemptredis.NewSecureController(runner, "bidora:test:"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if reservation, err := controller.Begin(ctx, liveScopes(uuid.NewString())); err == nil || reservation != nil {
		t.Fatal("Redis outage did not fail closed")
	}
}
