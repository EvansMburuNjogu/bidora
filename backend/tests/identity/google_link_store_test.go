package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type googleLinkDB struct {
	calls                             []string
	subjectExists, userIdentityExists bool
	fail                              string
}

func (d *googleLinkDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	d.calls = append(d.calls, sql)
	if d.fail != "" && strings.Contains(sql, d.fail) {
		return pgconn.CommandTag{}, errors.New("database failure")
	}
	if strings.Contains(sql, "AdvanceGoogleLinkAuthEpoch") || strings.Contains(sql, "RevokeAllUserSessions") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (d *googleLinkDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (d *googleLinkDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	d.calls = append(d.calls, sql)
	switch {
	case strings.Contains(sql, "GetGoogleLinkPrincipalForUpdate"):
		return identityRow(func(dest ...any) error { *(dest[0].(*int64)) = 4; *(dest[1].(*int64)) = 3; return nil })
	case strings.Contains(sql, "GetGoogleIdentityForUpdate"):
		if !d.subjectExists {
			return identityRow(func(...any) error { return pgx.ErrNoRows })
		}
		return identityRow(func(...any) error { return nil })
	case strings.Contains(sql, "GetGoogleIdentityForUserForUpdate"):
		if !d.userIdentityExists {
			return identityRow(func(...any) error { return pgx.ErrNoRows })
		}
		return identityRow(func(...any) error { return nil })
	default:
		return identityRow(func(...any) error { return errors.New("unexpected query row") })
	}
}
func validGoogleLinkCommit(t *testing.T) identityapp.GoogleLinkCommit {
	t.Helper()
	pair := sessionPair(t)
	linked := time.Now().UTC()
	idle, absolute, _ := session.Expiries(linked)
	return identityapp.GoogleLinkCommit{UserID: identityID(9), CurrentSessionID: identityID(8), ExternalIdentityID: identityID(1), LinkAuditID: identityID(2), ReplacementSessionID: identityID(3), LoginAuditID: identityID(4), CorrelationID: identityID(7), ExpectedAuthEpoch: 4, ExpectedCredentialVersion: 3, NewAuthEpoch: 5, Identity: googleFlowIdentity(t), SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), LinkedAt: linked, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
}
func TestGoogleLinkCommitterLinksRotatesAndAuditsInOneTransaction(t *testing.T) {
	db := &googleLinkDB{}
	runner := &writeRunnerFake{db: db}
	committer, err := identitypersistence.NewGoogleLinkCommitter(runner)
	if err != nil {
		t.Fatal(err)
	}
	if err = committer.CommitGoogleLink(context.Background(), validGoogleLinkCommit(t)); err != nil {
		t.Fatal(err)
	}
	if !runner.committed || runner.calls != 1 {
		t.Fatal("Google link transaction did not commit")
	}
	order := []string{"GetGoogleLinkPrincipalForUpdate", "GetGoogleIdentityForUpdate", "GetGoogleIdentityForUserForUpdate", "InsertGoogleIdentity", "AdvanceGoogleLinkAuthEpoch", "RevokeAllUserSessions", "InsertSession", "InsertGoogleIdentityLinkedAudit", "InsertLoginAudit"}
	if len(db.calls) != len(order) {
		t.Fatal("Google link query count drifted", len(db.calls))
	}
	for index, name := range order {
		if !strings.Contains(db.calls[index], name) {
			t.Fatal("Google link ordering drifted", index, name)
		}
	}
}
func TestGoogleLinkCommitterRejectsExistingBindingsAndRollsBack(t *testing.T) {
	for _, db := range []*googleLinkDB{{subjectExists: true}, {userIdentityExists: true}, {fail: "InsertGoogleIdentityLinkedAudit"}} {
		runner := &writeRunnerFake{db: db}
		committer, _ := identitypersistence.NewGoogleLinkCommitter(runner)
		err := committer.CommitGoogleLink(context.Background(), validGoogleLinkCommit(t))
		if err == nil || runner.committed {
			t.Fatal("conflicting/partial Google link committed", err)
		}
	}
	if committer, err := identitypersistence.NewGoogleLinkCommitter(nil); err == nil || committer != nil {
		t.Fatal("nil Google link runner accepted")
	}
	var zero *identitypersistence.GoogleLinkCommitter
	if err := zero.CommitGoogleLink(context.Background(), validGoogleLinkCommit(t)); err == nil {
		t.Fatal("zero Google link committer accepted")
	}
}
