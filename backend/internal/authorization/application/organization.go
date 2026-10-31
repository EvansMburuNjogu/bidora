package application

import (
	"context"
	"errors"
)

const (
	OrganizationProfileReadCapability      = "tenderpreneur.organization.profile.read"
	OrganizationProfileManageCapability    = "tenderpreneur.organization.profile.manage"
	OrganizationLetterheadReadCapability   = "tenderpreneur.organization.letterhead.read"
	OrganizationLetterheadManageCapability = "tenderpreneur.organization.letterhead.manage"
)

type OrganizationAuthorizer struct {
	reader OpportunityWorkflowGrantReader
}

func NewOrganizationAuthorizer(reader OpportunityWorkflowGrantReader) (*OrganizationAuthorizer, error) {
	if reader == nil {
		return nil, errors.New("organization_authorizer_invalid")
	}
	return &OrganizationAuthorizer{reader}, nil
}
func (a *OrganizationAuthorizer) Authorize(ctx context.Context, subject WorkflowSubject, capability string) (WorkflowGrant, error) {
	if ctx == nil || subject.UserID.IsZero() || subject.TenantID.IsZero() || subject.ContextGeneration < 1 || !organizationCapability(capability) {
		return WorkflowGrant{}, safe("organization_not_authorized", nil)
	}
	grant, err := a.reader.FindWorkflowGrant(ctx, subject, capability)
	if errors.Is(err, ErrNotAuthorized) {
		return WorkflowGrant{}, safe("organization_not_authorized", nil)
	}
	if err != nil {
		return WorkflowGrant{}, safe("authorization_unavailable", err)
	}
	if grant.TenantID != subject.TenantID || grant.AssignmentGeneration < 1 {
		return WorkflowGrant{}, safe("authorization_unavailable", nil)
	}
	return grant, nil
}
func organizationCapability(value string) bool {
	switch value {
	case OrganizationProfileReadCapability, OrganizationProfileManageCapability, OrganizationLetterheadReadCapability, OrganizationLetterheadManageCapability:
		return true
	}
	return false
}
