package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

type writeRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type passwordHasher interface{ Hash([]byte) (string, error) }
type persistenceClock interface{ Now() time.Time }

type PasswordRehasher struct {
	runner writeRunner
	hasher passwordHasher
	clock  persistenceClock
}

func NewPasswordRehasher(runner writeRunner, hasher passwordHasher, clock persistenceClock) (*PasswordRehasher, error) {
	if runner == nil || hasher == nil || clock == nil {
		return nil, errors.New("password_rehasher_invalid")
	}
	return &PasswordRehasher{runner, hasher, clock}, nil
}
func (r *PasswordRehasher) RehashPassword(ctx context.Context, userID identityapp.ID, expectedVersion int64, candidate []byte) error {
	if ctx == nil || userID.IsZero() || expectedVersion < 1 {
		return errors.New("password_rehash_invalid")
	}
	if r == nil || r.runner == nil || r.hasher == nil || r.clock == nil {
		return errors.New("password_rehasher_invalid")
	}
	phc, err := r.hasher.Hash(candidate)
	if err != nil {
		return err
	}
	if phc == "" {
		return errors.New("password_rehash_invalid")
	}
	at := r.clock.Now().UTC()
	if at.IsZero() {
		return errors.New("password_rehash_invalid")
	}
	return r.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("password_rehash_unavailable")
		}
		rows, err := identitysql.New(db).RehashPasswordCredential(txCtx, identitysql.RehashPasswordCredentialParams{PasswordPhc: phc, UpdatedAt: identityTime(at), UserID: identityID(userID), ExpectedCredentialVersion: expectedVersion})
		if err != nil {
			return err
		}
		if rows != 1 {
			return errors.New("password_rehash_conflict")
		}
		return nil
	})
}

type LoginCommitter struct{ runner writeRunner }

func NewLoginCommitter(runner writeRunner) (*LoginCommitter, error) {
	if runner == nil {
		return nil, errors.New("login_committer_invalid")
	}
	return &LoginCommitter{runner}, nil
}
func (c *LoginCommitter) CommitLoginSession(ctx context.Context, commit identityapp.SessionCommit) error {
	if ctx == nil || commit.SessionID.IsZero() || commit.AuditID.IsZero() || commit.UserID.IsZero() || commit.CorrelationID.IsZero() || commit.AuthEpoch < 1 || commit.AuthenticationMethod != identityapp.AuthenticationMethodPassword && commit.AuthenticationMethod != identityapp.AuthenticationMethodGoogle || commit.SessionHash == [32]byte{} || commit.CSRFHash == [32]byte{} || commit.CreatedAt.IsZero() {
		return errors.New("login_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return errors.New("login_committer_invalid")
	}
	idle, absolute, err := session.Expiries(commit.CreatedAt)
	if err != nil || !idle.Equal(commit.IdleExpiresAt) || !absolute.Equal(commit.AbsoluteExpiresAt) {
		return errors.New("login_commit_invalid")
	}
	summary, _ := json.Marshal(struct {
		AuthEpoch            int64                            `json:"auth_epoch"`
		AuthenticationMethod identityapp.AuthenticationMethod `json:"authentication_method"`
	}{commit.AuthEpoch, commit.AuthenticationMethod})
	return c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("login_commit_unavailable")
		}
		queries := identitysql.New(db)
		if err := queries.InsertSession(txCtx, identitysql.InsertSessionParams{ID: identityID(commit.SessionID), UserID: identityID(commit.UserID), TokenHash: commit.SessionHash[:], CsrfHash: commit.CSRFHash[:], AuthEpoch: commit.AuthEpoch, CreatedAt: identityTime(commit.CreatedAt), IdleExpiresAt: identityTime(commit.IdleExpiresAt), AbsoluteExpiresAt: identityTime(commit.AbsoluteExpiresAt), AssuranceLevel: commit.AssuranceLevel, IdleTimeoutSeconds: int32(commit.IdleExpiresAt.Sub(commit.CreatedAt).Seconds()), AbsoluteTimeoutSeconds: int32(commit.AbsoluteExpiresAt.Sub(commit.CreatedAt).Seconds()), AssuredAt: identityTime(commit.AssuredAt), AssuranceMethod: commit.AssuranceMethod}); err != nil {
			return err
		}
		return queries.InsertLoginAudit(txCtx, identitysql.InsertLoginAuditParams{ID: identityID(commit.AuditID), ActorID: identityID(commit.UserID), TargetID: identityID(commit.SessionID), OccurredAt: identityTime(commit.CreatedAt), CorrelationID: identityID(commit.CorrelationID), ChangeSummary: summary})
	})
}
func identityID(id identityapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()}
}
func identityTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
