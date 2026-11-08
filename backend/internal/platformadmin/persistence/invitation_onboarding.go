package persistence

import (
	identityapp "bidora/backend/internal/identity/application"
	platformapp "bidora/backend/internal/platformadmin/application"
	"context"
	"time"
)

func (tx *invitationTx) InvitedAccountExists(ctx context.Context, lookup string) (bool, error) {
	return tx.identity.InvitedAccountExists(ctx, lookup)
}
func (tx *invitationTx) RegisterInvitedIdentity(ctx context.Context, user, audit, correlation platformapp.ID, r platformapp.InvitationRecord, phc string, at time.Time) error {
	return tx.identity.RegisterInvitedIdentity(ctx, identityapp.ID(user), identityapp.ID(audit), identityapp.ID(correlation), r.EmailDisplay, r.EmailLookup, r.FirstName, r.LastName, phc, at)
}
