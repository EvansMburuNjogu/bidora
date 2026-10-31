package identity_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"bidora/backend/internal/identity/session"
)

type sessionReader struct{ next byte }

func (r *sessionReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.next
	}
	r.next++
	return len(p), nil
}

type secondReadFailure struct{ calls int }

func (r *secondReadFailure) Read(p []byte) (int, error) {
	r.calls++
	if r.calls > 1 {
		return 0, errors.New("secret-canary csrf entropy failure")
	}
	for i := range p {
		p[i] = 1
	}
	return len(p), nil
}

func TestApprovedSessionPolicyIsExact(t *testing.T) {
	policy := session.PilotCookiePolicy()
	if policy.Name != "__Host-bidora_session" || policy.Path != "/" || policy.SameSite != "Lax" || !policy.Secure || !policy.HttpOnly || !policy.HostOnly || policy.Idle != 30*time.Minute || policy.Absolute != 12*time.Hour || session.CSRFHeader != "X-CSRF-Token" {
		t.Fatal("approved session policy drifted", policy)
	}
	csrfPolicy := session.PilotCSRFCookiePolicy()
	if csrfPolicy.Name != "__Host-bidora_csrf" || csrfPolicy.Path != "/" || csrfPolicy.SameSite != "Lax" || !csrfPolicy.Secure || csrfPolicy.HttpOnly || !csrfPolicy.HostOnly || csrfPolicy.Absolute != 12*time.Hour {
		t.Fatal("approved csrf cookie policy drifted", csrfPolicy)
	}
	created := time.Date(2026, 10, 18, 10, 0, 0, 0, time.FixedZone("source", 3600))
	idle, absolute, err := session.Expiries(created)
	if err != nil || !idle.Equal(time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)) || !absolute.Equal(time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)) {
		t.Fatal("session expiries drifted", idle, absolute, err)
	}
	if _, _, err := session.Expiries(time.Time{}); err == nil || err.Error() != "session_time_invalid" {
		t.Fatal("zero session time accepted")
	}
}

func TestSessionAndCSRFTokensAreOpaqueDomainSeparatedAndRedacted(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&sessionReader{next: 6})
	if err != nil {
		t.Fatal(err)
	}
	sessionRaw, csrfRaw := pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()
	if len(sessionRaw) != 43 || len(csrfRaw) != 43 || pair.Session.IsZero() || pair.CSRF.IsZero() {
		t.Fatal("invalid session token pair")
	}
	if sessionRaw == csrfRaw {
		t.Fatal("session and CSRF raw values reused")
	}
	if pair.Session.Hash() == pair.CSRF.Hash() {
		t.Fatal("session and CSRF hashes lack domain separation")
	}
	sessionHash, err := session.HashPresentedSession(sessionRaw)
	if err != nil || sessionHash != pair.Session.Hash() {
		t.Fatal("session presentation hash mismatch")
	}
	csrfHash, err := session.HashPresentedCSRF(csrfRaw)
	if err != nil || csrfHash != pair.CSRF.Hash() {
		t.Fatal("CSRF presentation hash mismatch")
	}
	if match, err := session.MatchesCSRF(csrfRaw, pair.CSRF.Hash()); err != nil || !match {
		t.Fatal("valid CSRF rejected", err)
	}
	if match, err := session.MatchesCSRF(sessionRaw, pair.CSRF.Hash()); err != nil || match {
		t.Fatal("domain-swapped CSRF accepted", err)
	}
	for _, value := range []any{pair.Session, pair.CSRF} {
		formatted := fmt.Sprintf("%#v", value)
		if strings.Contains(formatted, sessionRaw) || strings.Contains(formatted, csrfRaw) {
			t.Fatal("token formatting leaked secret")
		}
	}
}

func TestSessionTokensRejectMalformedAndEntropyFailure(t *testing.T) {
	pair, _ := session.GeneratePairWithRandom(&sessionReader{next: 2})
	valid := pair.Session.RevealForCookie()
	for _, value := range []string{"", valid + "A", strings.Repeat("*", 43), valid[:42] + "="} {
		hash, err := session.HashPresentedSession(value)
		if err == nil || err.Error() != "session_token_invalid" || hash != [32]byte{} {
			t.Fatal("malformed session accepted", err)
		}
	}
	if pair, err := session.GeneratePairWithRandom(nil); err == nil || err.Error() != "session_random_invalid" || !pair.Session.IsZero() || !pair.CSRF.IsZero() {
		t.Fatal("nil entropy accepted")
	}
	reader := &secondReadFailure{}
	if pair, err := session.GeneratePairWithRandom(reader); err == nil || err.Error() != "session_random_failed" || !pair.Session.IsZero() || !pair.CSRF.IsZero() || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("partial entropy failure unsafe", err)
	}
}

func TestSessionTokenGenerationConcurrentUniqueness(t *testing.T) {
	var wg sync.WaitGroup
	seen := sync.Map{}
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pair, err := session.GeneratePair()
			if err != nil {
				errs <- err
				return
			}
			for _, raw := range []string{pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()} {
				if _, loaded := seen.LoadOrStore(raw, true); loaded {
					errs <- errors.New("duplicate token")
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
