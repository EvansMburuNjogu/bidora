package identity_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/googleoidc"
)

type googleKeySource struct {
	key   *rsa.PublicKey
	err   error
	calls int
}

func (s *googleKeySource) Key(context.Context, string) (*rsa.PublicKey, error) {
	s.calls++
	return s.key, s.err
}

type googleClock struct{ at time.Time }

func (c *googleClock) Now() time.Time { return c.at }

type googleHTTPResult struct {
	response *http.Response
	err      error
}
type googleHTTPDoer struct {
	results []googleHTTPResult
	calls   int
}

func (d *googleHTTPDoer) RoundTrip(request *http.Request) (*http.Response, error) {
	d.calls++
	if request.URL.String() != "https://www.googleapis.com/oauth2/v3/certs" || request.Method != http.MethodGet || request.Header.Get("Accept") != "application/json" {
		return nil, errors.New("unsafe Google JWKS request")
	}
	if len(d.results) == 0 {
		return nil, errors.New("unexpected Google JWKS request")
	}
	result := d.results[0]
	d.results = d.results[1:]
	if result.response != nil {
		result.response.Request = request
	}
	return result.response, result.err
}
func googleRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}
func signedGoogleToken(t *testing.T, key *rsa.PrivateKey, keyID string, now time.Time, payloadOverride string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": keyID, "typ": "JWT"})
	payload := payloadOverride
	if payload == "" {
		body, _ := json.Marshal(map[string]any{"iss": "https://accounts.google.com", "aud": "client.apps.googleusercontent.com", "sub": "12345678901234567890", "email": "User@example.test", "email_verified": true, "nonce": strings.Repeat("n", 43), "iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix()})
		payload = string(body)
	}
	head := base64.RawURLEncoding.EncodeToString(header)
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	digest := sha256.Sum256([]byte(head + "." + body))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return head + "." + body + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func jwksResponse(keyID string, key *rsa.PublicKey, cache string) *http.Response {
	exponent := big.NewInt(int64(key.E)).Bytes()
	body, _ := json.Marshal(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "alg": "RS256", "use": "sig", "kid": keyID, "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(exponent)}}})
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}, "Cache-Control": []string{cache}}, Body: io.NopCloser(strings.NewReader(string(body)))}
}
func TestGoogleIDTokenVerifierAuthenticatesSignedClaims(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	key := googleRSAKey(t)
	source := &googleKeySource{key: &key.PublicKey}
	verifier, err := googleoidc.NewTokenVerifierWithClock("client.apps.googleusercontent.com", source, &googleClock{at: now})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := verifier.Verify(context.Background(), signedGoogleToken(t, key, "key-1", now, ""), strings.Repeat("n", 43))
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "12345678901234567890" || identity.Email.Lookup() != "user@example.test" || source.calls != 1 {
		t.Fatal("verified Google token mapping drifted", identity)
	}
}
func TestGoogleIDTokenVerifierRejectsSignatureAndJSONAmbiguity(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	trusted := googleRSAKey(t)
	untrusted := googleRSAKey(t)
	source := &googleKeySource{key: &trusted.PublicKey}
	verifier, _ := googleoidc.NewTokenVerifierWithClock("client.apps.googleusercontent.com", source, &googleClock{at: now})
	duplicatePayload := fmt.Sprintf(`{"iss":"https://accounts.google.com","aud":"client.apps.googleusercontent.com","aud":"other","sub":"123","email":"user@example.test","email_verified":true,"nonce":"%s","iat":%d,"exp":%d}`, strings.Repeat("n", 43), now.Add(-time.Minute).Unix(), now.Add(time.Hour).Unix())
	for _, token := range []string{signedGoogleToken(t, untrusted, "key-1", now, ""), signedGoogleToken(t, trusted, "key-1", now, duplicatePayload), "not-a-jwt"} {
		if identity, err := verifier.Verify(context.Background(), token, strings.Repeat("n", 43)); err == nil || identity != (googleoidc.Identity{}) || err.Error() != "google_oidc_token_invalid" {
			t.Fatal("unsafe Google token accepted", identity, err)
		}
	}
	unavailable := &googleKeySource{err: errors.New("secret-canary JWKS")}
	verifier, _ = googleoidc.NewTokenVerifierWithClock("client.apps.googleusercontent.com", unavailable, &googleClock{at: now})
	if _, err := verifier.Verify(context.Background(), signedGoogleToken(t, trusted, "key-1", now, ""), strings.Repeat("n", 43)); err == nil || err.Error() != "google_oidc_verification_unavailable" || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
		t.Fatal("JWKS error leaked", err)
	}
}
func TestGoogleJWKSCacheRefreshesUnknownKeyWithBoundedRotation(t *testing.T) {
	now := time.Now().UTC()
	first := googleRSAKey(t)
	second := googleRSAKey(t)
	clock := &googleClock{at: now}
	client := &googleHTTPDoer{results: []googleHTTPResult{{response: jwksResponse("key-1", &first.PublicKey, "public, max-age=3600")}, {response: jwksResponse("key-2", &second.PublicKey, "max-age=7200")}}}
	keys, err := googleoidc.NewGoogleKeySetWithClock(client, clock)
	if err != nil {
		t.Fatal(err)
	}
	key, err := keys.Key(context.Background(), "key-1")
	if err != nil || key.N.Cmp(first.N) != 0 {
		t.Fatal("initial Google key fetch failed", err)
	}
	if _, err = keys.Key(context.Background(), "key-1"); err != nil || client.calls != 1 {
		t.Fatal("cached Google key not reused", err, client.calls)
	}
	if _, err = keys.Key(context.Background(), "key-2"); !errors.Is(err, googleoidc.ErrKeyNotFound) || client.calls != 1 {
		t.Fatal("unknown key bypassed refresh cooldown", err, client.calls)
	}
	clock.at = clock.at.Add(31 * time.Second)
	key, err = keys.Key(context.Background(), "key-2")
	if err != nil || key.N.Cmp(second.N) != 0 || client.calls != 2 {
		t.Fatal("rotated Google key not refreshed", err, client.calls)
	}
	if _, err = keys.Key(context.Background(), "key-1"); !errors.Is(err, googleoidc.ErrKeyNotFound) {
		t.Fatal("removed Google key survived refresh", err)
	}
}
func TestGoogleJWKSRejectsUnsafeResponses(t *testing.T) {
	key := googleRSAKey(t)
	for _, response := range []*http.Response{jwksResponse("key", &key.PublicKey, ""), {StatusCode: 500, Header: http.Header{"Content-Type": []string{"application/json"}, "Cache-Control": []string{"max-age=60"}}, Body: io.NopCloser(strings.NewReader(`{}`))}, {StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}, "Cache-Control": []string{"max-age=60"}}, Body: io.NopCloser(strings.NewReader(`{}`))}} {
		keys, _ := googleoidc.NewGoogleKeySetWithClock(&googleHTTPDoer{results: []googleHTTPResult{{response: response}}}, &googleClock{at: time.Now().UTC()})
		if _, err := keys.Key(context.Background(), "key"); err == nil || err.Error() != "google_oidc_jwks_unavailable" {
			t.Fatal("unsafe JWKS response accepted", err)
		}
	}
}
