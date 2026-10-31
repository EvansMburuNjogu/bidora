package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *WorkflowStore) CorrectRequirement(ctx context.Context, a application.WorkflowAuthority, c application.RequirementCorrectionCommit) (result application.RequirementCorrectionResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, responseWorkspaceVersion, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "CORRECT_REQUIREMENT", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			revision, err := q.GetRequirementCorrectionVersion(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			requirement, err := q.GetBidRequirement(txCtx, dbID(c.RequirementID))
			if err != nil {
				return err
			}
			result = application.RequirementCorrectionResult{Requirement: mapRequirement(requirement.ID, requirement.BidID, requirement.SourceVersionID, requirement.SourceLocation, requirement.Text, requirement.Mandatory, requirement.Category, requirement.Status, requirement.OwnerID, requirement.DueAt, requirement.Version, requirement.CreatedAt, requirement.UpdatedAt), ContentRevision: mapRequirementCorrection(revision.ID, revision.RequirementID, revision.BidID, revision.SourceVersionID, revision.SourceDocumentVersionID, revision.Revision, revision.SourceLocation, revision.Text, revision.Mandatory, revision.Category, revision.CorrectionReason, revision.CreatedBy, revision.CreatedAt), WorkspaceVersion: responseWorkspaceVersion, Replay: true}
			return nil
		}
		locked, err := q.LockRequirementForCorrection(txCtx, capturesql.LockRequirementForCorrectionParams{RequirementID: dbID(c.RequirementID), BidID: dbID(c.BidID), ExpectedRequirementVersion: c.ExpectedRequirementVersion, ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementCorrectionConflict
		}
		if err != nil {
			return err
		}
		revision := locked.ContentRevision + 1
		if err = q.InsertRequirementCorrection(txCtx, capturesql.InsertRequirementCorrectionParams{ID: dbID(c.ContentRevisionID), RequirementID: locked.ID, Revision: revision, SourceVersionID: locked.SourceVersionID, SourceDocumentVersionID: locked.SourceDocumentVersionID, SourceLocation: c.SourceLocation, Text: c.Text, Mandatory: c.Mandatory, Category: c.Category, CorrectionReason: c.CorrectionReason, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		affected, err := q.ApplyRequirementCorrection(txCtx, capturesql.ApplyRequirementCorrectionParams{SourceLocation: c.SourceLocation, Text: c.Text, Mandatory: c.Mandatory, Category: c.Category, ContentRevisionID: dbID(c.ContentRevisionID), UpdatedAt: dbTime(c.OccurredAt), ID: locked.ID, ExpectedVersion: c.ExpectedRequirementVersion})
		if err != nil || affected != 1 {
			return application.ErrRequirementCorrectionConflict
		}
		newRequirementVersion := c.ExpectedRequirementVersion + 1
		newWorkspaceVersion := c.ExpectedWorkspaceVersion + 1
		affected, err = q.AdvanceWorkspaceForRequirementChange(txCtx, capturesql.AdvanceWorkspaceForRequirementChangeParams{UpdatedAt: dbTime(c.OccurredAt), ID: locked.BidID, ExpectedVersion: c.ExpectedWorkspaceVersion})
		if err != nil || affected != 1 {
			return application.ErrRequirementCorrectionConflict
		}
		if err = q.InsertRequirementCorrectedAudit(txCtx, capturesql.InsertRequirementCorrectedAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), RequirementID: locked.ID, OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: locked.BidID, ContentRevisionID: dbID(c.ContentRevisionID), ContentRevision: revision, RequirementVersion: newRequirementVersion, WorkspaceVersion: newWorkspaceVersion}); err != nil {
			return err
		}
		if err = q.InsertRequirementCorrectedOutbox(txCtx, capturesql.InsertRequirementCorrectedOutboxParams{ID: dbID(c.OutboxID), BidID: locked.BidID, WorkspaceVersion: newWorkspaceVersion, RequirementID: locked.ID, ContentRevisionID: dbID(c.ContentRevisionID), ContentRevision: revision, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.ContentRevisionID, newWorkspaceVersion, c.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetBidRequirement(txCtx, locked.ID)
		if err != nil {
			return err
		}
		version, err := q.GetRequirementCorrectionVersion(txCtx, dbID(c.ContentRevisionID))
		if err != nil {
			return err
		}
		result = application.RequirementCorrectionResult{Requirement: mapRequirement(row.ID, row.BidID, row.SourceVersionID, row.SourceLocation, row.Text, row.Mandatory, row.Category, row.Status, row.OwnerID, row.DueAt, row.Version, row.CreatedAt, row.UpdatedAt), ContentRevision: mapRequirementCorrection(version.ID, version.RequirementID, version.BidID, version.SourceVersionID, version.SourceDocumentVersionID, version.Revision, version.SourceLocation, version.Text, version.Mandatory, version.Category, version.CorrectionReason, version.CreatedBy, version.CreatedAt), WorkspaceVersion: newWorkspaceVersion}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListRequirementCorrections(ctx context.Context, a application.WorkflowAuthority, bidID, requirementID application.ID) (result []application.RequirementContentRevision, err error) {
	if err = s.validRead(ctx, a, 100); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := capturesql.New(db).ListRequirementCorrectionHistory(txCtx, capturesql.ListRequirementCorrectionHistoryParams{RequirementID: dbID(requirementID), BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		result = make([]application.RequirementContentRevision, len(rows))
		for i, row := range rows {
			result[i] = mapRequirementCorrection(row.ID, row.RequirementID, row.BidID, row.SourceVersionID, row.SourceDocumentVersionID, row.Revision, row.SourceLocation, row.Text, row.Mandatory, row.Category, row.CorrectionReason, row.CreatedBy, row.CreatedAt)
		}
		return nil
	})
	return result, err
}
func mapRequirementCorrection(id, requirementID, bidID, sourceVersionID, sourceDocumentVersionID pgtype.UUID, revision int64, sourceLocation, text string, mandatory bool, category, reason string, createdBy pgtype.UUID, createdAt pgtype.Timestamptz) application.RequirementContentRevision {
	return application.RequirementContentRevision{ID: appID(id), RequirementID: appID(requirementID), BidID: appID(bidID), SourceVersionID: appID(sourceVersionID), SourceDocumentVersionID: appID(sourceDocumentVersionID), HasSourceDocument: sourceDocumentVersionID.Valid, Revision: revision, SourceLocation: sourceLocation, Text: text, Mandatory: mandatory, Category: category, CorrectionReason: reason, CreatedBy: appID(createdBy), CreatedAt: createdAt.Time.UTC()}
}
