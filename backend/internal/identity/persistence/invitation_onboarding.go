package persistence

import (
	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"context"
	"errors"
	"time"
)

func (tx *PlatformAdministrationIdentityTx) InvitedAccountExists(ctx context.Context, lookup string) (bool, error) {
	return tx.queries.InvitedAccountExists(ctx, lookup)
}
func (tx *PlatformAdministrationIdentityTx) RegisterInvitedIdentity(ctx context.Context, user, audit, correlation identityapp.ID, email, lookup, first, last, phc string, at time.Time) error {
	if err := tx.queries.InsertPasswordUser(ctx, identitysql.InsertPasswordUserParams{ID: platformIdentityID(user), Email: email, EmailLookup: lookup, FirstName: &first, LastName: &last, CreatedAt: platformTimestamp(at)}); err != nil {
		return err
	}
	if err := tx.queries.InsertPasswordIdentity(ctx, identitysql.InsertPasswordIdentityParams{UserID: platformIdentityID(user), PasswordPhc: phc, CreatedAt: platformTimestamp(at)}); err != nil {
		return err
	}
	n, err := tx.queries.VerifyNewInvitedIdentity(ctx, identitysql.VerifyNewInvitedIdentityParams{UserID: platformIdentityID(user), At: platformTimestamp(at)})
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("invitation_identity_conflict")
	}
	return tx.queries.InsertUserRegisteredAudit(ctx, identitysql.InsertUserRegisteredAuditParams{ID: platformIdentityID(audit), TargetID: platformIdentityID(user), OccurredAt: platformTimestamp(at), CorrelationID: platformIdentityID(correlation)})
}
