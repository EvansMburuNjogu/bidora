package smtpsecret

import (
	"context"
	"errors"

	"bidora/backend/internal/platformsettings/domain"
	settingssql "bidora/backend/internal/platformsettings/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

// SMTPUsername returns only the login name for an exact version. Callers must
// authorize Platform access first. Passwords/references never leave this vault.
func (v *Vault) SMTPUsername(ctx context.Context, id domain.ID) (string, error) {
	if ctx == nil || v == nil || v.runner == nil || id.IsZero() {
		return "", errors.New("smtp_username_unavailable")
	}
	var ref string
	err := v.runner.WithinReadOnlyTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		row, e := settingssql.New(db).GetIntegrationVersionInternal(c, pgtype.UUID{Bytes: [16]byte(id), Valid: true})
		if e != nil || row.SettingKind != "SMTP" || row.CredentialSecretRef == nil {
			return errors.New("smtp_username_unavailable")
		}
		ref = *row.CredentialSecretRef
		return nil
	})
	if err != nil {
		return "", errors.New("smtp_username_unavailable")
	}
	var username string
	err = v.WithSecret(ctx, ref, func(raw []byte) error {
		credentials, e := ParseCredentials(raw)
		if e != nil {
			return e
		}
		username = credentials.Username
		return nil
	})
	if err != nil {
		return "", errors.New("smtp_username_unavailable")
	}
	return username, nil
}
