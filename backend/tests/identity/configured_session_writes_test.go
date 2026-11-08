package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	app "bidora/backend/internal/identity/application"
	persistence "bidora/backend/internal/identity/persistence"
	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/postgres"
)

type policyWriteProbe struct {
	calls int
	err   error
}

func (p *policyWriteProbe) WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error {
	p.calls++
	return p.err
}
func TestConfiguredOrdinaryExpirySurvivesEverySessionPersistenceBoundary(t *testing.T) {
	policy := securitypolicy.Default()
	policy.IdleSeconds = 600
	policy.AbsoluteSeconds = 3600
	ctx, _ := securitypolicy.With(context.Background(), policy)
	t.Run("password_or_google_primary_login", func(t *testing.T) {
		db := &loginDB{}
		runner := &writeRunnerFake{db: db}
		store, _ := persistence.NewLoginCommitter(runner)
		commit := validSessionCommit(t)
		commit.IdleExpiresAt, commit.AbsoluteExpiresAt, _ = securitypolicy.Expiries(ctx, commit.CreatedAt, false)
		if err := store.CommitLoginSession(ctx, commit); err != nil || !runner.committed {
			t.Fatal("configured login persistence rejected", err)
		}
	})
	t.Run("google_link_rotation", func(t *testing.T) {
		runner := &writeRunnerFake{db: &googleLinkDB{}}
		store, _ := persistence.NewGoogleLinkCommitter(runner)
		commit := validGoogleLinkCommit(t)
		commit.IdleExpiresAt, commit.AbsoluteExpiresAt, _ = securitypolicy.Expiries(ctx, commit.LinkedAt, false)
		if err := store.CommitGoogleLink(ctx, commit); err != nil || !runner.committed {
			t.Fatal("configured Google link persistence rejected", err)
		}
	})
	t.Run("password_change_rotation", func(t *testing.T) {
		stop := errors.New("transaction probe")
		runner := &policyWriteProbe{err: stop}
		store, _ := persistence.NewPasswordChangeCommitter(runner)
		at := time.Now().UTC()
		idle, absolute, _ := securitypolicy.Expiries(ctx, at, false)
		commit := app.PasswordChangeCommit{UserID: identityID(1), CurrentSessionID: identityID(2), ReplacementSessionID: identityID(3), PasswordAuditID: identityID(4), LoginAuditID: identityID(5), CorrelationID: identityID(6), ExpectedAuthEpoch: 1, NewAuthEpoch: 2, ExpectedCredentialVersion: 1, NewCredentialVersion: 2, PasswordPHC: "synthetic", SessionHash: [32]byte{1}, CSRFHash: [32]byte{2}, ChangedAt: at, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
		if err := store.CommitPasswordChange(ctx, commit); !errors.Is(err, stop) || runner.calls != 1 {
			t.Fatal("configured password rotation rejected before transaction", err)
		}
		commit.IdleExpiresAt = at.Add(30 * time.Minute)
		if err := store.CommitPasswordChange(ctx, commit); err == nil || runner.calls != 1 {
			t.Fatal("unconfigured expiry bypassed boundary")
		}
	})
}
