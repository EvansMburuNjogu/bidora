package integration_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/oidcredis"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/postgres"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func TestLiveGoogleLinkHTTPFlowRotatesSessionAndRejectsReplay(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	redisFixture := redisIntegration(t)
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, redisFixture, prefix)
	attempts, redisClient := liveController(t, redisFixture, prefix)
	transactions, err := oidcredis.New(redisClient, prefix, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := identityapp.ID(uuid.New())
	sessionID := identityapp.ID(uuid.New())
	ownerEmail := "link-http-owner-" + uuid.NewString() + "@example.invalid"
	passwordValue := []byte("synthetic fresh password")
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	phc, err := hasher.Hash(passwordValue)
	if err != nil {
		t.Fatal(err)
	}
	dummyPHC, err := hasher.Hash([]byte("synthetic dummy password"))
	if err != nil {
		t.Fatal(err)
	}
	currentPair, err := session.GeneratePair()
	if err != nil {
		t.Fatal(err)
	}
	created := now.Add(-time.Minute)
	sessionHash := currentPair.Session.Hash()
	csrfHash := currentPair.CSRF.Hash()
	err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, err := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$2,$3,NULL,1,$3,$3)`, uuid.UUID(userID), ownerEmail, created); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO identity.authentication_identities (user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES ($1,$2,1,$3,$3,$3)`, uuid.UUID(userID), phc, created); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `INSERT INTO identity.sessions (id,user_id,token_hash,csrf_hash,auth_epoch,rotated_from_session_id,created_at,last_seen_at,idle_expires_at,absolute_expires_at,revoked_at,assurance_level,assured_at,assurance_method) VALUES ($1,$2,$3,$4,1,NULL,$5::timestamptz,$5::timestamptz,$5::timestamptz+interval '30 minutes',$5::timestamptz+interval '12 hours',NULL,'AAL1',$5::timestamptz,'PRIMARY_ONLY')`, uuid.UUID(sessionID), uuid.UUID(userID), sessionHash[:], csrfHash[:], created)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := sha256.Sum256([]byte(uuid.NewString()))
	deriver, err := attempt.NewDeriver(secret[:])
	if err != nil {
		t.Fatal(err)
	}
	passwordReader, err := identitypersistence.NewPasswordReader(control)
	if err != nil {
		t.Fatal(err)
	}
	authenticator, err := identityapp.NewAuthenticator(passwordReader, attempts, deriver, hasher, dummyPHC)
	if err != nil {
		t.Fatal(err)
	}
	rehasher, err := identitypersistence.NewPasswordRehasher(control, hasher, identityClock{now})
	if err != nil {
		t.Fatal(err)
	}
	proofService, err := identityapp.NewGoogleLinkReauthenticationService(authenticator, rehasher)
	if err != nil {
		t.Fatal(err)
	}
	providerKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	provider := &syntheticGoogleTransport{key: providerKey, email: "linked-google-" + uuid.NewString() + "@example.invalid", subject: "linked-http-sub-" + uuid.NewString()}
	keySet, err := googleoidc.NewGoogleKeySet(provider)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := googleoidc.NewTokenVerifier("synthetic-client.apps.googleusercontent.com", keySet)
	if err != nil {
		t.Fatal(err)
	}
	exchanger, err := googleoidc.NewCodeExchanger(provider, googleoidc.ExchangeConfig{ClientID: "synthetic-client.apps.googleusercontent.com", ClientSecret: "synthetic-secret", RedirectURI: "https://api.example.test/v1/auth/google/link/callback"})
	if err != nil {
		t.Fatal(err)
	}
	linkCommitter, err := identitypersistence.NewGoogleLinkCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	linkService, err := identityapp.NewGoogleLinkService(identityapp.SecureSessionTokenGenerator{}, identityIDs{}, identityClock{now}, linkCommitter)
	if err != nil {
		t.Fatal(err)
	}
	linkFlow, err := identityapp.NewGoogleOIDCLinkFlow(googleoidc.AuthorizationConfig{ClientID: "synthetic-client.apps.googleusercontent.com", RedirectURI: "https://api.example.test/v1/auth/google/link/callback"}, transactions, exchanger, verifier, linkService)
	if err != nil {
		t.Fatal(err)
	}
	initiation, err := identityapp.NewGoogleLinkInitiationService(proofService, linkFlow)
	if err != nil {
		t.Fatal(err)
	}
	sessionReader, err := identitypersistence.NewSessionReader(control)
	if err != nil {
		t.Fatal(err)
	}
	sessionValidator, err := identityapp.NewSessionValidator(sessionReader, identityClock{now})
	if err != nil {
		t.Fatal(err)
	}
	sessionToucher, err := identitypersistence.NewSessionToucher(control)
	if err != nil {
		t.Fatal(err)
	}
	protection, err := browser.NewProtection([]string{"https://app.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(sessionValidator, sessionToucher, protection, identityClock{now})
	if err != nil {
		t.Fatal(err)
	}
	ipResolver, err := browser.NewClientIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	startHandler, callbackHandler, err := httpapi.NewGoogleLinkHTTPHandlers(initiation, linkFlow, ipResolver, deriver, httpapi.GoogleHTTPConfig{SuccessRedirect: "https://app.example.test/settings/security/linked", FailureRedirect: "https://app.example.test/settings/security", TransactionTTL: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { c.Set("X-Request-ID", uuid.NewString()); return c.Next() })
	app.Post(httpapi.GoogleLinkStartPath, middleware.Handle, startHandler.Handle)
	app.Get(httpapi.GoogleLinkCallbackPath, middleware.Handle, callbackHandler.Handle)
	rawSession := currentPair.Session.RevealForCookie()
	rawCSRF := currentPair.CSRF.RevealForBootstrap()
	startRequest := httptest.NewRequest(http.MethodPost, httpapi.GoogleLinkStartPath, strings.NewReader(`{"email":"`+ownerEmail+`","password":"synthetic fresh password"}`))
	startRequest.Header.Set("Content-Type", "application/json")
	startRequest.Header.Set("Origin", "https://app.example.test")
	startRequest.Header.Set(session.CSRFHeader, rawCSRF)
	startRequest.AddCookie(&http.Cookie{Name: session.CookieName, Value: rawSession})
	startRequest.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: rawCSRF})
	startResponse, err := app.Test(startRequest)
	if err != nil {
		t.Fatal(err)
	}
	if startResponse.StatusCode != http.StatusSeeOther {
		t.Fatal("live Google link initiation failed", startResponse.StatusCode)
	}
	authorizationURL, err := url.Parse(startResponse.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	provider.nonce = authorizationURL.Query().Get("nonce")
	state := authorizationURL.Query().Get("state")
	correlation := startResponse.Cookies()[0]
	callbackRequest := httptest.NewRequest(http.MethodGet, httpapi.GoogleLinkCallbackPath+"?state="+url.QueryEscape(state)+"&code=synthetic-code", nil)
	callbackRequest.AddCookie(&http.Cookie{Name: session.CookieName, Value: rawSession})
	callbackRequest.AddCookie(correlation)
	callbackResponse, err := app.Test(callbackRequest)
	if err != nil {
		t.Fatal(err)
	}
	if callbackResponse.StatusCode != http.StatusSeeOther || callbackResponse.Header.Get("Location") != "https://app.example.test/settings/security/linked" || provider.tokenCalls != 1 || provider.jwksCalls != 1 {
		t.Fatal("live Google link callback failed", callbackResponse.StatusCode, provider.tokenCalls, provider.jwksCalls)
	}
	var replacement *http.Cookie
	for _, cookie := range callbackResponse.Cookies() {
		if cookie.Name == session.CookieName {
			replacement = cookie
		}
	}
	if replacement == nil || replacement.Value == rawSession {
		t.Fatal("replacement Google link session missing")
	}
	replacementHash, err := session.HashPresentedSession(replacement.Value)
	if err != nil {
		t.Fatal(err)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.users AS u JOIN identity.external_identities AS e ON e.user_id=u.id JOIN identity.sessions AS old ON old.id=$2 JOIN identity.sessions AS replacement ON replacement.user_id=u.id WHERE u.id=$1 AND u.auth_epoch=2 AND e.provider_subject=$3 AND old.revoked_at IS NOT NULL AND replacement.token_hash=$4 AND replacement.rotated_from_session_id=old.id AND replacement.revoked_at IS NULL`, uuid.UUID(userID), uuid.UUID(sessionID), provider.subject, replacementHash[:]); count != 1 {
		t.Fatal("live Google link HTTP rotation missing", count)
	}
	replayRequest := httptest.NewRequest(http.MethodGet, httpapi.GoogleLinkCallbackPath+"?state="+url.QueryEscape(state)+"&code=synthetic-code", nil)
	replayRequest.AddCookie(&http.Cookie{Name: session.CookieName, Value: rawSession})
	replayRequest.AddCookie(correlation)
	replayResponse, err := app.Test(replayRequest)
	if err != nil {
		t.Fatal(err)
	}
	if replayResponse.StatusCode != http.StatusUnauthorized || provider.tokenCalls != 1 {
		t.Fatal("revoked Google link session replay reached provider", replayResponse.StatusCode, provider.tokenCalls)
	}
}
