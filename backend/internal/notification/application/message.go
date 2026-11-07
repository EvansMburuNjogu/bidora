package application

import (
	"bidora/backend/internal/notification/domain"
	"bidora/backend/internal/notification/message"
	"context"
)

type MessageTx interface {
	InsertMessage(context.Context, domain.ID, message.Encrypted) error
}

func (s *Service) RecordMessage(ctx context.Context, id domain.ID, v message.Encrypted) error {
	if ctx == nil || id.IsZero() || v.KeyVersion < 1 || len(v.Nonce) != 12 || len(v.Ciphertext) < 16 {
		return ErrInvalid
	}
	return s.uow.WithinNotification(ctx, func(c context.Context, tx Tx) error {
		_, found, err := tx.GetNotification(c, id)
		if err != nil {
			return err
		}
		if !found {
			return ErrConflict
		}
		writer, ok := tx.(MessageTx)
		if !ok {
			return ErrUnavailable
		}
		return writer.InsertMessage(c, id, v)
	})
}
