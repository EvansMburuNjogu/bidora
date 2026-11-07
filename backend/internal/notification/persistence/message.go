package persistence

import (
	"bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/message"
	notificationsql "bidora/backend/internal/notification/persistence/sqlc"
	"context"
)

func (t *tx) InsertMessage(ctx context.Context, id domain.ID, v message.Encrypted) error {
	if err := t.q.PurgeExpiredNotificationMessages(ctx); err != nil {
		return err
	}
	return t.q.InsertNotificationMessage(ctx, notificationsql.InsertNotificationMessageParams{NotificationID: uuid(id), KeyVersion: v.KeyVersion, Nonce: v.Nonce, Ciphertext: v.Ciphertext})
}
