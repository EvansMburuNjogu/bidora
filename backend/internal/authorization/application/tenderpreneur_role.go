package application

import "errors"

type TenderpreneurRole string

const (
	OrganizationAdmin TenderpreneurRole = "ORGANIZATION_ADMIN"
	BidManager        TenderpreneurRole = "BID_MANAGER"
	Contributor       TenderpreneurRole = "CONTRIBUTOR"
	Approver          TenderpreneurRole = "APPROVER"
)

const TenderpreneurRoleBundleV1 = "ROLE_BUNDLE_V1"
const TenderpreneurRoleBundleV2 = "ROLE_BUNDLE_V2"

var legacyTenderpreneurCapabilities = map[TenderpreneurRole][]string{
	OrganizationAdmin: {"tenant.memberships.manage", "tenderpreneur.organization.letterhead.manage", "tenderpreneur.organization.letterhead.read", "tenderpreneur.organization.profile.manage", "tenderpreneur.organization.profile.read", "tenderpreneur.organization.subscription.manage", "tenderpreneur.organization.subscription.read"},
	BidManager:        {"tenderpreneur.bid.archive", "tenderpreneur.bid.decision.record", "tenderpreneur.bid.outcome.report", "tenderpreneur.bid.read", "tenderpreneur.bid.readiness.read", "tenderpreneur.bid.requirement.assign", "tenderpreneur.bid.requirement.manage", "tenderpreneur.bid.response.edit", "tenderpreneur.bid.workspace.create", "tenderpreneur.opportunity.capture_external", "tenderpreneur.opportunity.document.upload", "tenderpreneur.opportunity.read", "tenderpreneur.opportunity.verify_external", "tenderpreneur.submission.report"},
	Contributor:       {"tenderpreneur.bid.read", "tenderpreneur.bid.response.edit", "tenderpreneur.opportunity.document.upload", "tenderpreneur.opportunity.read"},
	Approver:          {"tenderpreneur.bid.proposal.approve", "tenderpreneur.bid.read", "tenderpreneur.bid.readiness.read", "tenderpreneur.opportunity.read", "tenderpreneur.submission.verify"},
}
var fixedTenderpreneurCapabilities = map[TenderpreneurRole][]string{
	OrganizationAdmin: {"tenant.memberships.manage", "tenderpreneur.organization.letterhead.manage", "tenderpreneur.organization.letterhead.read", "tenderpreneur.organization.profile.manage", "tenderpreneur.organization.profile.read", "tenderpreneur.organization.subscription.manage", "tenderpreneur.organization.subscription.read"},
	BidManager:        {"tenderpreneur.bid.archive", "tenderpreneur.bid.decision.record", "tenderpreneur.bid.outcome.report", "tenderpreneur.bid.proposal.review", "tenderpreneur.bid.read", "tenderpreneur.bid.readiness.read", "tenderpreneur.bid.requirement.assign", "tenderpreneur.bid.requirement.manage", "tenderpreneur.bid.response.edit", "tenderpreneur.bid.workspace.create", "tenderpreneur.opportunity.capture_external", "tenderpreneur.opportunity.document.upload", "tenderpreneur.opportunity.read", "tenderpreneur.opportunity.verify_external", "tenderpreneur.submission.report"},
	Contributor:       {"tenderpreneur.bid.proposal.review", "tenderpreneur.bid.read", "tenderpreneur.bid.response.edit", "tenderpreneur.opportunity.document.upload", "tenderpreneur.opportunity.read"},
	Approver:          {"tenderpreneur.bid.proposal.approve", "tenderpreneur.bid.proposal.review", "tenderpreneur.bid.read", "tenderpreneur.bid.readiness.read", "tenderpreneur.opportunity.read", "tenderpreneur.submission.verify"},
}

func ParseTenderpreneurRole(value string) (TenderpreneurRole, error) {
	role := TenderpreneurRole(value)
	if _, ok := fixedTenderpreneurCapabilities[role]; !ok {
		return "", errors.New("tenderpreneur_role_invalid")
	}
	return role, nil
}

func TenderpreneurRoleCapabilities(role TenderpreneurRole) ([]string, error) {
	return TenderpreneurRoleCapabilitiesForBundle(role, TenderpreneurRoleBundleV2)
}
func TenderpreneurRoleCapabilitiesForBundle(role TenderpreneurRole, bundle string) ([]string, error) {
	profiles := fixedTenderpreneurCapabilities
	if bundle == TenderpreneurRoleBundleV1 {
		profiles = legacyTenderpreneurCapabilities
	} else if bundle != TenderpreneurRoleBundleV2 {
		return nil, errors.New("tenderpreneur_role_bundle_invalid")
	}
	capabilities, ok := profiles[role]
	if !ok {
		return nil, errors.New("tenderpreneur_role_invalid")
	}
	return append([]string(nil), capabilities...), nil
}
