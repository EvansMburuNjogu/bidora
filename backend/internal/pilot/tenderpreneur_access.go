package pilot

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	identityapp "bidora/backend/internal/identity/application"
)

type tenderpreneurMembershipDirectory interface {
	List(context.Context, authzapp.ID) ([]authzapp.MembershipSummary, error)
}

// tenderpreneurPortalAccess adapts the authorization-owned active membership
// directory into navigation eligibility. Membership presence permits only the
// allowlisted portal destination; every operation still reauthorizes.
type tenderpreneurPortalAccess struct {
	directory tenderpreneurMembershipDirectory
}

func newTenderpreneurPortalAccess(directory tenderpreneurMembershipDirectory) (*tenderpreneurPortalAccess, error) {
	if directory == nil {
		return nil, errors.New("tenderpreneur_portal_access_invalid")
	}
	return &tenderpreneurPortalAccess{directory: directory}, nil
}

func (a *tenderpreneurPortalAccess) HasActiveTenderpreneurMembership(ctx context.Context, userID identityapp.ID) (bool, error) {
	if ctx == nil || userID.IsZero() || a == nil || a.directory == nil {
		return false, errors.New("tenderpreneur_portal_access_invalid")
	}
	memberships, err := a.directory.List(ctx, authzapp.ID(userID))
	if err != nil {
		return false, err
	}
	return len(memberships) > 0, nil
}

var _ identityapp.TenderpreneurPortalResolver = (*tenderpreneurPortalAccess)(nil)
