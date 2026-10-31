package persistence

import (
	"context"
	"errors"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type EmailVerificationCommitter struct{ runner writeRunner }

func NewEmailVerificationCommitter(runner writeRunner) (*EmailVerificationCommitter, error) {
	if runner == nil {
		return nil, errors.New("email_verification_committer_invalid")
	}
	return &EmailVerificationCommitter{runner}, nil
}
func (c *EmailVerificationCommitter) ConsumeEmailVerification(ctx context.Context, tokenHash [32]byte, auditID, correlationID identityapp.ID, at time.Time) (identityapp.EmailVerificationResult, error) {
	if ctx == nil || tokenHash == [32]byte{} || auditID.IsZero() || correlationID.IsZero() || at.IsZero() {
		return identityapp.EmailVerificationResult{}, errors.New("email_verification_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return identityapp.EmailVerificationResult{}, errors.New("email_verification_committer_invalid")
	}
	var result identityapp.EmailVerificationResult
	err := c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("email_verification_unavailable")
		}
		queries := identitysql.New(db)
		row, err := queries.ConsumeEmailVerificationChallenge(txCtx, identitysql.ConsumeEmailVerificationChallengeParams{ConsumedAt: identityTime(at), TokenHash: tokenHash[:]})
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrVerificationNotFound
		}
		if err != nil {
			return err
		}
		if !row.ID.Valid || !row.EmailVerifiedAt.Valid || row.AuthEpoch < 1 {
			return errors.New("email_verification_result_invalid")
		}
		userID := identityapp.ID(row.ID.Bytes)
		if userID.IsZero() {
			return errors.New("email_verification_result_invalid")
		}
		if err := queries.InsertEmailVerifiedAudit(txCtx, identitysql.InsertEmailVerifiedAuditParams{ID: identityID(auditID), ActorID: row.ID, OccurredAt: identityTime(at), CorrelationID: identityID(correlationID)}); err != nil {
			return err
		}
		result = identityapp.EmailVerificationResult{UserID: userID, AuthEpoch: row.AuthEpoch, VerifiedAt: row.EmailVerifiedAt.Time.UTC()}
		return nil
	})
	if err != nil {
		return identityapp.EmailVerificationResult{}, err
	}
	return result, nil
}
