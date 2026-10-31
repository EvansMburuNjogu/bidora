package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	entitlementpersistence "bidora/backend/internal/entitlementcheckpoint/persistence"
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DocumentProcessingStore struct {
	runner               workflowRunner
	tenantID             application.ID
	assignmentGeneration int64
}

func NewDocumentProcessingStore(runner workflowRunner, tenantID application.ID, assignmentGeneration int64) (*DocumentProcessingStore, error) {
	if runner == nil || tenantID.IsZero() || assignmentGeneration < 1 {
		return nil, errors.New("document_processing_store_invalid")
	}
	return &DocumentProcessingStore{runner: runner, tenantID: tenantID, assignmentGeneration: assignmentGeneration}, nil
}
func (s *DocumentProcessingStore) ClaimScan(ctx context.Context, at time.Time) (claim application.ProcessingClaim, found bool, err error) {
	if ctx == nil {
		return claim, false, errors.New("processing_context_required")
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, e := capturesql.New(db).ClaimNextScanJob(txCtx, dbTime(at))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		claim = application.ProcessingClaim{JobID: appID(row.ID), DocumentVersionID: appID(row.DocumentVersionID), OpportunityID: appID(row.OpportunityID), RequestedBy: appID(row.RequestedBy), TenantID: s.tenantID, AssignmentGeneration: s.assignmentGeneration, Capability: processingCapability(row.DocumentKind), ObjectKey: row.ObjectKey, ObjectVersion: row.ObjectVersion, MediaType: row.MediaType, SizeBytes: row.SizeBytes, Attempts: row.Attempts}
		found = true
		return nil
	})
	return claim, found, err
}
func (s *DocumentProcessingStore) ReleaseScan(ctx context.Context, id application.ID) error {
	return s.execOne(ctx, func(q *capturesql.Queries, c context.Context) (int64, error) { return q.ReleaseScanJob(c, dbID(id)) })
}
func (s *DocumentProcessingStore) ReleaseAnalysis(ctx context.Context, id application.ID) error {
	return s.execOne(ctx, func(q *capturesql.Queries, c context.Context) (int64, error) {
		return q.ReleaseAnalysisJob(c, dbID(id))
	})
}
func (s *DocumentProcessingStore) CancelJob(ctx context.Context, id application.ID, at time.Time) error {
	return s.execOne(ctx, func(q *capturesql.Queries, c context.Context) (int64, error) {
		return q.CancelRunningJob(c, capturesql.CancelRunningJobParams{CompletedAt: dbTime(at), ID: dbID(id)})
	})
}
func (s *DocumentProcessingStore) CancelAnalysisAndRequestRelease(ctx context.Context, claim application.ProcessingClaim, completionIDs [3]application.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		code := "AUTHORIZATION_REVOKED_BEFORE_PROVIDER"
		affected, err := q.FailExtractionRun(txCtx, capturesql.FailExtractionRunParams{FailureCode: &code, CompletedAt: dbTime(at), ID: dbID(claim.RunID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CancelRunningJob(txCtx, capturesql.CancelRunningJobParams{CompletedAt: dbTime(at), ID: dbID(claim.JobID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		_, _, err = entitlementapp.RequestCompletionWithinTransaction(txCtx, entitlementTx, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(claim.CheckpointID), ReservationID: entitlementapp.ID(claim.ReservationID), ExecutionID: entitlementapp.ID(claim.RunID), CorrelationID: entitlementapp.ID(claim.CorrelationID), Operation: entitlementapp.CompletionRelease}, [3]entitlementapp.ID{entitlementapp.ID(completionIDs[0]), entitlementapp.ID(completionIDs[1]), entitlementapp.ID(completionIDs[2])}, at)
		return err
	})
}

func (s *DocumentProcessingStore) CommitScan(ctx context.Context, claim application.ProcessingClaim, out application.ScanOutcome, auditID, outboxID, correlationID application.ID, at time.Time) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, err := q.LockRunningScanJob(txCtx, dbID(claim.JobID))
		if err != nil {
			return err
		}
		if row.DocumentVersionID != dbID(claim.DocumentVersionID) || row.RequestedBy != dbID(claim.RequestedBy) || row.ScanState != "QUARANTINED" {
			return errors.New("scan_job_conflict")
		}
		state, event := "REJECTED", "document.version.rejected.v1"
		if out.Clean {
			state, event = "CLEAN", "document.version.clean.v1"
		}
		affected, err := q.MarkDocumentVersionScanned(txCtx, capturesql.MarkDocumentVersionScannedParams{ScanState: state, ScanEngine: &out.Engine, ResultCode: &out.ResultCode, CompletedAt: dbTime(at), ID: row.DocumentVersionID})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CompleteScanJob(txCtx, capturesql.CompleteScanJobParams{CompletedAt: dbTime(at), ID: row.ID})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		if claim.Capability == "tenderpreneur.opportunity.document.upload" {
			if out.Clean {
				affected, err = q.ReleaseAnalysisAfterClean(txCtx, row.DocumentVersionID)
			} else {
				affected, err = q.CancelAnalysisAfterRejection(txCtx, capturesql.CancelAnalysisAfterRejectionParams{CompletedAt: dbTime(at), DocumentVersionID: row.DocumentVersionID})
			}
			if err != nil || affected != 1 {
				return affectedErr(err)
			}
		} else if claim.Capability != "tenderpreneur.bid.requirement.manage" && claim.Capability != "tenderpreneur.submission.report" && claim.Capability != "tenderpreneur.bid.outcome.report" {
			return errors.New("document_processing_capability_invalid")
		}
		if err = q.InsertDocumentScanAudit(txCtx, capturesql.InsertDocumentScanAuditParams{ID: dbID(auditID), ActorID: row.RequestedBy, VersionID: row.DocumentVersionID, OccurredAt: dbTime(at), CorrelationID: dbID(correlationID), ScanState: state, ResultCode: out.ResultCode}); err != nil {
			return err
		}
		return q.InsertDocumentScanOutbox(txCtx, capturesql.InsertDocumentScanOutboxParams{ID: dbID(outboxID), DocumentID: row.DocumentID, VersionID: row.DocumentVersionID, ScanState: state, EventType: event, OccurredAt: dbTime(at), ActorID: row.RequestedBy})
	})
}
func (s *DocumentProcessingStore) ClaimAnalysis(ctx context.Context, runID application.ID, model string, at time.Time) (claim application.ProcessingClaim, found bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, e := q.ClaimNextAnalysisJob(txCtx, dbTime(at))
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if e = q.InsertExtractionRun(txCtx, capturesql.InsertExtractionRunParams{ID: dbID(runID), JobID: row.ID, DocumentVersionID: row.DocumentVersionID, Model: model, StartedAt: dbTime(at)}); e != nil {
			return e
		}
		claim = application.ProcessingClaim{JobID: appID(row.ID), RunID: runID, DocumentVersionID: appID(row.DocumentVersionID), OpportunityID: appID(row.OpportunityID), RequestedBy: appID(row.RequestedBy), TenantID: s.tenantID, MeasurementID: appID(row.MeasurementID), CheckpointID: appID(row.CheckpointID), ReservationID: appID(row.ReservationID), CorrelationID: appID(row.CorrelationID), AssignmentGeneration: s.assignmentGeneration, EstimatedCredits: row.EstimatedCredits, Capability: processingCapability(row.DocumentKind), UsageKind: row.UsageKind, ObjectKey: row.ObjectKey, ObjectVersion: row.ObjectVersion, MediaType: row.MediaType, SizeBytes: row.SizeBytes, Attempts: row.Attempts}
		found = true
		return nil
	})
	return claim, found, err
}
func (s *DocumentProcessingStore) CommitExtraction(ctx context.Context, claim application.ProcessingClaim, out application.ExtractionOutput, suggestionIDs []application.ID, auditID, outboxID, correlationID application.ID, completionIDs [3]application.ID, at time.Time) error {
	if len(suggestionIDs) != len(out.Suggestions) {
		return errors.New("suggestion_id_mismatch")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		for i, row := range out.Suggestions {
			numeric := pgtype.Numeric{}
			if err := numeric.Scan(fmt.Sprintf("%.4f", row.Confidence)); err != nil {
				return err
			}
			var page *int32
			if row.CitationPage > 0 {
				value := row.CitationPage
				page = &value
			}
			if err := q.InsertRequirementSuggestion(txCtx, capturesql.InsertRequirementSuggestionParams{ID: dbID(suggestionIDs[i]), RunID: dbID(claim.RunID), DocumentVersionID: dbID(claim.DocumentVersionID), Ordinal: int32(i + 1), RequirementText: row.RequirementText, Category: row.Category, Mandatory: row.Mandatory, CitationPage: page, CitationLocation: row.CitationLocation, CitationQuote: row.CitationQuote, Confidence: numeric, CreatedAt: dbTime(at)}); err != nil {
				return err
			}
		}
		responseID := out.ProviderResponseID
		affected, err := q.CompleteExtractionRun(txCtx, capturesql.CompleteExtractionRunParams{ProviderResponseID: &responseID, CompletedAt: dbTime(at), ID: dbID(claim.RunID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CompleteAnalysisJob(txCtx, capturesql.CompleteAnalysisJobParams{State: "SUCCEEDED", CompletedAt: dbTime(at), ID: dbID(claim.JobID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		count := int32(len(out.Suggestions))
		if err = q.InsertExtractionAudit(txCtx, capturesql.InsertExtractionAuditParams{ID: dbID(auditID), ActorID: dbID(claim.RequestedBy), VersionID: dbID(claim.DocumentVersionID), OccurredAt: dbTime(at), CorrelationID: dbID(correlationID), RunID: dbID(claim.RunID), SuggestionCount: count}); err != nil {
			return err
		}
		if err = q.InsertExtractionOutbox(txCtx, capturesql.InsertExtractionOutboxParams{ID: dbID(outboxID), VersionID: dbID(claim.DocumentVersionID), RunID: dbID(claim.RunID), SuggestionCount: count, OccurredAt: dbTime(at), ActorID: dbID(claim.RequestedBy)}); err != nil {
			return err
		}
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		_, _, err = entitlementapp.RequestCompletionWithinTransaction(txCtx, entitlementTx, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(claim.CheckpointID), ReservationID: entitlementapp.ID(claim.ReservationID), ExecutionID: entitlementapp.ID(claim.RunID), CorrelationID: entitlementapp.ID(claim.CorrelationID), Operation: entitlementapp.CompletionSettle, ActualCredits: claim.EstimatedCredits}, [3]entitlementapp.ID{entitlementapp.ID(completionIDs[0]), entitlementapp.ID(completionIDs[1]), entitlementapp.ID(completionIDs[2])}, at)
		return err
	})
}
func (s *DocumentProcessingStore) FailExtractionAndRequestCompletion(ctx context.Context, claim application.ProcessingClaim, code string, settle bool, completionIDs [3]application.ID, at time.Time) error {
	if code != "PROVIDER_OUTCOME_UNKNOWN" && code != "PROVIDER_CONFIRMED_NO_EFFECT" {
		return errors.New("document_provider_failure_code_invalid")
	}
	if settle != (code == "PROVIDER_OUTCOME_UNKNOWN") {
		return errors.New("document_provider_failure_disposition_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		affected, err := q.FailExtractionRun(txCtx, capturesql.FailExtractionRunParams{FailureCode: &code, CompletedAt: dbTime(at), ID: dbID(claim.RunID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.CompleteAnalysisJob(txCtx, capturesql.CompleteAnalysisJobParams{State: "FAILED", CompletedAt: dbTime(at), ID: dbID(claim.JobID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		operation, actual := entitlementapp.CompletionRelease, int64(0)
		if settle {
			operation, actual = entitlementapp.CompletionSettle, claim.EstimatedCredits
		}
		entitlementTx, err := entitlementpersistence.BindTransaction(db)
		if err != nil {
			return err
		}
		_, _, err = entitlementapp.RequestCompletionWithinTransaction(txCtx, entitlementTx, entitlementapp.CompletionInput{CheckpointID: entitlementapp.ID(claim.CheckpointID), ReservationID: entitlementapp.ID(claim.ReservationID), ExecutionID: entitlementapp.ID(claim.RunID), CorrelationID: entitlementapp.ID(claim.CorrelationID), Operation: operation, ActualCredits: actual}, [3]entitlementapp.ID{entitlementapp.ID(completionIDs[0]), entitlementapp.ID(completionIDs[1]), entitlementapp.ID(completionIDs[2])}, at)
		return err
	})
}

func (s *DocumentProcessingStore) execOne(ctx context.Context, fn func(*capturesql.Queries, context.Context) (int64, error)) error {
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		affected, err := fn(capturesql.New(db), txCtx)
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		return nil
	})
}
func processingCapability(kind string) string {
	if kind == "BID_EVIDENCE" {
		return "tenderpreneur.bid.requirement.manage"
	}
	if kind == "TENDER_SOURCE" {
		return "tenderpreneur.opportunity.document.upload"
	}
	if kind == "SUBMISSION_PROOF" {
		return "tenderpreneur.submission.report"
	}
	if kind == "OUTCOME_PROOF" {
		return "tenderpreneur.bid.outcome.report"
	}
	return ""
}
func affectedErr(err error) error {
	if err != nil {
		return err
	}
	return errors.New("processing_state_conflict")
}
