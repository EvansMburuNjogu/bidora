package identity_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"bidora/backend/internal/identity/password"
)

type repeatReader byte

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("secret-canary entropy diagnostic")
}

func testPolicy() password.Policy {
	return password.Policy{MemoryKiB: 32, Iterations: 2, Parallelism: 1, SaltBytes: 16, KeyBytes: 32, MaxPasswordBytes: 128}
}

func TestApprovedPilotPasswordProfileIsExactAndExplicit(t *testing.T) {
	want := password.Policy{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 2, SaltBytes: 16, KeyBytes: 32, MaxPasswordBytes: 128}
	if got := password.PilotPolicy(); got != want {
		t.Fatal("approved profile drifted", got)
	}
	if h, err := password.New(password.Policy{}); err == nil || h != nil {
		t.Fatal("zero policy silently received pilot defaults")
	}
	for name, value := range map[string][]byte{"twelve bytes": []byte("123456789012"), "spaces are not trimmed": []byte("            "), "utf8 measured in bytes": []byte("éééééé")} {
		t.Run(name, func(t *testing.T) {
			if err := password.ValidateNew(value); err != nil {
				t.Fatal("approved password rejected", err)
			}
		})
	}
	if err := password.ValidateNew([]byte("12345678901")); err == nil || err.Error() != "password_too_short" {
		t.Fatal("short password accepted", err)
	}
	if err := password.ValidateNew([]byte(strings.Repeat("x", 129))); err == nil || err.Error() != "password_too_large" {
		t.Fatal("long password accepted", err)
	}
	invalid := append([]byte(strings.Repeat("x", 12)), 0xff)
	if err := password.ValidateNew(invalid); err == nil || err.Error() != "password_utf8_invalid" {
		t.Fatal("invalid UTF-8 accepted", err)
	}
}

func TestArgon2idHashVerifyAndRehashSignal(t *testing.T) {
	h, err := password.NewWithRandom(testPolicy(), repeatReader(7))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := h.Hash([]byte("synthetic password"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=32,t=2,p=1$") || strings.Contains(encoded, "synthetic password") {
		t.Fatal("unsafe PHC output")
	}
	if err := h.ValidatePHC(encoded); err != nil {
		t.Fatal("current PHC rejected", err)
	}
	verified, err := h.Verify([]byte("synthetic password"), encoded)
	if err != nil || !verified.Match || verified.NeedsRehash {
		t.Fatal("valid password rejected", verified, err)
	}
	wrong, err := h.Verify([]byte("wrong"), encoded)
	if err != nil || wrong.Match || wrong.NeedsRehash {
		t.Fatal("wrong password accepted", wrong, err)
	}
	changed := testPolicy()
	changed.Iterations = 3
	rehash, err := password.NewWithRandom(changed, repeatReader(8))
	if err != nil {
		t.Fatal(err)
	}
	if err := rehash.ValidatePHC(encoded); err == nil || err.Error() != "password_phc_policy_mismatch" {
		t.Fatal("dummy profile mismatch accepted", err)
	}
	verified, err = rehash.Verify([]byte("synthetic password"), encoded)
	if err != nil || !verified.Match || !verified.NeedsRehash {
		t.Fatal("policy drift not identified", verified, err)
	}
	limitOnly := testPolicy()
	limitOnly.MaxPasswordBytes = 256
	same, err := password.NewWithRandom(limitOnly, repeatReader(9))
	if err != nil {
		t.Fatal(err)
	}
	verified, err = same.Verify([]byte("synthetic password"), encoded)
	if err != nil || verified.NeedsRehash {
		t.Fatal("local input limit changed hash policy")
	}
}

func TestArgon2idPolicyAndInputBounds(t *testing.T) {
	base := testPolicy()
	cases := map[string]func(*password.Policy){
		"zero memory": func(p *password.Policy) { p.MemoryKiB = 0 }, "memory relation": func(p *password.Policy) { p.MemoryKiB = 15; p.Parallelism = 2 }, "memory cap": func(p *password.Policy) { p.MemoryKiB = 256*1024 + 1 },
		"zero iterations": func(p *password.Policy) { p.Iterations = 0 }, "iteration cap": func(p *password.Policy) { p.Iterations = 21 }, "zero parallelism": func(p *password.Policy) { p.Parallelism = 0 }, "parallelism cap": func(p *password.Policy) { p.Parallelism = 17; p.MemoryKiB = 136 },
		"short salt": func(p *password.Policy) { p.SaltBytes = 15 }, "long salt": func(p *password.Policy) { p.SaltBytes = 65 }, "short key": func(p *password.Policy) { p.KeyBytes = 15 }, "long key": func(p *password.Policy) { p.KeyBytes = 65 },
		"zero password cap": func(p *password.Policy) { p.MaxPasswordBytes = 0 }, "password cap": func(p *password.Policy) { p.MaxPasswordBytes = 1024*1024 + 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := base
			mutate(&p)
			if h, err := password.NewWithRandom(p, repeatReader(1)); err == nil || h != nil || err.Error() != "password_policy_invalid" {
				t.Fatal("invalid policy accepted", err)
			}
		})
	}
	if h, err := password.NewWithRandom(base, nil); err == nil || h != nil || err.Error() != "password_random_invalid" {
		t.Fatal("nil entropy accepted")
	}
	h, _ := password.NewWithRandom(base, repeatReader(1))
	large := make([]byte, base.MaxPasswordBytes+1)
	if _, err := h.Hash(large); err == nil || err.Error() != "password_too_large" {
		t.Fatal("oversized hash input accepted")
	}
	if _, err := h.Verify(large, "secret-canary"); err == nil || err.Error() != "password_too_large" {
		t.Fatal("oversized verify input accepted")
	}
	var zero *password.Hasher
	if err := zero.ValidatePHC("value"); err == nil || err.Error() != "password_hasher_invalid" {
		t.Fatal("nil hasher validated PHC")
	}
	if _, err := zero.Hash(nil); err == nil || err.Error() != "password_hasher_invalid" {
		t.Fatal("nil hasher accepted")
	}
}

func TestArgon2idRejectsMalformedOrHostilePHCBeforeWork(t *testing.T) {
	h, _ := password.NewWithRandom(testPolicy(), repeatReader(2))
	valid, _ := h.Hash([]byte("value"))
	parts := strings.Split(valid, "$")
	cases := []string{"", strings.Repeat("x", 2049), "$argon2i$v=19$m=32,t=2,p=1$AAAA$AAAA", "$argon2id$v=16$m=32,t=2,p=1$AAAA$AAAA", "$argon2id$v=19$m=262145,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAA", "$argon2id$v=19$m=32,t=21,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAA", "$argon2id$v=19$m=32,t=2,p=0$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAA", "$argon2id$v=19$m=32,t=2,p=1$***$AAAA", strings.Join([]string{parts[0], parts[1], parts[2], parts[3], parts[4], "AA=="}, "$")}
	for i, value := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			result, err := h.Verify([]byte("value"), value)
			if err == nil || err.Error() != "password_phc_invalid" || result != (password.Verification{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("malformed PHC accepted/leaked", result, err)
			}
		})
	}
}

func TestArgon2idEntropyFailureAndConcurrentUse(t *testing.T) {
	h, _ := password.NewWithRandom(testPolicy(), failingReader{})
	if encoded, err := h.Hash([]byte("value")); err == nil || err.Error() != "password_random_failed" || encoded != "" || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("entropy failure unsafe", encoded, err)
	}
	h, _ = password.NewWithRandom(testPolicy(), repeatReader(4))
	encoded, _ := h.Hash([]byte("value"))
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := h.Verify([]byte("value"), encoded)
			if err != nil || !result.Match {
				errs <- errors.New("concurrent verification failed")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
