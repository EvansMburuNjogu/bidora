package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/googleoidc"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type googleDB struct {
	existing    bool
	emailExists bool
	disabled    bool
	execErr     error
	calls       []string
}

func (d *googleDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	d.calls = append(d.calls, sql)
	if d.execErr != nil {
		return pgconn.CommandTag{}, d.execErr
	}
	if strings.Contains(sql, "TouchGoogleIdentity") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (d *googleDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *googleDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	d.calls = append(d.calls, sql)
	if strings.Contains(sql, "GetGoogleIdentityForUpdate") {
		return identityRow(func(dest ...any) error {
			if !d.existing {
				return pgx.ErrNoRows
			}
			*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(7)), Valid: true}
			*(dest[1].(*int64)) = 1
			if d.disabled {
				*(dest[2].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			}
			*(dest[3].(*string)) = "stored@example.test"
			*(dest[4].(*string)) = "original@example.test"
			return nil
		})
	}
	return identityRow(func(dest ...any) error {
		if !d.emailExists {
			return pgx.ErrNoRows
		}
		*(dest[0].(*pgtype.UUID)) = pgtype.UUID{Bytes: [16]byte(identityID(8)), Valid: true}
		return nil
	})
}
func googleCommit(t *testing.T) identityapp.GoogleIdentityCommit {
	t.Helper()
	address, err := email.Parse("GoogleUser@example.test")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	return identityapp.GoogleIdentityCommit{UserID: identityID(1), ExternalIdentityID: identityID(2), AuditID: identityID(3), CorrelationID: identityID(4), Identity: googleoidc.Identity{Subject: "google-subject", Email: address, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}, AuthenticatedAt: now}
}
func TestGoogleIdentityResolverUsesSubjectAndDoesNotOverwriteEmail(t *testing.T) {
	db := &googleDB{existing: true}
	runner := &writeRunnerFake{db: db}
	resolver, err := identitypersistence.NewGoogleIdentityResolver(runner)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.ResolveGoogleIdentity(context.Background(), googleCommit(t))
	if err != nil {
		t.Fatal(err)
	}
	if !runner.committed || result.UserID != identityID(7) || result.AuthEpoch != 1 || result.Created || len(db.calls) != 2 || !strings.Contains(db.calls[1], "TouchGoogleIdentity") {
		t.Fatal("existing Google subject resolution drifted", result, db.calls)
	}
}
func TestGoogleIdentityResolverCreatesOnlyWhenEmailIsUnused(t *testing.T) {
	db := &googleDB{}
	runner := &writeRunnerFake{db: db}
	resolver, _ := identitypersistence.NewGoogleIdentityResolver(runner)
	result, err := resolver.ResolveGoogleIdentity(context.Background(), googleCommit(t))
	if err != nil {
		t.Fatal(err)
	}
	if !runner.committed || !result.Created || result.UserID != identityID(1) || len(db.calls) != 5 {
		t.Fatal("Google signup persistence drifted", result, db.calls)
	}
	for index, name := range []string{"GetGoogleIdentityForUpdate", "GetUserByEmailLookupForGoogleLink", "InsertGoogleUser", "InsertGoogleIdentity", "InsertGoogleUserRegisteredAudit"} {
		if !strings.Contains(db.calls[index], name) {
			t.Fatal("Google signup ordering drifted", index)
		}
	}
}
func TestGoogleIdentityResolverBlocksEmailOnlyLinkAndDisabledSubject(t *testing.T) {
	for _, tc := range []struct {
		name string
		db   *googleDB
		want error
	}{{"email", &googleDB{emailExists: true}, identityapp.ErrGoogleLinkRequired}, {"disabled", &googleDB{existing: true, disabled: true}, identityapp.ErrGoogleAuthenticationDenied}, {"database", &googleDB{execErr: errors.New("database unavailable")}, nil}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &writeRunnerFake{db: tc.db}
			resolver, _ := identitypersistence.NewGoogleIdentityResolver(runner)
			result, err := resolver.ResolveGoogleIdentity(context.Background(), googleCommit(t))
			if err == nil || result != (identityapp.GoogleIdentityResult{}) || runner.committed {
				t.Fatal("unsafe Google identity resolution", result, err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal("Google policy sentinel lost", err)
			}
		})
	}
	if resolver, err := identitypersistence.NewGoogleIdentityResolver(nil); err == nil || resolver != nil {
		t.Fatal("nil runner accepted")
	}
	var zero *identitypersistence.GoogleIdentityResolver
	if _, err := zero.ResolveGoogleIdentity(context.Background(), googleCommit(t)); err == nil {
		t.Fatal("zero resolver accepted")
	}
}
