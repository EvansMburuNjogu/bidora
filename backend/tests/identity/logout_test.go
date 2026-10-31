package identity_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitypersistence "bidora/backend/internal/identity/persistence"
)

type logoutCommitterFake struct {
	revoked bool
	err     error
	commit  identityapp.LogoutCommit
	calls   int
}

func (c *logoutCommitterFake) CommitLogout(_ context.Context, commit identityapp.LogoutCommit) (bool, error) {
	c.calls++
	c.commit = commit
	return c.revoked, c.err
}
func TestLogoutServiceCommitsCurrentSessionAudit(t *testing.T) {
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	committer := &logoutCommitterFake{revoked: true}
	service, err := identityapp.NewLogoutService(committer, &loginIDsFake{id: identityID(4)}, loginClockFake{at: at})
	if err != nil {
		t.Fatal(err)
	}
	principal := identityapp.SessionPrincipal{SessionID: identityID(1), UserID: identityID(2)}
	if err = service.Logout(context.Background(), principal, identityID(3)); err != nil {
		t.Fatal(err)
	}
	if committer.calls != 1 || committer.commit.SessionID != identityID(1) || committer.commit.UserID != identityID(2) || committer.commit.AuditID != identityID(4) || committer.commit.CorrelationID != identityID(3) || !committer.commit.OccurredAt.Equal(at) {
		t.Fatal("logout orchestration drifted", committer.commit)
	}
}
func TestLogoutServiceFailsClosedAndTreatsAlreadyRevokedAsSuccess(t *testing.T) {
	principal := identityapp.SessionPrincipal{SessionID: identityID(1), UserID: identityID(2)}
	committer := &logoutCommitterFake{}
	service, _ := identityapp.NewLogoutService(committer, &loginIDsFake{id: identityID(4)}, loginClockFake{at: time.Now()})
	if err := service.Logout(context.Background(), principal, identityID(3)); err != nil {
		t.Fatal("already revoked logout not idempotent", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*logoutCommitterFake, *loginIDsFake, *loginClockFake, *identityapp.SessionPrincipal, *identityapp.ID)
	}{{"commit", func(c *logoutCommitterFake, _ *loginIDsFake, _ *loginClockFake, _ *identityapp.SessionPrincipal, _ *identityapp.ID) {
		c.err = errors.New("secret-canary db")
	}}, {"id", func(_ *logoutCommitterFake, i *loginIDsFake, _ *loginClockFake, _ *identityapp.SessionPrincipal, _ *identityapp.ID) {
		i.err = errors.New("id")
	}}, {"clock", func(_ *logoutCommitterFake, _ *loginIDsFake, c *loginClockFake, _ *identityapp.SessionPrincipal, _ *identityapp.ID) {
		c.at = time.Time{}
	}}, {"principal", func(_ *logoutCommitterFake, _ *loginIDsFake, _ *loginClockFake, p *identityapp.SessionPrincipal, _ *identityapp.ID) {
		*p = identityapp.SessionPrincipal{}
	}}, {"correlation", func(_ *logoutCommitterFake, _ *loginIDsFake, _ *loginClockFake, _ *identityapp.SessionPrincipal, id *identityapp.ID) {
		*id = identityapp.ID{}
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			c := &logoutCommitterFake{}
			ids := &loginIDsFake{id: identityID(4)}
			clock := loginClockFake{at: time.Now()}
			p := principal
			correlation := identityID(3)
			tc.mutate(c, ids, &clock, &p, &correlation)
			service, _ := identityapp.NewLogoutService(c, ids, clock)
			err := service.Logout(context.Background(), p, correlation)
			if err == nil || !(err.Error() == "logout_unavailable" || err.Error() == "logout_context_invalid") || strings.Contains(err.Error(), "secret-canary") {
				t.Fatal("logout failure unsafe", err)
			}
		})
	}
	if service, err := identityapp.NewLogoutService(nil, &loginIDsFake{}, loginClockFake{}); err == nil || service != nil {
		t.Fatal("nil logout dependency accepted")
	}
	var zero *identityapp.LogoutService
	if err := zero.Logout(context.Background(), principal, identityID(3)); err == nil {
		t.Fatal("zero logout service accepted")
	}
}
func TestLogoutCommitterRevokesThenAuditsAtomically(t *testing.T) {
	db := &loginDB{}
	runner := &writeRunnerFake{db: db}
	committer, err := identitypersistence.NewLogoutCommitter(runner)
	if err != nil {
		t.Fatal(err)
	}
	commit := identityapp.LogoutCommit{SessionID: identityID(1), UserID: identityID(2), AuditID: identityID(3), CorrelationID: identityID(4), OccurredAt: time.Now()}
	revoked, err := committer.CommitLogout(context.Background(), commit)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked || !runner.committed || len(db.sql) != 2 || !strings.Contains(db.sql[0], "RevokeSessionByID") || !strings.Contains(db.sql[1], "InsertLogoutAudit") {
		t.Fatal("logout transaction ordering drifted", db.sql)
	}
	db = &loginDB{zeroRowsFor: "RevokeSessionByID"}
	runner = &writeRunnerFake{db: db}
	committer, _ = identitypersistence.NewLogoutCommitter(runner)
	revoked, err = committer.CommitLogout(context.Background(), commit)
	if err != nil || revoked || !runner.committed || len(db.sql) != 1 {
		t.Fatal("already revoked logout unsafe", revoked, err, db.sql)
	}
}
func TestLogoutCommitterRollsBackFailures(t *testing.T) {
	commit := identityapp.LogoutCommit{SessionID: identityID(1), UserID: identityID(2), AuditID: identityID(3), CorrelationID: identityID(4), OccurredAt: time.Now()}
	for _, tc := range []struct {
		name string
		db   *loginDB
	}{{"revoke", &loginDB{failFor: "RevokeSessionByID"}}, {"audit", &loginDB{failFor: "InsertLogoutAudit"}}} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &writeRunnerFake{db: tc.db}
			committer, _ := identitypersistence.NewLogoutCommitter(runner)
			revoked, err := committer.CommitLogout(context.Background(), commit)
			if err == nil || revoked || runner.committed {
				t.Fatal("logout failure committed", revoked, err)
			}
		})
	}
	if committer, err := identitypersistence.NewLogoutCommitter(nil); err == nil || committer != nil {
		t.Fatal("nil logout runner accepted")
	}
	var zero *identitypersistence.LogoutCommitter
	if _, err := zero.CommitLogout(context.Background(), commit); err == nil {
		t.Fatal("zero logout committer accepted")
	}
}
