package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type registrationDB struct {
	calls  []string
	args   [][]any
	failAt int
	err    error
}

func (d *registrationDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	d.calls = append(d.calls, sql)
	d.args = append(d.args, args)
	if len(d.calls) == d.failAt {
		return pgconn.CommandTag{}, d.err
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (d *registrationDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *registrationDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return identityRow(func(...any) error { return errors.New("unexpected query row") })
}
func registrationCommit(t *testing.T) identityapp.SelfRegistrationCommit {
	t.Helper()
	address, err := email.Parse("User@example.test")
	if err != nil {
		t.Fatal(err)
	}
	return identityapp.SelfRegistrationCommit{UserID: identityID(1), AuditID: identityID(2), CorrelationID: identityID(3), Email: address, FirstName: "Amina", LastName: "Njoroge", PasswordPHC: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA", CreatedAt: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
}
func TestSelfRegistrationCommitterPersistsUserPasswordAndAuditAtomically(t *testing.T) {
	db := &registrationDB{}
	runner := &writeRunnerFake{db: db}
	committer, err := identitypersistence.NewSelfRegistrationCommitter(runner)
	if err != nil {
		t.Fatal(err)
	}
	if err = committer.CommitSelfRegistration(context.Background(), registrationCommit(t)); err != nil {
		t.Fatal(err)
	}
	if !runner.committed || len(db.calls) != 3 {
		t.Fatal("registration transaction drifted", len(db.calls))
	}
	for index, name := range []string{"InsertPasswordUser", "InsertPasswordIdentity", "InsertUserRegisteredAudit"} {
		if !strings.Contains(db.calls[index], name) {
			t.Fatal("registration ordering drifted", index)
		}
	}
	if len(db.args[2]) != 4 {
		t.Fatal("registration audit gained unreviewed payload")
	}
}
func TestSelfRegistrationCommitterConcealsOnlyEmailLookupConflict(t *testing.T) {
	commit := registrationCommit(t)
	duplicate := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_lookup_key"}
	for _, tc := range []struct {
		name     string
		db       postgres.DBTX
		existing bool
	}{{"existing", &registrationDB{failAt: 1, err: duplicate}, true}, {"id collision", &registrationDB{failAt: 1, err: &pgconn.PgError{Code: "23505", ConstraintName: "users_pkey"}}, false}, {"password", &registrationDB{failAt: 2, err: errors.New("secret-canary password database")}, false}, {"audit", &registrationDB{failAt: 3, err: errors.New("secret-canary audit database")}, false}, {"nil db", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &writeRunnerFake{db: tc.db}
			committer, _ := identitypersistence.NewSelfRegistrationCommitter(runner)
			err := committer.CommitSelfRegistration(context.Background(), commit)
			if err == nil || runner.committed || errors.Is(err, identityapp.ErrIdentityAlreadyExists) != tc.existing {
				t.Fatal("registration failure mapping drifted", err)
			}
		})
	}
	if committer, err := identitypersistence.NewSelfRegistrationCommitter(nil); err == nil || committer != nil {
		t.Fatal("nil runner accepted")
	}
	runner := &writeRunnerFake{db: &registrationDB{}}
	committer, _ := identitypersistence.NewSelfRegistrationCommitter(runner)
	if err := committer.CommitSelfRegistration(nil, commit); err == nil || runner.calls != 0 {
		t.Fatal("nil context crossed transaction")
	}
	var zero *identitypersistence.SelfRegistrationCommitter
	if err := zero.CommitSelfRegistration(context.Background(), commit); err == nil {
		t.Fatal("zero committer accepted")
	}
}
