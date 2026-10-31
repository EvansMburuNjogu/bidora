package integration

import (
	auth "bidora/backend/internal/authorization/application"
	organization "bidora/backend/internal/organization/application"
	"context"
	"errors"
)

type ControlAuthorizer interface {
	Authorize(context.Context, auth.WorkflowSubject, string) (auth.WorkflowGrant, error)
}
type AuthorizationAdapter struct{ control ControlAuthorizer }

func NewAuthorizationAdapter(control ControlAuthorizer) (*AuthorizationAdapter, error) {
	if control == nil {
		return nil, errors.New("organization_authorization_adapter_invalid")
	}
	return &AuthorizationAdapter{control}, nil
}
func (a *AuthorizationAdapter) Authorize(ctx context.Context, subject organization.Subject, capability string) (organization.Authority, error) {
	grant, err := a.control.Authorize(ctx, auth.WorkflowSubject{UserID: auth.ID(subject.IdentityID), TenantID: auth.ID(subject.TenantID), ContextGeneration: subject.ContextGeneration}, capability)
	if err != nil {
		return organization.Authority{}, err
	}
	if grant.TenantID != auth.ID(subject.TenantID) || grant.AssignmentGeneration < 1 {
		return organization.Authority{}, errors.New("organization_authorization_inconsistent")
	}
	return organization.Authority{ActorID: subject.IdentityID, TenantID: subject.TenantID, AssignmentGeneration: grant.AssignmentGeneration}, nil
}
