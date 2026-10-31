package persistence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type workflowRunner interface {
	transactionRunner
	WithinReadOnlyTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type WorkflowStore struct {
	runner               workflowRunner
	tenantID             application.ID
	assignmentGeneration int64
}

func NewWorkflowStore(runner workflowRunner, tenantID application.ID, generation int64) (*WorkflowStore, error) {
	if runner == nil || tenantID.IsZero() || generation < 1 {
		return nil, errors.New("workflow_store_invalid")
	}
	return &WorkflowStore{runner: runner, tenantID: tenantID, assignmentGeneration: generation}, nil
}
func (s *WorkflowStore) Verify(ctx context.Context, authority application.WorkflowAuthority, commit application.VerifyCommit) (result application.VerificationResult, err error) {
	if err = s.valid(ctx, authority); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, version, err := claimWorkflow(txCtx, q, commit.CommandID, authority.ActorID, "VERIFY_EXTERNAL_OPPORTUNITY", commit.KeyHash, commit.RequestHash, commit.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			result = application.VerificationResult{OpportunityID: commit.OpportunityID, SourceVersionID: ref, Version: version, Replay: true}
			return nil
		}
		row, err := q.LockOpportunityForVerification(txCtx, capturesql.LockOpportunityForVerificationParams{OpportunityID: dbID(commit.OpportunityID), ExpectedVersion: commit.ExpectedVersion, SourceVersionID: dbID(commit.SourceVersionID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return workflowConflict("opportunity_version_or_source")
		}
		if err != nil {
			return err
		}
		if row.Title == "" {
			return errors.New("verified_title_required")
		}
		if err = q.InsertVerifiedSourceVersion(txCtx, capturesql.InsertVerifiedSourceVersionParams{ID: dbID(commit.VerifiedSourceVersionID), OpportunityID: row.ID, Revision: row.Revision + 1, Title: row.Title, BuyerName: row.BuyerName, SourceUri: row.SourceUri, SourceDocumentVersionID: row.SourceDocumentVersionID, ClosesAt: row.ClosesAt, SourceTimezone: row.SourceTimezone, SourceDateText: row.SourceDateText, RequestHash: row.RequestHash, CreatedAt: dbTime(commit.OccurredAt), ActorID: dbID(authority.ActorID)}); err != nil {
			return err
		}
		affected, err := q.AdvanceVerifiedOpportunity(txCtx, capturesql.AdvanceVerifiedOpportunityParams{SourceVersionID: dbID(commit.VerifiedSourceVersionID), UpdatedAt: dbTime(commit.OccurredAt), ID: row.ID, ExpectedVersion: commit.ExpectedVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return workflowConflict("opportunity_version")
		}
		newVersion := commit.ExpectedVersion + 1
		if err = q.InsertOpportunityVerifiedAudit(txCtx, capturesql.InsertOpportunityVerifiedAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(authority.ActorID), TargetID: row.ID, OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), SourceVersionID: dbID(commit.VerifiedSourceVersionID)}); err != nil {
			return err
		}
		if err = q.InsertOpportunityVerifiedOutbox(txCtx, capturesql.InsertOpportunityVerifiedOutboxParams{ID: dbID(commit.OutboxID), ActorID: dbID(authority.ActorID), AggregateID: row.ID, AggregateVersion: newVersion, SourceVersionID: dbID(commit.VerifiedSourceVersionID), OccurredAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, commit.CommandID, commit.VerifiedSourceVersionID, newVersion, commit.OccurredAt); err != nil {
			return err
		}
		result = application.VerificationResult{OpportunityID: commit.OpportunityID, SourceVersionID: commit.VerifiedSourceVersionID, Version: newVersion}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) CreateWorkspace(ctx context.Context, authority application.WorkflowAuthority, commit application.WorkspaceCommit) (result application.WorkspaceResult, err error) {
	if err = s.valid(ctx, authority); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, version, err := claimWorkflow(txCtx, q, commit.CommandID, authority.ActorID, "CREATE_BID_WORKSPACE", commit.KeyHash, commit.RequestHash, commit.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			result = application.WorkspaceResult{WorkspaceID: ref, Version: version, Replay: true}
			return nil
		}
		row, err := q.LockVerifiedOpportunityForWorkspace(txCtx, capturesql.LockVerifiedOpportunityForWorkspaceParams{OpportunityID: dbID(commit.OpportunityID), ExpectedVersion: commit.ExpectedVersion, SourceVersionID: dbID(commit.SourceVersionID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return workflowConflict("opportunity_not_verified_or_stale")
		}
		if err != nil {
			return err
		}
		if _, lookupErr := q.GetActiveWorkspaceByOpportunity(txCtx, row.ID); lookupErr == nil {
			return workflowConflict("active_workspace_exists")
		} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return lookupErr
		}
		if err = q.InsertBidWorkspace(txCtx, capturesql.InsertBidWorkspaceParams{ID: dbID(commit.WorkspaceID), OpportunityID: row.ID, SourceVersionID: row.SourceVersionID, OwnerID: dbID(commit.OwnerID), ActorID: dbID(authority.ActorID), CreatedAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertBidWorkspaceCreatedAudit(txCtx, capturesql.InsertBidWorkspaceCreatedAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(authority.ActorID), TargetID: dbID(commit.WorkspaceID), OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), OpportunityID: row.ID, SourceVersionID: row.SourceVersionID, OwnerID: dbID(commit.OwnerID)}); err != nil {
			return err
		}
		if err = q.InsertBidWorkspaceCreatedOutbox(txCtx, capturesql.InsertBidWorkspaceCreatedOutboxParams{ID: dbID(commit.OutboxID), ActorID: dbID(authority.ActorID), AggregateID: dbID(commit.WorkspaceID), OpportunityID: row.ID, SourceVersionID: row.SourceVersionID, OwnerID: dbID(commit.OwnerID), OccurredAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, commit.CommandID, commit.WorkspaceID, 1, commit.OccurredAt); err != nil {
			return err
		}
		result = application.WorkspaceResult{WorkspaceID: commit.WorkspaceID, Version: 1}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) RecordDecision(ctx context.Context, authority application.WorkflowAuthority, commit application.DecisionCommit) (result application.DecisionResult, err error) {
	if err = s.valid(ctx, authority); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, version, err := claimWorkflow(txCtx, q, commit.CommandID, authority.ActorID, "RECORD_BID_DECISION", commit.KeyHash, commit.RequestHash, commit.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			result = application.DecisionResult{DecisionID: ref, WorkspaceID: commit.WorkspaceID, Version: version, Decision: commit.Decision, Replay: true}
			return nil
		}
		row, err := q.LockBidWorkspaceForDecision(txCtx, capturesql.LockBidWorkspaceForDecisionParams{ID: dbID(commit.WorkspaceID), ExpectedVersion: commit.ExpectedVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return workflowConflict("bid_workspace_stale_or_archived")
		}
		if err != nil {
			return err
		}
		if row.State == commit.Decision {
			return workflowConflict("duplicate_bid_decision")
		}
		newVersion := commit.ExpectedVersion + 1
		if err = q.InsertBidDecision(txCtx, capturesql.InsertBidDecisionParams{ID: dbID(commit.DecisionID), BidID: row.ID, Sequence: int32(commit.ExpectedVersion), WorkspaceVersion: newVersion, Decision: commit.Decision, Rationale: commit.Rationale, ActorID: dbID(authority.ActorID), DecidedAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		affected, err := q.AdvanceBidWorkspaceDecision(txCtx, capturesql.AdvanceBidWorkspaceDecisionParams{State: commit.Decision, UpdatedAt: dbTime(commit.OccurredAt), ID: row.ID, ExpectedVersion: commit.ExpectedVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return workflowConflict("bid_workspace_version")
		}
		if err = q.InsertBidDecisionAudit(txCtx, capturesql.InsertBidDecisionAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(authority.ActorID), TargetID: row.ID, OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), DecisionID: dbID(commit.DecisionID), Decision: commit.Decision, WorkspaceVersion: newVersion}); err != nil {
			return err
		}
		if err = q.InsertBidDecisionOutbox(txCtx, capturesql.InsertBidDecisionOutboxParams{ID: dbID(commit.OutboxID), ActorID: dbID(authority.ActorID), AggregateID: row.ID, AggregateVersion: newVersion, DecisionID: dbID(commit.DecisionID), Decision: commit.Decision, OccurredAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, commit.CommandID, commit.DecisionID, newVersion, commit.OccurredAt); err != nil {
			return err
		}
		result = application.DecisionResult{DecisionID: commit.DecisionID, WorkspaceID: commit.WorkspaceID, Version: newVersion, Decision: commit.Decision}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) valid(ctx context.Context, a application.WorkflowAuthority) error {
	if ctx == nil || s == nil || s.runner == nil || a.ActorID.IsZero() || a.TenantID != s.tenantID || a.AssignmentGeneration != s.assignmentGeneration {
		return errors.New("workflow_tenant_lease_mismatch")
	}
	return nil
}
func claimWorkflow(ctx context.Context, q *capturesql.Queries, id, actor application.ID, operation string, keyHash, requestHash [32]byte, at time.Time) (bool, application.ID, int64, error) {
	_, err := q.ClaimWorkflowIdempotency(ctx, capturesql.ClaimWorkflowIdempotencyParams{ID: dbID(id), ActorID: dbID(actor), Operation: operation, KeyHash: keyHash[:], RequestHash: requestHash[:], CreatedAt: dbTime(at)})
	if err == nil {
		return false, application.ID{}, 0, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, application.ID{}, 0, err
	}
	row, err := q.GetWorkflowIdempotencyForUpdate(ctx, capturesql.GetWorkflowIdempotencyForUpdateParams{ActorID: dbID(actor), Operation: operation, KeyHash: keyHash[:]})
	if err != nil {
		return false, application.ID{}, 0, err
	}
	if !bytes.Equal(row.RequestHash, requestHash[:]) {
		return false, application.ID{}, 0, workflowConflict("idempotency_conflict")
	}
	if row.State == "PENDING" {
		return false, application.ID{}, 0, workflowConflict("idempotency_pending")
	}
	if row.State != "COMPLETED" || row.ResponseStatus == nil || *row.ResponseStatus != 201 || !row.ResponseRef.Valid || row.ResponseVersion == nil || *row.ResponseVersion < 1 {
		return false, application.ID{}, 0, workflowConflict("idempotency_result_invalid")
	}
	return true, appID(row.ResponseRef), *row.ResponseVersion, nil
}
func workflowConflict(reason string) error {
	return fmt.Errorf("%w: %s", application.ErrWorkflowConflict, reason)
}

func completeWorkflow(ctx context.Context, q *capturesql.Queries, id, responseRef application.ID, version int64, at time.Time) error {
	status := int32(201)
	affected, err := q.CompleteWorkflowIdempotency(ctx, capturesql.CompleteWorkflowIdempotencyParams{ResponseStatus: &status, ResponseRef: dbID(responseRef), ResponseVersion: &version, CompletedAt: dbTime(at), ID: dbID(id)})
	if err != nil {
		return err
	}
	if affected != 1 {
		return workflowConflict("idempotency_completion")
	}
	return nil
}
