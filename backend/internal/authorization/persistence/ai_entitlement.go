package persistence

import (
	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

type AIEntitlementAuthorityTx struct{ queries *authzsql.Queries }

func NewAIEntitlementAuthorityTx(db postgres.DBTX) (*AIEntitlementAuthorityTx, error) {
	if db == nil {
		return nil, errors.New("ai_entitlement_authority_tx_invalid")
	}
	return &AIEntitlementAuthorityTx{authzsql.New(db)}, nil
}
func (t *AIEntitlementAuthorityTx) CurrentCapabilityAllowed(ctx context.Context, userID, tenantID authzapp.ID, assignmentGeneration int64, capability string) (bool, error) {
	if ctx == nil || userID.IsZero() || tenantID.IsZero() || assignmentGeneration < 1 || !aiCapability(capability) {
		return false, nil
	}
	row, err := t.queries.LockCurrentBackgroundCapability(ctx, authzsql.LockCurrentBackgroundCapabilityParams{ActorUserID: authorizationID(userID), TenantID: authorizationID(tenantID), Capability: capability})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.AssignmentGeneration == assignmentGeneration, nil
}
func aiCapability(value string) bool {
	return value == authzapp.UploadTenderDocumentCapability || value == authzapp.ManageRequirementCapability || value == authzapp.EditTechnicalResponseCapability
}
