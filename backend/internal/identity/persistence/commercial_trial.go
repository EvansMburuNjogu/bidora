package persistence

import (
	"context"
	"errors"

	identityapp "bidora/backend/internal/identity/application"
	identitysql "bidora/backend/internal/identity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type CommercialTrialIdentityTx struct{ queries *identitysql.Queries }

func NewCommercialTrialIdentityTx(db postgres.DBTX) (*CommercialTrialIdentityTx, error) {
	if db == nil {
		return nil, errors.New("commercial_trial_identity_tx_invalid")
	}
	return &CommercialTrialIdentityTx{queries: identitysql.New(db)}, nil
}

func (tx *CommercialTrialIdentityTx) LockEligible(ctx context.Context, userID identityapp.ID) (bool, error) {
	_, err := tx.queries.LockCommercialTrialIdentity(ctx, identityID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
