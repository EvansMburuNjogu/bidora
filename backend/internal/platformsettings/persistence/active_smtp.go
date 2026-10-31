package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/platformsettings/domain"
	settingssql "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type ActiveSMTPStore struct{ runner settingsReadRunner }

func NewActiveSMTPStore(runner settingsReadRunner) (*ActiveSMTPStore, error) {
	if runner == nil {
		return nil, errors.New("active_smtp_store_invalid")
	}
	return &ActiveSMTPStore{runner: runner}, nil
}

func (s *ActiveSMTPStore) ActiveSMTP(ctx context.Context) (result domain.Version, err error) {
	if ctx == nil || s == nil || s.runner == nil {
		return result, errors.New("active_smtp_unavailable")
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, queryErr := settingssql.New(db).GetActiveSMTPDeliveryConfiguration(txCtx)
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return errors.New("active_smtp_not_configured")
		}
		if queryErr != nil {
			return queryErr
		}
		configuration, configurationHash, queryErr := domain.NewConfiguration(domain.SMTP, &domain.SMTPConfiguration{Host: row.Host, Port: row.Port, CredentialSecretRef: row.CredentialSecretRef, TrustRootsSecretRef: row.TrustRootsSecretRef, SenderName: row.SenderName, SenderAddress: row.SenderAddress, ReplyToAddress: row.ReplyToAddress, ConnectTimeoutSeconds: row.ConnectTimeoutSeconds, CommandTimeoutSeconds: row.CommandTimeoutSeconds, DeliveryTimeoutSeconds: row.DeliveryTimeoutSeconds, MaxConcurrency: row.MaxConcurrency}, nil)
		if queryErr != nil || !row.ID.Valid || row.ID.Bytes == [16]byte{} || row.Version < 1 || len(row.ConfigurationHash) != 32 {
			return errors.New("active_smtp_invalid")
		}
		var storedHash [32]byte
		copy(storedHash[:], row.ConfigurationHash)
		if configurationHash != storedHash {
			return errors.New("active_smtp_invalid")
		}
		result = domain.Version{ID: id(row.ID), Kind: domain.SMTP, State: domain.Active, Version: row.Version, Configuration: configuration, ConfigurationHash: configurationHash}
		return nil
	})
	if err != nil {
		return domain.Version{}, errors.New("active_smtp_unavailable")
	}
	return result, nil
}
