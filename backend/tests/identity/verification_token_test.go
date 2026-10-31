package identity_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"bidora/backend/internal/identity/verification"
)

type tokenReader byte

func (r tokenReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

type brokenTokenReader struct{}

func (brokenTokenReader) Read([]byte) (int, error) {
	return 0, errors.New("secret-canary entropy failure")
}

func TestVerificationTokenGenerationHashAndRedaction(t *testing.T) {
	token, err := verification.GenerateWithRandom(tokenReader(9))
	if err != nil {
		t.Fatal(err)
	}
	raw := token.RevealForDelivery()
	if len(raw) != 43 || strings.Contains(raw, "=") || token.IsZero() {
		t.Fatal("invalid token encoding")
	}
	decoded, _ := base64.RawURLEncoding.DecodeString(raw)
	want := sha256.Sum256(decoded)
	if token.Hash() != want {
		t.Fatal("token hash mismatch")
	}
	presented, err := verification.HashPresented(raw)
	if err != nil || presented != want {
		t.Fatal("presented token did not reproduce hash", err)
	}
	if strings.Contains(fmt.Sprintf("%v", token), raw) || strings.Contains(fmt.Sprintf("%#v", token), raw) {
		t.Fatal("token formatting leaked secret")
	}
	copyHash := token.Hash()
	copyHash[0]++
	if token.Hash() != want {
		t.Fatal("caller mutated token hash")
	}
}

func TestVerificationTokenRejectsMalformedAndEntropyFailure(t *testing.T) {
	valid, _ := verification.GenerateWithRandom(tokenReader(1))
	cases := []string{"", valid.RevealForDelivery() + "A", strings.Repeat("*", 43), valid.RevealForDelivery()[:42] + "="}
	for i, value := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			hash, err := verification.HashPresented(value)
			if err == nil || err.Error() != "verification_token_invalid" || hash != [32]byte{} || value != "" && strings.Contains(fmt.Sprintf("%#v", err), value) {
				t.Fatal("malformed token accepted/leaked", err)
			}
		})
	}
	if token, err := verification.GenerateWithRandom(nil); err == nil || err.Error() != "verification_random_invalid" || !token.IsZero() {
		t.Fatal("nil random accepted")
	}
	if token, err := verification.GenerateWithRandom(brokenTokenReader{}); err == nil || err.Error() != "verification_random_failed" || !token.IsZero() || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("entropy failure leaked", err)
	}
}

func TestVerificationExpiryPolicyAndConcurrentGeneration(t *testing.T) {
	created := time.Date(2026, 10, 18, 10, 0, 0, 0, time.FixedZone("source", 3600))
	expires, err := verification.ExpiresAt(created)
	if err != nil || !expires.Equal(time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)) || verification.Lifetime != 30*time.Minute {
		t.Fatal("expiry policy drifted", expires, err)
	}
	if _, err := verification.ExpiresAt(time.Time{}); err == nil || err.Error() != "verification_time_invalid" {
		t.Fatal("zero issue time accepted")
	}
	var wg sync.WaitGroup
	seen := sync.Map{}
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := verification.Generate()
			if err != nil {
				errs <- err
				return
			}
			if _, loaded := seen.LoadOrStore(token.RevealForDelivery(), true); loaded {
				errs <- errors.New("duplicate token")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
