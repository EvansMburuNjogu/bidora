package persistence

import (
	application "bidora/backend/internal/opportunity/application"
	qsql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func mapOutcomeUpload(r qsql.GetOutcomeProofUploadRow) application.OutcomeProofUpload {
	var h [32]byte
	copy(h[:], r.ExpectedSha256)
	return application.OutcomeProofUpload{ID: appID(r.ID), DocumentID: appID(r.DocumentID), BidID: appID(r.BidID), CompletedVersionID: appID(r.CompletedVersionID), ObjectKey: r.ObjectKey, SizeBytes: r.ExpectedSizeBytes, SHA256: h, State: r.State, ExpiresAt: r.ExpiresAt.Time.UTC()}
}
func (s *WorkflowStore) ReserveOutcomeProof(ctx context.Context, a application.WorkflowAuthority, c application.OutcomeProofReservation) (result application.OutcomeProofUpload, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		replay, ref, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "INITIATE_OUTCOME_PROOF_UPLOAD", c.KeyHash, c.RequestHash, c.CreatedAt)
		if errors.Is(e, application.ErrWorkflowConflict) {
			return application.ErrReportedOutcomeConflict
		}
		if e != nil {
			return e
		}
		if replay {
			r, e := q.GetOutcomeProofUpload(tc, qsql.GetOutcomeProofUploadParams{ID: dbID(ref), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
			if e != nil {
				return e
			}
			result = mapOutcomeUpload(r)
			result.Replay = true
			return nil
		}
		if _, e = q.LockSubmittedBidForOutcomeProof(tc, dbID(c.BidID)); errors.Is(e, pgx.ErrNoRows) {
			return application.ErrReportedOutcomeConflict
		} else if e != nil {
			return e
		}
		if e = q.InsertOutcomeProofDocument(tc, qsql.InsertOutcomeProofDocumentParams{ID: dbID(c.DocumentID), ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt), BidID: dbID(c.BidID)}); e != nil {
			return e
		}
		if e = q.InsertOutcomeProofUpload(tc, qsql.InsertOutcomeProofUploadParams{ID: dbID(c.UploadID), DocumentID: dbID(c.DocumentID), ObjectKey: c.ObjectKey, SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ExpiresAt: dbTime(c.ExpiresAt), ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt), BidID: dbID(c.BidID)}); e != nil {
			return e
		}
		if e = q.InsertOutcomeProofAudit(tc, qsql.InsertOutcomeProofAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "OutcomeProofUploadInitiated", TargetType: "document", TargetID: dbID(c.DocumentID), OccurredAt: dbTime(c.CreatedAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.UploadID, 1, c.CreatedAt); e != nil {
			return e
		}
		r, e := q.GetOutcomeProofUpload(tc, qsql.GetOutcomeProofUploadParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		result = mapOutcomeUpload(r)
		return nil
	})
	return
}
func (s *WorkflowStore) GetOutcomeProofUpload(ctx context.Context, a application.WorkflowAuthority, bid, upload application.ID) (result application.OutcomeProofUpload, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		r, e := qsql.New(db).GetOutcomeProofUpload(tc, qsql.GetOutcomeProofUploadParams{ID: dbID(upload), BidID: dbID(bid), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		result = mapOutcomeUpload(r)
		return nil
	})
	return
}
func (s *WorkflowStore) GetCompletedOutcomeProof(ctx context.Context, a application.WorkflowAuthority, bid, upload application.ID) (result application.OutcomeProofResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		r, e := qsql.New(db).GetOutcomeProofCompletion(tc, qsql.GetOutcomeProofCompletionParams{ID: dbID(upload), BidID: dbID(bid), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		result = application.OutcomeProofResult{UploadID: appID(r.UploadID), DocumentID: appID(r.DocumentID), VersionID: appID(r.VersionID), JobID: appID(r.ScanJobID), ScanState: r.ScanState}
		return nil
	})
	return
}
func (s *WorkflowStore) CompleteOutcomeProof(ctx context.Context, a application.WorkflowAuthority, c application.OutcomeProofCompletion) (result application.OutcomeProofResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		replay, _, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "COMPLETE_OUTCOME_PROOF_UPLOAD", c.KeyHash, c.RequestHash, c.OccurredAt)
		if errors.Is(e, application.ErrWorkflowConflict) {
			return application.ErrReportedOutcomeConflict
		}
		if e != nil {
			return e
		}
		if replay {
			r, e := q.GetOutcomeProofCompletion(tc, qsql.GetOutcomeProofCompletionParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
			if e != nil {
				return e
			}
			result = application.OutcomeProofResult{UploadID: appID(r.UploadID), DocumentID: appID(r.DocumentID), VersionID: appID(r.VersionID), JobID: appID(r.ScanJobID), ScanState: r.ScanState, Replay: true}
			return nil
		}
		u, e := q.LockOutcomeProofUpload(tc, qsql.LockOutcomeProofUploadParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		if u.State != "INITIATED" || appID(u.DocumentID) != c.DocumentID || u.ObjectKey != c.ObjectKey || c.ObjectVersion == "" || u.ExpectedSizeBytes != c.SizeBytes || !bytes.Equal(u.ExpectedSha256, c.SHA256[:]) || c.OccurredAt.After(u.ExpiresAt.Time) {
			return application.ErrReportedOutcomeConflict
		}
		if e = q.InsertTenderDocumentVersion(tc, qsql.InsertTenderDocumentVersionParams{ID: dbID(c.VersionID), DocumentID: u.DocumentID, MediaType: "application/pdf", SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ObjectKey: c.ObjectKey, ObjectVersion: c.ObjectVersion, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		n, e := q.CompleteDocumentUploadSession(tc, qsql.CompleteDocumentUploadSessionParams{VersionID: dbID(c.VersionID), CompletedAt: dbTime(c.OccurredAt), ID: u.ID})
		if e != nil || n != 1 {
			return affectedErr(e)
		}
		n, e = q.ActivateTenderDocument(tc, qsql.ActivateTenderDocumentParams{UpdatedAt: dbTime(c.OccurredAt), ID: u.DocumentID})
		if e != nil || n != 1 {
			return affectedErr(e)
		}
		if e = q.InsertDocumentJob(tc, qsql.InsertDocumentJobParams{ID: dbID(c.JobID), Kind: "MALWARE_SCAN", DocumentVersionID: dbID(c.VersionID), State: "QUEUED", ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		if e = q.InsertOutcomeProofAudit(tc, qsql.InsertOutcomeProofAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "OutcomeProofUploadCompleted", TargetType: "document_version", TargetID: dbID(c.VersionID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID)}); e != nil {
			return e
		}
		if e = q.InsertEvidenceVersionQuarantinedOutbox(tc, qsql.InsertEvidenceVersionQuarantinedOutboxParams{ID: dbID(c.OutboxID), DocumentID: u.DocumentID, VersionID: dbID(c.VersionID), ScanJobID: dbID(c.JobID), BidID: dbID(c.BidID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.VersionID, 2, c.OccurredAt); e != nil {
			return e
		}
		result = application.OutcomeProofResult{UploadID: c.UploadID, DocumentID: c.DocumentID, VersionID: c.VersionID, JobID: c.JobID, ScanState: "QUARANTINED"}
		return nil
	})
	return
}
func (s *WorkflowStore) FindReportedOutcomeReplay(ctx context.Context, a application.WorkflowAuthority, bid application.ID, key, request [32]byte) (result application.ReportOutcomeResult, found bool, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		r, e := q.GetReportedOutcomeIdempotency(tc, qsql.GetReportedOutcomeIdempotencyParams{ActorID: dbID(a.ActorID), KeyHash: key[:]})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		found = true
		if !bytes.Equal(r.RequestHash, request[:]) {
			return application.ErrReportedOutcomeConflict
		}
		if r.State != "COMPLETED" || r.ResponseStatus == nil || *r.ResponseStatus != 201 || !r.ResponseRef.Valid {
			return errors.New("outcome_pending")
		}
		v, e := q.GetReportedOutcome(tc, r.ResponseRef)
		if e != nil {
			return e
		}
		result = application.ReportOutcomeResult{Outcome: mapOutcome(v), Replay: true}
		return nil
	})
	return
}
func (s *WorkflowStore) CommitReportedOutcome(ctx context.Context, a application.WorkflowAuthority, c application.ReportOutcomeCommit) (result application.ReportOutcomeResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		replay, ref, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "REPORT_BID_OUTCOME", c.KeyHash, c.RequestHash, c.OccurredAt)
		if errors.Is(e, application.ErrWorkflowConflict) {
			return application.ErrReportedOutcomeConflict
		}
		if e != nil {
			return e
		}
		if replay {
			v, e := q.GetReportedOutcome(tc, dbID(ref))
			if e != nil {
				return e
			}
			result = application.ReportOutcomeResult{Outcome: mapOutcome(v), Replay: true}
			return nil
		}
		locked, e := q.LockBidForReportedOutcome(tc, dbID(c.BidID))
		if e != nil {
			return e
		}
		if locked.Version != c.ExpectedWorkspaceVersion || locked.CurrentSequence != c.ExpectedSequence || appID(locked.ActiveOutcomeID) != c.SupersedesID {
			return application.ErrReportedOutcomeConflict
		}
		if !c.ProofVersionID.IsZero() {
			if _, e = q.ValidateOutcomeProof(tc, qsql.ValidateOutcomeProofParams{VersionID: dbID(c.ProofVersionID), BidID: dbID(c.BidID)}); e != nil {
				return application.ErrReportedOutcomeConflict
			}
		}
		next := c.ExpectedSequence + 1
		var lessons, correction *string
		if c.Lessons != "" {
			lessons = &c.Lessons
		}
		if c.CorrectionReason != "" {
			correction = &c.CorrectionReason
		}
		e = q.InsertReportedOutcome(tc, qsql.InsertReportedOutcomeParams{ID: dbID(c.OutcomeID), BidID: dbID(c.BidID), Sequence: next, Outcome: c.Outcome, SourceClass: c.SourceClass, Reason: c.Reason, Lessons: lessons, ReportedText: c.ReportedText, ReportedAt: dbTime(c.ReportedAt), RecordedAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID), ProofVersionID: dbID(c.ProofVersionID), SupersedesID: dbID(c.SupersedesID), CorrectionReason: correction})
		if e != nil {
			return e
		}
		if e = q.InsertReportedOutcomeAudit(tc, qsql.InsertReportedOutcomeAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), OutcomeID: dbID(c.OutcomeID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), Sequence: next, ReportedOutcome: c.Outcome}); e != nil {
			return e
		}
		if e = q.InsertReportedOutcomeOutbox(tc, qsql.InsertReportedOutcomeOutboxParams{ID: dbID(c.OutboxID), BidID: dbID(c.BidID), Sequence: int64(next), OutcomeID: dbID(c.OutcomeID), ReportedOutcome: c.Outcome, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.OutcomeID, int64(next), c.OccurredAt); e != nil {
			return e
		}
		v, e := q.GetReportedOutcome(tc, dbID(c.OutcomeID))
		if e != nil {
			return e
		}
		result = application.ReportOutcomeResult{Outcome: mapOutcome(v)}
		return nil
	})
	return
}
func (s *WorkflowStore) GetCurrentReportedOutcome(ctx context.Context, a application.WorkflowAuthority, bid application.ID) (result application.ReportedOutcome, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		v, e := qsql.New(db).GetCurrentReportedOutcome(tc, dbID(bid))
		if errors.Is(e, pgx.ErrNoRows) {
			return application.ErrReportedOutcomeNotFound
		}
		if e != nil {
			return e
		}
		result = mapOutcome(v)
		return nil
	})
	return
}
func (s *WorkflowStore) ListReportedOutcomes(ctx context.Context, a application.WorkflowAuthority, bid application.ID) (result []application.ReportedOutcome, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		rows, e := qsql.New(db).ListReportedOutcomes(tc, dbID(bid))
		if e != nil {
			return e
		}
		if len(rows) > 100 {
			return application.ErrReportedOutcomeReadLimit
		}
		result = make([]application.ReportedOutcome, len(rows))
		for i, v := range rows {
			result[i] = mapOutcome(v)
		}
		return nil
	})
	return
}
func mapOutcome(v qsql.BidReportedOutcome) application.ReportedOutcome {
	lessons, correction := "", ""
	if v.Lessons != nil {
		lessons = *v.Lessons
	}
	if v.CorrectionReason != nil {
		correction = *v.CorrectionReason
	}
	return application.ReportedOutcome{ID: appID(v.ID), BidID: appID(v.BidID), ProofVersionID: appID(v.ProofDocumentVersionID), SupersedesID: appID(v.SupersedesOutcomeID), ReportedBy: appID(v.ReportedBy), Sequence: v.Sequence, Outcome: v.Outcome, VerificationStatus: v.VerificationStatus, Provenance: v.Provenance, SourceClass: v.SourceClass, Reason: v.Reason, Lessons: lessons, ReportedTimeText: v.ReportedOutcomeTimeText, CorrectionReason: correction, ReportedAt: v.ReportedOutcomeAt.Time.UTC(), RecordedAt: v.RecordedAt.Time.UTC()}
}
