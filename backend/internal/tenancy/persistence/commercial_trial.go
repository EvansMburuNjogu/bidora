package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	tenancysql "bidora/backend/internal/tenancy/persistence/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CommercialTrialTenantFacts struct {
	Active, Tenderpreneur, ProvisioningComplete bool
}

type CommercialTrialTenantTx struct{ queries *tenancysql.Queries }

func NewCommercialTrialTenantTx(db postgres.DBTX) (*CommercialTrialTenantTx, error) {
	if db == nil {
		return nil, errors.New("commercial_trial_tenant_tx_invalid")
	}
	return &CommercialTrialTenantTx{queries: tenancysql.New(db)}, nil
}

func (tx *CommercialTrialTenantTx) LockEligibility(ctx context.Context, tenantID tenancyapp.ID) (CommercialTrialTenantFacts, error) {
	row, err := tx.queries.LockCommercialTrialTenant(ctx, pgtype.UUID{Bytes: [16]byte(tenantID), Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return CommercialTrialTenantFacts{}, nil
	}
	if err != nil {
		return CommercialTrialTenantFacts{}, err
	}
	return CommercialTrialTenantFacts{Active: row.State == "ACTIVE", Tenderpreneur: row.TenantType == "TENDERPRENEUR", ProvisioningComplete: row.HasActiveAssignment && row.CompatibleSchemaCount == 2}, nil
}
