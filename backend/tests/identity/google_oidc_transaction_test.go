package identity_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/oidcredis"
)

func googleTransaction(t *testing.T) googleoidc.Transaction {
	t.Helper()
	raw := make([]byte, googleoidc.TransactionSecretBytes*4)
	for index := range raw {
		raw[index] = byte(index + 1)
	}
	transaction, err := googleoidc.GenerateTransactionWithRandom(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return transaction
}
func TestGoogleOIDCTransactionGeneratesStateNonceAndS256PKCE(t *testing.T) {
	transaction := googleTransaction(t)
	if transaction.IsZero() {
		t.Fatal("generated Google transaction is zero")
	}
	material := transaction.MaterialForStore()
	if material.IsZero() || transaction.StateForAuthorization() == material.Nonce || material.Nonce == material.Verifier || transaction.BrowserBindingForCookie() == transaction.StateForAuthorization() {
		t.Fatal("Google transaction secrets are not independent")
	}
	hash := sha256.Sum256([]byte(material.Verifier))
	if got := base64.RawURLEncoding.EncodeToString(hash[:]); got != transaction.PKCEChallenge() {
		t.Fatal("PKCE S256 challenge drifted")
	}
	stateHash, err := googleoidc.HashPresentedState(transaction.StateForAuthorization())
	if err != nil || stateHash == [32]byte{} {
		t.Fatal("state hashing failed", err)
	}
	bindingHash, err := googleoidc.HashPresentedBrowserBinding(transaction.BrowserBindingForCookie())
	if err != nil || bindingHash != material.BrowserBindingHash {
		t.Fatal("browser binding hashing failed", err)
	}
	if matches, err := googleoidc.MatchesBrowserBinding(transaction.BrowserBindingForCookie(), material.BrowserBindingHash); err != nil || !matches {
		t.Fatal("browser binding match failed", err)
	}
	if matches, err := googleoidc.MatchesBrowserBinding(transaction.StateForAuthorization(), material.BrowserBindingHash); err != nil || matches {
		t.Fatal("wrong browser binding accepted", err)
	}
	formatted := fmt.Sprintf("%#v %#v", transaction, material)
	for _, secret := range []string{transaction.StateForAuthorization(), material.Nonce, material.Verifier, transaction.BrowserBindingForCookie()} {
		if strings.Contains(formatted, secret) {
			t.Fatal("Google transaction formatting leaked secret")
		}
	}
	if value, err := googleoidc.GenerateTransactionWithRandom(nil); err == nil || !value.IsZero() {
		t.Fatal("nil entropy accepted")
	}
	if _, err := googleoidc.HashPresentedState("bad"); err == nil {
		t.Fatal("malformed state accepted")
	}
}
func TestGoogleOIDCLinkMaterialIsPurposeAndPrincipalBound(t *testing.T) {
	transaction := googleTransaction(t)
	var userID, sessionID [16]byte
	userID[15], sessionID[15] = 1, 2
	material, err := transaction.MaterialForLink(userID, sessionID, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if material.IsZero() || material.IsSignIn() || !material.MatchesLink(userID, sessionID, 7) {
		t.Fatal("Google link material binding failed")
	}
	otherUser := userID
	otherUser[15] = 3
	if material.MatchesLink(otherUser, sessionID, 7) || material.MatchesLink(userID, sessionID, 8) {
		t.Fatal("Google link material crossed principal or epoch")
	}
	if _, err := transaction.MaterialForLink([16]byte{}, sessionID, 7, 3); err == nil {
		t.Fatal("zero link principal accepted")
	}
}

func TestGoogleOIDCRedisStoreHashesKeyAndConsumesOnce(t *testing.T) {
	transaction := googleTransaction(t)
	material := transaction.MaterialForStore()
	runner := &scriptRunnerFake{results: []scriptResult{{value: int64(1)}, {value: material.Nonce + "." + material.Verifier + "." + hex.EncodeToString(material.BrowserBindingHash[:]) + "." + string(material.Purpose) + "." + hex.EncodeToString(material.PrincipalBindingHash[:]) + "." + strconv.FormatInt(material.CredentialVersion, 10)}, {value: nil}}}
	store, err := oidcredis.New(runner, "bidora:test", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Create(context.Background(), transaction.StateForAuthorization(), material); err != nil {
		t.Fatal(err)
	}
	consumed, err := store.Consume(context.Background(), transaction.StateForAuthorization())
	if err != nil || consumed != material {
		t.Fatal("Google transaction consume failed", consumed, err)
	}
	if _, err = store.Consume(context.Background(), transaction.StateForAuthorization()); err == nil || err.Error() != "google_oidc_transaction_invalid" {
		t.Fatal("Google transaction replay accepted", err)
	}
	if len(runner.calls) != 3 || len(runner.calls[0].keys) != 1 || strings.Contains(runner.calls[0].keys[0], transaction.StateForAuthorization()) || !strings.Contains(runner.calls[0].keys[0], "{bidora:test}") {
		t.Fatal("unsafe Google transaction Redis key", runner.calls)
	}
	if len(runner.calls[0].args) != 2 || runner.calls[0].args[1] != (5*time.Minute).Milliseconds() {
		t.Fatal("explicit transaction TTL missing")
	}
}
func TestGoogleOIDCRedisStoreFailsClosed(t *testing.T) {
	transaction := googleTransaction(t)
	material := transaction.MaterialForStore()
	for _, tc := range []struct {
		name      string
		results   []scriptResult
		operation func(*oidcredis.Store) error
		want      string
	}{{"collision", []scriptResult{{value: int64(0)}}, func(s *oidcredis.Store) error {
		return s.Create(context.Background(), transaction.StateForAuthorization(), material)
	}, "google_oidc_transaction_unavailable"}, {"dependency", []scriptResult{{err: errors.New("secret-canary Redis")}}, func(s *oidcredis.Store) error {
		return s.Create(context.Background(), transaction.StateForAuthorization(), material)
	}, "google_oidc_transaction_unavailable"}, {"corrupt", []scriptResult{{value: "corrupt"}}, func(s *oidcredis.Store) error {
		_, err := s.Consume(context.Background(), transaction.StateForAuthorization())
		return err
	}, "google_oidc_transaction_invalid"}} {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := oidcredis.New(&scriptRunnerFake{results: tc.results}, "bidora", time.Minute)
			err := tc.operation(store)
			if err == nil || err.Error() != tc.want || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("Google transaction failure unsafe", err)
			}
		})
	}
	for _, ttl := range []time.Duration{0, 30 * time.Second, 16 * time.Minute} {
		if store, err := oidcredis.New(&scriptRunnerFake{}, "bidora", ttl); err == nil || store != nil {
			t.Fatal("unsafe TTL accepted", ttl)
		}
	}
	if store, err := oidcredis.New(nil, "bidora", time.Minute); err == nil || store != nil {
		t.Fatal("nil runner accepted")
	}
}
