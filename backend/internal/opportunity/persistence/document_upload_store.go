package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

func (s *WorkflowStore) ReserveUpload(ctx context.Context, a application.WorkflowAuthority, c application.UploadReservation) (result application.UploadSession, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "INITIATE_TENDER_DOCUMENT_UPLOAD", c.KeyHash, c.RequestHash, c.CreatedAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetDocumentUploadSession(txCtx, capturesql.GetDocumentUploadSessionParams{ID: dbID(ref), OpportunityID: dbID(c.OpportunityID), ActorID: dbID(a.ActorID)})
			if err != nil {
				return err
			}
			result = mapUploadSession(row)
			result.Replay = true
			return nil
		}
		if _, err = q.LockOpportunityForDocumentUpload(txCtx, dbID(c.OpportunityID)); err != nil {
			return err
		}
		if err = q.InsertTenderDocument(txCtx, capturesql.InsertTenderDocumentParams{ID: dbID(c.DocumentID), OpportunityID: dbID(c.OpportunityID), Title: c.Title, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt)}); err != nil {
			return err
		}
		if err = q.InsertDocumentUploadSession(txCtx, capturesql.InsertDocumentUploadSessionParams{ID: dbID(c.UploadID), DocumentID: dbID(c.DocumentID), OpportunityID: dbID(c.OpportunityID), ObjectKey: c.ObjectKey, MediaType: c.MediaType, SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ExpiresAt: dbTime(c.ExpiresAt), ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt)}); err != nil {
			return err
		}
		if err = q.InsertDocumentUploadInitiatedAudit(txCtx, capturesql.InsertDocumentUploadInitiatedAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), DocumentID: dbID(c.DocumentID), OccurredAt: dbTime(c.CreatedAt), CorrelationID: dbID(c.CorrelationID), OpportunityID: dbID(c.OpportunityID), UploadID: dbID(c.UploadID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.UploadID, 1, c.CreatedAt); err != nil {
			return err
		}
		row, err := q.GetDocumentUploadSession(txCtx, capturesql.GetDocumentUploadSessionParams{ID: dbID(c.UploadID), OpportunityID: dbID(c.OpportunityID), ActorID: dbID(a.ActorID)})
		if err != nil {
			return err
		}
		result = mapUploadSession(row)
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetUpload(ctx context.Context, a application.WorkflowAuthority, opportunityID, uploadID application.ID) (result application.UploadSession, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetDocumentUploadSession(txCtx, capturesql.GetDocumentUploadSessionParams{ID: dbID(uploadID), OpportunityID: dbID(opportunityID), ActorID: dbID(a.ActorID)})
		if err != nil {
			return err
		}
		result = mapUploadSession(row)
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetCompletedUpload(ctx context.Context, a application.WorkflowAuthority, opportunityID, uploadID application.ID) (result application.UploadResult, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetCompletedDocumentUploadResult(txCtx, capturesql.GetCompletedDocumentUploadResultParams{ID: dbID(uploadID), OpportunityID: dbID(opportunityID), ActorID: dbID(a.ActorID)})
		if err != nil {
			return err
		}
		result = application.UploadResult{UploadID: appID(row.UploadID), DocumentID: appID(row.DocumentID), DocumentVersionID: appID(row.DocumentVersionID), ScanJobID: appID(row.ScanJobID), AnalysisJobID: appID(row.AnalysisJobID), ScanState: row.ScanState, AnalysisState: row.AnalysisState}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) CompleteUpload(ctx context.Context, a application.WorkflowAuthority, c application.UploadCompletion) (result application.UploadResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, _, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "COMPLETE_TENDER_DOCUMENT_UPLOAD", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetCompletedDocumentUploadResult(txCtx, capturesql.GetCompletedDocumentUploadResultParams{ID: dbID(c.UploadID), OpportunityID: dbID(c.OpportunityID), ActorID: dbID(a.ActorID)})
			if err != nil {
				return err
			}
			result = application.UploadResult{UploadID: appID(row.UploadID), DocumentID: appID(row.DocumentID), DocumentVersionID: appID(row.DocumentVersionID), ScanJobID: appID(row.ScanJobID), AnalysisJobID: appID(row.AnalysisJobID), ScanState: row.ScanState, AnalysisState: row.AnalysisState, Replay: true}
			return nil
		}
		session, err := q.LockDocumentUploadSession(txCtx, capturesql.LockDocumentUploadSessionParams{ID: dbID(c.UploadID), OpportunityID: dbID(c.OpportunityID), ActorID: dbID(a.ActorID)})
		if err != nil {
			return err
		}
		if session.State != "INITIATED" || session.DocumentID != dbID(c.DocumentID) || session.ObjectKey != c.ObjectKey || session.ExpectedMediaType != c.MediaType || session.ExpectedSizeBytes != c.SizeBytes || !bytesEqual(session.ExpectedSha256, c.SHA256[:]) {
			return errors.New("document_upload_conflict")
		}
		if c.OccurredAt.After(session.ExpiresAt.Time) {
			return errors.New("document_upload_expired")
		}
		if err = q.InsertTenderDocumentVersion(txCtx, capturesql.InsertTenderDocumentVersionParams{ID: dbID(c.VersionID), DocumentID: session.DocumentID, MediaType: c.MediaType, SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ObjectKey: c.ObjectKey, ObjectVersion: c.ObjectVersion, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		affected, err := q.CompleteDocumentUploadSession(txCtx, capturesql.CompleteDocumentUploadSessionParams{VersionID: dbID(c.VersionID), CompletedAt: dbTime(c.OccurredAt), ID: session.ID})
		if err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return errors.New("document_upload_conflict")
		}
		affected, err = q.ActivateTenderDocument(txCtx, capturesql.ActivateTenderDocumentParams{UpdatedAt: dbTime(c.OccurredAt), ID: session.DocumentID})
		if err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return errors.New("document_state_conflict")
		}
		if err = q.InsertDocumentJob(txCtx, capturesql.InsertDocumentJobParams{ID: dbID(c.ScanJobID), Kind: "MALWARE_SCAN", DocumentVersionID: dbID(c.VersionID), State: "QUEUED", ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		blocked := "DOCUMENT_NOT_CLEAN"
		if err = q.InsertDocumentJob(txCtx, capturesql.InsertDocumentJobParams{ID: dbID(c.AnalysisJobID), Kind: "TENDER_ANALYSIS", DocumentVersionID: dbID(c.VersionID), State: "WAITING_FOR_CLEAN", BlockedReason: &blocked, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertDocumentUploadCompletedAudit(txCtx, capturesql.InsertDocumentUploadCompletedAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), VersionID: dbID(c.VersionID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), DocumentID: session.DocumentID}); err != nil {
			return err
		}
		if err = q.InsertDocumentVersionQuarantinedOutbox(txCtx, capturesql.InsertDocumentVersionQuarantinedOutboxParams{ID: dbID(c.OutboxID), DocumentID: session.DocumentID, VersionID: dbID(c.VersionID), ScanJobID: dbID(c.ScanJobID), AnalysisJobID: dbID(c.AnalysisJobID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.VersionID, 2, c.OccurredAt); err != nil {
			return err
		}
		result = application.UploadResult{UploadID: c.UploadID, DocumentID: c.DocumentID, DocumentVersionID: c.VersionID, ScanJobID: c.ScanJobID, AnalysisJobID: c.AnalysisJobID, ScanState: "QUARANTINED", AnalysisState: "WAITING_FOR_CLEAN"}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetDocumentJob(ctx context.Context, a application.WorkflowAuthority, jobID application.ID) (result application.DocumentJob, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetOwnedDocumentJob(txCtx, capturesql.GetOwnedDocumentJobParams{ID: dbID(jobID), ActorID: dbID(a.ActorID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrDocumentJobNotFound
		}
		if err != nil {
			return err
		}
		result = application.DocumentJob{ID: appID(row.ID), DocumentVersionID: appID(row.DocumentVersionID), DocumentID: appID(row.DocumentID), OpportunityID: appID(row.OpportunityID), BidID: appID(row.BidID), HasOpportunity: row.OpportunityID.Valid, HasBid: row.BidID.Valid, DocumentKind: row.DocumentKind, Kind: row.Kind, State: row.State, Attempts: row.Attempts, CreatedAt: row.CreatedAt.Time.UTC(), StartedAt: timePointer(row.StartedAt), CompletedAt: timePointer(row.CompletedAt)}
		if row.BlockedReason != nil {
			result.BlockedReason = *row.BlockedReason
		}
		return nil
	})
	return result, err
}
func mapUploadSession(row capturesql.GetDocumentUploadSessionRow) application.UploadSession {
	var sum [32]byte
	copy(sum[:], row.ExpectedSha256)
	return application.UploadSession{ID: appID(row.ID), DocumentID: appID(row.DocumentID), OpportunityID: appID(row.OpportunityID), ObjectKey: row.ObjectKey, MediaType: row.ExpectedMediaType, SizeBytes: row.ExpectedSizeBytes, SHA256: sum, State: row.State, ExpiresAt: row.ExpiresAt.Time.UTC(), CompletedVersionID: appID(row.CompletedVersionID)}
}
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
