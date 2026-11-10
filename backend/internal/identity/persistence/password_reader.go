package persistence

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type readOnlyRunner interface {
	WithinReadOnlyTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type PasswordReader struct{ runner readOnlyRunner }

func NewPasswordReader(runner readOnlyRunner) (*PasswordReader, error) {
	if runner == nil {
		return nil, errors.New("password_reader_invalid")
	}
	return &PasswordReader{runner}, nil
}
func (r *PasswordReader) GetRecoveryAccountFacts(ctx context.Context, userID identityapp.ID) (result identityapp.RecoveryAccountFacts, err error) {
	if ctx == nil || userID.IsZero() || r == nil || r.runner == nil {
		return result, errors.New("recovery_preparation_invalid")
	}
	err = r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := identitysql.New(db).GetCurrentAccountRecoveryPreparation(txCtx, identityID(userID))
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrRecoveryPreparationNotFound
		} else if err != nil {
			return err
		}
		result = identityapp.RecoveryAccountFacts{EmailVerified: row.EmailVerified, AccountDisabled: row.AccountDisabled, PasswordCredentialPresent: row.PasswordCredentialPresent, GoogleIdentityPresent: row.GoogleIdentityPresent}
		return nil
	})
	return result, err
}

// LookupInvitationIdentity reads the identity independently of its sign-in method.
func (r *PasswordReader) LookupInvitationIdentity(ctx context.Context, lookup string) (facts identityapp.RecoveryAccountFacts, err error) {
	if ctx == nil || r == nil || r.runner == nil {
		return facts, errors.New("invitation_identity_unavailable")
	}
	err = r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := identitysql.New(db)
		id, e := q.GetUserByEmailLookupForGoogleLink(txCtx, lookup)
		if errors.Is(e, pgx.ErrNoRows) {
			return identityapp.ErrCredentialNotFound
		}
		if e != nil {
			return e
		}
		row, e := q.GetCurrentAccountRecoveryPreparation(txCtx, id)
		if e != nil {
			return e
		}
		facts = identityapp.RecoveryAccountFacts{EmailVerified: row.EmailVerified, AccountDisabled: row.AccountDisabled, PasswordCredentialPresent: row.PasswordCredentialPresent, GoogleIdentityPresent: row.GoogleIdentityPresent}
		return nil
	})
	return facts, err
}
func (r *PasswordReader) LookupPasswordCredential(ctx context.Context, lookup string) (identityapp.CredentialRecord, error) {
	if ctx == nil {
		return identityapp.CredentialRecord{}, errors.New("password_lookup_invalid")
	}
	if r == nil || r.runner == nil {
		return identityapp.CredentialRecord{}, errors.New("password_reader_invalid")
	}
	var result identityapp.CredentialRecord
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if txCtx == nil || db == nil {
			return errors.New("password_lookup_unavailable")
		}
		row, err := identitysql.New(db).GetPasswordLoginRecord(txCtx, lookup)
		if errors.Is(err, pgx.ErrNoRows) {
			return identityapp.ErrCredentialNotFound
		}
		if err != nil {
			return err
		}
		result = identityapp.CredentialRecord{UserID: identityapp.ID(row.ID.Bytes), PasswordPHC: row.PasswordPhc, AuthEpoch: row.AuthEpoch, CredentialVersion: row.CredentialVersion}
		if row.EmailVerifiedAt.Valid {
			value := row.EmailVerifiedAt.Time.UTC()
			result.EmailVerifiedAt = &value
		}
		if row.DisabledAt.Valid {
			value := row.DisabledAt.Time.UTC()
			result.DisabledAt = &value
		}
		return nil
	})
	if err != nil {
		return identityapp.CredentialRecord{}, err
	}
	return result, nil
}
