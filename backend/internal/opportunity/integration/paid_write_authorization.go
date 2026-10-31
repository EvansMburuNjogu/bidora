package integration

import (
	authzapp "bidora/backend/internal/authorization/application"
	commercialdomain "bidora/backend/internal/commercial/domain"
	"context"
	"errors"
)

type PaidWriteAccessGuard interface {
	RequirePaidWriteAccess(context.Context, commercialdomain.ID) error
}
type PaidWriteWorkflowAuthorizer struct {
	control ControlWorkflowAuthorizer
	guard   PaidWriteAccessGuard
}

func NewPaidWriteWorkflowAuthorizer(control ControlWorkflowAuthorizer, guard PaidWriteAccessGuard) (*PaidWriteWorkflowAuthorizer, error) {
	if control == nil || guard == nil {
		return nil, errors.New("paid_write_workflow_authorizer_invalid")
	}
	return &PaidWriteWorkflowAuthorizer{control, guard}, nil
}
func NewPaidWorkflowAuthorizationAdapter(control ControlWorkflowAuthorizer, guard PaidWriteAccessGuard) (*WorkflowAuthorizationAdapter, error) {
	wrapped, err := NewPaidWriteWorkflowAuthorizer(control, guard)
	if err != nil {
		return nil, err
	}
	return NewWorkflowAuthorizationAdapter(wrapped)
}
func (a *PaidWriteWorkflowAuthorizer) gate(ctx context.Context, subject authzapp.WorkflowSubject, grant authzapp.WorkflowGrant, err error) (authzapp.WorkflowGrant, error) {
	if err != nil {
		return authzapp.WorkflowGrant{}, err
	}
	if grant.TenantID != subject.TenantID {
		return authzapp.WorkflowGrant{}, errors.New("paid_write_workflow_authorization_inconsistent")
	}
	if err = a.guard.RequirePaidWriteAccess(ctx, commercialdomain.ID(subject.TenantID)); err != nil {
		return authzapp.WorkflowGrant{}, err
	}
	return grant, nil
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeVerification(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeVerification(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeWorkspaceCreation(ctx context.Context, s authzapp.WorkflowSubject, owner authzapp.ID) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeWorkspaceCreation(ctx, s, owner)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeDecision(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeDecision(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeRequirementCreate(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeRequirementCreate(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeRequirementAssign(ctx context.Context, s authzapp.WorkflowSubject, owner authzapp.ID) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeRequirementAssign(ctx, s, owner)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeTenderDocumentUpload(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeTenderDocumentUpload(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeResponseEdit(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeResponseEdit(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeProposalReview(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeProposalReview(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeProposalApproval(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeProposalApproval(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeSubmissionReport(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeSubmissionReport(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeSubmissionVerify(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeSubmissionVerify(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeReportedOutcome(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeReportedOutcome(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeBidArchive(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeBidArchive(ctx, s)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeReviewAssignment(ctx context.Context, s authzapp.WorkflowSubject, user authzapp.ID) (authzapp.WorkflowGrant, error) {
	g, e := a.control.AuthorizeReviewAssignment(ctx, s, user)
	return a.gate(ctx, s, g, e)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeDocumentProcessing(ctx context.Context, user, tenant authzapp.ID, generation int64, capability string) error {
	if e := a.control.AuthorizeDocumentProcessing(ctx, user, tenant, generation, capability); e != nil {
		return e
	}
	return a.guard.RequirePaidWriteAccess(ctx, commercialdomain.ID(tenant))
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeOpportunityRead(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	return a.control.AuthorizeOpportunityRead(ctx, s)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeBidRead(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	return a.control.AuthorizeBidRead(ctx, s)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeReadiness(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	return a.control.AuthorizeReadiness(ctx, s)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeDocumentJobRead(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, error) {
	return a.control.AuthorizeDocumentJobRead(ctx, s)
}
func (a *PaidWriteWorkflowAuthorizer) AuthorizeOwnershipContinuity(ctx context.Context, s authzapp.WorkflowSubject) (authzapp.WorkflowGrant, []authzapp.ContinuityMember, error) {
	return a.control.AuthorizeOwnershipContinuity(ctx, s)
}

type PaidWriteCaptureAuthorizer struct {
	control ControlCaptureAuthorizer
	guard   PaidWriteAccessGuard
}

func NewPaidWriteCaptureAuthorizer(control ControlCaptureAuthorizer, guard PaidWriteAccessGuard) (*PaidWriteCaptureAuthorizer, error) {
	if control == nil || guard == nil {
		return nil, errors.New("paid_write_capture_authorizer_invalid")
	}
	return &PaidWriteCaptureAuthorizer{control, guard}, nil
}
func NewPaidCaptureAuthorizationAdapter(control ControlCaptureAuthorizer, guard PaidWriteAccessGuard) (*CaptureAuthorizationAdapter, error) {
	wrapped, err := NewPaidWriteCaptureAuthorizer(control, guard)
	if err != nil {
		return nil, err
	}
	return NewCaptureAuthorizationAdapter(wrapped)
}
func (a *PaidWriteCaptureAuthorizer) Authorize(ctx context.Context, s authzapp.CaptureSubject) (authzapp.CaptureGrant, error) {
	g, e := a.control.Authorize(ctx, s)
	if e != nil {
		return authzapp.CaptureGrant{}, e
	}
	if g.TenantID != s.TenantID {
		return authzapp.CaptureGrant{}, errors.New("paid_write_capture_authorization_inconsistent")
	}
	if e = a.guard.RequirePaidWriteAccess(ctx, commercialdomain.ID(s.TenantID)); e != nil {
		return authzapp.CaptureGrant{}, e
	}
	return g, nil
}

var _ ControlWorkflowAuthorizer = (*PaidWriteWorkflowAuthorizer)(nil)
var _ ControlCaptureAuthorizer = (*PaidWriteCaptureAuthorizer)(nil)
