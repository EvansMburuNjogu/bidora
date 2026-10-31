package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type CommercialTrialAuthorityTx struct{ queries *authzsql.Queries }

func NewCommercialTrialAuthorityTx(db postgres.DBTX) (*CommercialTrialAuthorityTx, error) {
	if db == nil {
		return nil, errors.New("commercial_trial_authority_tx_invalid")
	}
	return &CommercialTrialAuthorityTx{queries: authzsql.New(db)}, nil
}

func (tx *CommercialTrialAuthorityTx) LockActiveMembership(ctx context.Context, userID, tenantID authzapp.ID, contextGeneration int64) (authzapp.ID, bool, error) {
	row, err := tx.queries.LockCommercialTrialMembership(ctx, authzsql.LockCommercialTrialMembershipParams{UserID: platformAuthorizationID(userID), TenantID: platformAuthorizationID(tenantID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return authzapp.ID{}, false, nil
	}
	if err != nil {
		return authzapp.ID{}, false, err
	}
	if contextGeneration < 1 || row.ContextGeneration != contextGeneration {
		return authzapp.ID{}, false, nil
	}
	return authzapp.ID(row.ID.Bytes), true, nil
}

func (tx *CommercialTrialAuthorityTx) AuthorizeSubscription(ctx context.Context, userID, tenantID authzapp.ID, contextGeneration int64, capability string) (bool, error) {
	if contextGeneration < 1 {
		return false, errors.New("commercial_subscription_context_invalid")
	}
	row, err := tx.queries.LockCommercialTrialMembership(ctx, authzsql.LockCommercialTrialMembershipParams{UserID: platformAuthorizationID(userID), TenantID: platformAuthorizationID(tenantID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.ContextGeneration != contextGeneration {
		return false, nil
	}
	return tx.SubscriptionCapabilityAllowed(ctx, authzapp.ID(row.ID.Bytes), capability)
}

func (tx *CommercialTrialAuthorityTx) SubscriptionCapabilityAllowed(ctx context.Context, membershipID authzapp.ID, capability string) (bool, error) {
	if capability != "tenderpreneur.organization.subscription.read" && capability != "tenderpreneur.organization.subscription.manage" {
		return false, errors.New("commercial_subscription_capability_invalid")
	}
	allowed, err := tx.queries.CommercialSubscriptionCapabilityAllowed(ctx, authzsql.CommercialSubscriptionCapabilityAllowedParams{TargetMembershipID: platformAuthorizationID(membershipID), Capability: capability})
	return allowed != nil && *allowed, err
}

func (tx *CommercialTrialAuthorityTx) SubscriptionManageAllowed(ctx context.Context, membershipID authzapp.ID) (bool, error) {
	allowed, err := tx.queries.CommercialTrialCapabilityAllowed(ctx, platformAuthorizationID(membershipID))
	return allowed != nil && *allowed, err
}
