package persistence

import (
	app "bidora/backend/internal/identity/application"
	q "bidora/backend/internal/identity/persistence/sqlc"
	"context"
	"time"
)

func (tx *PlatformAdministrationIdentityTx) ApplyInvitedName(ctx context.Context, id app.ID, first, last string, at time.Time) error {
	n, e := tx.queries.ApplyPlatformInvitedName(ctx, q.ApplyPlatformInvitedNameParams{UserID: platformIdentityID(id), FirstName: first, LastName: last, At: platformTimestamp(at)})
	if e == nil && n != 1 {
		return app.ErrPlatformIdentityConflict
	}
	return e
}
