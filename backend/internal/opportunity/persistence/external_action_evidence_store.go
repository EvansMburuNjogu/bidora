package persistence

import (
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

func (s *WorkflowStore) CommitExternalActionEvidence(ctx context.Context, a application.WorkflowAuthority, c application.ExternalActionEvidenceCommit) (result application.ExternalActionEvidenceResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "REPORT_EXTERNAL_ACTION_EVIDENCE", c.KeyHash, c.RequestHash, c.OccurredAt)
		if errors.Is(err, application.ErrWorkflowConflict) {
			return application.ErrExternalActionEvidenceConflict
		}
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetExternalActionEvidence(txCtx, capturesql.GetExternalActionEvidenceParams{ID: dbID(ref), BidID: dbID(c.BidID)})
			if err != nil {
				return err
			}
			result = application.ExternalActionEvidenceResult{Evidence: mapExternalActionEvidence(row), Replay: true}
			return nil
		}
		locked, err := q.LockPackageForExternalEvidence(txCtx, capturesql.LockPackageForExternalEvidenceParams{PackageManifestID: dbID(c.PackageManifestID), BidID: dbID(c.BidID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrExternalActionEvidenceConflict
		} else if err != nil {
			return err
		}
		if locked.PackageStatus != "UNAPPROVED_PREPARATION_ONLY" || locked.WorkspaceVersion != c.ExpectedWorkspaceVersion {
			return application.ErrExternalActionEvidenceConflict
		}
		if !c.SupersedesEvidenceID.IsZero() {
			if _, err := q.ValidateSupersededExternalEvidence(txCtx, capturesql.ValidateSupersededExternalEvidenceParams{ID: dbID(c.SupersedesEvidenceID), BidID: dbID(c.BidID)}); errors.Is(err, pgx.ErrNoRows) {
				return application.ErrExternalActionEvidenceConflict
			} else if err != nil {
				return err
			}
		}
		err = q.InsertExternalActionEvidence(txCtx, capturesql.InsertExternalActionEvidenceParams{ID: dbID(c.EvidenceID), BidID: dbID(c.BidID), PackageManifestID: dbID(c.PackageManifestID), ExternalSystemName: c.ExternalSystemName, ExternalReference: c.ExternalReference, ReportedActionTimeText: c.ReportedActionTimeText, ReportedActionAt: dbTime(c.ReportedActionAt), ReportNote: c.ReportNote, SupersedesEvidenceID: dbID(c.SupersedesEvidenceID), ActorID: dbID(a.ActorID), RecordedAt: dbTime(c.OccurredAt)})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return application.ErrExternalActionEvidenceConflict
			}
			return err
		}
		if err = q.InsertExternalActionEvidenceAudit(txCtx, capturesql.InsertExternalActionEvidenceAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), EvidenceID: dbID(c.EvidenceID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), PackageManifestID: dbID(c.PackageManifestID)}); err != nil {
			return err
		}
		if err = q.InsertExternalActionEvidenceOutbox(txCtx, capturesql.InsertExternalActionEvidenceOutboxParams{ID: dbID(c.OutboxID), EvidenceID: dbID(c.EvidenceID), BidID: dbID(c.BidID), PackageManifestID: dbID(c.PackageManifestID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.EvidenceID, 1, c.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetExternalActionEvidence(txCtx, capturesql.GetExternalActionEvidenceParams{ID: dbID(c.EvidenceID), BidID: dbID(c.BidID)})
		if err != nil {
			return err
		}
		result = application.ExternalActionEvidenceResult{Evidence: mapExternalActionEvidence(row)}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetExternalActionEvidence(ctx context.Context, a application.WorkflowAuthority, bidID, evidenceID application.ID) (result application.ExternalActionEvidence, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetExternalActionEvidence(txCtx, capturesql.GetExternalActionEvidenceParams{ID: dbID(evidenceID), BidID: dbID(bidID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrExternalActionEvidenceNotFound
		} else if err != nil {
			return err
		}
		result = mapExternalActionEvidence(row)
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListExternalActionEvidence(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, cursor application.ReadCursor, limit int32) (result []application.ExternalActionEvidence, err error) {
	if err = s.validRead(ctx, a, limit); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if _, err := q.ValidateProposalPackageBidRead(txCtx, dbID(bidID)); errors.Is(err, pgx.ErrNoRows) {
			return application.ErrExternalActionEvidenceNotFound
		} else if err != nil {
			return err
		}
		cursorAt, cursorID := readCursor(cursor)
		rows, err := q.ListExternalActionEvidence(txCtx, capturesql.ListExternalActionEvidenceParams{BidID: dbID(bidID), CursorAt: cursorAt, CursorID: cursorID, PageLimit: limit})
		if err != nil {
			return err
		}
		result = make([]application.ExternalActionEvidence, len(rows))
		for i, row := range rows {
			result[i] = mapExternalActionEvidence(row)
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetExternalEvidenceReconciliation(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, observedAt time.Time) (result application.ExternalEvidenceReconciliation, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		counts, err := q.GetExternalEvidenceReconciliationCounts(txCtx, dbID(bidID))
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrExternalActionEvidenceNotFound
		} else if err != nil {
			return err
		}
		rows, err := q.ListActiveExternalEvidenceReconciliation(txCtx, dbID(bidID))
		if err != nil {
			return err
		}
		if len(rows) > 100 {
			return application.ErrExternalEvidenceReconciliationLimit
		}
		result = application.ExternalEvidenceReconciliation{BidID: appID(counts.BidID), CurrentProposalVersionID: appID(counts.CurrentRevisionID), ReportCount: counts.ReportCount, ActiveReportCount: counts.ActiveReportCount, ActiveReports: make([]application.ActiveExternalEvidenceFact, len(rows)), Submitted: counts.Submitted, ObservedAt: observedAt}
		for i, row := range rows {
			result.ActiveReports[i] = application.ActiveExternalEvidenceFact{EvidenceID: appID(row.EvidenceID), PackageManifestID: appID(row.PackageManifestID), ProposalVersionID: appID(row.ProposalVersionID), SupersedesEvidenceID: appID(row.SupersedesEvidenceID), ReportedActionAt: row.ReportedActionAt.Time.UTC(), RecordedAt: row.RecordedAt.Time.UTC(), ReadinessObservedAt: row.ReadinessObservedAt.Time.UTC(), VerificationStatus: row.VerificationStatus, PackageStatus: row.PackageStatus, ReadinessReady: row.ReadinessReady, LineageDepth: row.LineageDepth, LineageTruncated: row.LineageTruncated}
		}
		return nil
	})
	return result, err
}
func mapExternalActionEvidence(row capturesql.BidExternalActionEvidence) application.ExternalActionEvidence {
	return application.ExternalActionEvidence{ID: appID(row.ID), BidID: appID(row.BidID), PackageManifestID: appID(row.PackageManifestID), SupersedesEvidenceID: appID(row.SupersedesEvidenceID), RecordedBy: appID(row.RecordedBy), ReportType: row.ReportType, VerificationStatus: row.VerificationStatus, ExternalSystemName: row.ExternalSystemName, ExternalReference: row.ExternalReference, ReportedActionTimeText: row.ReportedActionTimeText, ReportNote: row.ReportNote, ReportedActionAt: row.ReportedActionAt.Time.UTC(), RecordedAt: row.RecordedAt.Time.UTC()}
}
