package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/platformsettings/domain"
	settingssql "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

// SMTPTestRecipientDirectory resolves only the current verified email of an
// active Platform Administrator. It returns no broader identity profile.
type SMTPTestRecipientDirectory struct{ runner transactionRunner }

func NewSMTPTestRecipientDirectory(runner transactionRunner) (*SMTPTestRecipientDirectory, error) {
	if runner == nil {
		return nil, errors.New("smtp_test_recipient_directory_invalid")
	}
	return &SMTPTestRecipientDirectory{runner: runner}, nil
}

func (d *SMTPTestRecipientDirectory) VerifiedEmail(ctx context.Context, actorID domain.ID) (result email.Address, err error) {
	if ctx == nil || d == nil || d.runner == nil || actorID.IsZero() {
		return result, errors.New("smtp_test_recipient_invalid")
	}
	err = d.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		value, queryErr := settingssql.New(db).GetVerifiedPlatformSettingTestEmail(txCtx, uuid(actorID))
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return errors.New("smtp_test_recipient_unavailable")
		}
		if queryErr != nil {
			return queryErr
		}
		result, queryErr = email.Parse(value)
		return queryErr
	})
	if err != nil {
		return email.Address{}, errors.New("smtp_test_recipient_unavailable")
	}
	return result, nil
}
