package application

import (
	"context"
	"errors"
)

type BootstrapService struct{ uow BootstrapUnitOfWork }

func NewBootstrapService(uow BootstrapUnitOfWork) (*BootstrapService, error) {
	if uow == nil {
		return nil, errors.New("platform_bootstrap_service_invalid")
	}
	return &BootstrapService{uow: uow}, nil
}

func (s *BootstrapService) Bootstrap(ctx context.Context, commit BootstrapCommit) (result BootstrapResult, err error) {
	if ctx == nil || s == nil || s.uow == nil || commit.OwnerUserID.IsZero() || commit.AssignmentID.IsZero() || commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.CorrelationID.IsZero() || commit.OccurredAt.IsZero() {
		return result, errors.New("platform_bootstrap_invalid")
	}
	at := commit.OccurredAt.UTC()
	err = s.uow.WithinBootstrap(ctx, func(txCtx context.Context, tx BootstrapTx) error {
		governance, txErr := tx.LockGovernance(txCtx)
		if txErr != nil {
			return txErr
		}
		if governance.BootstrapConsumed || governance.Version < 1 {
			return ErrBootstrapConsumed
		}
		history, txErr := tx.CountOwnerHistory(txCtx)
		if txErr != nil {
			return txErr
		}
		if history != 0 {
			return ErrBootstrapConsumed
		}
		exists, txErr := tx.ProfileExists(txCtx, commit.OwnerUserID)
		if txErr != nil {
			return txErr
		}
		if exists {
			return ErrBootstrapIneligible
		}
		identity, txErr := tx.LockIdentity(txCtx, commit.OwnerUserID, at)
		if txErr != nil {
			return txErr
		}
		if identity.UserID != commit.OwnerUserID || identity.AuthEpoch < 1 || identity.AffiliationKey == "" {
			return ErrBootstrapIneligible
		}
		tied, txErr := tx.HasTenantTies(txCtx, identity.UserID, identity.AffiliationKey, at)
		if txErr != nil {
			return txErr
		}
		if tied {
			return ErrBootstrapIneligible
		}
		newEpoch, txErr := tx.PromoteIdentity(txCtx, identity.UserID, identity.AuthEpoch, at)
		if txErr != nil {
			return txErr
		}
		if newEpoch != identity.AuthEpoch+1 {
			return ErrConcurrentChange
		}
		revoked, txErr := tx.RevokeSessions(txCtx, identity.UserID, at)
		if txErr != nil {
			return txErr
		}
		if txErr = tx.CreateAdministratorProfile(txCtx, identity.UserID, at); txErr != nil {
			return txErr
		}
		if txErr = tx.AssignInitialOwner(txCtx, commit.AssignmentID, identity.UserID, at); txErr != nil {
			return txErr
		}
		governanceVersion, txErr := tx.ConsumeBootstrap(txCtx, identity.UserID, governance.Version, at)
		if txErr != nil {
			return txErr
		}
		if governanceVersion != governance.Version+1 {
			return ErrConcurrentChange
		}
		if txErr = tx.AppendBootstrapEvidence(txCtx, BootstrapEvidence{
			AuditID: commit.AuditID, OutboxID: commit.OutboxID, CorrelationID: commit.CorrelationID,
			OwnerUserID: identity.UserID, GovernanceVersion: governanceVersion, OccurredAt: at,
		}); txErr != nil {
			return txErr
		}
		result = BootstrapResult{OwnerUserID: identity.UserID, AuthEpoch: newEpoch, ContextGeneration: 1, GovernanceVersion: governanceVersion, RevokedSessions: revoked}
		return nil
	})
	return result, err
}
