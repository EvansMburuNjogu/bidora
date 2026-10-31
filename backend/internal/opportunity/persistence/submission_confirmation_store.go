package persistence

import (
	application "bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

func mapProofUpload(r capturesql.GetSubmissionProofUploadRow) application.SubmissionProofUpload {
	var h [32]byte
	copy(h[:], r.ExpectedSha256)
	return application.SubmissionProofUpload{ID: appID(r.ID), DocumentID: appID(r.DocumentID), BidID: appID(r.BidID), EvidenceID: appID(r.ExternalActionEvidenceID), CompletedVersionID: appID(r.CompletedVersionID), ObjectKey: r.ObjectKey, SizeBytes: r.ExpectedSizeBytes, SHA256: h, State: r.State, ExpiresAt: r.ExpiresAt.Time.UTC()}
}
func (s *WorkflowStore) ReserveSubmissionProof(ctx context.Context, a application.WorkflowAuthority, c application.SubmissionProofReservation) (result application.SubmissionProofUpload, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "INITIATE_SUBMISSION_PROOF_UPLOAD", c.KeyHash, c.RequestHash, c.CreatedAt)
		if errors.Is(e, application.ErrWorkflowConflict) {
			return application.ErrSubmissionConflict
		}
		if e != nil {
			return e
		}
		if replay {
			r, e := q.GetSubmissionProofUpload(tc, capturesql.GetSubmissionProofUploadParams{ID: dbID(ref), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID), ActorID: dbID(a.ActorID)})
			if e != nil {
				return e
			}
			result = mapProofUpload(r)
			result.Replay = true
			return nil
		}
		if _, e = q.LockExternalReportForProofUpload(tc, capturesql.LockExternalReportForProofUploadParams{EvidenceID: dbID(c.EvidenceID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)}); errors.Is(e, pgx.ErrNoRows) {
			return application.ErrSubmissionConflict
		} else if e != nil {
			return e
		}
		if e = q.InsertSubmissionProofDocument(tc, capturesql.InsertSubmissionProofDocumentParams{ID: dbID(c.DocumentID), ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID)}); e != nil {
			return e
		}
		if e = q.InsertSubmissionProofUpload(tc, capturesql.InsertSubmissionProofUploadParams{ID: dbID(c.UploadID), DocumentID: dbID(c.DocumentID), ObjectKey: c.ObjectKey, SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ExpiresAt: dbTime(c.ExpiresAt), ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID)}); e != nil {
			return e
		}
		if e = q.InsertSubmissionProofAudit(tc, capturesql.InsertSubmissionProofAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "SubmissionProofUploadInitiated", TargetType: "document", TargetID: dbID(c.DocumentID), OccurredAt: dbTime(c.CreatedAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.UploadID, 1, c.CreatedAt); e != nil {
			return e
		}
		r, e := q.GetSubmissionProofUpload(tc, capturesql.GetSubmissionProofUploadParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		result = mapProofUpload(r)
		return nil
	})
	return
}
func (s *WorkflowStore) GetSubmissionProofUpload(ctx context.Context, a application.WorkflowAuthority, bid, evidence, upload application.ID) (result application.SubmissionProofUpload, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		r, e := capturesql.New(db).GetSubmissionProofUpload(tc, capturesql.GetSubmissionProofUploadParams{ID: dbID(upload), BidID: dbID(bid), EvidenceID: dbID(evidence), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		result = mapProofUpload(r)
		return nil
	})
	return
}
func (s *WorkflowStore) GetCompletedSubmissionProof(ctx context.Context, a application.WorkflowAuthority, bid, evidence, upload application.ID) (result application.SubmissionProofResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		r, e := capturesql.New(db).GetSubmissionProofCompletion(tc, capturesql.GetSubmissionProofCompletionParams{ID: dbID(upload), BidID: dbID(bid), EvidenceID: dbID(evidence), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		result = application.SubmissionProofResult{UploadID: appID(r.UploadID), DocumentID: appID(r.DocumentID), VersionID: appID(r.VersionID), JobID: appID(r.ScanJobID), ScanState: r.ScanState}
		return nil
	})
	return
}
func (s *WorkflowStore) CompleteSubmissionProof(ctx context.Context, a application.WorkflowAuthority, c application.SubmissionProofCompletion) (result application.SubmissionProofResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, _, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "COMPLETE_SUBMISSION_PROOF_UPLOAD", c.KeyHash, c.RequestHash, c.OccurredAt)
		if errors.Is(e, application.ErrWorkflowConflict) {
			return application.ErrSubmissionConflict
		}
		if e != nil {
			return e
		}
		if replay {
			r, e := q.GetSubmissionProofCompletion(tc, capturesql.GetSubmissionProofCompletionParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID), ActorID: dbID(a.ActorID)})
			if e != nil {
				return e
			}
			result = application.SubmissionProofResult{UploadID: appID(r.UploadID), DocumentID: appID(r.DocumentID), VersionID: appID(r.VersionID), JobID: appID(r.ScanJobID), ScanState: r.ScanState, Replay: true}
			return nil
		}
		u, e := q.LockSubmissionProofUpload(tc, capturesql.LockSubmissionProofUploadParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID), ActorID: dbID(a.ActorID)})
		if e != nil {
			return e
		}
		if u.State != "INITIATED" || appID(u.DocumentID) != c.DocumentID || u.ObjectKey != c.ObjectKey || c.ObjectVersion == "" || u.ExpectedMediaType != "application/pdf" || u.ExpectedSizeBytes != c.SizeBytes || !bytes.Equal(u.ExpectedSha256, c.SHA256[:]) || c.OccurredAt.After(u.ExpiresAt.Time) {
			return application.ErrSubmissionConflict
		}
		if e = q.InsertTenderDocumentVersion(tc, capturesql.InsertTenderDocumentVersionParams{ID: dbID(c.VersionID), DocumentID: u.DocumentID, MediaType: "application/pdf", SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ObjectKey: c.ObjectKey, ObjectVersion: c.ObjectVersion, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		n, e := q.CompleteDocumentUploadSession(tc, capturesql.CompleteDocumentUploadSessionParams{VersionID: dbID(c.VersionID), CompletedAt: dbTime(c.OccurredAt), ID: u.ID})
		if e != nil || n != 1 {
			return affectedErr(e)
		}
		n, e = q.ActivateTenderDocument(tc, capturesql.ActivateTenderDocumentParams{UpdatedAt: dbTime(c.OccurredAt), ID: u.DocumentID})
		if e != nil || n != 1 {
			return affectedErr(e)
		}
		if e = q.InsertDocumentJob(tc, capturesql.InsertDocumentJobParams{ID: dbID(c.JobID), Kind: "MALWARE_SCAN", DocumentVersionID: dbID(c.VersionID), State: "QUEUED", ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		if e = q.InsertSubmissionProofAudit(tc, capturesql.InsertSubmissionProofAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "SubmissionProofUploadCompleted", TargetType: "document_version", TargetID: dbID(c.VersionID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID)}); e != nil {
			return e
		}
		if e = q.InsertEvidenceVersionQuarantinedOutbox(tc, capturesql.InsertEvidenceVersionQuarantinedOutboxParams{ID: dbID(c.OutboxID), DocumentID: u.DocumentID, VersionID: dbID(c.VersionID), ScanJobID: dbID(c.JobID), BidID: dbID(c.BidID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.VersionID, 2, c.OccurredAt); e != nil {
			return e
		}
		result = application.SubmissionProofResult{UploadID: c.UploadID, DocumentID: c.DocumentID, VersionID: c.VersionID, JobID: c.JobID, ScanState: "QUARANTINED"}
		return nil
	})
	return
}
func (s *WorkflowStore) FindSubmissionReplay(ctx context.Context, a application.WorkflowAuthority, bid application.ID, key, request [32]byte) (result application.SubmissionConfirmationResult, found bool, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		r, e := q.GetSubmissionConfirmationIdempotency(tc, capturesql.GetSubmissionConfirmationIdempotencyParams{ActorID: dbID(a.ActorID), KeyHash: key[:]})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		found = true
		if !bytes.Equal(r.RequestHash, request[:]) {
			return application.ErrSubmissionConflict
		}
		if r.State != "COMPLETED" || r.ResponseStatus == nil || *r.ResponseStatus != 201 || !r.ResponseRef.Valid {
			return errors.New("submission_pending")
		}
		v, e := q.GetSubmissionConfirmation(tc, r.ResponseRef)
		if e != nil {
			return e
		}
		result = application.SubmissionConfirmationResult{Confirmation: mapSubmission(v), Replay: true}
		return nil
	})
	return
}
func (s *WorkflowStore) CommitSubmissionConfirmation(ctx context.Context, a application.WorkflowAuthority, c application.SubmissionConfirmationCommit) (result application.SubmissionConfirmationResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "CONFIRM_EXTERNAL_SUBMISSION", c.KeyHash, c.RequestHash, c.OccurredAt)
		if errors.Is(e, application.ErrWorkflowConflict) {
			return application.ErrSubmissionConflict
		}
		if e != nil {
			return e
		}
		if replay {
			v, e := q.GetSubmissionConfirmation(tc, dbID(ref))
			if e != nil {
				return e
			}
			result = application.SubmissionConfirmationResult{Confirmation: mapSubmission(v), Replay: true}
			return nil
		}
		x, e := q.LockSubmissionConfirmationCandidate(tc, capturesql.LockSubmissionConfirmationCandidateParams{ObservedAt: dbTime(c.OccurredAt), BidID: dbID(c.BidID), EvidenceID: dbID(c.EvidenceID), ManifestID: dbID(c.ManifestID), ProofVersionID: dbID(c.ProofVersionID), VerifierID: dbID(a.ActorID)})
		if errors.Is(e, pgx.ErrNoRows) {
			return application.ErrSubmissionConflict
		}
		if e != nil {
			return e
		}
		var blockers []string
		if json.Unmarshal(x.ReadinessBlockers, &blockers) != nil || !x.ReadinessReady || len(blockers) > 0 || x.WorkspaceVersion != c.ExpectedWorkspaceVersion || x.MandatoryUnsatisfied > 0 || x.OpenUnassigned > 0 || x.OpenOverdue > 0 || x.MandatoryResponseMissing > 0 || x.ExpiredEvidenceCount > 0 {
			return application.ErrSubmissionConflict
		}
		next := x.WorkspaceVersion + 1
		n, e := q.MarkWorkspaceSubmitted(tc, capturesql.MarkWorkspaceSubmittedParams{UpdatedAt: dbTime(c.OccurredAt), ID: dbID(c.BidID), ExpectedVersion: x.WorkspaceVersion})
		if e != nil || n != 1 {
			return application.ErrSubmissionConflict
		}
		if e = q.InsertSubmissionConfirmation(tc, capturesql.InsertSubmissionConfirmationParams{ID: dbID(c.ConfirmationID), BidID: dbID(c.BidID), EvidenceID: x.EvidenceID, ManifestID: x.PackageManifestID, ApprovalID: x.ApprovalID, ProofVersionID: x.ProofVersionID, ReporterID: x.RecordedBy, VerifierID: dbID(a.ActorID), ReportedActionAt: x.ReportedActionAt, ReportRecordedAt: x.RecordedAt, ConfirmedAt: dbTime(c.OccurredAt), Rationale: c.Rationale, WorkspaceVersion: next}); e != nil {
			return e
		}
		if e = q.InsertSubmissionConfirmationAudit(tc, capturesql.InsertSubmissionConfirmationAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), ConfirmationID: dbID(c.ConfirmationID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), EvidenceID: x.EvidenceID, ManifestID: x.PackageManifestID, ApprovalID: x.ApprovalID, ProofVersionID: x.ProofVersionID, WorkspaceVersion: next}); e != nil {
			return e
		}
		if e = q.InsertSubmissionConfirmationOutbox(tc, capturesql.InsertSubmissionConfirmationOutboxParams{ID: dbID(c.OutboxID), BidID: dbID(c.BidID), WorkspaceVersion: next, ConfirmationID: dbID(c.ConfirmationID), EvidenceID: x.EvidenceID, ManifestID: x.PackageManifestID, ApprovalID: x.ApprovalID, ProofVersionID: x.ProofVersionID, ActorID: dbID(a.ActorID), OccurredAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.ConfirmationID, next, c.OccurredAt); e != nil {
			return e
		}
		v, e := q.GetSubmissionConfirmation(tc, dbID(c.ConfirmationID))
		if e != nil {
			return e
		}
		result = application.SubmissionConfirmationResult{Confirmation: mapSubmission(v)}
		return nil
	})
	return
}
func (s *WorkflowStore) GetSubmissionConfirmationByBid(ctx context.Context, a application.WorkflowAuthority, bid application.ID) (result application.SubmissionConfirmation, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		v, e := capturesql.New(db).GetSubmissionConfirmationByBid(tc, dbID(bid))
		if errors.Is(e, pgx.ErrNoRows) {
			return application.ErrSubmissionNotFound
		}
		if e != nil {
			return e
		}
		result = mapSubmission(v)
		return nil
	})
	return
}
func mapSubmission(v capturesql.BidSubmissionConfirmation) application.SubmissionConfirmation {
	return application.SubmissionConfirmation{ID: appID(v.ID), BidID: appID(v.BidID), EvidenceID: appID(v.ExternalActionEvidenceID), ManifestID: appID(v.PackageManifestID), ApprovalID: appID(v.ProposalApprovalID), ProofVersionID: appID(v.ProofDocumentVersionID), ReporterID: appID(v.ReporterID), VerifierID: appID(v.VerifierID), ReportedActionAt: v.ReportedActionAt.Time.UTC(), ReportRecordedAt: v.ReportRecordedAt.Time.UTC(), ConfirmedAt: v.ConfirmedAt.Time.UTC(), Rationale: v.Rationale, SubmissionBasis: v.SubmissionBasis, PolicyVersion: v.PolicyVersion, WorkspaceVersion: v.WorkspaceVersion}
}
