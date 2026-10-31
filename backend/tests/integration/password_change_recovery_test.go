package integration_test

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/deliverysmtp"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/recoveryredis"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLivePasswordChangeAndRecoveryRotateEpochSessionsAndSingleUseToken(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	redisFixture := redisIntegration(t)
	prefix := "bidora:test:recovery:" + uuid.NewString()
	defer cleanupPrefix(t, redisFixture, prefix)
	hasher, e := password.New(password.PilotPolicy())
	if e != nil {
		t.Fatal(e)
	}
	current := []byte("synthetic current password")
	currentPHC, e := hasher.Hash(current)
	if e != nil {
		t.Fatal(e)
	}
	dummy, e := hasher.Hash([]byte("synthetic dummy password"))
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := identityapp.ID(uuid.New())
	emailValue := "password-flow-" + uuid.NewString() + "@example.invalid"
	e = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, e := db.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES($1,$2,$2,$3,NULL,1,$3,$3)`, uuid.UUID(user), emailValue, now.Add(-time.Minute)); e != nil {
			return e
		}
		_, e := db.Exec(ctx, `INSERT INTO identity.authentication_identities(user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES($1,$2,1,$3,$3,$3)`, uuid.UUID(user), currentPHC, now.Add(-time.Minute))
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	pair, e := session.GeneratePair()
	if e != nil {
		t.Fatal(e)
	}
	sessionID := identityapp.ID(uuid.New())
	committer, _ := identitypersistence.NewLoginCommitter(control)
	e = committer.CommitLoginSession(context.Background(), identityapp.SessionCommit{SessionID: sessionID, AuditID: identityapp.ID(uuid.New()), UserID: user, CorrelationID: identityapp.ID(uuid.New()), SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, AuthenticationMethod: identityapp.AuthenticationMethodPassword, AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", CreatedAt: now, IdleExpiresAt: now.Add(30 * time.Minute), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuredAt: now})
	if e != nil {
		t.Fatal(e)
	}
	runner, e := attemptredis.NewClient(redisFixture.config)
	if e != nil {
		t.Fatal(e)
	}
	defer runner.Close()
	attempts, e := attemptredis.NewSecureController(runner, prefix)
	if e != nil {
		t.Fatal(e)
	}
	secret := sha256.Sum256([]byte(uuid.NewString()))
	deriver, e := attempt.NewDeriver(secret[:])
	if e != nil {
		t.Fatal(e)
	}
	reader, _ := identitypersistence.NewPasswordReader(control)
	auth, _ := identityapp.NewAuthenticator(reader, attempts, deriver, hasher, dummy)
	rehash, _ := identitypersistence.NewPasswordRehasher(control, hasher, identityClock{now})
	reauth, _ := identityapp.NewGoogleLinkReauthenticationService(auth, rehash)
	changeStore, _ := identitypersistence.NewPasswordChangeCommitter(control)
	change, _ := identityapp.NewPasswordChangeService(reauth, hasher, identityapp.SecureSessionTokenGenerator{}, identityIDs{}, identityClock{now.Add(time.Minute)}, changeStore)
	network, e := deriver.Network(netip.MustParseAddr("198.51.100.9"))
	if e != nil {
		t.Fatal(e)
	}
	changed, e := change.Change(context.Background(), identityapp.PasswordChangeInput{Principal: identityapp.SessionPrincipal{SessionID: sessionID, UserID: user, AuthEpoch: 1}, Credentials: identityapp.AuthenticateInput{Email: emailValue, Password: current, NetworkScope: network}, NewPassword: []byte("synthetic changed password"), CorrelationID: identityapp.ID(uuid.New())})
	if e != nil {
		t.Fatal(e)
	}
	if changed.Session.IsZero() {
		t.Fatal("replacement session missing")
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.sessions WHERE user_id=$1 AND revoked_at IS NULL AND auth_epoch=2`, uuid.UUID(user)); count != 1 {
		t.Fatal("password change replacement session drifted", count)
	}
	smtp := newPilotSMTPFixture(t, emailValue)
	sender, _ := email.Parse("noreply@example.invalid")
	delivery, e := deliverysmtp.New(deliverysmtp.Config{Host: "127.0.0.1", Port: uint16(smtp.listener.Addr().(*net.TCPAddr).Port), Username: "smtp-user", Password: "smtp-password", Sender: sender, TLS: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost", RootCAs: smtp.roots}, Timeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	recoveryStore, _ := identitypersistence.NewPasswordRecoveryStore(control)
	request, _ := identityapp.NewPasswordRecoveryRequestService(recoveryStore, delivery, identityapp.SecureVerificationTokenGenerator{}, identityIDs{}, identityClock{now.Add(2 * time.Minute)})
	if e = request.Request(context.Background(), emailValue, identityapp.ID(uuid.New())); e != nil {
		var pgErr *pgconn.PgError
		if errors.As(e, &pgErr) {
			t.Fatalf("password recovery issuance failed: sqlstate=%s table=%s column=%s constraint=%s routine=%s", pgErr.Code, pgErr.TableName, pgErr.ColumnName, pgErr.ConstraintName, pgErr.Routine)
		}
		t.Fatal(e)
	}
	var raw string
	select {
	case message := <-smtp.message:
		for _, field := range strings.Fields(message) {
			if _, x := verification.HashPresented(field); x == nil {
				raw = field
				break
			}
		}
	case <-time.After(time.Second):
		t.Fatal("recovery SMTP missing")
	}
	if raw == "" {
		t.Fatal("recovery token missing")
	}
	hash, _ := verification.HashPresented(raw)
	if count := rowCount(t, control, `SELECT count(*) FROM identity.password_recovery_challenges WHERE user_id=$1 AND token_hash=$2 AND consumed_at IS NULL`, uuid.UUID(user), hash[:]); count != 1 {
		t.Fatal("recovery hash persistence drifted", count)
	}
	reset, _ := identityapp.NewPasswordRecoveryResetService(recoveryStore, hasher, identityIDs{}, identityClock{now.Add(3 * time.Minute)})
	input := identityapp.PasswordRecoveryResetInput{Token: raw, NewPassword: []byte("synthetic recovered password"), CorrelationID: identityapp.ID(uuid.New())}
	if e = reset.Reset(context.Background(), input); e != nil {
		t.Fatal(e)
	}
	if e = reset.Reset(context.Background(), input); e == nil || e.Error() != "password_recovery_reset_invalid" {
		t.Fatal("recovery token replay accepted", e)
	}
	if count := rowCount(t, control, `SELECT count(*) FROM identity.sessions WHERE user_id=$1 AND revoked_at IS NULL`, uuid.UUID(user)); count != 0 {
		t.Fatal("recovery did not revoke sessions", count)
	}
	recoveryAttempts, e := recoveryredis.New(runner, prefix)
	if e != nil {
		t.Fatal(e)
	}
	scopes := liveScopes(uuid.NewString())
	for i := 0; i < 3; i++ {
		if e = recoveryAttempts.Allow(context.Background(), scopes); e != nil {
			t.Fatal(e)
		}
	}
	if e = recoveryAttempts.Allow(context.Background(), scopes); e == nil || e != identityapp.ErrRecoveryAttemptDenied {
		t.Fatal("fourth recovery identity attempt not denied", e)
	}
}
