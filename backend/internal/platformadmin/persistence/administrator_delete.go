package persistence

import (
	"bidora/backend/internal/identity/email"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	app "bidora/backend/internal/platformadmin/application"
	"context"
	"time"
)

func (tx *roleChangeTx) DeleteDisabledAdministrator(ctx context.Context, user app.AdministratorIdentity, at time.Time) error {
	if !user.Disabled {
		return app.ErrConcurrentChange
	}
	address, err := email.Parse(user.EmailDisplay)
	if err != nil {
		return app.ErrInvitationInvalid
	}
	tied, err := tx.HasTenantTies(ctx, user.UserID, address.Lookup(), at)
	if err != nil {
		return err
	}
	if tied {
		return app.ErrInvitationConflict
	}
	// No outstanding capability may attach the former offer to a fresh account
	// that subsequently registers the released mailbox.
	if err = tx.ExpireReplacedInvitationChanges(ctx, address.Lookup(), at); err != nil {
		return err
	}
	if err = tx.ReplaceOpenInvitations(ctx, address.Lookup(), at); err != nil {
		return err
	}
	changed, err := identitysql.New(tx.db).DeleteDisabledPlatformIdentity(ctx, identitysql.DeleteDisabledPlatformIdentityParams{UserID: platformID(user.UserID), ExpectedAuthEpoch: user.AuthEpoch})
	if err != nil {
		return err
	}
	if changed != 1 {
		return app.ErrConcurrentChange
	}
	return nil
}
