package persistence

import (
	"context"
	"errors"

	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/postgres"
)

// SettingsAuthorityTx exposes only the Platform Administration owner module's
// authority operation to outer transaction composers.
type SettingsAuthorityTx struct{ tx *authorityTx }

func NewSettingsAuthorityTx(db postgres.DBTX) (*SettingsAuthorityTx, error) {
	if db == nil {
		return nil, errors.New("platform_settings_authority_tx_invalid")
	}
	tx, err := newAuthorityTx(db)
	if err != nil {
		return nil, err
	}
	return &SettingsAuthorityTx{tx}, nil
}

// PasswordFallbackAvailable shares the governance fence with owner/identity
// changes before a provider policy can change. Password and MFA stay enabled.
func (t *SettingsAuthorityTx) PasswordFallbackAvailable(ctx context.Context) (bool, error) {
	if t == nil || t.tx == nil {
		return false, errors.New("platform_settings_authority_tx_invalid")
	}
	if _, err := t.tx.platform.LockPlatformGovernanceState(ctx); err != nil {
		return false, err
	}
	return t.tx.platform.HasPlatformPasswordOwner(ctx)
}

func (t *SettingsAuthorityTx) LockConfigurationWriter(ctx context.Context) error {
	if t == nil || t.tx == nil {
		return errors.New("platform_settings_authority_tx_invalid")
	}
	_, err := t.tx.platform.LockPlatformGovernanceState(ctx)
	return err
}

func (t *SettingsAuthorityTx) Resolve(ctx context.Context, query platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	if t == nil || t.tx == nil {
		return platformapp.AuthorityFacts{}, errors.New("platform_settings_authority_tx_invalid")
	}
	return platformapp.ResolveAuthorityWithinTransaction(ctx, t.tx, query)
}
