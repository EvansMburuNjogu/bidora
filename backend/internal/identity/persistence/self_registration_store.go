package persistence

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgconn"
)

type SelfRegistrationCommitter struct{ runner writeRunner }

func NewSelfRegistrationCommitter(runner writeRunner) (*SelfRegistrationCommitter, error) {
	if runner == nil {
		return nil, errors.New("self_registration_committer_invalid")
	}
	return &SelfRegistrationCommitter{runner}, nil
}
func (c *SelfRegistrationCommitter) CommitSelfRegistration(ctx context.Context, commit identityapp.SelfRegistrationCommit) error {
	if ctx == nil || commit.UserID.IsZero() || commit.AuditID.IsZero() || commit.CorrelationID.IsZero() || commit.Email.IsZero() || !identityapp.ValidPersonalName(commit.FirstName) || !identityapp.ValidPersonalName(commit.LastName) || commit.PasswordPHC == "" || commit.CreatedAt.IsZero() {
		return errors.New("self_registration_commit_invalid")
	}
	if c == nil || c.runner == nil {
		return errors.New("self_registration_committer_invalid")
	}
	err := c.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("self_registration_unavailable")
		}
		queries := identitysql.New(db)
		firstName, lastName := commit.FirstName, commit.LastName
		if err := queries.InsertPasswordUser(txCtx, identitysql.InsertPasswordUserParams{ID: identityID(commit.UserID), Email: commit.Email.Display(), EmailLookup: commit.Email.Lookup(), FirstName: &firstName, LastName: &lastName, CreatedAt: identityTime(commit.CreatedAt)}); err != nil {
			return err
		}
		if err := queries.InsertPasswordIdentity(txCtx, identitysql.InsertPasswordIdentityParams{UserID: identityID(commit.UserID), PasswordPhc: commit.PasswordPHC, CreatedAt: identityTime(commit.CreatedAt)}); err != nil {
			return err
		}
		return queries.InsertUserRegisteredAudit(txCtx, identitysql.InsertUserRegisteredAuditParams{ID: identityID(commit.AuditID), TargetID: identityID(commit.UserID), OccurredAt: identityTime(commit.CreatedAt), CorrelationID: identityID(commit.CorrelationID)})
	})
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "users_email_lookup_key" {
			return identityapp.ErrIdentityAlreadyExists
		}
		return err
	}
	return nil
}
