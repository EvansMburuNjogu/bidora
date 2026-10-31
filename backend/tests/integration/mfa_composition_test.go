package integration_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/mfa"
	"bidora/backend/internal/identity/mfaredis"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
)

type liveMFAClock struct{ at time.Time }

func (c *liveMFAClock) Now() time.Time { return c.at }

type liveMFAScopes struct{ deriver *attempt.Deriver }

func (s liveMFAScopes) DeriveMFAScope(p string, u, target identityapp.ID) ([32]byte, error) {
	k, e := s.deriver.MFA(p, [16]byte(u), [16]byte(target))
	return [32]byte(k), e
}

type liveMFAAttempts struct{ controller *mfaredis.Controller }

func (a liveMFAAttempts) BeginMFA(ctx context.Context, scope [32]byte) (identityapp.MFAAttemptReservation, error) {
	r, e := a.controller.Begin(ctx, scope)
	if errors.Is(e, mfaredis.ErrDenied) {
		return nil, identityapp.ErrMFAAttemptDenied
	}
	if e != nil {
		return nil, e
	}
	return liveMFAReservation{r}, nil
}

type liveMFAReservation struct{ inner mfaredis.Reservation }

func (r liveMFAReservation) Finish(ctx context.Context, success bool) error {
	e := r.inner.Finish(ctx, success)
	if errors.Is(e, mfaredis.ErrDenied) {
		return identityapp.ErrMFAAttemptDenied
	}
	return e
}
func liveTOTP(t *testing.T, encoded string, at time.Time) string {
	t.Helper()
	secret, e := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encoded)
	if e != nil {
		t.Fatal(e)
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(at.Unix()/30))
	h := hmac.New(sha1.New, secret)
	_, _ = h.Write(b[:])
	sum := h.Sum(nil)
	o := sum[len(sum)-1] & 15
	v := (uint32(sum[o])&127)<<24 | (uint32(sum[o+1]) << 16) | (uint32(sum[o+2]) << 8) | uint32(sum[o+3])
	return fmt.Sprintf("%06d", v%1_000_000)
}
func liveMFASessionID(t *testing.T, pool *postgres.Pool, token session.SessionToken) identityapp.ID {
	t.Helper()
	h := token.Hash()
	var id uuid.UUID
	e := pool.WithinReadOnlyTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		return db.QueryRow(ctx, `SELECT id FROM identity.sessions WHERE token_hash=$1`, h[:]).Scan(&id)
	})
	if e != nil {
		t.Fatal(e)
	}
	return identityapp.ID(id)
}
func mfaPrincipal(id, user identityapp.ID, epoch int64, level, method string, at time.Time) identityapp.SessionPrincipal {
	return identityapp.SessionPrincipal{SessionID: id, UserID: user, AuthEpoch: epoch, AssuranceLevel: level, AssuranceMethod: method, AssuredAt: at, AbsoluteExpiresAt: at.Add(12 * time.Hour)}
}

func TestLiveMFACompositionEnrollmentLoginStepUpReplacementAndReplayRaces(t *testing.T) {
	control := livePool(t, "BIDORA_TEST_CONTROL_RUNTIME_DSN")
	redisFixture := redisIntegration(t)
	prefix := "bidora:test:mfa:" + uuid.NewString()
	defer cleanupPrefix(t, redisFixture, prefix)
	runner, e := attemptredis.NewClient(redisFixture.config)
	if e != nil {
		t.Fatal(e)
	}
	defer runner.Close()
	limiter, e := mfaredis.New(runner, prefix)
	if e != nil {
		t.Fatal(e)
	}
	secretKey := [32]byte{}
	for i := range secretKey {
		secretKey[i] = byte(i + 1)
	}
	keyRing, e := mfa.NewKeyRing(1, map[int][32]byte{1: secretKey})
	if e != nil {
		t.Fatal(e)
	}
	hmacSecret := [32]byte{}
	for i := range hmacSecret {
		hmacSecret[i] = byte(200 - i)
	}
	deriver, e := attempt.NewDeriver(hmacSecret[:])
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC().Truncate(time.Second)
	clock := &liveMFAClock{at: now}
	store, e := identitypersistence.NewMFAStore(control)
	if e != nil {
		t.Fatal(e)
	}
	service, e := identityapp.NewMFAService(store, keyRing, identityapp.SecureSessionTokenGenerator{}, identityIDs{}, clock, liveMFAAttempts{limiter}, liveMFAScopes{deriver})
	if e != nil {
		t.Fatal(e)
	}
	user := identityapp.ID(uuid.New())
	emailValue := "mfa-live-" + uuid.NewString() + "@example.invalid"
	created := now.Add(-time.Minute)
	e = control.WithinTransaction(context.Background(), func(ctx context.Context, db postgres.DBTX) error {
		if _, x := db.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,disabled_at,auth_epoch,created_at,updated_at) VALUES($1,$2,$2,$3,NULL,1,$3,$3)`, uuid.UUID(user), emailValue, created); x != nil {
			return x
		}
		_, x := db.Exec(ctx, `INSERT INTO identity.authentication_identities(user_id,password_phc,credential_version,password_changed_at,created_at,updated_at) VALUES($1,'$argon2id$v=19$m=65536,t=3,p=2$c3ludGhldGljMTIzNDU2Nw$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',1,$2,$2,$2)`, uuid.UUID(user), created)
		return x
	})
	if e != nil {
		t.Fatal(e)
	}
	initialPair, e := session.GeneratePair()
	if e != nil {
		t.Fatal(e)
	}
	initialID := identityapp.ID(uuid.New())
	loginStore, _ := identitypersistence.NewLoginCommitter(control)
	e = loginStore.CommitLoginSession(context.Background(), identityapp.SessionCommit{SessionID: initialID, AuditID: identityapp.ID(uuid.New()), UserID: user, CorrelationID: identityapp.ID(uuid.New()), SessionHash: initialPair.Session.Hash(), CSRFHash: initialPair.CSRF.Hash(), AuthEpoch: 1, AuthenticationMethod: identityapp.AuthenticationMethodPassword, AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", CreatedAt: now, IdleExpiresAt: now.Add(30 * time.Minute), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuredAt: now})
	if e != nil {
		t.Fatal(e)
	}
	primary := mfaPrincipal(initialID, user, 1, "AAL1", "PRIMARY_ONLY", now)
	enrollment, e := service.Enroll(context.Background(), primary, identityapp.ID(uuid.New()))
	if e != nil {
		t.Fatal(e)
	}
	activation, e := service.Confirm(context.Background(), primary, identityapp.ID(uuid.New()), liveTOTP(t, enrollment.Secret, clock.at))
	if e != nil {
		t.Fatal(e)
	}
	if len(activation.RecoveryCodes) != 10 {
		t.Fatal("activation recovery count drifted")
	}
	activeID := liveMFASessionID(t, control, activation.Login.Session)
	if activeID.IsZero() {
		t.Fatal("activation replacement session missing")
	}
	clock.at = clock.at.Add(30 * time.Second)
	gate, e := service.BeginLogin(context.Background(), user, 2, identityapp.AuthenticationMethodPassword)
	if e != nil || !gate.Required {
		t.Fatal("MFA login gate missing", e)
	}
	login, e := service.CompleteLogin(context.Background(), gate.Token, liveTOTP(t, enrollment.Secret, clock.at), "TOTP", identityapp.ID(uuid.New()))
	if e != nil {
		t.Fatal(e)
	}
	loginID := liveMFASessionID(t, control, login.Session)
	clock.at = clock.at.Add(30 * time.Second)
	stepped, e := service.StepUp(context.Background(), mfaPrincipal(loginID, user, 2, "AAL1", "PRIMARY_ONLY", clock.at), identityapp.ID(uuid.New()), liveTOTP(t, enrollment.Secret, clock.at), "TOTP")
	if e != nil {
		t.Fatal(e)
	}
	stepID := liveMFASessionID(t, control, stepped.Session)
	clock.at = clock.at.Add(time.Second)
	recovered, e := service.StepUp(context.Background(), mfaPrincipal(stepID, user, 2, "AAL2", "TOTP", clock.at.Add(-time.Second)), identityapp.ID(uuid.New()), activation.RecoveryCodes[0], "RECOVERY_CODE")
	if e != nil {
		t.Fatal(e)
	}
	recoveryID := liveMFASessionID(t, control, recovered.Session)
	if _, e = service.StepUp(context.Background(), mfaPrincipal(recoveryID, user, 2, "AAL2", "RECOVERY_CODE", clock.at), identityapp.ID(uuid.New()), activation.RecoveryCodes[0], "RECOVERY_CODE"); e == nil {
		t.Fatal("recovery code replay accepted")
	}
	codes, e := service.RegenerateRecoveryCodes(context.Background(), mfaPrincipal(recoveryID, user, 2, "AAL2", "RECOVERY_CODE", clock.at), identityapp.ID(uuid.New()))
	if e != nil || len(codes) != 10 {
		t.Fatal("recovery regeneration failed", e)
	}
	replacement, e := service.BeginReplacement(context.Background(), mfaPrincipal(recoveryID, user, 2, "AAL2", "RECOVERY_CODE", clock.at), identityapp.ID(uuid.New()))
	if e != nil {
		t.Fatal(e)
	}
	clock.at = clock.at.Add(30 * time.Second)
	replaced, e := service.ConfirmReplacement(context.Background(), mfaPrincipal(recoveryID, user, 2, "AAL2", "RECOVERY_CODE", clock.at.Add(-30*time.Second)), identityapp.ID(uuid.New()), liveTOTP(t, replacement.Secret, clock.at))
	if e != nil {
		t.Fatal(e)
	}
	replacementID := liveMFASessionID(t, control, replaced.Login.Session)
	if count := rowCount(t, control, `SELECT count(*) FROM identity.sessions WHERE user_id=$1 AND revoked_at IS NULL AND auth_epoch=3`, uuid.UUID(user)); count != 1 {
		t.Fatal("replacement session rotation drifted", count)
	}
	type raceResult struct {
		login identityapp.LoginResult
		err   error
	}
	runStepUpRace := func(principal identityapp.SessionPrincipal, code, kind string) identityapp.LoginResult {
		var wg sync.WaitGroup
		wg.Add(2)
		results := make(chan raceResult, 2)
		for range 2 {
			go func() {
				defer wg.Done()
				login, raceErr := service.StepUp(context.Background(), principal, identityapp.ID(uuid.New()), code, kind)
				results <- raceResult{login: login, err: raceErr}
			}()
		}
		wg.Wait()
		close(results)
		success := 0
		var winner identityapp.LoginResult
		for result := range results {
			if result.err == nil {
				success++
				winner = result.login
			}
		}
		if success != 1 {
			t.Fatalf("concurrent %s replay winners: %d", kind, success)
		}
		return winner
	}
	clock.at = clock.at.Add(time.Second)
	recoveryWinner := runStepUpRace(mfaPrincipal(replacementID, user, 3, "AAL2", "TOTP", clock.at.Add(-time.Second)), replaced.RecoveryCodes[0], "RECOVERY_CODE")
	recoveryWinnerID := liveMFASessionID(t, control, recoveryWinner.Session)
	clock.at = clock.at.Add(30 * time.Second)
	totpWinner := runStepUpRace(mfaPrincipal(recoveryWinnerID, user, 3, "AAL2", "RECOVERY_CODE", clock.at.Add(-30*time.Second)), liveTOTP(t, replacement.Secret, clock.at), "TOTP")
	if liveMFASessionID(t, control, totpWinner.Session).IsZero() {
		t.Fatal("concurrent TOTP winner session missing")
	}
	clock.at = clock.at.Add(30 * time.Second)
	loginGate, e := service.BeginLogin(context.Background(), user, 3, identityapp.AuthenticationMethodPassword)
	if e != nil || !loginGate.Required {
		t.Fatal("replacement login gate missing", e)
	}
	loginResults := make(chan error, 2)
	var loginWG sync.WaitGroup
	loginWG.Add(2)
	loginCode := liveTOTP(t, replacement.Secret, clock.at)
	for range 2 {
		go func() {
			defer loginWG.Done()
			_, raceErr := service.CompleteLogin(context.Background(), loginGate.Token, loginCode, "TOTP", identityapp.ID(uuid.New()))
			loginResults <- raceErr
		}()
	}
	loginWG.Wait()
	close(loginResults)
	loginSuccess := 0
	for raceErr := range loginResults {
		if raceErr == nil {
			loginSuccess++
		}
	}
	if loginSuccess != 1 {
		t.Fatal("concurrent MFA login transaction winners", loginSuccess)
	}
}
