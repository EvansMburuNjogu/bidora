package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/mfaredis"
	"github.com/google/uuid"
)

func TestLiveMFARedisFifthFailureAndSubsequentAttemptsAreDenied(t *testing.T) {
	fixture := redisIntegration(t)
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, fixture, prefix)
	runner, err := attemptredis.NewClient(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	controller, err := mfaredis.New(runner, prefix)
	if err != nil {
		t.Fatal(err)
	}
	var scope [32]byte
	scope[31] = 7
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for attempt := 1; attempt <= 5; attempt++ {
		reservation, beginErr := controller.Begin(ctx, scope)
		if beginErr != nil {
			t.Fatalf("attempt %d begin failed early: %v", attempt, beginErr)
		}
		finishErr := reservation.Finish(ctx, false)
		if attempt < 5 && finishErr != nil {
			t.Fatalf("attempt %d denied early: %v", attempt, finishErr)
		}
		if attempt == 5 && !errors.Is(finishErr, mfaredis.ErrDenied) {
			t.Fatalf("fifth failure did not establish denial: %v", finishErr)
		}
	}
	if reservation, beginErr := controller.Begin(ctx, scope); reservation != nil || !errors.Is(beginErr, mfaredis.ErrDenied) {
		t.Fatal("subsequent MFA attempt bypassed denial", beginErr)
	}
}
