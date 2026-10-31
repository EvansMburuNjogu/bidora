package persistence

import (
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *WorkflowStore) GetBidArchivePreparation(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, observedAt time.Time) (result application.BidArchivePreparation, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetBidArchivePreparation(txCtx, capturesql.GetBidArchivePreparationParams{ObservedAt: dbTime(observedAt), BidID: dbID(bidID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrArchivePreparationNotFound
		} else if err != nil {
			return err
		}
		result = application.BidArchivePreparation{BidID: appID(row.BidID), ProposalVersionID: appID(row.ProposalVersionID), WorkspaceState: row.WorkspaceState, WorkspaceVersion: row.WorkspaceVersion, ArchivedAt: timePointer(row.ArchivedAt), RequirementCount: row.RequirementCount, OpenRequirementCount: row.OpenRequirementCount, OpenReviewThreadCount: row.OpenReviewThreadCount, PendingUploadCount: row.PendingUploadCount, PendingJobCount: row.PendingJobCount, PackageManifestCount: row.PackageManifestCount, ExternalReportCount: row.ExternalReportCount, ActiveExternalReportCount: row.ActiveExternalReportCount, HasSubmissionConfirmation: row.HasSubmissionConfirmation, CurrentReportedOutcome: row.CurrentReportedOutcome, ObservedAt: observedAt}
		return nil
	})
	return result, err
}
