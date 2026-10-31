package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzpersistence "bidora/backend/internal/authorization/persistence"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/postgres"
)

// FinancialAuthorityTx exposes a transaction-bound owner adapter for Commercial
// maker/checker operations without exposing Platform Administration tables.
type FinancialAuthorityTx struct {
	authority   *authorityTx
	affiliation *authzpersistence.PlatformAdministrationAffiliationTx
}

func NewFinancialAuthorityTx(db postgres.DBTX) (*FinancialAuthorityTx, error) {
	if db == nil {
		return nil, errors.New("platform_financial_authority_tx_invalid")
	}
	authority, err := newAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	affiliation, err := authzpersistence.NewPlatformAdministrationAffiliationTx(db)
	if err != nil {
		return nil, err
	}
	return &FinancialAuthorityTx{authority: authority, affiliation: affiliation}, nil
}

func (tx *FinancialAuthorityTx) ResolveGlobal(ctx context.Context, query platformapp.AuthorityQuery) (platformapp.AuthorityFacts, bool, error) {
	if ctx == nil {
		return platformapp.AuthorityFacts{}, false, errors.New("platform_financial_authority_query_invalid")
	}
	identity, err := tx.authority.GetIdentityAuthority(ctx, query)
	if err != nil {
		return platformapp.AuthorityFacts{}, false, err
	}
	facts, err := platformapp.ResolveAuthorityWithinTransaction(ctx, tx.authority, query)
	if err != nil {
		return platformapp.AuthorityFacts{}, false, err
	}
	tie, err := tx.affiliation.HasTenantTies(ctx, authzapp.PlatformAffiliationQuery{UserID: authzapp.ID(query.UserID), AffiliationKey: identity.AffiliationKey, ObservedAt: query.ObservedAt})
	if err != nil {
		return platformapp.AuthorityFacts{}, false, err
	}
	return facts, tie, nil
}

func (tx *FinancialAuthorityTx) Resolve(ctx context.Context, query platformapp.AuthorityQuery, tenantID platformapp.ID) (platformapp.AuthorityFacts, bool, error) {
	if ctx == nil || tenantID.IsZero() {
		return platformapp.AuthorityFacts{}, false, errors.New("platform_financial_authority_query_invalid")
	}
	facts, err := platformapp.ResolveAuthorityWithinTransaction(ctx, tx.authority, query)
	if err != nil {
		return platformapp.AuthorityFacts{}, false, err
	}
	tie, err := tx.affiliation.HasAffectedTenantTie(ctx, authzapp.ID(query.UserID), authzapp.ID(tenantID))
	if err != nil {
		return platformapp.AuthorityFacts{}, false, err
	}
	return facts, tie, nil
}
