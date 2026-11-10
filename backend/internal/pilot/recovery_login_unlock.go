package pilot

import (
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/attemptredis"
	"context"
)

type recoveryLoginUnlocker struct {
	attempts *attemptredis.Controller
	deriver  *attempt.Deriver
}

func (u recoveryLoginUnlocker) ClearRecoveryLoginLock(ctx context.Context, lookup string) error {
	key, err := u.deriver.Identity(lookup)
	if err != nil {
		return err
	}
	return u.attempts.ClearRecoveredIdentity(ctx, key)
}
