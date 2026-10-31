package persistence

import (
	"context"
	"errors"

	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/postgres"
)

// TenantDirectoryAuthorityTx exposes only current Platform Administration
// authority to the Tenancy-owned read transaction composer.
type TenantDirectoryAuthorityTx struct{ tx *authorityTx }

func NewTenantDirectoryAuthorityTx(db postgres.DBTX) (*TenantDirectoryAuthorityTx, error) {
	if db == nil {
		return nil, errors.New("platform_tenant_directory_authority_tx_invalid")
	}
	tx, err := newAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	return &TenantDirectoryAuthorityTx{tx: tx}, nil
}

func (t *TenantDirectoryAuthorityTx) Resolve(ctx context.Context, query platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	if t == nil || t.tx == nil {
		return platformapp.AuthorityFacts{}, errors.New("platform_tenant_directory_authority_tx_invalid")
	}
	return platformapp.ResolveAuthorityWithinTransaction(ctx, t.tx, query)
}
