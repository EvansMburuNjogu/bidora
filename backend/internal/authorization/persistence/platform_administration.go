package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

// PlatformAdministrationAffiliationTx owns tenant-membership and pending
// tenant-invitation checks used by Platform Administration eligibility policy.
type PlatformAdministrationAffiliationTx struct{ queries *authzsql.Queries }

func NewPlatformAdministrationAffiliationTx(db postgres.DBTX) (*PlatformAdministrationAffiliationTx, error) {
	if db == nil {
		return nil, errors.New("platform_affiliation_tx_invalid")
	}
	return &PlatformAdministrationAffiliationTx{queries: authzsql.New(db)}, nil
}

func (tx *PlatformAdministrationAffiliationTx) HasTenantTies(ctx context.Context, query authzapp.PlatformAffiliationQuery) (bool, error) {
	if ctx == nil || query.UserID.IsZero() || query.AffiliationKey == "" || query.ObservedAt.IsZero() {
		return false, errors.New("platform_affiliation_query_invalid")
	}
	hasTies, err := tx.queries.PlatformIdentityHasTenantTies(ctx, authzsql.PlatformIdentityHasTenantTiesParams{
		UserID: platformAuthorizationID(query.UserID), EmailLookup: query.AffiliationKey,
		ObservedAt: pgtype.Timestamptz{Time: query.ObservedAt.UTC(), Valid: true},
	})
	if err != nil {
		return false, err
	}
	if hasTies == nil {
		return false, errors.New("platform_affiliation_result_invalid")
	}
	return *hasTies, nil
}

func (tx *PlatformAdministrationAffiliationTx) HasAffectedTenantTie(ctx context.Context, userID, tenantID authzapp.ID) (bool, error) {
	if ctx == nil || userID.IsZero() || tenantID.IsZero() {
		return false, errors.New("platform_financial_affiliation_query_invalid")
	}
	value, err := tx.queries.FinancialActorHasAffectedTenantTie(ctx, authzsql.FinancialActorHasAffectedTenantTieParams{UserID: platformAuthorizationID(userID), TenantID: platformAuthorizationID(tenantID)})
	if err != nil {
		return false, err
	}
	return value, nil
}

func platformAuthorizationID(id authzapp.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: !id.IsZero()}
}
