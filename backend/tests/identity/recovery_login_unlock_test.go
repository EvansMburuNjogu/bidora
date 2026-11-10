package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	persistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type recoveryUnlockDB struct {
	rowErr, auditErr error
	noUpdate         bool
}

func (d *recoveryUnlockDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *recoveryUnlockDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	return identityRow(func(dest ...any) error {
		if d.rowErr != nil {
			return d.rowErr
		}
		if !strings.Contains(sql, "consumed_at IS NULL") || !strings.Contains(sql, "expires_at>") || !strings.Contains(sql, "email_verified_at IS NOT NULL") {
			return errors.New("recovery proof checks missing")
		}
		*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(1)), Valid: true}
		*(dest[1].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(2)), Valid: true}
		*(dest[2].(*int64)) = 2
		*(dest[3].(*int64)) = 3
		*(dest[4].(*string)) = "verified@example.test"
		return nil
	})
}
func (d *recoveryUnlockDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "InsertPasswordRecoveredAudit") && d.auditErr != nil {
		return pgconn.CommandTag{}, d.auditErr
	}
	if d.noUpdate {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

type recoveryUnlockRunner struct {
	db        *recoveryUnlockDB
	commitErr error
	committed bool
}

func (r *recoveryUnlockRunner) WithinTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	if err := fn(ctx, r.db); err != nil {
		return err
	}
	if r.commitErr != nil {
		return r.commitErr
	}
	r.committed = true
	return nil
}

type recoveryUnlockProbe struct {
	runner *recoveryUnlockRunner
	called int
	lookup string
	err    error
}

func (p *recoveryUnlockProbe) ClearRecoveryLoginLock(_ context.Context, lookup string) error {
	if !p.runner.committed {
		return errors.New("unlock before commit")
	}
	p.called++
	p.lookup = lookup
	return p.err
}
func TestRecoveryUnlockRunsOnlyAfterProvenCommittedPasswordReset(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		rowErr, commitErr, auditErr, unlockErr error
		wrongUser, noUpdate                    bool
		wantCalls                              int
	}{
		{name: "success", wantCalls: 1},
		{name: "invalid expired or replayed proof", rowErr: pgx.ErrNoRows},
		{name: "wrong target user", wrongUser: true},
		{name: "concurrent credential update", noUpdate: true},
		{name: "audit rollback", auditErr: errors.New("audit unavailable")},
		{name: "commit rollback", commitErr: errors.New("commit unavailable")},
		{name: "unlock unavailable is not success", unlockErr: errors.New("redis unavailable"), wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &recoveryUnlockRunner{db: &recoveryUnlockDB{rowErr: tc.rowErr, auditErr: tc.auditErr, noUpdate: tc.noUpdate}, commitErr: tc.commitErr}
			probe := &recoveryUnlockProbe{runner: runner, err: tc.unlockErr}
			store, err := persistence.NewPasswordRecoveryStore(runner, probe)
			if err != nil {
				t.Fatal(err)
			}
			c := app.PasswordRecoveryResetCommit{AuditID: identityID(3), CorrelationID: identityID(4), TokenHash: [32]byte{1}, PasswordPHC: "synthetic-phc", ResetAt: time.Now().UTC()}
			if tc.wrongUser {
				c.UserID = identityID(9)
			}
			err = store.CommitPasswordRecoveryReset(context.Background(), c)
			if (err == nil) != (tc.name == "success") {
				t.Fatal("unexpected reset outcome", err)
			}
			if probe.called != tc.wantCalls {
				t.Fatal("unlock ran without committed proof")
			}
			if probe.called > 0 && probe.lookup != "verified@example.test" {
				t.Fatal("unlock did not use database-bound identity")
			}
		})
	}
}
func TestRecoveryUnlockClearsOnlyOwnIdentityKeys(t *testing.T) {
	for _, result := range []scriptResult{{value: int64(2)}, {value: int64(0)}, {err: errors.New("redis unavailable")}, {value: int64(3)}} {
		runner := &scriptRunnerFake{results: []scriptResult{result}}
		controller, _ := attemptredis.NewController(runner, redisRandom(1), "bidora")
		key := attemptScopes().Identity
		err := controller.ClearRecoveredIdentity(context.Background(), key)
		if (err == nil) != (result.err == nil && (result.value == int64(0) || result.value == int64(2))) {
			t.Fatal("incorrect Redis failure handling")
		}
		if len(runner.calls) != 1 || len(runner.calls[0].keys) != 2 {
			t.Fatal("unexpected recovery scope")
		}
		want := fmt.Sprintf("%x", key[:])
		if runner.calls[0].keys[0] != "bidora:{login-attempt}:i:fail:"+want || runner.calls[0].keys[1] != "bidora:{login-attempt}:i:block:"+want {
			t.Fatal("network/other identity state affected")
		}
		if err = controller.ClearRecoveredIdentity(context.Background(), attempt.Key{}); err == nil || len(runner.calls) != 1 {
			t.Fatal("zero identity accepted")
		}
	}
}
