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

type identityRow func(...any) error

func (f identityRow) Scan(dest ...any) error { return f(dest...) }

type identityDB struct {
	err                error
	sql                string
	args               []any
	verified, disabled pgtype.Timestamptz
}

func (d *identityDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}
func (d *identityDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *identityDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.sql = sql
	d.args = args
	if d.err != nil {
		return identityRow(func(...any) error { return d.err })
	}
	return identityRow(func(dest ...any) error {
		*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(7)), Valid: true}
		*(dest[1].(*string)) = "User@Example.com"
		*(dest[2].(*pgtype.Timestamptz)) = d.verified
		*(dest[3].(*pgtype.Timestamptz)) = d.disabled
		*(dest[4].(*int64)) = 5
		*(dest[5].(*string)) = "secret-phc"
		*(dest[6].(*int64)) = 3
		*(dest[7].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return nil
	})
}

type readRunner struct {
	db        postgres.DBTX
	calls     int
	committed bool
}

func (r *readRunner) WithinReadOnlyTransaction(ctx context.Context, fn func(context.Context, postgres.DBTX) error) error {
	r.calls++
	err := fn(ctx, r.db)
	r.committed = err == nil
	return err
}

func TestPasswordReaderMapsGeneratedLoginRecordInReadOnlyBoundary(t *testing.T) {
	verified := time.Date(2026, 10, 18, 9, 0, 0, 0, time.FixedZone("source", 3600))
	db := &identityDB{verified: pgtype.Timestamptz{Time: verified, Valid: true}}
	runner := &readRunner{db: db}
	reader, err := identitypersistence.NewPasswordReader(runner)
	if err != nil {
		t.Fatal(err)
	}
	record, err := reader.LookupPasswordCredential(context.Background(), "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if record.UserID != identityID(7) || record.PasswordPHC != "secret-phc" || record.AuthEpoch != 5 || record.CredentialVersion != 3 || record.EmailVerifiedAt == nil || !record.EmailVerifiedAt.Equal(verified.UTC()) || record.DisabledAt != nil || !runner.committed || runner.calls != 1 {
		t.Fatal("login record mapping drifted", record)
	}
	if !strings.Contains(db.sql, "GetPasswordLoginRecord") || len(db.args) != 1 || db.args[0] != "user@example.com" {
		t.Fatal("unexpected login query")
	}
}

func TestPasswordReaderMapsNotFoundAndDependencyFailures(t *testing.T) {
	for _, tc := range []struct {
		name         string
		db           postgres.DBTX
		wantNotFound bool
	}{{"not found", &identityDB{err: pgx.ErrNoRows}, true}, {"database", &identityDB{err: errors.New("secret-canary database diagnostic")}, false}, {"nil db", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &readRunner{db: tc.db}
			reader, _ := identitypersistence.NewPasswordReader(runner)
			record, err := reader.LookupPasswordCredential(context.Background(), "user@example.com")
			if err == nil || record != (identityapp.CredentialRecord{}) || runner.committed {
				t.Fatal("dependency failure accepted", record, err)
			}
			if errors.Is(err, identityapp.ErrCredentialNotFound) != tc.wantNotFound {
				t.Fatal("not-found mapping drifted", err)
			}
		})
	}
	if reader, err := identitypersistence.NewPasswordReader(nil); err == nil || reader != nil {
		t.Fatal("nil runner accepted")
	}
	runner := &readRunner{db: &identityDB{}}
	reader, _ := identitypersistence.NewPasswordReader(runner)
	if _, err := reader.LookupPasswordCredential(nil, "user@example.com"); err == nil || runner.calls != 0 {
		t.Fatal("nil context crossed transaction")
	}
	var zero *identitypersistence.PasswordReader
	if _, err := zero.LookupPasswordCredential(context.Background(), "user@example.com"); err == nil {
		t.Fatal("zero reader accepted")
	}
}
