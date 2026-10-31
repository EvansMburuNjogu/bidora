package persistence

import (
	"bytes"
	"context"
	"errors"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

type DocumentMeasurementStore struct {
	runner               workflowRunner
	tenantID             application.ID
	assignmentGeneration int64
}

func NewDocumentMeasurementStore(runner workflowRunner, tenantID application.ID, assignmentGeneration int64) (*DocumentMeasurementStore, error) {
	if runner == nil || tenantID.IsZero() || assignmentGeneration < 1 {
		return nil, errors.New("document_measurement_store_invalid")
	}
	return &DocumentMeasurementStore{runner: runner, tenantID: tenantID, assignmentGeneration: assignmentGeneration}, nil
}

func (s *DocumentMeasurementStore) FindPendingDocumentReservation(ctx context.Context) (pending application.PendingDocumentReservation, found bool, err error) {
	if ctx == nil {
		return pending, false, errors.New("document_measurement_context_required")
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, queryErr := capturesql.New(db).FindPendingDocumentMeasurementReservation(txCtx)
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return nil
		}
		if queryErr != nil {
			return queryErr
		}
		pending = application.PendingDocumentReservation{
			MeasurementID:        appID(row.MeasurementID),
			JobID:                appID(row.JobID),
			DocumentVersionID:    appID(row.DocumentVersionID),
			RequestedBy:          appID(row.RequesterUserID),
			TenantID:             s.tenantID,
			CorrelationID:        appID(row.CorrelationID),
			AssignmentGeneration: row.AssignmentGeneration,
			MeasuredUnits:        row.MeasuredUnits,
			EstimatedCredits:     row.EstimatedCredits,
			Unit:                 row.Unit,
			UsageKind:            row.UsageKind,
		}
		found = true
		return nil
	})
	return pending, found, err
}

func (s *DocumentMeasurementStore) RecoverExpiredDocumentMeasurement(ctx context.Context, outcomeID application.ID) (bool, error) {
	if ctx == nil || outcomeID.IsZero() {
		return false, errors.New("document_measurement_recovery_invalid")
	}
	recovered := false
	err := s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		_, queryErr := capturesql.New(db).RecoverExpiredDocumentMeasurementJob(txCtx, dbID(outcomeID))
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return nil
		}
		if queryErr != nil {
			return queryErr
		}
		recovered = true
		return nil
	})
	return recovered, err
}

func (s *DocumentMeasurementStore) ClaimDocumentMeasurement(ctx context.Context, lease application.DocumentMeasurementLeaseClaim) (claim application.DocumentMeasurementClaim, found bool, err error) {
	if ctx == nil || lease.LeaseID.IsZero() || !lease.Identity.Valid() {
		return claim, false, errors.New("document_measurement_claim_invalid")
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		identity := lease.Identity
		row, queryErr := capturesql.New(db).ClaimNextDocumentMeasurementJob(txCtx, capturesql.ClaimNextDocumentMeasurementJobParams{ParserKey: identity.ParserKey, ParserVersion: identity.ParserVersion, PdfSemantics: identity.PDFSemantics, DocxSemantics: identity.DOCXSemantics, ProfileVersion: identity.ProfileVersion, LeaseID: dbID(lease.LeaseID)})
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return nil
		}
		if queryErr != nil {
			return queryErr
		}
		if len(row.Sha256) != 32 || !row.MeasurementLeaseID.Valid || !row.MeasurementLeaseExpiresAt.Valid || row.MeasurementAttempts < 1 || row.MeasurementAttempts > application.DocumentMeasurementMaxAttempts {
			return errors.New("document_measurement_claim_row_invalid")
		}
		var digest [32]byte
		copy(digest[:], row.Sha256)
		claim = application.DocumentMeasurementClaim{
			JobID:                appID(row.ID),
			DocumentVersionID:    appID(row.DocumentVersionID),
			OpportunityID:        appID(row.OpportunityID),
			RequestedBy:          appID(row.RequestedBy),
			TenantID:             s.tenantID,
			AssignmentGeneration: s.assignmentGeneration,
			LeaseID:              appID(row.MeasurementLeaseID),
			LeaseExpiresAt:       row.MeasurementLeaseExpiresAt.Time.UTC(),
			ObjectKey:            row.ObjectKey,
			ObjectVersion:        row.ObjectVersion,
			MediaType:            row.MediaType,
			Capability:           processingCapability(row.DocumentKind),
			ParserKey:            row.ParserKey,
			ParserVersion:        row.ParserVersion,
			SemanticsVersion:     row.SemanticsVersion,
			ProfileVersion:       row.ProfileVersion,
			SizeBytes:            row.SizeBytes,
			SHA256:               digest,
			Attempts:             row.Attempts,
			MeasurementAttempts:  row.MeasurementAttempts,
		}
		if claim.Capability != "tenderpreneur.opportunity.document.upload" {
			return errors.New("document_measurement_kind_invalid")
		}
		found = true
		return nil
	})
	return claim, found, err
}

func (s *DocumentMeasurementStore) RetryDocumentMeasurement(ctx context.Context, claim application.DocumentMeasurementClaim, outcomeID application.ID, code string, at, next time.Time) error {
	expectedBackoff := application.DocumentMeasurementFirstBackoff
	if claim.MeasurementAttempts == 2 {
		expectedBackoff = application.DocumentMeasurementSecondBackoff
	}
	if !transientMeasurementCode(code) || claim.MeasurementAttempts < 1 || claim.MeasurementAttempts >= application.DocumentMeasurementMaxAttempts || next.Sub(at) != expectedBackoff {
		return errors.New("document_measurement_retry_invalid")
	}
	return s.finishAttempt(ctx, claim, outcomeID, "TRANSIENT_FAILURE", code, at, next, "RETRY")
}
func (s *DocumentMeasurementStore) FailDocumentMeasurement(ctx context.Context, claim application.DocumentMeasurementClaim, outcomeID application.ID, code string, at time.Time) error {
	if !terminalMeasurementCode(code) {
		return errors.New("document_measurement_failure_invalid")
	}
	return s.finishAttempt(ctx, claim, outcomeID, "TERMINAL_FAILURE", code, at, time.Time{}, "FAIL")
}
func (s *DocumentMeasurementStore) CancelDocumentMeasurement(ctx context.Context, claim application.DocumentMeasurementClaim, outcomeID application.ID, at time.Time) error {
	return s.finishAttempt(ctx, claim, outcomeID, "TERMINAL_FAILURE", "AUTHORIZATION_DENIED", at, time.Time{}, "CANCEL")
}

func (s *DocumentMeasurementStore) finishAttempt(ctx context.Context, claim application.DocumentMeasurementClaim, outcomeID application.ID, outcome, code string, at, next time.Time, disposition string) error {
	if ctx == nil || outcomeID.IsZero() || claim.JobID.IsZero() || claim.LeaseID.IsZero() || at.IsZero() {
		return errors.New("document_measurement_attempt_completion_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		failure := code
		nextValue := dbTime(next)
		if err := q.InsertDocumentMeasurementAttemptOutcome(txCtx, capturesql.InsertDocumentMeasurementAttemptOutcomeParams{ID: dbID(outcomeID), LeaseID: dbID(claim.LeaseID), Outcome: outcome, FailureCode: &failure, OccurredAt: dbTime(at), NextAttemptAt: nextValue}); err != nil {
			return err
		}
		var affected int64
		var err error
		switch disposition {
		case "RETRY":
			affected, err = q.RetryDocumentMeasurementJob(txCtx, capturesql.RetryDocumentMeasurementJobParams{NextAttemptAt: nextValue, ID: dbID(claim.JobID), LeaseID: dbID(claim.LeaseID)})
		case "FAIL":
			affected, err = q.FailDocumentMeasurementJob(txCtx, capturesql.FailDocumentMeasurementJobParams{OccurredAt: dbTime(at), FailureCode: &failure, ID: dbID(claim.JobID), LeaseID: dbID(claim.LeaseID)})
		case "CANCEL":
			affected, err = q.CancelDocumentMeasurementJob(txCtx, capturesql.CancelDocumentMeasurementJobParams{OccurredAt: dbTime(at), ID: dbID(claim.JobID), LeaseID: dbID(claim.LeaseID)})
		default:
			return errors.New("document_measurement_attempt_disposition_invalid")
		}
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		return nil
	})
}

func transientMeasurementCode(code string) bool {
	switch code {
	case "OBJECT_STORAGE_UNAVAILABLE", "OBJECT_CLOSE_UNAVAILABLE", "PARSER_UNAVAILABLE", "CREDIT_CALCULATION_UNAVAILABLE", "ID_GENERATION_UNAVAILABLE":
		return true
	}
	return false
}
func terminalMeasurementCode(code string) bool {
	switch code {
	case "INPUT_HASH_MISMATCH", "INPUT_SIZE_MISMATCH", "UNSUPPORTED_OR_MISMATCHED_FORMAT", "ENCRYPTED_DOCUMENT", "CORRUPT_DOCUMENT", "PAGE_LIMIT_EXCEEDED", "CHARACTER_LIMIT_EXCEEDED", "ARCHIVE_LIMIT_EXCEEDED", "XML_LIMIT_EXCEEDED", "RESOURCE_LIMIT_EXCEEDED", "PARSER_TIMEOUT", "MEASUREMENT_INPUT_INVALID", "MEASUREMENT_RESULT_MISMATCH", "MEASUREMENT_UNKNOWN_FAILURE", "MEASUREMENT_RETRY_EXHAUSTED":
		return true
	}
	return false
}

func (s *DocumentMeasurementStore) CommitDocumentMeasurement(ctx context.Context, commit application.DocumentMeasurementCommit) error {
	claim, measurement := commit.Claim, commit.Measurement
	if ctx == nil || commit.MeasurementID.IsZero() || commit.AuditID.IsZero() || commit.OutboxID.IsZero() || commit.CorrelationID.IsZero() || commit.AttemptOutcomeID.IsZero() || commit.OccurredAt.IsZero() || claim.LeaseID.IsZero() || claim.TenantID != s.tenantID || claim.AssignmentGeneration != s.assignmentGeneration || measurement.TenantID != claim.TenantID || measurement.DocumentVersionID != claim.DocumentVersionID || measurement.MediaType != claim.MediaType || measurement.ByteSize != claim.SizeBytes || measurement.SHA256 != claim.SHA256 || measurement.ParserKey != claim.ParserKey || measurement.ParserVersion != claim.ParserVersion || measurement.SemanticsVersion != claim.SemanticsVersion || measurement.ProfileVersion != claim.ProfileVersion || commit.EstimatedCredits < 1 || (commit.UsageKind != "DOCUMENT_PDF" && commit.UsageKind != "DOCUMENT_DOCX") {
		return errors.New("document_measurement_commit_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, err := q.LockDocumentMeasurementJob(txCtx, capturesql.LockDocumentMeasurementJobParams{ID: dbID(claim.JobID), LeaseID: dbID(claim.LeaseID)})
		if err != nil {
			return err
		}
		if appID(row.DocumentVersionID) != claim.DocumentVersionID || appID(row.RequestedBy) != claim.RequestedBy || row.MediaType != claim.MediaType || row.SizeBytes != claim.SizeBytes || !bytes.Equal(row.Sha256, claim.SHA256[:]) || row.ScanState != application.CleanScanState {
			return errors.New("document_measurement_job_conflict")
		}
		params := capturesql.InsertDocumentMeasurementParams{
			ID:                   dbID(commit.MeasurementID),
			JobID:                dbID(claim.JobID),
			DocumentVersionID:    dbID(claim.DocumentVersionID),
			RequesterUserID:      dbID(claim.RequestedBy),
			AssignmentGeneration: claim.AssignmentGeneration,
			CorrelationID:        dbID(commit.CorrelationID),
			MediaType:            measurement.MediaType,
			ByteSize:             measurement.ByteSize,
			Sha256:               measurement.SHA256[:],
			ParserKey:            measurement.ParserKey,
			ParserVersion:        measurement.ParserVersion,
			SemanticsVersion:     measurement.SemanticsVersion,
			ProfileVersion:       measurement.ProfileVersion,
			Unit:                 string(measurement.Unit),
			MeasuredUnits:        measurement.MeasuredUnits,
			ApplicabilityHash:    measurement.ApplicabilityHash[:],
			UsageKind:            commit.UsageKind,
			EstimatedCredits:     commit.EstimatedCredits,
			CreatedAt:            dbTime(commit.OccurredAt),
		}
		if err = q.InsertDocumentMeasurement(txCtx, params); err != nil {
			return err
		}
		if err = q.InsertDocumentMeasurementAttemptOutcome(txCtx, capturesql.InsertDocumentMeasurementAttemptOutcomeParams{ID: dbID(commit.AttemptOutcomeID), LeaseID: dbID(claim.LeaseID), Outcome: "SUCCEEDED", OccurredAt: dbTime(commit.OccurredAt)}); err != nil {
			return err
		}
		affected, err := q.MarkDocumentMeasurementPendingReservation(txCtx, capturesql.MarkDocumentMeasurementPendingReservationParams{ID: dbID(claim.JobID), LeaseID: dbID(claim.LeaseID)})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		if err = q.InsertDocumentMeasurementAudit(txCtx, capturesql.InsertDocumentMeasurementAuditParams{ID: dbID(commit.AuditID), ActorID: dbID(claim.RequestedBy), MeasurementID: dbID(commit.MeasurementID), OccurredAt: dbTime(commit.OccurredAt), CorrelationID: dbID(commit.CorrelationID), DocumentVersionID: dbID(claim.DocumentVersionID), Unit: string(measurement.Unit), MeasuredUnits: measurement.MeasuredUnits, UsageKind: commit.UsageKind, EstimatedCredits: commit.EstimatedCredits, ParserKey: measurement.ParserKey, ParserVersion: measurement.ParserVersion, SemanticsVersion: measurement.SemanticsVersion, ProfileVersion: measurement.ProfileVersion, ApplicabilityHash: measurement.ApplicabilityHash[:]}); err != nil {
			return err
		}
		return q.InsertDocumentMeasurementOutbox(txCtx, capturesql.InsertDocumentMeasurementOutboxParams{ID: dbID(commit.OutboxID), ActorID: dbID(claim.RequestedBy), MeasurementID: dbID(commit.MeasurementID), JobID: dbID(claim.JobID), DocumentVersionID: dbID(claim.DocumentVersionID), Unit: string(measurement.Unit), MeasuredUnits: measurement.MeasuredUnits, UsageKind: commit.UsageKind, EstimatedCredits: commit.EstimatedCredits, ApplicabilityHash: measurement.ApplicabilityHash[:], OccurredAt: dbTime(commit.OccurredAt)})
	})
}

func (s *DocumentMeasurementStore) LinkDocumentMeasurementEntitlement(ctx context.Context, pending application.PendingDocumentReservation, checkpointID, auditID application.ID, at time.Time) error {
	if ctx == nil || checkpointID.IsZero() || auditID.IsZero() || at.IsZero() || pending.TenantID != s.tenantID || pending.AssignmentGeneration != s.assignmentGeneration {
		return errors.New("document_measurement_entitlement_link_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		existing, err := q.GetDocumentMeasurementEntitlement(txCtx, dbID(pending.MeasurementID))
		if err == nil {
			if appID(existing) == checkpointID {
				return nil
			}
			return errors.New("document_measurement_entitlement_conflict")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		matches, err := q.DocumentMeasurementCheckpointMatches(txCtx, capturesql.DocumentMeasurementCheckpointMatchesParams{CheckpointID: dbID(checkpointID), MeasurementID: dbID(pending.MeasurementID), JobID: dbID(pending.JobID), RequesterUserID: dbID(pending.RequestedBy), AssignmentGeneration: pending.AssignmentGeneration, UsageKind: pending.UsageKind, EstimatedCredits: pending.EstimatedCredits, TenantID: dbID(s.tenantID)})
		if err != nil {
			return err
		}
		if !matches {
			return errors.New("document_measurement_checkpoint_mismatch")
		}
		affected, err := q.InsertDocumentMeasurementEntitlement(txCtx, capturesql.InsertDocumentMeasurementEntitlementParams{MeasurementID: dbID(pending.MeasurementID), CheckpointID: dbID(checkpointID), LinkedAt: dbTime(at)})
		if err != nil {
			return err
		}
		if affected == 0 {
			existing, lookupErr := q.GetDocumentMeasurementEntitlement(txCtx, dbID(pending.MeasurementID))
			if lookupErr != nil {
				return lookupErr
			}
			if appID(existing) != checkpointID {
				return errors.New("document_measurement_entitlement_conflict")
			}
			return nil
		}
		if affected != 1 {
			return errors.New("document_measurement_entitlement_conflict")
		}
		return q.InsertDocumentMeasurementEntitlementAudit(txCtx, capturesql.InsertDocumentMeasurementEntitlementAuditParams{ID: dbID(auditID), ActorID: dbID(pending.RequestedBy), MeasurementID: dbID(pending.MeasurementID), OccurredAt: dbTime(at), CorrelationID: dbID(pending.CorrelationID), CheckpointID: dbID(checkpointID)})
	})
}

var _ application.DocumentMeasurementStore = (*DocumentMeasurementStore)(nil)
