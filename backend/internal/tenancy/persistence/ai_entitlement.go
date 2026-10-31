package persistence

import (
	"bidora/backend/internal/postgres"
	tenancyapp "bidora/backend/internal/tenancy/application"
	tenancysql "bidora/backend/internal/tenancy/persistence/sqlc"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type AIEntitlementAssignmentFacts struct {
	TenantID   tenancyapp.ID
	Generation int64
	Eligible   bool
}
type AIEntitlementTenantTx struct{ queries *tenancysql.Queries }

func NewAIEntitlementTenantTx(db postgres.DBTX) (*AIEntitlementTenantTx, error) {
	if db == nil {
		return nil, errors.New("ai_entitlement_tenant_tx_invalid")
	}
	return &AIEntitlementTenantTx{tenancysql.New(db)}, nil
}
func (t *AIEntitlementTenantTx) LockAssignment(ctx context.Context, tenantID tenancyapp.ID) (AIEntitlementAssignmentFacts, bool, error) {
	if ctx == nil || tenantID.IsZero() {
		return AIEntitlementAssignmentFacts{}, false, errors.New("ai_entitlement_assignment_invalid")
	}
	row, err := t.queries.LockAIEntitlementTenantAssignment(ctx, pgtype.UUID{Bytes: [16]byte(tenantID), Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return AIEntitlementAssignmentFacts{}, false, nil
	}
	if err != nil {
		return AIEntitlementAssignmentFacts{}, false, err
	}
	return AIEntitlementAssignmentFacts{TenantID: tenancyapp.ID(row.ID.Bytes), Generation: row.AssignmentGeneration, Eligible: row.State == "ACTIVE" && row.TenantType == "TENDERPRENEUR" && row.HasActiveAssignment && row.HasCompatibleCheckpointSchema}, true, nil
}
