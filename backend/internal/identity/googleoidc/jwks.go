package googleoidc

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const googleJWKSEndpoint = "https://www.googleapis.com/oauth2/v3/certs"
const maxJWKSBytes = 64 * 1024
const unknownKeyRefreshInterval = 30 * time.Second
const maximumJWKSCacheLifetime = 6 * time.Hour

type HTTPRoundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}
type KeySet struct {
	client                 HTTPRoundTripper
	clock                  Clock
	mu                     sync.Mutex
	keys                   map[string]*rsa.PublicKey
	expiresAt, lastRefresh time.Time
}

func NewGoogleKeySet(client HTTPRoundTripper) (*KeySet, error) {
	return NewGoogleKeySetWithClock(client, systemClock{})
}
func NewGoogleKeySetWithClock(client HTTPRoundTripper, clock Clock) (*KeySet, error) {
	if client == nil || clock == nil {
		return nil, errors.New("google_oidc_jwks_config_invalid")
	}
	return &KeySet{client: client, clock: clock}, nil
}
func (s *KeySet) Key(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	if s == nil || s.client == nil || s.clock == nil || ctx == nil {
		return nil, errors.New("google_oidc_jwks_unavailable")
	}
	if !boundedASCII(keyID, 1, 255) {
		return nil, ErrKeyNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now().UTC()
	if now.IsZero() {
		return nil, errors.New("google_oidc_jwks_unavailable")
	}
	if key := s.keys[keyID]; key != nil && now.Before(s.expiresAt) {
		return clonePublicKey(key), nil
	}
	if now.Before(s.expiresAt) && !s.lastRefresh.IsZero() && now.Sub(s.lastRefresh) < unknownKeyRefreshInterval {
		return nil, ErrKeyNotFound
	}
	keys, expiresAt, err := s.fetch(ctx, now)
	if err != nil {
		return nil, &jwksError{cause: err}
	}
	s.keys, s.expiresAt, s.lastRefresh = keys, expiresAt, now
	if key := keys[keyID]; key != nil {
		return clonePublicKey(key), nil
	}
	return nil, ErrKeyNotFound
}
func (s *KeySet) fetch(ctx context.Context, now time.Time) (map[string]*rsa.PublicKey, time.Time, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, googleJWKSEndpoint, nil)
	if err != nil {
		return nil, time.Time{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := s.client.RoundTrip(request)
	if err != nil {
		return nil, time.Time{}, err
	}
	if response == nil || response.Body == nil {
		return nil, time.Time{}, errors.New("google_oidc_jwks_response_invalid")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, time.Time{}, errors.New("google_oidc_jwks_response_invalid")
	}
	if response.Request != nil && (response.Request.URL == nil || response.Request.URL.String() != googleJWKSEndpoint) {
		return nil, time.Time{}, errors.New("google_oidc_jwks_redirect_rejected")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if contentType != "application/json" {
		return nil, time.Time{}, errors.New("google_oidc_jwks_response_invalid")
	}
	lifetime, err := cacheLifetime(response.Header.Get("Cache-Control"))
	if err != nil {
		return nil, time.Time{}, err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSBytes+1))
	if err != nil || len(body) > maxJWKSBytes {
		return nil, time.Time{}, errors.New("google_oidc_jwks_response_invalid")
	}
	if uniqueJSON(body) != nil {
		return nil, time.Time{}, errors.New("google_oidc_jwks_response_invalid")
	}
	var document struct {
		Keys []struct {
			KeyType   string `json:"kty"`
			Algorithm string `json:"alg"`
			Use       string `json:"use"`
			KeyID     string `json:"kid"`
			Modulus   string `json:"n"`
			Exponent  string `json:"e"`
		} `json:"keys"`
	}
	if json.Unmarshal(body, &document) != nil || len(document.Keys) < 1 || len(document.Keys) > 32 {
		return nil, time.Time{}, errors.New("google_oidc_jwks_response_invalid")
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, candidate := range document.Keys {
		if candidate.KeyType != "RSA" || candidate.Algorithm != "RS256" || candidate.Use != "sig" || !boundedASCII(candidate.KeyID, 1, 255) {
			return nil, time.Time{}, errors.New("google_oidc_jwks_key_invalid")
		}
		if _, duplicate := keys[candidate.KeyID]; duplicate {
			return nil, time.Time{}, errors.New("google_oidc_jwks_key_invalid")
		}
		modulusBytes, err := base64.RawURLEncoding.Strict().DecodeString(candidate.Modulus)
		if err != nil || len(modulusBytes) < 256 || len(modulusBytes) > 512 {
			return nil, time.Time{}, errors.New("google_oidc_jwks_key_invalid")
		}
		exponentBytes, err := base64.RawURLEncoding.Strict().DecodeString(candidate.Exponent)
		if err != nil || len(exponentBytes) < 1 || len(exponentBytes) > 4 {
			return nil, time.Time{}, errors.New("google_oidc_jwks_key_invalid")
		}
		exponentBig := new(big.Int).SetBytes(exponentBytes)
		if !exponentBig.IsInt64() {
			return nil, time.Time{}, errors.New("google_oidc_jwks_key_invalid")
		}
		exponent := exponentBig.Int64()
		if exponent < 3 || exponent > 1<<31-1 || exponent%2 == 0 {
			return nil, time.Time{}, errors.New("google_oidc_jwks_key_invalid")
		}
		modulus := new(big.Int).SetBytes(modulusBytes)
		if modulus.BitLen() < 2048 || modulus.BitLen() > 4096 {
			return nil, time.Time{}, errors.New("google_oidc_jwks_key_invalid")
		}
		keys[candidate.KeyID] = &rsa.PublicKey{N: modulus, E: int(exponent)}
	}
	return keys, now.Add(lifetime), nil
}
func cacheLifetime(header string) (time.Duration, error) {
	for _, directive := range strings.Split(header, ",") {
		name, value, found := strings.Cut(strings.TrimSpace(directive), "=")
		if !found || strings.ToLower(name) != "max-age" {
			continue
		}
		seconds, err := strconv.ParseInt(strings.Trim(value, "\""), 10, 64)
		if err != nil || seconds < 60 {
			return 0, errors.New("google_oidc_jwks_cache_invalid")
		}
		maximumSeconds := int64(maximumJWKSCacheLifetime / time.Second)
		if seconds > maximumSeconds {
			return maximumJWKSCacheLifetime, nil
		}
		return time.Duration(seconds) * time.Second, nil
	}
	return 0, errors.New("google_oidc_jwks_cache_invalid")
}
func clonePublicKey(key *rsa.PublicKey) *rsa.PublicKey {
	if key == nil || key.N == nil {
		return nil
	}
	return &rsa.PublicKey{N: new(big.Int).Set(key.N), E: key.E}
}

type jwksError struct{ cause error }

func (e *jwksError) Error() string                 { return "google_oidc_jwks_unavailable" }
func (e *jwksError) Unwrap() error                 { return e.cause }
func (e *jwksError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
