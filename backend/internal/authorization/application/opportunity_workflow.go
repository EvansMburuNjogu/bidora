package application

import (
	"context"
	"errors"
)

const VerifyExternalOpportunityCapability = "tenderpreneur.opportunity.verify_external"
const CreateBidWorkspaceCapability = "tenderpreneur.bid.workspace.create"
const RecordBidDecisionCapability = "tenderpreneur.bid.decision.record"
const ReadOpportunityCapability = "tenderpreneur.opportunity.read"
const ReadBidCapability = "tenderpreneur.bid.read"
const ManageRequirementCapability = "tenderpreneur.bid.requirement.manage"
const AssignRequirementCapability = "tenderpreneur.bid.requirement.assign"
const ReadReadinessCapability = "tenderpreneur.bid.readiness.read"
const UploadTenderDocumentCapability = "tenderpreneur.opportunity.document.upload"
const EditTechnicalResponseCapability = "tenderpreneur.bid.response.edit"
const ManageTenantMembershipCapability = "tenant.memberships.manage"
const ReviewProposalCapability = "tenderpreneur.bid.proposal.review"
const ApproveProposalCapability = "tenderpreneur.bid.proposal.approve"
const ReportSubmissionCapability = "tenderpreneur.submission.report"
const VerifySubmissionCapability = "tenderpreneur.submission.verify"
const ReportOutcomeCapability = "tenderpreneur.bid.outcome.report"
const ArchiveBidCapability = "tenderpreneur.bid.archive"

type WorkflowSubject struct {
	UserID, TenantID  ID
	ContextGeneration int64
}
type WorkflowGrant struct {
	TenantID             ID
	AssignmentGeneration int64
}
type ContinuityMember struct {
	MembershipID, UserID ID
	ContextGeneration    int64
}
type OpportunityWorkflowGrantReader interface {
	FindWorkflowGrant(context.Context, WorkflowSubject, string) (WorkflowGrant, error)
	ValidateWorkspaceOwner(context.Context, ID, ID) error
	ValidateCurrentCapability(context.Context, ID, ID, int64, string) error
	ListActiveContinuityMembers(context.Context, ID) ([]ContinuityMember, error)
}
type OpportunityWorkflowAuthorizer struct {
	reader OpportunityWorkflowGrantReader
}

func NewOpportunityWorkflowAuthorizer(reader OpportunityWorkflowGrantReader) (*OpportunityWorkflowAuthorizer, error) {
	if reader == nil {
		return nil, errors.New("opportunity_workflow_authorizer_invalid")
	}
	return &OpportunityWorkflowAuthorizer{reader}, nil
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeVerification(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, VerifyExternalOpportunityCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeDocumentProcessing(ctx context.Context, userID, tenantID ID, assignmentGeneration int64, capability string) error {
	if ctx == nil || userID.IsZero() || tenantID.IsZero() || assignmentGeneration < 1 || capability != UploadTenderDocumentCapability && capability != ManageRequirementCapability && capability != EditTechnicalResponseCapability && capability != ReportSubmissionCapability && capability != ReportOutcomeCapability {
		return safe("workflow_not_authorized", nil)
	}
	if a == nil || a.reader == nil {
		return safe("authorization_unavailable", nil)
	}
	if err := a.reader.ValidateCurrentCapability(ctx, userID, tenantID, assignmentGeneration, capability); errors.Is(err, ErrNotAuthorized) {
		return safe("workflow_not_authorized", nil)
	} else if err != nil {
		return safe("authorization_unavailable", err)
	}
	return nil
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeDocumentJobRead(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	grant, err := a.authorize(ctx, subject, UploadTenderDocumentCapability)
	if err == nil {
		return grant, nil
	}
	grant, err = a.authorize(ctx, subject, ManageRequirementCapability)
	if err == nil {
		return grant, nil
	}
	grant, err = a.authorize(ctx, subject, ReportSubmissionCapability)
	if err == nil {
		return grant, nil
	}
	return a.authorize(ctx, subject, ReportOutcomeCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeTenderDocumentUpload(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, UploadTenderDocumentCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeRequirementCreate(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ManageRequirementCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeRequirementAssign(ctx context.Context, subject WorkflowSubject, ownerID ID) (WorkflowGrant, error) {
	if ownerID.IsZero() {
		return WorkflowGrant{}, safe("workflow_not_authorized", nil)
	}
	grant, err := a.authorize(ctx, subject, AssignRequirementCapability)
	if err != nil {
		return WorkflowGrant{}, err
	}
	if err = a.reader.ValidateWorkspaceOwner(ctx, subject.TenantID, ownerID); err != nil {
		return WorkflowGrant{}, safe("workflow_not_authorized", err)
	}
	return grant, nil
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeReadiness(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ReadReadinessCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeOpportunityRead(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ReadOpportunityCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeBidRead(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ReadBidCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeOwnershipContinuity(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, []ContinuityMember, error) {
	grant, err := a.authorize(ctx, subject, ManageTenantMembershipCapability)
	if err != nil {
		return WorkflowGrant{}, nil, err
	}
	members, err := a.reader.ListActiveContinuityMembers(ctx, subject.TenantID)
	if err != nil {
		return WorkflowGrant{}, nil, safe("authorization_unavailable", err)
	}
	if len(members) > 200 {
		return WorkflowGrant{}, nil, safe("ownership_continuity_limit", nil)
	}
	return grant, members, nil
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeResponseEdit(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, EditTechnicalResponseCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeProposalReview(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ReviewProposalCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeProposalApproval(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ApproveProposalCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeSubmissionReport(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ReportSubmissionCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeSubmissionVerify(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, VerifySubmissionCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeReportedOutcome(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ReportOutcomeCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeBidArchive(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, ArchiveBidCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeReviewAssignment(ctx context.Context, subject WorkflowSubject, assigneeID ID) (WorkflowGrant, error) {
	if assigneeID.IsZero() {
		return WorkflowGrant{}, safe("workflow_not_authorized", nil)
	}
	grant, err := a.authorize(ctx, subject, ReviewProposalCapability)
	if err != nil {
		return WorkflowGrant{}, err
	}
	if err = a.reader.ValidateWorkspaceOwner(ctx, subject.TenantID, assigneeID); err != nil {
		return WorkflowGrant{}, safe("workflow_not_authorized", err)
	}
	if err = a.reader.ValidateCurrentCapability(ctx, assigneeID, subject.TenantID, grant.AssignmentGeneration, ReadBidCapability); err != nil {
		return WorkflowGrant{}, safe("workflow_not_authorized", err)
	}
	return grant, nil
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeDecision(ctx context.Context, subject WorkflowSubject) (WorkflowGrant, error) {
	return a.authorize(ctx, subject, RecordBidDecisionCapability)
}
func (a *OpportunityWorkflowAuthorizer) AuthorizeWorkspaceCreation(ctx context.Context, subject WorkflowSubject, ownerID ID) (WorkflowGrant, error) {
	if ownerID.IsZero() {
		return WorkflowGrant{}, safe("workflow_not_authorized", nil)
	}
	grant, err := a.authorize(ctx, subject, CreateBidWorkspaceCapability)
	if err != nil {
		return WorkflowGrant{}, err
	}
	if err = a.reader.ValidateWorkspaceOwner(ctx, subject.TenantID, ownerID); err != nil {
		return WorkflowGrant{}, safe("workflow_not_authorized", err)
	}
	return grant, nil
}
func (a *OpportunityWorkflowAuthorizer) authorize(ctx context.Context, subject WorkflowSubject, capability string) (WorkflowGrant, error) {
	if ctx == nil || subject.UserID.IsZero() || subject.TenantID.IsZero() || subject.ContextGeneration < 1 {
		return WorkflowGrant{}, safe("workflow_not_authorized", nil)
	}
	if a == nil || a.reader == nil {
		return WorkflowGrant{}, safe("authorization_unavailable", nil)
	}
	grant, err := a.reader.FindWorkflowGrant(ctx, subject, capability)
	if errors.Is(err, ErrNotAuthorized) {
		return WorkflowGrant{}, safe("workflow_not_authorized", nil)
	}
	if err != nil {
		return WorkflowGrant{}, safe("authorization_unavailable", err)
	}
	if grant.TenantID != subject.TenantID || grant.AssignmentGeneration < 1 {
		return WorkflowGrant{}, safe("authorization_unavailable", nil)
	}
	return grant, nil
}
