package integration_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/oidcredis"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type syntheticGoogleTransport struct {
	mu                    sync.Mutex
	key                   *rsa.PrivateKey
	nonce, email, subject string
	tokenCalls, jwksCalls int
}

func (t *syntheticGoogleTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch request.URL.String() {
	case "https://oauth2.googleapis.com/token":
		if request.Method != http.MethodPost {
			return nil, errors.New("wrong token method")
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, 8193))
		if err != nil || len(body) > 8192 {
			return nil, errors.New("token request invalid")
		}
		form, err := url.ParseQuery(string(body))
		if err != nil || form.Get("code") != "synthetic-code" || form.Get("code_verifier") == "" || form.Get("grant_type") != "authorization_code" {
			return nil, errors.New("token form invalid")
		}
		t.tokenCalls++
		token, err := t.idToken(time.Now().UTC())
		if err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(map[string]string{"id_token": token, "access_token": "discarded"})
		return syntheticGoogleResponse(request, http.StatusOK, "application/json", string(payload), ""), nil
	case "https://www.googleapis.com/oauth2/v3/certs":
		if request.Method != http.MethodGet {
			return nil, errors.New("wrong JWKS method")
		}
		t.jwksCalls++
		exponent := big.NewInt(int64(t.key.E)).Bytes()
		payload, _ := json.Marshal(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "synthetic-key", "n": base64.RawURLEncoding.EncodeToString(t.key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(exponent)}}})
		return syntheticGoogleResponse(request, http.StatusOK, "application/json", string(payload), "max-age=3600"), nil
	default:
		return nil, errors.New("unexpected outbound target")
	}
}
func (t *syntheticGoogleTransport) idToken(now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "synthetic-key", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{"iss": "https://accounts.google.com", "aud": "synthetic-client.apps.googleusercontent.com", "sub": t.subject, "email": t.email, "email_verified": true, "nonce": t.nonce, "iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix()})
	head := base64.RawURLEncoding.EncodeToString(header)
	body := base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(head + "." + body))
	signature, err := rsa.SignPKCS1v15(rand.Reader, t.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return head + "." + body + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
func syntheticGoogleResponse(request *http.Request, status int, contentType, body, cache string) *http.Response {
	header := http.Header{"Content-Type": []string{contentType}}
	if cache != "" {
		header.Set("Cache-Control", cache)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}
}
func TestLiveGoogleHTTPFlowCommitsSessionAndRejectsReplay(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	redisFixture := redisIntegration(t)
	redisClient, err := attemptredis.NewClient(redisFixture.config)
	if err != nil {
		t.Fatal(err)
	}
	defer redisClient.Close()
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, redisFixture, prefix)
	transactions, err := oidcredis.New(redisClient, prefix, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	emailValue := "google-http-" + uuid.NewString() + "@example.invalid"
	provider := &syntheticGoogleTransport{key: privateKey, email: emailValue, subject: "google-http-sub-" + uuid.NewString()}
	keys, err := googleoidc.NewGoogleKeySet(provider)
	if err != nil {
		t.Fatal(err)
	}
	tokenVerifier, err := googleoidc.NewTokenVerifier("synthetic-client.apps.googleusercontent.com", keys)
	if err != nil {
		t.Fatal(err)
	}
	exchanger, err := googleoidc.NewCodeExchanger(provider, googleoidc.ExchangeConfig{ClientID: "synthetic-client.apps.googleusercontent.com", ClientSecret: "synthetic-client-secret", RedirectURI: "https://api.example.test/v1/auth/google/callback"})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := identitypersistence.NewGoogleIdentityResolver(control)
	if err != nil {
		t.Fatal(err)
	}
	claimsValidator, err := googleoidc.NewValidator(googleoidc.Config{ClientID: "synthetic-client.apps.googleusercontent.com"})
	if err != nil {
		t.Fatal(err)
	}
	identityService, err := identityapp.NewGoogleAuthenticationService(claimsValidator, resolver, identityIDs{}, liveIssueClock{})
	if err != nil {
		t.Fatal(err)
	}
	flow, err := identityapp.NewGoogleOIDCFlow(googleoidc.AuthorizationConfig{ClientID: "synthetic-client.apps.googleusercontent.com", RedirectURI: "https://api.example.test/v1/auth/google/callback"}, transactions, exchanger, tokenVerifier, identityService)
	if err != nil {
		t.Fatal(err)
	}
	committer, err := identitypersistence.NewLoginCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := identityapp.NewSessionIssuer(identityapp.SecureSessionTokenGenerator{}, identityIDs{}, liveIssueClock{}, committer)
	if err != nil {
		t.Fatal(err)
	}
	googleLogin, err := identityapp.NewGoogleLoginService(flow, issuer)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := browser.NewProtection([]string{"https://app.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	startHandler, callbackHandler, err := httpapi.NewGoogleHTTPHandlers(flow, googleLogin, protection, httpapi.GoogleHTTPConfig{SuccessRedirect: "https://app.example.test/auth/complete", FailureRedirect: "https://app.example.test/login", TransactionTTL: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { c.Set("X-Request-ID", uuid.NewString()); return c.Next() })
	app.Post(httpapi.GoogleStartPath, startHandler.Handle)
	app.Get(httpapi.GoogleCallbackPath, callbackHandler.Handle)
	startRequest := httptest.NewRequest(http.MethodPost, httpapi.GoogleStartPath, nil)
	startRequest.Header.Set("Origin", "https://app.example.test")
	startResponse, err := app.Test(startRequest)
	if err != nil {
		t.Fatal(err)
	}
	if startResponse.StatusCode != http.StatusSeeOther || len(startResponse.Cookies()) != 1 {
		t.Fatal("live Google start failed", startResponse.StatusCode)
	}
	authorizationURL, err := url.Parse(startResponse.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	provider.nonce = authorizationURL.Query().Get("nonce")
	state := authorizationURL.Query().Get("state")
	if provider.nonce == "" || state == "" {
		t.Fatal("live Google authorization binding missing")
	}
	correlationCookie := startResponse.Cookies()[0]
	callbackRequest := httptest.NewRequest(http.MethodGet, httpapi.GoogleCallbackPath+"?state="+url.QueryEscape(state)+"&code=synthetic-code", nil)
	callbackRequest.AddCookie(correlationCookie)
	callbackResponse, err := app.Test(callbackRequest)
	if err != nil {
		t.Fatal(err)
	}
	if callbackResponse.StatusCode != http.StatusSeeOther || callbackResponse.Header.Get("Location") != "https://app.example.test/auth/complete" || provider.tokenCalls != 1 || provider.jwksCalls != 1 {
		t.Fatal("live Google callback failed", callbackResponse.StatusCode, provider.tokenCalls, provider.jwksCalls)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range callbackResponse.Cookies() {
		if cookie.Name == session.CookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("live Google session cookie missing")
	}
	sessionHash, err := session.HashPresentedSession(sessionCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.external_identities AS e JOIN identity.users AS u ON u.id=e.user_id JOIN identity.sessions AS s ON s.user_id=u.id WHERE e.provider='GOOGLE' AND e.provider_subject=$1 AND u.email=$2 AND s.token_hash=$3`, provider.subject, emailValue, sessionHash[:]); count != 1 {
		t.Fatal("live Google identity/session transaction chain missing", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships AS m JOIN identity.users AS u ON u.id=m.user_id WHERE u.email=$1`, emailValue); count != 0 {
		t.Fatal("live Google HTTP flow granted membership", count)
	}
	replayRequest := httptest.NewRequest(http.MethodGet, httpapi.GoogleCallbackPath+"?state="+url.QueryEscape(state)+"&code=synthetic-code", nil)
	replayRequest.AddCookie(correlationCookie)
	replayResponse, err := app.Test(replayRequest)
	if err != nil {
		t.Fatal(err)
	}
	if replayResponse.StatusCode != http.StatusSeeOther || replayResponse.Header.Get("Location") != "https://app.example.test/login" || provider.tokenCalls != 1 {
		t.Fatal("Google callback replay reached provider", replayResponse.StatusCode, provider.tokenCalls)
	}
}
