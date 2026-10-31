package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/verification"
)

type verificationCommitterFake struct {
	result             identityapp.EmailVerificationResult
	err                error
	hash               [32]byte
	audit, correlation identityapp.ID
	at                 time.Time
	calls              int
}

func (c *verificationCommitterFake) ConsumeEmailVerification(_ context.Context, hash [32]byte, audit, correlation identityapp.ID, at time.Time) (identityapp.EmailVerificationResult, error) {
	c.calls++
	c.hash = hash
	c.audit = audit
	c.correlation = correlation
	c.at = at
	return c.result, c.err
}
func verificationInput(t *testing.T) identityapp.EmailVerificationInput {
	t.Helper()
	token, err := verification.GenerateWithRandom(tokenReader(7))
	if err != nil {
		t.Fatal(err)
	}
	return identityapp.EmailVerificationInput{Token: token.RevealForDelivery(), CorrelationID: identityID(8)}
}
func verificationService(t *testing.T, committer *verificationCommitterFake, ids *loginIDsFake, clock loginClockFake) *identityapp.EmailVerificationService {
	t.Helper()
	service, err := identityapp.NewEmailVerificationService(committer, ids, clock)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestEmailVerificationConsumesHashAndReturnsAuditedResult(t *testing.T) {
	verified := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	committer := &verificationCommitterFake{result: identityapp.EmailVerificationResult{UserID: identityID(3), AuthEpoch: 2, VerifiedAt: verified}}
	ids := &loginIDsFake{id: identityID(4)}
	clock := loginClockFake{at: verified}
	service := verificationService(t, committer, ids, clock)
	input := verificationInput(t)
	result, err := service.Verify(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	wantHash, _ := verification.HashPresented(input.Token)
	if result.UserID != identityID(3) || result.AuthEpoch != 2 || !result.VerifiedAt.Equal(verified) || committer.hash != wantHash || committer.audit != identityID(4) || committer.correlation != identityID(8) || committer.calls != 1 {
		t.Fatal("verification orchestration drifted", result)
	}
	if strings.Contains(fmt.Sprintf("%#v", result), input.Token) {
		t.Fatal("verification result leaked token")
	}
}
func TestEmailVerificationConcealsMalformedExpiredAndReplayedTokens(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*identityapp.EmailVerificationInput, *verificationCommitterFake)
	}{{"malformed", func(i *identityapp.EmailVerificationInput, _ *verificationCommitterFake) { i.Token = "malformed" }}, {"missing correlation", func(i *identityapp.EmailVerificationInput, _ *verificationCommitterFake) {
		i.CorrelationID = identityapp.ID{}
	}}, {"expired or replayed", func(_ *identityapp.EmailVerificationInput, c *verificationCommitterFake) {
		c.err = identityapp.ErrVerificationNotFound
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			committer := &verificationCommitterFake{}
			ids := &loginIDsFake{id: identityID(4)}
			service := verificationService(t, committer, ids, loginClockFake{at: time.Now()})
			input := verificationInput(t)
			tc.mutate(&input, committer)
			result, err := service.Verify(context.Background(), input)
			if err == nil || err.Error() != "verification_invalid" || result != (identityapp.EmailVerificationResult{}) || errors.Is(err, identityapp.ErrVerificationNotFound) {
				t.Fatal("verification state leaked", result, err)
			}
			if tc.name != "expired or replayed" && committer.calls != 0 {
				t.Fatal("invalid request crossed transaction")
			}
		})
	}
}
func TestEmailVerificationDependenciesFailClosed(t *testing.T) {
	cause := errors.New("secret-canary verification database")
	cases := []struct {
		name  string
		setup func(*verificationCommitterFake, *loginIDsFake, *loginClockFake)
	}{{"id", func(_ *verificationCommitterFake, i *loginIDsFake, _ *loginClockFake) { i.err = cause }}, {"zero id", func(_ *verificationCommitterFake, i *loginIDsFake, _ *loginClockFake) { i.id = identityapp.ID{} }}, {"clock", func(_ *verificationCommitterFake, _ *loginIDsFake, c *loginClockFake) { c.at = time.Time{} }}, {"commit", func(c *verificationCommitterFake, _ *loginIDsFake, _ *loginClockFake) { c.err = cause }}, {"invalid result", func(c *verificationCommitterFake, _ *loginIDsFake, _ *loginClockFake) {
		c.result = identityapp.EmailVerificationResult{}
	}}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			committer := &verificationCommitterFake{result: identityapp.EmailVerificationResult{UserID: identityID(1), AuthEpoch: 1, VerifiedAt: time.Now()}}
			ids := &loginIDsFake{id: identityID(2)}
			clock := loginClockFake{at: time.Now()}
			tc.setup(committer, ids, &clock)
			service := verificationService(t, committer, ids, clock)
			result, err := service.Verify(context.Background(), verificationInput(t))
			if err == nil || err.Error() != "verification_unavailable" || result != (identityapp.EmailVerificationResult{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("verification dependency failure unsafe", result, err)
			}
		})
	}
	if service, err := identityapp.NewEmailVerificationService(nil, &loginIDsFake{}, loginClockFake{}); err == nil || service != nil {
		t.Fatal("nil committer accepted")
	}
	service := verificationService(t, &verificationCommitterFake{}, &loginIDsFake{id: identityID(1)}, loginClockFake{at: time.Now()})
	if _, err := service.Verify(nil, verificationInput(t)); err == nil || err.Error() != "verification_invalid" {
		t.Fatal("nil context accepted")
	}
	var zero *identityapp.EmailVerificationService
	if _, err := zero.Verify(context.Background(), verificationInput(t)); err == nil {
		t.Fatal("zero service accepted")
	}
}
