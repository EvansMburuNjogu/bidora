package application

import (
	"context"
	"errors"

	"bidora/backend/internal/platformadmin/domain"
)

type AuthorityService struct{ uow AuthorityUnitOfWork }

func NewAuthorityService(uow AuthorityUnitOfWork) (*AuthorityService, error) {
	if uow == nil {
		return nil, errors.New("platform_authority_service_invalid")
	}
	return &AuthorityService{uow: uow}, nil
}

func (s *AuthorityService) GetAuthority(ctx context.Context, query AuthorityQuery) (result AuthorityFacts, err error) {
	if ctx == nil || s == nil || s.uow == nil || query.UserID.IsZero() || query.SessionID.IsZero() || query.ObservedAt.IsZero() {
		return result, errors.New("platform_authority_query_invalid")
	}
	err = s.uow.WithinAuthority(ctx, func(txCtx context.Context, tx AuthorityTx) error {
		var resolveErr error
		result, resolveErr = ResolveAuthorityWithinTransaction(txCtx, tx, query)
		return resolveErr
	})
	return result, err
}

// ResolveAuthorityWithinTransaction evaluates current authority using owner adapters already bound to one control transaction.
func ResolveAuthorityWithinTransaction(ctx context.Context, tx AuthorityTx, query AuthorityQuery) (AuthorityFacts, error) {
	facts, _, err := resolveAuthorityDetails(ctx, tx, query)
	return facts, err
}

func resolveAuthority(ctx context.Context, tx AuthorityTx, query AuthorityQuery) (AuthorityFacts, error) {
	return ResolveAuthorityWithinTransaction(ctx, tx, query)
}

func resolveAuthorityDetails(ctx context.Context, tx AuthorityTx, query AuthorityQuery) (AuthorityFacts, IdentityAuthorityFacts, error) {
	identity, err := tx.GetIdentityAuthority(ctx, query)
	if err != nil {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, err
	}
	platform, err := tx.GetPlatformAuthority(ctx, query.UserID)
	if err != nil {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, err
	}
	if identity.UserID != query.UserID || identity.AffiliationKey == "" || platform.UserID != query.UserID || identity.AuthEpoch < 1 || platform.ContextGeneration < 1 || platform.AdministratorVersion < 1 || len(platform.RoleKeys) < 1 || len(platform.RoleKeys) > 6 {
		return AuthorityFacts{}, IdentityAuthorityFacts{}, ErrAuthorityNotFound
	}
	roles := make([]domain.Role, len(platform.RoleKeys))
	for index, key := range platform.RoleKeys {
		role, parseErr := domain.ParseRole(key)
		if parseErr != nil {
			return AuthorityFacts{}, IdentityAuthorityFacts{}, ErrAuthorityNotFound
		}
		roles[index] = role
	}
	return AuthorityFacts{
		UserID: identity.UserID, AuthEpoch: identity.AuthEpoch,
		ContextGeneration: platform.ContextGeneration, AdministratorVersion: platform.AdministratorVersion,
		AssuranceLevel: identity.AssuranceLevel, AssuranceMethod: identity.AssuranceMethod,
		AssuredAt: identity.AssuredAt.UTC(), IdleExpiresAt: identity.IdleExpiresAt.UTC(),
		AbsoluteExpiresAt: identity.AbsoluteExpiresAt.UTC(), Roles: roles,
	}, identity, nil
}
