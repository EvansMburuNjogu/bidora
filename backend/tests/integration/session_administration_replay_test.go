package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

func createLiveIdentitySession(t *testing.T, control *postgres.Pool, user identityapp.ID, epoch int64, now time.Time) (identityapp.ID, session.SessionToken) {
	t.Helper()
	pair, err := session.GeneratePair()
	if err != nil {
		t.Fatal(err)
	}
	id := identityapp.ID(uuid.New())
	committer, err := identitypersistence.NewLoginCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	err = committer.CommitLoginSession(context.Background(), identityapp.SessionCommit{SessionID: id, AuditID: identityapp.ID(uuid.New()), UserID: user, CorrelationID: identityapp.ID(uuid.New()), SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), AuthEpoch: epoch, AuthenticationMethod: identityapp.AuthenticationMethodPassword, AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", CreatedAt: now, IdleExpiresAt: now.Add(30 * time.Minute), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuredAt: now})
	if err != nil {
		t.Fatal(err)
	}
	return id, pair.Session
}

func TestLiveSessionInventoryConcurrentRevokeOtherAndRevokeAll(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	user, otherUser := identityapp.ID(uuid.New()), identityapp.ID(uuid.New())
	err := control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		for index, id := range []identityapp.ID{user, otherUser} {
			email := "session-admin-" + uuid.NewString() + "@example.invalid"
			if _, execErr := db.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES($1,$2,$2,$3,NULL,1,$3,$3)`, uuid.UUID(id), email, now.Add(-time.Minute*time.Duration(index+1))); execErr != nil {
				return execErr
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	currentID, currentToken := createLiveIdentitySession(t, control, user, 1, now)
	targetID, _ := createLiveIdentitySession(t, control, user, 1, now.Add(time.Second))
	remainingID, _ := createLiveIdentitySession(t, control, user, 1, now.Add(2*time.Second))
	otherUserSession, _ := createLiveIdentitySession(t, control, otherUser, 1, now)
	principal := identityapp.SessionPrincipal{SessionID: currentID, UserID: user, AuthEpoch: 1, AbsoluteExpiresAt: now.Add(12 * time.Hour)}
	reader, err := identitypersistence.NewSessionReader(control)
	if err != nil {
		t.Fatal(err)
	}
	inventoryService, err := identityapp.NewSessionInventoryService(reader, identityClock{now.Add(3 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := inventoryService.Get(context.Background(), principal)
	if err != nil || inventory.ActiveSessionCount != 3 || inventory.OtherSessionCount != 2 {
		t.Fatal("live account-owned inventory drifted", inventory, err)
	}
	store, err := identitypersistence.NewSessionAdministrationStore(control)
	if err != nil {
		t.Fatal(err)
	}
	service, err := identityapp.NewSessionAdministrationService(store, identityIDs{}, identityClock{now.Add(4 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	input := identityapp.RevokeOtherSessionInput{Principal: principal, TargetSessionID: targetID, CorrelationID: identityapp.ID(uuid.New()), IdempotencyKey: uuid.NewString(), SessionAuthenticatedAt: now}
	type revokeResult struct {
		value identityapp.RevokeOtherSessionResult
		err   error
	}
	results := make(chan revokeResult, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			value, revokeErr := service.RevokeOther(context.Background(), input)
			results <- revokeResult{value: value, err: revokeErr}
		}()
	}
	wait.Wait()
	close(results)
	revoked, replayed := 0, 0
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.value.Revoked {
			revoked++
		}
		if result.value.Replay {
			replayed++
		}
	}
	if revoked != 1 || replayed != 1 {
		t.Fatal("concurrent revoke-other idempotency drifted", revoked, replayed)
	}
	if rowCount(t, control, `SELECT count(*) FROM identity.session_revocation_commands WHERE actor_user_id=$1`, uuid.UUID(user)) != 1 || rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE actor_id=$1 AND action='OtherSessionRevoked'`, uuid.UUID(user)) != 1 || rowCount(t, control, `SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE actor_id=$1 AND event_type='identity.session.revoked_by_owner.v1'`, uuid.UUID(user)) != 1 {
		t.Fatal("revoke-other command/audit/outbox was not atomic")
	}
	all, err := service.RevokeAll(context.Background(), principal, identityapp.ID(uuid.New()), now)
	if err != nil || all.AuthEpoch != 2 || all.RevokedCount != 2 {
		t.Fatal("revoke-all result drifted", all, err)
	}
	if rowCount(t, control, `SELECT count(*) FROM identity.sessions WHERE user_id=$1 AND revoked_at IS NULL`, uuid.UUID(user)) != 0 || rowCount(t, control, `SELECT count(*) FROM identity.users WHERE id=$1 AND auth_epoch=2`, uuid.UUID(user)) != 1 {
		t.Fatal("revoke-all state drifted")
	}
	if rowCount(t, control, `SELECT count(*) FROM identity.sessions WHERE id=$1 AND revoked_at IS NULL`, uuid.UUID(otherUserSession)) != 1 || rowCount(t, control, `SELECT count(*) FROM identity.sessions WHERE id=$1 AND revoked_at IS NOT NULL`, uuid.UUID(remainingID)) != 1 {
		t.Fatal("revoke-all crossed account boundary or missed owned session")
	}
	validator, err := identityapp.NewSessionValidator(reader, identityClock{now.Add(5 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = validator.Validate(context.Background(), currentToken.RevealForCookie()); err == nil || err.Error() != "session_invalid" {
		t.Fatal("stale session remained valid after revoke-all", err)
	}
	if rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE actor_id=$1 AND action='AllSessionsRevoked'`, uuid.UUID(user)) != 1 || rowCount(t, control, `SELECT count(*) FROM bidora_control_eventing.outbox_events WHERE actor_id=$1 AND event_type='identity.sessions.revoked_all.v1'`, uuid.UUID(user)) != 1 {
		t.Fatal("revoke-all audit/outbox drifted")
	}
}

func TestLiveConcurrentPasswordResetTokenHasExactlyOneWinner(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	now := time.Now().UTC().Truncate(time.Microsecond)
	hasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	oldPHC, err := hasher.Hash([]byte("synthetic old recovery password"))
	if err != nil {
		t.Fatal(err)
	}
	user := identityapp.ID(uuid.New())
	email := "reset-race-" + uuid.NewString() + "@example.invalid"
	token, err := verification.Generate()
	if err != nil {
		t.Fatal(err)
	}
	challenge := identityapp.ID(uuid.New())
	tokenHash := token.Hash()
	err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, execErr := db.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES($1,$2,$2,$3,NULL,1,$3,$3)`, uuid.UUID(user), email, now.Add(-time.Minute)); execErr != nil {
			return execErr
		}
		if _, execErr := db.Exec(ctx, `INSERT INTO identity.authentication_identities(user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES($1,$2,1,$3,$3,$3)`, uuid.UUID(user), oldPHC, now.Add(-time.Minute)); execErr != nil {
			return execErr
		}
		_, execErr := db.Exec(ctx, `INSERT INTO identity.password_recovery_challenges(id,user_id,token_hash,purpose,created_at,expires_at) VALUES($1,$2,$3,'PASSWORD_RECOVERY',$4::timestamptz,$4::timestamptz+interval '30 minutes')`, uuid.UUID(challenge), uuid.UUID(user), tokenHash[:], now)
		return execErr
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = createLiveIdentitySession(t, control, user, 1, now)
	store, err := identitypersistence.NewPasswordRecoveryStore(control)
	if err != nil {
		t.Fatal(err)
	}
	service, err := identityapp.NewPasswordRecoveryResetService(store, hasher, identityIDs{}, identityClock{now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	errorsOut := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			errorsOut <- service.Reset(context.Background(), identityapp.PasswordRecoveryResetInput{Token: token.RevealForDelivery(), NewPassword: []byte("synthetic reset race password"), CorrelationID: identityapp.ID(uuid.New())})
		}()
	}
	wait.Wait()
	close(errorsOut)
	success, denied := 0, 0
	for resetErr := range errorsOut {
		if resetErr == nil {
			success++
		} else if resetErr.Error() == "password_recovery_reset_invalid" {
			denied++
		} else {
			t.Fatal(resetErr)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatal("concurrent reset replay winners drifted", success, denied)
	}
	if rowCount(t, control, `SELECT count(*) FROM identity.password_recovery_challenges WHERE id=$1 AND consumed_at IS NOT NULL`, uuid.UUID(challenge)) != 1 || rowCount(t, control, `SELECT count(*) FROM identity.authentication_identities WHERE user_id=$1 AND credential_version=2`, uuid.UUID(user)) != 1 || rowCount(t, control, `SELECT count(*) FROM identity.users WHERE id=$1 AND auth_epoch=2`, uuid.UUID(user)) != 1 || rowCount(t, control, `SELECT count(*) FROM identity.sessions WHERE user_id=$1 AND revoked_at IS NULL`, uuid.UUID(user)) != 0 || rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE actor_id=$1 AND action='PasswordRecovered'`, uuid.UUID(user)) != 1 {
		t.Fatal("concurrent reset durable state drifted")
	}
}
