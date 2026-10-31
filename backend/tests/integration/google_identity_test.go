package integration_test

import (
	"context"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/googleoidc"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"github.com/google/uuid"
)

func TestLiveGoogleIdentitySignupSubjectSigninAndEmailLinkDenial(t *testing.T) {
	runtime := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	clientID := "synthetic-client.apps.googleusercontent.com"
	validator, err := googleoidc.NewValidator(googleoidc.Config{ClientID: clientID})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := identitypersistence.NewGoogleIdentityResolver(runtime)
	if err != nil {
		t.Fatal(err)
	}
	service, err := identityapp.NewGoogleAuthenticationService(validator, resolver, identityIDs{}, liveIssueClock{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	nonce := "synthetic-google-nonce-012345678901234567890123"
	emailValue := "google-" + uuid.NewString() + "@example.invalid"
	claims := googleoidc.Claims{Issuer: "https://accounts.google.com", Audience: clientID, Subject: "google-sub-" + uuid.NewString(), Email: emailValue, EmailVerified: true, Nonce: nonce, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	first, err := service.Authenticate(context.Background(), identityapp.GoogleAuthenticationInput{Claims: claims, ExpectedNonce: nonce, CorrelationID: identityapp.ID(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || first.UserID.IsZero() || first.AuthEpoch != 1 {
		t.Fatal("live Google signup failed", first)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.external_identities WHERE user_id=$1 AND provider='GOOGLE' AND provider_subject=$2 AND email_at_link=$3`, uuid.UUID(first.UserID), claims.Subject, emailValue); count != 1 {
		t.Fatal("Google subject binding missing", count)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM tenancy.memberships WHERE user_id=$1`, uuid.UUID(first.UserID)); count != 0 {
		t.Fatal("Google signup granted membership", count)
	}
	committer, err := identitypersistence.NewLoginCommitter(runtime)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := identityapp.NewSessionIssuer(identityapp.SecureSessionTokenGenerator{}, identityIDs{}, liveIssueClock{}, committer)
	if err != nil {
		t.Fatal(err)
	}
	login, err := issuer.Issue(context.Background(), identityapp.AuthenticatedSessionInput{UserID: first.UserID, CorrelationID: identityapp.ID(uuid.New()), AuthEpoch: first.AuthEpoch, Method: identityapp.AuthenticationMethodGoogle})
	if err != nil {
		t.Fatal(err)
	}
	if login.Session.IsZero() || login.CSRF.IsZero() {
		t.Fatal("Google session secrets missing after commit")
	}
	sessionHash := login.Session.Hash()
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.sessions WHERE user_id=$1 AND token_hash=$2 AND auth_epoch=$3`, uuid.UUID(first.UserID), sessionHash[:], first.AuthEpoch); count != 1 {
		t.Fatal("Google session commit missing", count)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE actor_id=$1 AND action='UserLoggedIn' AND change_summary->>'authentication_method'='GOOGLE'`, uuid.UUID(first.UserID)); count != 1 {
		t.Fatal("Google login audit method missing", count)
	}
	changed := claims
	changed.Email = "changed-" + uuid.NewString() + "@example.invalid"
	changed.IssuedAt = time.Now().UTC().Add(-time.Minute)
	changed.ExpiresAt = time.Now().UTC().Add(time.Hour)
	second, err := service.Authenticate(context.Background(), identityapp.GoogleAuthenticationInput{Claims: changed, ExpectedNonce: nonce, CorrelationID: identityapp.ID(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.UserID != first.UserID {
		t.Fatal("stable Google subject did not retain user", second)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.users WHERE id=$1 AND email=$2`, uuid.UUID(first.UserID), emailValue); count != 1 {
		t.Fatal("Google claim silently overwrote canonical email", count)
	}
	collision := claims
	collision.Subject = "other-sub-" + uuid.NewString()
	collision.IssuedAt = time.Now().UTC().Add(-time.Minute)
	collision.ExpiresAt = time.Now().UTC().Add(time.Hour)
	result, err := service.Authenticate(context.Background(), identityapp.GoogleAuthenticationInput{Claims: collision, ExpectedNonce: nonce, CorrelationID: identityapp.ID(uuid.New())})
	if err == nil || err.Error() != "google_authentication_invalid" || result != (identityapp.GoogleIdentityResult{}) {
		t.Fatal("matching email auto-linked Google identity", result, err)
	}
	if count := rowCount(t, runtime, `SELECT count(*) FROM identity.external_identities WHERE user_id=$1`, uuid.UUID(first.UserID)); count != 1 {
		t.Fatal("email collision created second Google binding", count)
	}
}
