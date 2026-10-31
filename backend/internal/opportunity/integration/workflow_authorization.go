package integration

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	opportunityapp "bidora/backend/internal/opportunity/application"
)

type ControlWorkflowAuthorizer interface {
	AuthorizeVerification(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeWorkspaceCreation(context.Context, authzapp.WorkflowSubject, authzapp.ID) (authzapp.WorkflowGrant, error)
	AuthorizeDecision(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeOpportunityRead(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeBidRead(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeRequirementCreate(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeRequirementAssign(context.Context, authzapp.WorkflowSubject, authzapp.ID) (authzapp.WorkflowGrant, error)
	AuthorizeReadiness(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeTenderDocumentUpload(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeDocumentJobRead(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeResponseEdit(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeProposalReview(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeProposalApproval(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeSubmissionReport(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeSubmissionVerify(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeReportedOutcome(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeBidArchive(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error)
	AuthorizeReviewAssignment(context.Context, authzapp.WorkflowSubject, authzapp.ID) (authzapp.WorkflowGrant, error)
	AuthorizeOwnershipContinuity(context.Context, authzapp.WorkflowSubject) (authzapp.WorkflowGrant, []authzapp.ContinuityMember, error)
	AuthorizeDocumentProcessing(context.Context, authzapp.ID, authzapp.ID, int64, string) error
}
type WorkflowAuthorizationAdapter struct{ control ControlWorkflowAuthorizer }

func NewWorkflowAuthorizationAdapter(control ControlWorkflowAuthorizer) (*WorkflowAuthorizationAdapter, error) {
	if control == nil {
		return nil, errors.New("workflow_authorization_adapter_invalid")
	}
	return &WorkflowAuthorizationAdapter{control}, nil
}
func (a *WorkflowAuthorizationAdapter) AuthorizeVerification(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeVerification(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeWorkspaceCreation(ctx context.Context, subject opportunityapp.WorkflowSubject, ownerID opportunityapp.ID) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeWorkspaceCreation(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration}, authzapp.ID(ownerID))
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeDocumentProcessing(ctx context.Context, userID, tenantID opportunityapp.ID, assignmentGeneration int64, capability string) error {
	return a.control.AuthorizeDocumentProcessing(ctx, authzapp.ID(userID), authzapp.ID(tenantID), assignmentGeneration, capability)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeOwnershipContinuity(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, []opportunityapp.ContinuityMember, error) {
	grant, members, err := a.control.AuthorizeOwnershipContinuity(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	authority, err := workflowAuthority(subject, grant, err)
	if err != nil {
		return opportunityapp.WorkflowAuthority{}, nil, err
	}
	mapped := make([]opportunityapp.ContinuityMember, len(members))
	for i, m := range members {
		mapped[i] = opportunityapp.ContinuityMember{MembershipID: opportunityapp.ID(m.MembershipID), UserID: opportunityapp.ID(m.UserID), ContextGeneration: m.ContextGeneration}
	}
	return authority, mapped, nil
}
func (a *WorkflowAuthorizationAdapter) AuthorizeReviewAssignment(ctx context.Context, subject opportunityapp.WorkflowSubject, assigneeID opportunityapp.ID) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeReviewAssignment(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration}, authzapp.ID(assigneeID))
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeResponseEdit(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeResponseEdit(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeProposalReview(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeProposalReview(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeProposalApproval(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeProposalApproval(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeSubmissionReport(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeSubmissionReport(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeSubmissionVerify(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeSubmissionVerify(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeReportedOutcome(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeReportedOutcome(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeBidArchive(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeBidArchive(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeDocumentJobRead(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeDocumentJobRead(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeTenderDocumentUpload(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeTenderDocumentUpload(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeRequirementCreate(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeRequirementCreate(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeRequirementAssign(ctx context.Context, subject opportunityapp.WorkflowSubject, ownerID opportunityapp.ID) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeRequirementAssign(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration}, authzapp.ID(ownerID))
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeReadiness(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeReadiness(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeOpportunityRead(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeOpportunityRead(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeBidRead(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeBidRead(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func (a *WorkflowAuthorizationAdapter) AuthorizeDecision(ctx context.Context, subject opportunityapp.WorkflowSubject) (opportunityapp.WorkflowAuthority, error) {
	grant, err := a.control.AuthorizeDecision(ctx, authzapp.WorkflowSubject{UserID: authzapp.ID(subject.IdentityID), TenantID: authzapp.ID(subject.SelectedTenantID), ContextGeneration: subject.ContextGeneration})
	return workflowAuthority(subject, grant, err)
}
func workflowAuthority(subject opportunityapp.WorkflowSubject, grant authzapp.WorkflowGrant, err error) (opportunityapp.WorkflowAuthority, error) {
	if err != nil {
		return opportunityapp.WorkflowAuthority{}, err
	}
	if grant.TenantID != authzapp.ID(subject.SelectedTenantID) || grant.AssignmentGeneration < 1 {
		return opportunityapp.WorkflowAuthority{}, errors.New("workflow_authorization_inconsistent")
	}
	return opportunityapp.WorkflowAuthority{ActorID: subject.IdentityID, TenantID: subject.SelectedTenantID, AssignmentGeneration: grant.AssignmentGeneration}, nil
}
