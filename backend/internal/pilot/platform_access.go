package pilot

import (
	"context"
	"errors"
	"sort"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	platformadmin "bidora/backend/internal/platformadmin"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
)

type platformMFAStatus interface {
	HasActiveMFA(context.Context, identityapp.ID) (bool, error)
}

// NewPlatformPortalResolver exposes the same resolver to trusted deployment
// diagnostics; it does not issue sessions or bypass identity authority checks.
func NewPlatformPortalResolver(platform *platformadmin.Module, mfa platformMFAStatus) identityapp.PlatformPortalResolver {
	return platformPortalAdapter{platform: platform, mfa: mfa}
}

type platformPortalAdapter struct {
	platform *platformadmin.Module
	mfa      platformMFAStatus
}

func (a platformPortalAdapter) ResolvePlatformPortal(ctx context.Context, principal identityapp.SessionPrincipal, observedAt time.Time) (identityapp.PlatformPortalAccess, error) {
	if ctx == nil || a.platform == nil || a.mfa == nil || principal.UserID.IsZero() || principal.SessionID.IsZero() || observedAt.IsZero() {
		return identityapp.PlatformPortalAccess{}, errors.New("platform_portal_unavailable")
	}
	facts, err := a.platform.GetAuthority(ctx, platformapp.AuthorityQuery{UserID: platformapp.ID(principal.UserID), SessionID: platformapp.ID(principal.SessionID), ObservedAt: observedAt})
	if errors.Is(err, platformapp.ErrAuthorityNotFound) {
		return identityapp.PlatformPortalAccess{}, identityapp.ErrPlatformAccessForbidden
	}
	if err != nil {
		return identityapp.PlatformPortalAccess{}, err
	}
	if facts.UserID != platformapp.ID(principal.UserID) || facts.AuthEpoch != principal.AuthEpoch || facts.ContextGeneration < 1 {
		return identityapp.PlatformPortalAccess{}, identityapp.ErrPlatformAccessForbidden
	}
	enrolled, err := a.mfa.HasActiveMFA(ctx, principal.UserID)
	if err != nil {
		return identityapp.PlatformPortalAccess{}, err
	}
	set := make(map[string]struct{})
	for _, role := range facts.Roles {
		capabilities, capabilityErr := domain.Capabilities(role)
		if capabilityErr != nil {
			return identityapp.PlatformPortalAccess{}, capabilityErr
		}
		for _, capability := range capabilities {
			set[capability] = struct{}{}
		}
	}
	capabilities := make([]string, 0, len(set))
	for capability := range set {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	if len(capabilities) == 0 {
		return identityapp.PlatformPortalAccess{}, identityapp.ErrPlatformAccessForbidden
	}
	return identityapp.PlatformPortalAccess{ContextGeneration: facts.ContextGeneration, Capabilities: capabilities, MFAEnrolled: enrolled, AssuranceLevel: facts.AssuranceLevel, AssuranceMethod: facts.AssuranceMethod, AssuredAt: facts.AssuredAt}, nil
}
