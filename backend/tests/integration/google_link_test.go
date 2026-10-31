package integration_test

import (
	"context"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func TestLiveGoogleLinkRotatesEpochSessionsAndAuditsAtomically(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := identityapp.ID(uuid.New())
	currentSessionID := identityapp.ID(uuid.New())
	emailValue := "link-owner-" + uuid.NewString() + "@example.invalid"
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	phc, err := hasher.Hash([]byte("synthetic password value"))
	if err != nil {
		t.Fatal(err)
	}
	currentPair, err := session.GeneratePair()
	if err != nil {
		t.Fatal(err)
	}
	created := now.Add(-time.Minute)
	currentSessionHash := currentPair.Session.Hash()
	currentCSRFHash := currentPair.CSRF.Hash()
	err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, err := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$2,$3,NULL,1,$3,$3)`, uuid.UUID(userID), emailValue, created); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO identity.authentication_identities (user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES ($1,$2,1,$3,$3,$3)`, uuid.UUID(userID), phc, created); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `INSERT INTO identity.sessions (id,user_id,token_hash,csrf_hash,auth_epoch,rotated_from_session_id,created_at,last_seen_at,idle_expires_at,absolute_expires_at,revoked_at,assurance_level,assured_at,assurance_method) VALUES ($1,$2,$3,$4,1,NULL,$5::timestamptz,$5::timestamptz,$5::timestamptz+interval '30 minutes',$5::timestamptz+interval '12 hours',NULL,'AAL1',$5::timestamptz,'PRIMARY_ONLY')`, uuid.UUID(currentSessionID), uuid.UUID(userID), currentSessionHash[:], currentCSRFHash[:], created)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	address, err := email.Parse("DifferentGoogleEmail-" + uuid.NewString() + "@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	identity := googleoidc.Identity{Subject: "linked-sub-" + uuid.NewString(), Email: address, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	committer, err := identitypersistence.NewGoogleLinkCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	service, err := identityapp.NewGoogleLinkService(identityapp.SecureSessionTokenGenerator{}, identityIDs{}, identityClock{now}, committer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.LinkVerified(context.Background(), identityapp.GoogleLinkProof{UserID: userID, SessionID: currentSessionID, AuthEpoch: 1, CredentialVersion: 1}, identity, identityapp.ID(uuid.New()))
	if err != nil {
		t.Fatal(err)
	}
	replacementHash := result.Session.Hash()
	if count := rowCount(t, control, `SELECT count(*) FROM identity.users AS u JOIN identity.external_identities AS e ON e.user_id=u.id JOIN identity.sessions AS old ON old.id=$2 JOIN identity.sessions AS replacement ON replacement.user_id=u.id WHERE u.id=$1 AND u.auth_epoch=2 AND e.provider='GOOGLE' AND e.provider_subject=$3 AND e.email_at_link=$4 AND old.revoked_at IS NOT NULL AND replacement.token_hash=$5 AND replacement.auth_epoch=2 AND replacement.rotated_from_session_id=old.id AND replacement.revoked_at IS NULL`, uuid.UUID(userID), uuid.UUID(currentSessionID), identity.Subject, address.Display(), replacementHash[:]); count != 1 {
		t.Fatal("live Google link rotation missing", count)
	}
	if count := rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE actor_id=$1 AND action IN ('GoogleIdentityLinked','UserLoggedIn')`, uuid.UUID(userID)); count != 2 {
		t.Fatal("live Google link audits missing", count)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM tenancy.memberships WHERE user_id=$1`, uuid.UUID(userID)); count != 0 {
		t.Fatal("Google link granted membership", count)
	}
	second := identity
	second.Subject = "other-sub-" + uuid.NewString()
	failed, err := service.LinkVerified(context.Background(), identityapp.GoogleLinkProof{UserID: userID, SessionID: currentSessionID, AuthEpoch: 1, CredentialVersion: 1}, second, identityapp.ID(uuid.New()))
	if err == nil || err.Error() != "google_link_invalid" || failed != (identityapp.LoginResult{}) {
		t.Fatal("stale Google link proof accepted", failed, err)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.external_identities WHERE user_id=$1`, uuid.UUID(userID)); count != 1 {
		t.Fatal("failed relink changed bindings", count)
	}
}
