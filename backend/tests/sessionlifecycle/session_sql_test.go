// Package sessionlifecycle_test runs real session SQL only against an explicitly
// named disposable database. It never consumes an existing user's token or email.
package sessionlifecycle_test

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	identitypersistence "bidora/backend/internal/identity/persistence"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/securitypolicy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func dsn(t *testing.T) string {
	t.Helper()
	s := os.Getenv("BIDORA_SESSION_AUDIT_DSN")
	if s == "" {
		t.Skip("requires dedicated disposable BIDORA_SESSION_AUDIT_DSN")
	}
	u, e := url.Parse(s)
	if e != nil || !strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "bidora_auth_audit_") {
		t.Fatal("session integration tests refuse non-disposable database names")
	}
	return s
}
func tm(at time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: at.UTC(), Valid: true} }
func uid(id uuid.UUID) pgtype.UUID       { return pgtype.UUID{Bytes: [16]byte(id), Valid: true} }

type fixture struct {
	user, session uuid.UUID
	hash, csrf    []byte
	created       time.Time
}

// Keep seed operations on the caller's transaction/connection and never deliver mail.
func seed(t *testing.T, tx pgx.Tx, admin bool, at time.Time, idle, absolute int32) fixture {
	t.Helper()
	f := fixture{user: uuid.New(), session: uuid.New(), hash: make([]byte, 32), csrf: make([]byte, 32), created: at}
	_, _ = rand.Read(f.hash)
	_, _ = rand.Read(f.csrf)
	kind := "TENDERPRENEUR"
	level := "AAL1"
	if admin {
		kind = "PLATFORM_ADMIN"
		level = "AAL2"
	}
	ctx := context.Background()
	mail := "session-audit-" + f.user.String() + "@example.invalid"
	if _, e := tx.Exec(ctx, `INSERT INTO identity.users(id,email,email_lookup,email_verified_at,account_type,created_at,updated_at) VALUES($1,$2,$2,$3,$4,$3,$3)`, f.user, mail, at, kind); e != nil {
		t.Fatal(e)
	}
	if _, e := tx.Exec(ctx, `INSERT INTO identity.sessions(id,user_id,token_hash,csrf_hash,auth_epoch,created_at,last_seen_at,idle_expires_at,absolute_expires_at,assurance_level,assurance_method,assured_at,idle_timeout_seconds,absolute_timeout_seconds) VALUES($1,$2,$3,$4,1,$5,$5,$6,$7,$8,'EMAIL_OTP',$5,$9,$10)`, f.session, f.user, f.hash, f.csrf, at, at.Add(time.Duration(idle)*time.Second), at.Add(time.Duration(absolute)*time.Second), level, idle, absolute); e != nil {
		t.Fatal(e)
	}
	return f
}
func transaction(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	ctx := context.Background()
	c, e := pgx.Connect(ctx, dsn(t))
	if e != nil {
		t.Fatal("private test database unavailable")
	}
	tx, e := c.Begin(ctx)
	if e != nil {
		c.Close(ctx)
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx); _ = c.Close(ctx) })
	return ctx, tx
}
func touch(f fixture, at time.Time) identitysql.TouchActiveSessionParams {
	return identitysql.TouchActiveSessionParams{ID: uid(f.session), ObservedAt: tm(at), PolicyIdleSeconds: 1800, PolicyAbsoluteSeconds: 43200, AdminIdleSeconds: 604800, AdminAbsoluteSeconds: 604800}
}
func get(f fixture, at time.Time) identitysql.GetActiveSessionParams {
	return identitysql.GetActiveSessionParams{TokenHash: f.hash, ObservedAt: tm(at), PolicyIdleSeconds: 1800, PolicyAbsoluteSeconds: 43200, AdminIdleSeconds: 604800, AdminAbsoluteSeconds: 604800}
}
func TestOutOfOrderRenewalIsMonotonicAndNeverReverifiesEmail(t *testing.T) {
	ctx, tx := transaction(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	f := seed(t, tx, true, at, 604800, 604800)
	q := identitysql.New(tx)
	for _, when := range []time.Time{at.Add(2 * time.Second), at.Add(time.Second)} {
		n, e := q.TouchActiveSession(ctx, touch(f, when))
		if e != nil || n != 1 {
			t.Fatal("out-of-order valid activity was falsely logged out", n, e)
		}
	}
	var last, assured, verified, absolute time.Time
	var hash []byte
	if e := tx.QueryRow(ctx, `SELECT s.last_seen_at,s.assured_at,u.email_verified_at,s.absolute_expires_at,s.csrf_hash FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id WHERE s.id=$1`, f.session).Scan(&last, &assured, &verified, &absolute, &hash); e != nil {
		t.Fatal(e)
	}
	if !last.Equal(at.Add(2*time.Second)) || !assured.Equal(at) || !verified.Equal(at) || !absolute.Equal(at.Add(7*24*time.Hour)) || string(hash) != string(f.csrf) {
		t.Fatal("renewal reset assurance, verified email, CSRF or absolute lifetime")
	}
}
func TestConcurrentRenewalCannotInvalidateOrRotateOneSession(t *testing.T) {
	ctx, tx := transaction(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	f := seed(t, tx, true, at, 604800, 604800)
	if e := tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	pool, e := pgxpool.New(ctx, dsn(t))
	if e != nil {
		t.Fatal("private test pool unavailable")
	}
	defer pool.Close()
	q := identitysql.New(pool)
	var group sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			n, e := q.TouchActiveSession(ctx, touch(f, at.Add(time.Duration(16-i)*time.Millisecond)))
			if e != nil {
				errs <- e
			} else if n != 1 {
				errs <- errors.New("concurrent renewal falsely invalidated the session")
			}
		}(i)
	}
	group.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	var count int
	var last time.Time
	if e = pool.QueryRow(ctx, `SELECT count(*),max(last_seen_at) FROM identity.sessions WHERE user_id=$1 AND revoked_at IS NULL`, f.user).Scan(&count, &last); e != nil || count != 1 || !last.Equal(at.Add(16*time.Millisecond)) {
		t.Fatal("concurrent renewal was not monotonic/single-session", count, e)
	}
}
func TestPolicyChangesKeepValidUsersAndDoNotResurrectExpiredSessions(t *testing.T) {
	ctx, tx := transaction(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	one := seed(t, tx, true, at, 604800, 604800)
	two := seed(t, tx, false, at, 1800, 43200)
	expired := seed(t, tx, false, at.Add(-20*time.Minute), 1800, 43200)
	revoker, e := identitypersistence.NewSessionPolicyRevokerTx(tx)
	if e != nil {
		t.Fatal(e)
	}
	p := securitypolicy.Default()
	p.IdleSeconds = 300
	p.AbsoluteSeconds = 3600
	p.AdminIdleSeconds = 300
	p.AdminAbsoluteSeconds = 3600
	n, e := revoker.ApplySessionPolicy(ctx, [16]byte(one.session), p, at.Add(time.Second))
	if e != nil || n < 1 {
		t.Fatal("policy tightening did not revoke the genuinely expired session", n, e)
	}
	q := identitysql.New(tx)
	for _, f := range []fixture{one, two} {
		if _, e = q.GetActiveSession(ctx, get(f, at.Add(time.Second))); e != nil {
			t.Fatal("saving policy logged out an unrelated valid user", e)
		}
	}
	if _, e = q.GetActiveSession(ctx, get(expired, at)); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("expired session survived policy tightening", e)
	}
	p = securitypolicy.Default()
	if _, e = revoker.ApplySessionPolicy(ctx, [16]byte(one.session), p, at.Add(2*time.Second)); e != nil {
		t.Fatal(e)
	}
	if _, e = q.GetActiveSession(ctx, get(expired, at)); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("policy relaxation resurrected an expired session")
	}
	var seconds int32
	if e = tx.QueryRow(ctx, `SELECT idle_timeout_seconds FROM identity.sessions WHERE id=$1`, one.session).Scan(&seconds); e != nil || seconds != 300 {
		t.Fatal("relaxation extended an existing bound", seconds, e)
	}
}
func TestInvalidRevokedEpochChangedIdleAndAbsoluteExpiredSessionsFailClosed(t *testing.T) {
	ctx, tx := transaction(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	q := identitysql.New(tx)
	for _, reason := range []string{"unknown", "revoked", "epoch", "idle", "absolute"} {
		t.Run(reason, func(t *testing.T) {
			f := seed(t, tx, false, at, 300, 3600)
			when := at.Add(time.Second)
			switch reason {
			case "unknown":
				f.hash = make([]byte, 32)
			case "revoked":
				_, _ = tx.Exec(ctx, `UPDATE identity.sessions SET revoked_at=$2 WHERE id=$1`, f.session, at)
			case "epoch":
				_, _ = tx.Exec(ctx, `UPDATE identity.users SET auth_epoch=auth_epoch+1 WHERE id=$1`, f.user)
			case "idle":
				when = at.Add(300 * time.Second)
			case "absolute":
				when = at.Add(3600 * time.Second)
			}
			if _, e := q.GetActiveSession(ctx, get(f, when)); !errors.Is(e, pgx.ErrNoRows) {
				t.Fatal("invalid session accepted", reason, e)
			}
			if reason == "unknown" {
				return
			} // Validation rejects the token before any ID-based touch can run.
			n, e := q.TouchActiveSession(ctx, touch(f, when))
			if e != nil || n != 0 {
				t.Fatal("invalid session renewed or resurrected", reason, n, e)
			}
		})
	}
}
func TestSingleDevicePolicyRevokesOtherDevicesNotRenewals(t *testing.T) {
	ctx, tx := transaction(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	f := seed(t, tx, true, at, 604800, 604800)
	if _, e := tx.Exec(ctx, `UPDATE platform_settings.session_policy SET allow_multiple_sessions=false WHERE id=true`); e != nil {
		t.Fatal(e)
	}
	newer := uuid.New()
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	if _, e := tx.Exec(ctx, `INSERT INTO identity.sessions(id,user_id,token_hash,csrf_hash,auth_epoch,created_at,last_seen_at,idle_expires_at,absolute_expires_at,assurance_level,assurance_method,assured_at,idle_timeout_seconds,absolute_timeout_seconds) VALUES($1,$2,$3,$4,1,$5,$5,$6,$6,'AAL2','EMAIL_OTP',$5,604800,604800)`, newer, f.user, hash, f.csrf, at.Add(time.Second), at.Add(time.Second+7*24*time.Hour)); e != nil {
		t.Fatal(e)
	}
	q := identitysql.New(tx)
	if _, e := q.GetActiveSession(ctx, get(f, at.Add(2*time.Second))); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("new device did not revoke older session")
	}
	f.session = newer
	f.hash = hash
	for i := 0; i < 2; i++ {
		if n, e := q.TouchActiveSession(ctx, touch(f, at.Add(time.Duration(i+2)*time.Second))); e != nil || n != 1 {
			t.Fatal("lease renewal was mistaken for another device login", e)
		}
	}
}
