package persistence

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
)

type LogoutCommitter struct{ runner writeRunner }

func NewLogoutCommitter(runner writeRunner) (*LogoutCommitter, error) {
	if runner == nil {
		return nil, errors.New("logout_committer_invalid")
	}
	return &LogoutCommitter{runner}, nil
}
func (c *LogoutCommitter) CommitLogout(ctx context.Context, commit identityapp.LogoutCommit) (bool, error) {
	if ctx == nil || commit.SessionID.IsZero() || commit.UserID.IsZero() || commit.AuditID.IsZero() || commit.CorrelationID.IsZero() || commit.OccurredAt.IsZero() {
		return false, errors.New("logout_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return false, errors.New("logout_committer_invalid")
	}
	revoked := false
	err := c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("logout_commit_unavailable")
		}
		queries := identitysql.New(db)
		rows, err := queries.RevokeSessionByID(txCtx, identitysql.RevokeSessionByIDParams{RevokedAt: identityTime(commit.OccurredAt), ID: identityID(commit.SessionID), UserID: identityID(commit.UserID)})
		if err != nil {
			return err
		}
		if rows == 0 {
			return nil
		}
		if rows != 1 {
			return errors.New("logout_revoke_count_invalid")
		}
		if err = queries.InsertLogoutAudit(txCtx, identitysql.InsertLogoutAuditParams{ID: identityID(commit.AuditID), ActorID: identityID(commit.UserID), TargetID: identityID(commit.SessionID), OccurredAt: identityTime(commit.OccurredAt), CorrelationID: identityID(commit.CorrelationID)}); err != nil {
			return err
		}
		revoked = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return revoked, nil
}
