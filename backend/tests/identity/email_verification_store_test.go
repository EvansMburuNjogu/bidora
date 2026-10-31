package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type verificationDB struct {
	rowErr, auditErr error
	verified         time.Time
	calls            []string
	args             [][]any
}

func (d *verificationDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.calls = append(d.calls, sql)
	d.args = append(d.args, args)
	if strings.Contains(sql, "InsertEmailVerifiedAudit") && d.auditErr != nil {
		return pgconn.CommandTag{}, d.auditErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (d *verificationDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *verificationDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.calls = append(d.calls, sql)
	d.args = append(d.args, args)
	if d.rowErr != nil {
		return identityRow(func(...any) error { return d.rowErr })
	}
	return identityRow(func(dest ...any) error {
		*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(3)), Valid: true}
		*(dest[1].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: d.verified, Valid: true}
		*(dest[2].(*int64)) = 2
		return nil
	})
}
func validConsumeArgs() (hash [32]byte, audit, correlation identityapp.ID, at time.Time) {
	hash[31] = 9
	return hash, identityID(4), identityID(5), time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
}
func TestEmailVerificationCommitterConsumesThenAuditsInOneTransaction(t *testing.T) {
	hash, audit, correlation, at := validConsumeArgs()
	db := &verificationDB{verified: at}
	runner := &writeRunnerFake{db: db}
	committer, err := identitypersistence.NewEmailVerificationCommitter(runner)
	if err != nil {
		t.Fatal(err)
	}
	result, err := committer.ConsumeEmailVerification(context.Background(), hash, audit, correlation, at)
	if err != nil {
		t.Fatal(err)
	}
	if !runner.committed || len(db.calls) != 2 || !strings.Contains(db.calls[0], "ConsumeEmailVerificationChallenge") || !strings.Contains(db.calls[1], "InsertEmailVerifiedAudit") || result.UserID != identityID(3) || result.AuthEpoch != 2 || !result.VerifiedAt.Equal(at) {
		t.Fatal("verification persistence drifted", result)
	}
	if got := db.args[0][1].([]byte); len(got) != 32 || got[31] != 9 {
		t.Fatal("token hash mapping drifted")
	}
	if len(db.args[1]) != 4 {
		t.Fatal("audit gained unreviewed payload")
	}
}
func TestEmailVerificationCommitterRollsBackMissingReplayAndAuditFailure(t *testing.T) {
	hash, audit, correlation, at := validConsumeArgs()
	cause := errors.New("secret-canary audit database")
	for _, tc := range []struct {
		name     string
		db       postgres.DBTX
		notFound bool
	}{{"missing", &verificationDB{rowErr: pgx.ErrNoRows}, true}, {"consume database", &verificationDB{rowErr: cause}, false}, {"audit", &verificationDB{verified: at, auditErr: cause}, false}, {"nil db", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &writeRunnerFake{db: tc.db}
			committer, _ := identitypersistence.NewEmailVerificationCommitter(runner)
			result, err := committer.ConsumeEmailVerification(context.Background(), hash, audit, correlation, at)
			if err == nil || result != (identityapp.EmailVerificationResult{}) || runner.committed || errors.Is(err, identityapp.ErrVerificationNotFound) != tc.notFound {
				t.Fatal("verification failure committed/leaked", result, err)
			}
		})
	}
	if committer, err := identitypersistence.NewEmailVerificationCommitter(nil); err == nil || committer != nil {
		t.Fatal("nil runner accepted")
	}
	runner := &writeRunnerFake{db: &verificationDB{}}
	committer, _ := identitypersistence.NewEmailVerificationCommitter(runner)
	if _, err := committer.ConsumeEmailVerification(nil, hash, audit, correlation, at); err == nil || runner.calls != 0 {
		t.Fatal("nil context crossed transaction")
	}
	var zero *identitypersistence.EmailVerificationCommitter
	if _, err := zero.ConsumeEmailVerification(context.Background(), hash, audit, correlation, at); err == nil {
		t.Fatal("zero committer accepted")
	}
}
