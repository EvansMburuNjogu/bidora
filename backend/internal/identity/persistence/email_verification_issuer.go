package persistence

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/identity/verification"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type EmailVerificationIssuer struct{ runner writeRunner }

func NewEmailVerificationIssuer(runner writeRunner) (*EmailVerificationIssuer, error) {
	if runner == nil {
		return nil, errors.New("email_verification_issuer_invalid")
	}
	return &EmailVerificationIssuer{runner}, nil
}
func (i *EmailVerificationIssuer) IssueEmailVerification(ctx context.Context, challenge identityapp.EmailVerificationChallenge) (identityapp.EmailVerificationIssuance, error) {
	if ctx == nil || challenge.ChallengeID.IsZero() || challenge.UserID.IsZero() || challenge.AuditID.IsZero() || challenge.CorrelationID.IsZero() || challenge.TokenHash == ([32]byte{}) || challenge.CreatedAt.IsZero() || challenge.ExpiresAt.IsZero() || !challenge.ExpiresAt.Equal(challenge.CreatedAt.UTC().Add(verification.Lifetime)) {
		return identityapp.EmailVerificationIssuance{}, errors.New("email_verification_issue_invalid")
	}
	if i == nil || i.runner == nil {
		return identityapp.EmailVerificationIssuance{}, errors.New("email_verification_issuer_invalid")
	}
	var result identityapp.EmailVerificationIssuance
	err := i.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("email_verification_issue_unavailable")
		}
		queries := identitysql.New(db)
		recipient, err := queries.LockEmailVerificationIssuanceTarget(txCtx, identityID(challenge.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrVerificationTargetUnavailable
		}
		if err != nil {
			return err
		}
		if _, err = queries.InvalidateEmailVerificationChallenges(txCtx, identitysql.InvalidateEmailVerificationChallengesParams{InvalidatedAt: identityTime(challenge.CreatedAt), UserID: identityID(challenge.UserID)}); err != nil {
			return err
		}
		if err = queries.InsertEmailVerificationChallenge(txCtx, identitysql.InsertEmailVerificationChallengeParams{ID: identityID(challenge.ChallengeID), UserID: identityID(challenge.UserID), TokenHash: challenge.TokenHash[:], ExpiresAt: identityTime(challenge.ExpiresAt), CreatedAt: identityTime(challenge.CreatedAt)}); err != nil {
			return err
		}
		if err = queries.InsertEmailVerificationIssuedAudit(txCtx, identitysql.InsertEmailVerificationIssuedAuditParams{ID: identityID(challenge.AuditID), ActorID: identityID(challenge.ActorID), TargetID: identityID(challenge.UserID), OccurredAt: identityTime(challenge.CreatedAt), CorrelationID: identityID(challenge.CorrelationID)}); err != nil {
			return err
		}
		result = identityapp.EmailVerificationIssuance{UserID: challenge.UserID, Recipient: recipient, ExpiresAt: challenge.ExpiresAt.UTC()}
		return nil
	})
	if err != nil {
		return identityapp.EmailVerificationIssuance{}, err
	}
	return result, nil
}
