package integration_test

import (
	"context"
	"crypto/sha256"
	"net/netip"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/password"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type identityIDs struct{}

func (identityIDs) NewID() (identityapp.ID, error) { return identityapp.ID(uuid.New()), nil }

type identityClock struct{ at time.Time }

func (c identityClock) Now() time.Time         { return c.at }
func identityUUID(id identityapp.ID) uuid.UUID { return uuid.UUID(id) }

func TestLiveLoginSessionTouchAndLogout(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	observer := livePool(t, "BIDORA_TEST_CONTROL_MIGRATOR_DSN")
	redisFixture := redisIntegration(t)
	prefix := "bidora:test:" + uuid.NewString()
	defer cleanupPrefix(t, redisFixture, prefix)
	attempts, _ := liveController(t, redisFixture, prefix)
	currentHasher, err := password.New(password.PilotPolicy())
	if err != nil {
		t.Fatal(err)
	}
	stalePolicy := password.PilotPolicy()
	stalePolicy.MemoryKiB = 32 * 1024
	stalePolicy.Iterations = 2
	staleHasher, err := password.New(stalePolicy)
	if err != nil {
		t.Fatal(err)
	}
	passwordValue := []byte("synthetic password value")
	stalePHC, err := staleHasher.Hash(passwordValue)
	if err != nil {
		t.Fatal(err)
	}
	dummyPHC, err := currentHasher.Hash([]byte("synthetic dummy value"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := identityapp.ID(uuid.New())
	emailValue := "pilot-" + uuid.NewString() + "@example.invalid"
	err = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, err := db.Exec(ctx, `INSERT INTO identity.users (id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES ($1,$2,$3,$4,NULL,1,$5,$5)`, identityUUID(userID), emailValue, emailValue, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
			return err
		}
		_, err := db.Exec(ctx, `INSERT INTO identity.authentication_identities (user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES ($1,$2,1,$3,$3,$3)`, identityUUID(userID), stalePHC, now.Add(-time.Minute))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	deriver, err := attempt.NewDeriver([]byte("short"))
	if err == nil {
		t.Fatal("short attempt secret accepted")
	}
	secret := sha256.Sum256([]byte(uuid.NewString()))
	deriver, err = attempt.NewDeriver(secret[:])
	if err != nil {
		t.Fatal(err)
	}
	networkScope, err := deriver.Network(netip.MustParseAddr("198.51.100.7"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := identitypersistence.NewPasswordReader(control)
	if err != nil {
		t.Fatal(err)
	}
	authenticator, err := identityapp.NewAuthenticator(reader, attempts, deriver, currentHasher, dummyPHC)
	if err != nil {
		t.Fatal(err)
	}
	rehasher, err := identitypersistence.NewPasswordRehasher(control, currentHasher, identityClock{now})
	if err != nil {
		t.Fatal(err)
	}
	committer, err := identitypersistence.NewLoginCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	login, err := identityapp.NewLoginService(authenticator, rehasher, identityapp.SecureSessionTokenGenerator{}, identityIDs{}, identityClock{now}, committer)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := login.Login(context.Background(), identityapp.LoginInput{Credentials: identityapp.AuthenticateInput{Email: emailValue, Password: []byte("wrong synthetic password"), NetworkScope: networkScope}, CorrelationID: identityapp.ID(uuid.New())}); err == nil || err.Error() != "credentials_invalid" || result != (identityapp.LoginResult{}) {
		t.Fatal("live wrong-password result unsafe", err)
	}
	result, err := login.Login(context.Background(), identityapp.LoginInput{Credentials: identityapp.AuthenticateInput{Email: emailValue, Password: passwordValue, NetworkScope: networkScope}, CorrelationID: identityapp.ID(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.IsZero() || result.CSRF.IsZero() || !result.IdleExpiresAt.Equal(now.Add(30*time.Minute)) || !result.AbsoluteExpiresAt.Equal(now.Add(12*time.Hour)) {
		t.Fatal("live login result drifted")
	}
	var credentialVersion int64
	var persistedPHC string
	if err = control.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT credential_version,password_phc FROM identity.authentication_identities WHERE user_id=$1`, identityUUID(userID)).Scan(&credentialVersion, &persistedPHC)
	}); err != nil {
		t.Fatal(err)
	}
	verification, err := currentHasher.Verify(passwordValue, persistedPHC)
	if err != nil || !verification.Match || verification.NeedsRehash || credentialVersion != 2 {
		t.Fatal("live password rehash not persisted", credentialVersion, err)
	}
	sessionHash := result.Session.Hash()
	var sessionID identityapp.ID
	if err = control.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		var raw uuid.UUID
		err := db.QueryRow(ctx, `SELECT id FROM identity.sessions WHERE token_hash=$1`, sessionHash[:]).Scan(&raw)
		sessionID = identityapp.ID(raw)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if sessionID.IsZero() {
		t.Fatal("live session missing")
	}
	if rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$1 AND action='UserLoggedIn'`, uuid.UUID(sessionID)) != 1 {
		t.Fatal("live login audit missing")
	}
	sessionReader, err := identitypersistence.NewSessionReader(control)
	if err != nil {
		t.Fatal(err)
	}
	validator, err := identityapp.NewSessionValidator(sessionReader, identityClock{now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := validator.Validate(context.Background(), result.Session.RevealForCookie())
	if err != nil {
		t.Fatal(err)
	}
	if principal.SessionID != sessionID || principal.UserID != userID || principal.CSRFHash != result.CSRF.Hash() {
		t.Fatal("live session principal drifted")
	}
	toucher, err := identitypersistence.NewSessionToucher(control)
	if err != nil {
		t.Fatal(err)
	}
	touchedAt := now.Add(2 * time.Minute)
	if err = toucher.TouchSession(context.Background(), sessionID, touchedAt); err != nil {
		t.Fatal(err)
	}
	var lastSeen time.Time
	if err = control.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT last_seen_at FROM identity.sessions WHERE id=$1`, uuid.UUID(sessionID)).Scan(&lastSeen)
	}); err != nil || !lastSeen.Equal(touchedAt) {
		t.Fatal("live session touch missing", lastSeen, err)
	}
	logoutCommitter, err := identitypersistence.NewLogoutCommitter(control)
	if err != nil {
		t.Fatal(err)
	}
	logout, err := identityapp.NewLogoutService(logoutCommitter, identityIDs{}, identityClock{now.Add(3 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err = logout.Logout(context.Background(), principal, identityapp.ID(uuid.New())); err != nil {
		t.Fatal(err)
	}
	if rowCount(t, observer, `SELECT count(*) FROM bidora_control_audit.audit_events WHERE target_id=$1 AND action='UserLoggedOut'`, uuid.UUID(sessionID)) != 1 {
		t.Fatal("live logout audit missing")
	}
	validator, err = identityapp.NewSessionValidator(sessionReader, identityClock{now.Add(4 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if principal, err = validator.Validate(context.Background(), result.Session.RevealForCookie()); err == nil || err.Error() != "session_invalid" || principal != (identityapp.SessionPrincipal{}) {
		t.Fatal("revoked live session remained valid", err)
	}
}
