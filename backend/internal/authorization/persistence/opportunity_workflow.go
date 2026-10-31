package persistence

import (
	"context"
	"errors"

	authzapp "bidora/backend/internal/authorization/application"
	authzsql "bidora/backend/internal/authorization/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type OpportunityWorkflowGrantReader struct{ runner readOnlyRunner }

func NewOpportunityWorkflowGrantReader(runner readOnlyRunner) (*OpportunityWorkflowGrantReader, error) {
	if runner == nil {
		return nil, errors.New("opportunity_workflow_reader_invalid")
	}
	return &OpportunityWorkflowGrantReader{runner}, nil
}
func (r *OpportunityWorkflowGrantReader) FindWorkflowGrant(ctx context.Context, subject authzapp.WorkflowSubject, capability string) (authzapp.WorkflowGrant, error) {
	if ctx == nil || subject.UserID.IsZero() || subject.TenantID.IsZero() || subject.ContextGeneration < 1 || (capability != authzapp.VerifyExternalOpportunityCapability && capability != authzapp.CreateBidWorkspaceCapability && capability != authzapp.RecordBidDecisionCapability && capability != authzapp.ReadOpportunityCapability && capability != authzapp.ReadBidCapability && capability != authzapp.ManageRequirementCapability && capability != authzapp.AssignRequirementCapability && capability != authzapp.ReadReadinessCapability && capability != authzapp.UploadTenderDocumentCapability && capability != authzapp.EditTechnicalResponseCapability && capability != authzapp.ReviewProposalCapability && capability != authzapp.ApproveProposalCapability && capability != authzapp.ReportSubmissionCapability && capability != authzapp.VerifySubmissionCapability && capability != authzapp.ReportOutcomeCapability && capability != authzapp.ArchiveBidCapability && capability != authzapp.ManageTenantMembershipCapability) {
		return authzapp.WorkflowGrant{}, authzapp.ErrNotAuthorized
	}
	var result authzapp.WorkflowGrant
	err := r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := authzsql.New(db).AuthorizeOpportunityWorkflow(txCtx, authzsql.AuthorizeOpportunityWorkflowParams{ActorUserID: authorizationID(subject.UserID), TenantID: authorizationID(subject.TenantID), ContextGeneration: subject.ContextGeneration, Capability: capability})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrNotAuthorized
		}
		if err != nil {
			return err
		}
		result = authzapp.WorkflowGrant{TenantID: subject.TenantID, AssignmentGeneration: row.AssignmentGeneration}
		return nil
	})
	return result, err
}
func (r *OpportunityWorkflowGrantReader) ValidateCurrentCapability(ctx context.Context, userID, tenantID authzapp.ID, assignmentGeneration int64, capability string) error {
	if ctx == nil || userID.IsZero() || tenantID.IsZero() || assignmentGeneration < 1 || (capability != authzapp.UploadTenderDocumentCapability && capability != authzapp.ManageRequirementCapability && capability != authzapp.EditTechnicalResponseCapability && capability != authzapp.ReadBidCapability && capability != authzapp.ReportSubmissionCapability && capability != authzapp.ReportOutcomeCapability) {
		return authzapp.ErrNotAuthorized
	}
	return r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := authzsql.New(db).AuthorizeCurrentBackgroundCapability(txCtx, authzsql.AuthorizeCurrentBackgroundCapabilityParams{ActorUserID: authorizationID(userID), TenantID: authorizationID(tenantID), Capability: capability})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrNotAuthorized
		}
		if err != nil {
			return err
		}
		if row.AssignmentGeneration != assignmentGeneration {
			return authzapp.ErrNotAuthorized
		}
		return nil
	})
}
func (r *OpportunityWorkflowGrantReader) ListActiveContinuityMembers(ctx context.Context, tenantID authzapp.ID) (result []authzapp.ContinuityMember, err error) {
	if ctx == nil || tenantID.IsZero() {
		return nil, authzapp.ErrNotAuthorized
	}
	err = r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := authzsql.New(db).ListActiveContinuityMembers(txCtx, authorizationID(tenantID))
		if err != nil {
			return err
		}
		result = make([]authzapp.ContinuityMember, len(rows))
		for i, row := range rows {
			result[i] = authzapp.ContinuityMember{MembershipID: authzapp.ID(row.MembershipID.Bytes), UserID: authzapp.ID(row.UserID.Bytes), ContextGeneration: row.ContextGeneration}
		}
		return nil
	})
	return result, err
}
func (r *OpportunityWorkflowGrantReader) ValidateWorkspaceOwner(ctx context.Context, tenantID, ownerID authzapp.ID) error {
	if ctx == nil || tenantID.IsZero() || ownerID.IsZero() {
		return authzapp.ErrNotAuthorized
	}
	return r.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		membership, err := authzsql.New(db).AuthorizeWorkspaceOwner(txCtx, authzsql.AuthorizeWorkspaceOwnerParams{OwnerUserID: authorizationID(ownerID), TenantID: authorizationID(tenantID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return authzapp.ErrNotAuthorized
		}
		if err != nil {
			return err
		}
		if !membership.Valid {
			return authzapp.ErrNotAuthorized
		}
		return nil
	})
}
