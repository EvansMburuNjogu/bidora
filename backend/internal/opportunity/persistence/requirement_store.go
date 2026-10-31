package persistence

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *WorkflowStore) CreateRequirement(ctx context.Context, a application.WorkflowAuthority, commit application.RequirementCommit) (result application.RequirementResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, workspaceVersion, err := claimWorkflow(txCtx, q, commit.CommandID, a.ActorID, "CREATE_BID_REQUIREMENT", commit.KeyHash, commit.RequestHash, commit.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetBidRequirement(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			result = application.RequirementResult{Requirement: mapRequirement(row.ID, row.BidID, row.SourceVersionID, row.SourceLocation, row.Text, row.Mandatory, row.Category, row.Status, row.OwnerID, row.DueAt, row.Version, row.CreatedAt, row.UpdatedAt), WorkspaceVersion: workspaceVersion, Replay: true}
			return nil
		}
		bid, err := q.LockBidForRequirementCreation(txCtx, capturesql.LockBidForRequirementCreationParams{ID: dbID(commit.BidID), ExpectedVersion: commit.ExpectedWorkspaceVersion, SourceVersionID: dbID(commit.SourceVersionID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementConflict
		}
		if err != nil {
			return err
		}
		owner := pgtype.UUID{}
		if commit.HasOwner {
			owner = dbID(commit.OwnerID)
		}
		due := pgtype.Timestamptz{}
		if commit.DueAt != nil {
			due = dbTime(*commit.DueAt)
		}
		if err = q.InsertBidRequirement(txCtx, capturesql.InsertBidRequirementParams{ID: dbID(commit.RequirementID), BidID: bid.ID, SourceVersionID: bid.SourceVersionID, SourceLocation: commit.SourceLocation, Text: commit.Text, Mandatory: commit.Mandatory, Category: commit.Category, OwnerID: owner, DueAt: due, ActorID: dbID(a.ActorID), CreatedAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertRequirementCorrection(txCtx, capturesql.InsertRequirementCorrectionParams{ID: dbID(commit.RequirementID), RequirementID: dbID(commit.RequirementID), Revision: 1, SourceVersionID: bid.SourceVersionID, SourceLocation: commit.SourceLocation, Text: commit.Text, Mandatory: commit.Mandatory, Category: commit.Category, CorrectionReason: "Initial requirement capture", ActorID: dbID(a.ActorID), CreatedAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		newWorkspaceVersion := commit.ExpectedWorkspaceVersion + 1
		affected, err := q.AdvanceWorkspaceForRequirementChange(txCtx, capturesql.AdvanceWorkspaceForRequirementChangeParams{UpdatedAt: dbTime(commit.OccurredAt), ID: bid.ID, ExpectedVersion: commit.ExpectedWorkspaceVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrRequirementConflict
		}
		if err = q.InsertRequirementCreatedAudit(txCtx, capturesql.InsertRequirementCreatedAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(a.ActorID), TargetID: dbID(commit.RequirementID), OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), BidID: bid.ID, SourceVersionID: bid.SourceVersionID}); err != nil {
			return err
		}
		if err = q.InsertRequirementCreatedOutbox(txCtx, capturesql.InsertRequirementCreatedOutboxParams{ID: dbID(commit.OutboxID), BidID: bid.ID, WorkspaceVersion: newWorkspaceVersion, RequirementID: dbID(commit.RequirementID), SourceVersionID: bid.SourceVersionID, OccurredAt: dbTime(commit.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, commit.CommandID, commit.RequirementID, newWorkspaceVersion, commit.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetBidRequirement(txCtx, dbID(commit.RequirementID))
		if err != nil {
			return err
		}
		result = application.RequirementResult{Requirement: mapRequirement(row.ID, row.BidID, row.SourceVersionID, row.SourceLocation, row.Text, row.Mandatory, row.Category, row.Status, row.OwnerID, row.DueAt, row.Version, row.CreatedAt, row.UpdatedAt), WorkspaceVersion: newWorkspaceVersion}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) AssignRequirement(ctx context.Context, a application.WorkflowAuthority, commit application.AssignmentCommit) (result application.RequirementResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, workspaceVersion, err := claimWorkflow(txCtx, q, commit.CommandID, a.ActorID, "ASSIGN_BID_REQUIREMENT", commit.KeyHash, commit.RequestHash, commit.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetBidRequirement(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			result = application.RequirementResult{Requirement: mapRequirement(row.ID, row.BidID, row.SourceVersionID, row.SourceLocation, row.Text, row.Mandatory, row.Category, row.Status, row.OwnerID, row.DueAt, row.Version, row.CreatedAt, row.UpdatedAt), WorkspaceVersion: workspaceVersion, Replay: true}
			return nil
		}
		locked, err := q.LockRequirementForAssignment(txCtx, capturesql.LockRequirementForAssignmentParams{RequirementID: dbID(commit.RequirementID), BidID: dbID(commit.BidID), ExpectedRequirementVersion: commit.ExpectedRequirementVersion, ExpectedWorkspaceVersion: commit.ExpectedWorkspaceVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementConflict
		}
		if err != nil {
			return err
		}
		due := pgtype.Timestamptz{}
		if commit.DueAt != nil {
			due = dbTime(*commit.DueAt)
		}
		affected, err := q.AssignBidRequirement(txCtx, capturesql.AssignBidRequirementParams{OwnerID: dbID(commit.OwnerID), DueAt: due, UpdatedAt: dbTime(commit.OccurredAt), ID: locked.ID, ExpectedVersion: commit.ExpectedRequirementVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrRequirementConflict
		}
		newWorkspaceVersion := commit.ExpectedWorkspaceVersion + 1
		affected, err = q.AdvanceWorkspaceForRequirementChange(txCtx, capturesql.AdvanceWorkspaceForRequirementChangeParams{UpdatedAt: dbTime(commit.OccurredAt), ID: locked.BidID, ExpectedVersion: commit.ExpectedWorkspaceVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrRequirementConflict
		}
		newRequirementVersion := commit.ExpectedRequirementVersion + 1
		if err = q.InsertRequirementAssignedAudit(txCtx, capturesql.InsertRequirementAssignedAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(a.ActorID), TargetID: locked.ID, OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), BidID: locked.BidID, OwnerID: dbID(commit.OwnerID), RequirementVersion: newRequirementVersion}); err != nil {
			return err
		}
		if err = q.InsertRequirementAssignedOutbox(txCtx, capturesql.InsertRequirementAssignedOutboxParams{ID: dbID(commit.OutboxID), BidID: locked.BidID, WorkspaceVersion: newWorkspaceVersion, RequirementID: locked.ID, OwnerID: dbID(commit.OwnerID), OccurredAt: dbTime(commit.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, commit.CommandID, commit.RequirementID, newWorkspaceVersion, commit.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetBidRequirement(txCtx, locked.ID)
		if err != nil {
			return err
		}
		result = application.RequirementResult{Requirement: mapRequirement(row.ID, row.BidID, row.SourceVersionID, row.SourceLocation, row.Text, row.Mandatory, row.Category, row.Status, row.OwnerID, row.DueAt, row.Version, row.CreatedAt, row.UpdatedAt), WorkspaceVersion: newWorkspaceVersion}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListRequirements(ctx context.Context, a application.WorkflowAuthority, bidID application.ID) (result []application.Requirement, err error) {
	if err = s.validRead(ctx, a, 100); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := capturesql.New(db).ListBidRequirements(txCtx, dbID(bidID))
		if err != nil {
			return err
		}
		result = make([]application.Requirement, len(rows))
		for i, row := range rows {
			result[i] = mapRequirement(row.ID, row.BidID, row.SourceVersionID, row.SourceLocation, row.Text, row.Mandatory, row.Category, row.Status, row.OwnerID, row.DueAt, row.Version, row.CreatedAt, row.UpdatedAt)
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetReadiness(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, at time.Time) (result application.Readiness, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetBidReadiness(txCtx, capturesql.GetBidReadinessParams{ObservedAt: dbTime(at), ID: dbID(bidID)})
		if err != nil {
			return err
		}
		result = application.Readiness{BidID: appID(row.ID), WorkspaceState: row.State, WorkspaceVersion: row.Version, OpenMandatoryCount: row.OpenMandatoryCount, UnassignedMandatoryCount: row.UnassignedMandatoryCount, OverdueCount: row.OverdueCount, ExpiredEvidenceCount: row.ExpiredEvidenceCount, Blockers: []string{}}
		if row.State != "BID" {
			result.Blockers = append(result.Blockers, "BID_DECISION_REQUIRED")
		}
		if row.OpenMandatoryCount > 0 {
			result.Blockers = append(result.Blockers, "MANDATORY_REQUIREMENTS_OPEN")
		}
		if row.UnassignedMandatoryCount > 0 {
			result.Blockers = append(result.Blockers, "MANDATORY_REQUIREMENTS_UNASSIGNED")
		}
		if row.OverdueCount > 0 {
			result.Blockers = append(result.Blockers, "REQUIREMENTS_OVERDUE")
		}
		if row.ExpiredEvidenceCount > 0 {
			result.Blockers = append(result.Blockers, "REQUIREMENT_EVIDENCE_EXPIRED")
		}
		result.Ready = len(result.Blockers) == 0
		return nil
	})
	return result, err
}
func mapRequirement(id, bidID, sourceID pgtype.UUID, location, text string, mandatory bool, category, status string, owner pgtype.UUID, due pgtype.Timestamptz, version int64, created, updated pgtype.Timestamptz) application.Requirement {
	return application.Requirement{ID: appID(id), BidID: appID(bidID), SourceVersionID: appID(sourceID), OwnerID: appID(owner), HasOwner: owner.Valid, SourceLocation: location, Text: text, Mandatory: mandatory, Category: category, Status: status, DueAt: timePointer(due), Version: version, CreatedAt: created.Time.UTC(), UpdatedAt: updated.Time.UTC()}
}
