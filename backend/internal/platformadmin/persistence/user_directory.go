package persistence

import (
	identityapp "bidora/backend/internal/identity/application"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformsql "bidora/backend/internal/platformadmin/persistence/sqlc"
	"context"
	"time"
)

func (tx *roleChangeTx) RemoveAdministratorProfile(ctx context.Context, p platformapp.AdministratorProfile, at time.Time) error {
	n, e := tx.platform.RemovePlatformAdministratorProfile(ctx, platformsql.RemovePlatformAdministratorProfileParams{UserID: platformID(p.UserID), ExpectedVersion: p.Version, At: platformTime(at)})
	if e == nil && n != 1 {
		return platformapp.ErrConcurrentChange
	}
	return e
}
func (tx *invitationTx) ApplyInvitedName(ctx context.Context, id platformapp.ID, first, last string, at time.Time) error {
	return tx.identity.ApplyInvitedName(ctx, identityapp.ID(id), first, last, at)
}
