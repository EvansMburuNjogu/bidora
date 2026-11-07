package persistence

import (
	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

type PasswordRecoveryStore struct{ runner writeRunner }

func NewPasswordRecoveryStore(runner writeRunner) (*PasswordRecoveryStore, error) {
	if runner == nil {
		return nil, errors.New("password_recovery_store_invalid")
	}
	return &PasswordRecoveryStore{runner}, nil
}
func (s *PasswordRecoveryStore) IssuePasswordRecovery(ctx context.Context, c identityapp.PasswordRecoveryIssueCommand) (result identityapp.PasswordRecoveryIssuance, err error) {
	if ctx == nil || c.ChallengeID.IsZero() || c.AuditID.IsZero() || c.CorrelationID.IsZero() || c.Email.IsZero() || c.TokenHash == [32]byte{} || c.CreatedAt.IsZero() {
		return result, errors.New("password_recovery_issue_invalid")
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := identitysql.New(db)
		candidate, err := q.GetPasswordRecoveryCandidate(txCtx, c.Email.Lookup())
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		if err = q.InvalidateActivePasswordRecoveryChallenges(txCtx, identitysql.InvalidateActivePasswordRecoveryChallengesParams{At: identityTime(c.CreatedAt), UserID: candidate.ID}); err != nil {
			return err
		}
		expiry := securitypolicy.From(txCtx).PasswordResetSeconds
		if err = q.InsertPasswordRecoveryChallenge(txCtx, identitysql.InsertPasswordRecoveryChallengeParams{ID: identityID(c.ChallengeID), UserID: candidate.ID, TokenHash: c.TokenHash[:], CreatedAt: identityTime(c.CreatedAt), ExpirySeconds: expiry}); err != nil {
			return err
		}
		if err = q.InsertPasswordRecoveryRequestedAudit(txCtx, identitysql.InsertPasswordRecoveryRequestedAuditParams{ID: identityID(c.AuditID), UserID: candidate.ID, ChallengeID: identityID(c.ChallengeID), At: identityTime(c.CreatedAt), CorrelationID: identityID(c.CorrelationID), ExpirySeconds: expiry}); err != nil {
			return err
		}
		result = identityapp.PasswordRecoveryIssuance{Issued: true, UserID: identityapp.ID(candidate.ID.Bytes), Recipient: candidate.Email, ExpiresAt: c.CreatedAt.UTC().Add(time.Duration(expiry) * time.Second)}
		return nil
	})
	return result, err
}
func (s *PasswordRecoveryStore) CommitPasswordRecoveryReset(ctx context.Context, c identityapp.PasswordRecoveryResetCommit) (err error) {
	if ctx == nil || c.AuditID.IsZero() || c.CorrelationID.IsZero() || c.TokenHash == [32]byte{} || c.PasswordPHC == "" || c.ResetAt.IsZero() {
		return errors.New("password_recovery_reset_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := identitysql.New(db)
		row, err := q.LockValidPasswordRecoveryChallenge(txCtx, identitysql.LockValidPasswordRecoveryChallengeParams{TokenHash: c.TokenHash[:], At: identityTime(c.ResetAt)})
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrPasswordRecoveryInvalid
		} else if err != nil {
			return err
		}
		if !c.UserID.IsZero() && c.UserID != identityapp.ID(row.UserID.Bytes) {
			return identityapp.ErrPasswordRecoveryInvalid
		}
		affected, err := q.ResetPasswordCredential(txCtx, identitysql.ResetPasswordCredentialParams{PasswordPhc: c.PasswordPHC, At: identityTime(c.ResetAt), UserID: row.UserID, ExpectedCredentialVersion: row.CredentialVersion})
		if err != nil || affected != 1 {
			return identityapp.ErrPasswordRecoveryInvalid
		}
		affected, err = q.AdvancePasswordRecoveryAuthEpoch(txCtx, identitysql.AdvancePasswordRecoveryAuthEpochParams{At: identityTime(c.ResetAt), UserID: row.UserID, ExpectedAuthEpoch: row.AuthEpoch})
		if err != nil || affected != 1 {
			return identityapp.ErrPasswordRecoveryInvalid
		}
		if _, err = q.RevokeAllUserSessions(txCtx, identitysql.RevokeAllUserSessionsParams{RevokedAt: identityTime(c.ResetAt), UserID: row.UserID}); err != nil {
			return err
		}
		affected, err = q.ConsumePasswordRecoveryChallenge(txCtx, identitysql.ConsumePasswordRecoveryChallengeParams{At: identityTime(c.ResetAt), ID: row.ChallengeID})
		if err != nil || affected != 1 {
			return identityapp.ErrPasswordRecoveryInvalid
		}
		return q.InsertPasswordRecoveredAudit(txCtx, identitysql.InsertPasswordRecoveredAuditParams{ID: identityID(c.AuditID), UserID: row.UserID, At: identityTime(c.ResetAt), CorrelationID: identityID(c.CorrelationID)})
	})
}
