package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/securitypolicy"
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
func (c *LoginCommitter) SessionAdministrator(ctx context.Context, userID identityapp.ID) (administrator bool, err error) {
	if ctx == nil || c == nil || c.runner == nil || userID.IsZero() {
		return false, errors.New("login_commit_invalid")
	}
	err = c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("login_commit_unavailable")
		}
		kind, e := identitysql.New(db).GetUserAccountType(txCtx, identityID(userID))
		if e != nil {
			return e
		}
		administrator = kind != nil && *kind == "PLATFORM_ADMIN"
		return nil
	})
	return administrator, err
}
func (c *LoginCommitter) CommitLoginSession(ctx context.Context, commit identityapp.SessionCommit) error {
	if ctx == nil {
		return errors.New("login_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return errors.New("login_committer_invalid")
	}
	return c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		return CommitLoginSessionWithin(txCtx, db, commit)
	})
}

// CommitRegistrationSession rechecks the verified identity and current security
// state while holding the user row lock before inserting session evidence.
func (c *LoginCommitter) CommitRegistrationSession(ctx context.Context, commit identityapp.SessionCommit) error {
	if ctx == nil || c == nil || c.runner == nil || commit.UserID.IsZero() || commit.AuthEpoch < 1 || commit.CreatedAt.IsZero() {
		return errors.New("registration_session_invalid")
	}
	return c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("registration_session_unavailable")
		}
		_, err := identitysql.New(db).LockRegistrationContinuationUser(txCtx, identitysql.LockRegistrationContinuationUserParams{UserID: identityID(commit.UserID), AuthEpoch: commit.AuthEpoch, At: identityTime(commit.CreatedAt)})
		if err != nil {
			return err
		}
		return CommitLoginSessionWithin(txCtx, db, commit)
	})
}

// CommitLoginSessionWithin keeps session/audit insertion in the owner's existing
// transaction, so promotion cannot commit without its replacement session.
func CommitLoginSessionWithin(ctx context.Context, db postgres.DBTX, commit identityapp.SessionCommit) error {
	if ctx == nil || db == nil || commit.SessionID.IsZero() || commit.AuditID.IsZero() || commit.UserID.IsZero() || commit.CorrelationID.IsZero() || commit.AuthEpoch < 1 || commit.SessionHash == [32]byte{} || commit.CSRFHash == [32]byte{} || commit.CreatedAt.IsZero() {
		return errors.New("login_commit_invalid")
	}
	if commit.AuthenticationMethod != identityapp.AuthenticationMethodPassword && commit.AuthenticationMethod != identityapp.AuthenticationMethodGoogle && commit.AuthenticationMethod != identityapp.AuthenticationMethodSessionContinuation {
		return errors.New("login_commit_invalid")
	}
	queries := identitysql.New(db)
	kind, err := queries.GetUserAccountType(ctx, identityID(commit.UserID))
	if err != nil {
		return err
	}
	administrator := kind != nil && *kind == "PLATFORM_ADMIN"
	idle, absolute, err := securitypolicy.Expiries(ctx, commit.CreatedAt, administrator)
	if err != nil || !idle.Equal(commit.IdleExpiresAt) || !absolute.Equal(commit.AbsoluteExpiresAt) {
		return errors.New("login_commit_invalid")
	}
	summary, _ := json.Marshal(struct {
		AuthEpoch            int64                            `json:"auth_epoch"`
		AuthenticationMethod identityapp.AuthenticationMethod `json:"authentication_method"`
	}{commit.AuthEpoch, commit.AuthenticationMethod})
	if err = queries.InsertSession(ctx, identitysql.InsertSessionParams{ID: identityID(commit.SessionID), UserID: identityID(commit.UserID), TokenHash: commit.SessionHash[:], CsrfHash: commit.CSRFHash[:], AuthEpoch: commit.AuthEpoch, CreatedAt: identityTime(commit.CreatedAt), IdleExpiresAt: identityTime(commit.IdleExpiresAt), AbsoluteExpiresAt: identityTime(commit.AbsoluteExpiresAt), AssuranceLevel: commit.AssuranceLevel, IdleTimeoutSeconds: int32(commit.IdleExpiresAt.Sub(commit.CreatedAt).Seconds()), AbsoluteTimeoutSeconds: int32(commit.AbsoluteExpiresAt.Sub(commit.CreatedAt).Seconds()), AssuredAt: identityTime(commit.AssuredAt), AssuranceMethod: commit.AssuranceMethod}); err != nil {
		return err
	}
	return queries.InsertLoginAudit(ctx, identitysql.InsertLoginAuditParams{ID: identityID(commit.AuditID), ActorID: identityID(commit.UserID), TargetID: identityID(commit.SessionID), OccurredAt: identityTime(commit.CreatedAt), CorrelationID: identityID(commit.CorrelationID), ChangeSummary: summary})
}

func identityID(id identityapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()}
}
func identityTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
