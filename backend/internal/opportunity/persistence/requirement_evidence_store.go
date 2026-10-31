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

func (s *WorkflowStore) ReserveEvidenceUpload(ctx context.Context, a application.WorkflowAuthority, c application.EvidenceUploadReservation) (result application.EvidenceUploadSession, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "INITIATE_REQUIREMENT_EVIDENCE_UPLOAD", c.KeyHash, c.RequestHash, c.CreatedAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetEvidenceUploadSession(txCtx, capturesql.GetEvidenceUploadSessionParams{ID: dbID(ref), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
			if err != nil {
				return err
			}
			result = mapEvidenceSession(row)
			result.Replay = true
			return nil
		}
		if _, err = q.LockBidForEvidenceUpload(txCtx, dbID(c.BidID)); errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementEvidenceConflict
		} else if err != nil {
			return err
		}
		if err = q.InsertBidEvidenceDocument(txCtx, capturesql.InsertBidEvidenceDocumentParams{ID: dbID(c.DocumentID), BidID: dbID(c.BidID), Title: c.Title, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt)}); err != nil {
			return err
		}
		if err = q.InsertEvidenceUploadSession(txCtx, capturesql.InsertEvidenceUploadSessionParams{ID: dbID(c.UploadID), DocumentID: dbID(c.DocumentID), BidID: dbID(c.BidID), ObjectKey: c.ObjectKey, MediaType: c.MediaType, SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ExpiresAt: dbTime(c.ExpiresAt), EvidenceExpiresAt: dbNullableTime(c.EvidenceExpiresAt), ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.CreatedAt)}); err != nil {
			return err
		}
		if err = q.InsertEvidenceUploadAudit(txCtx, capturesql.InsertEvidenceUploadAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "RequirementEvidenceUploadInitiated", TargetType: "document", TargetID: dbID(c.DocumentID), OccurredAt: dbTime(c.CreatedAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), UploadID: dbID(c.UploadID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.UploadID, 1, c.CreatedAt); err != nil {
			return err
		}
		row, err := q.GetEvidenceUploadSession(txCtx, capturesql.GetEvidenceUploadSessionParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
		if err != nil {
			return err
		}
		result = mapEvidenceSession(row)
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetEvidenceUpload(ctx context.Context, a application.WorkflowAuthority, bidID, uploadID application.ID) (result application.EvidenceUploadSession, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetEvidenceUploadSession(txCtx, capturesql.GetEvidenceUploadSessionParams{ID: dbID(uploadID), BidID: dbID(bidID), ActorID: dbID(a.ActorID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementEvidenceConflict
		}
		if err != nil {
			return err
		}
		result = mapEvidenceSession(row)
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetCompletedEvidenceUpload(ctx context.Context, a application.WorkflowAuthority, bidID, uploadID application.ID) (result application.EvidenceUploadResult, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetCompletedEvidenceUploadResult(txCtx, capturesql.GetCompletedEvidenceUploadResultParams{ID: dbID(uploadID), BidID: dbID(bidID), ActorID: dbID(a.ActorID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementEvidenceConflict
		}
		if err != nil {
			return err
		}
		result = application.EvidenceUploadResult{UploadID: appID(row.UploadID), DocumentID: appID(row.DocumentID), DocumentVersionID: appID(row.DocumentVersionID), ScanJobID: appID(row.ScanJobID), ScanState: row.ScanState, EvidenceExpiresAt: timePointer(row.ExpiresAt)}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) CompleteEvidenceUpload(ctx context.Context, a application.WorkflowAuthority, c application.EvidenceUploadCompletion) (result application.EvidenceUploadResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, _, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "COMPLETE_REQUIREMENT_EVIDENCE_UPLOAD", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetCompletedEvidenceUploadResult(txCtx, capturesql.GetCompletedEvidenceUploadResultParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
			if err != nil {
				return err
			}
			result = application.EvidenceUploadResult{UploadID: appID(row.UploadID), DocumentID: appID(row.DocumentID), DocumentVersionID: appID(row.DocumentVersionID), ScanJobID: appID(row.ScanJobID), ScanState: row.ScanState, EvidenceExpiresAt: timePointer(row.ExpiresAt), Replay: true}
			return nil
		}
		session, err := q.LockEvidenceUploadSession(txCtx, capturesql.LockEvidenceUploadSessionParams{ID: dbID(c.UploadID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementEvidenceConflict
		}
		if err != nil {
			return err
		}
		if session.State != "INITIATED" || session.DocumentID != dbID(c.DocumentID) || session.ObjectKey != c.ObjectKey || session.ExpectedMediaType != c.MediaType || session.ExpectedSizeBytes != c.SizeBytes || !bytesEqual(session.ExpectedSha256, c.SHA256[:]) || c.OccurredAt.After(session.ExpiresAt.Time) {
			return application.ErrRequirementEvidenceConflict
		}
		if err = q.InsertTenderDocumentVersion(txCtx, capturesql.InsertTenderDocumentVersionParams{ID: dbID(c.VersionID), DocumentID: session.DocumentID, MediaType: c.MediaType, SizeBytes: c.SizeBytes, Sha256: c.SHA256[:], ObjectKey: c.ObjectKey, ObjectVersion: c.ObjectVersion, ExpiresAt: session.EvidenceExpiresAt, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		affected, err := q.CompleteDocumentUploadSession(txCtx, capturesql.CompleteDocumentUploadSessionParams{VersionID: dbID(c.VersionID), CompletedAt: dbTime(c.OccurredAt), ID: session.ID})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrRequirementEvidenceConflict
		}
		affected, err = q.ActivateTenderDocument(txCtx, capturesql.ActivateTenderDocumentParams{UpdatedAt: dbTime(c.OccurredAt), ID: session.DocumentID})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrRequirementEvidenceConflict
		}
		if err = q.InsertDocumentJob(txCtx, capturesql.InsertDocumentJobParams{ID: dbID(c.ScanJobID), Kind: "MALWARE_SCAN", DocumentVersionID: dbID(c.VersionID), State: "QUEUED", ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertEvidenceUploadAudit(txCtx, capturesql.InsertEvidenceUploadAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "RequirementEvidenceUploadCompleted", TargetType: "document_version", TargetID: dbID(c.VersionID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), UploadID: dbID(c.UploadID)}); err != nil {
			return err
		}
		if err = q.InsertEvidenceVersionQuarantinedOutbox(txCtx, capturesql.InsertEvidenceVersionQuarantinedOutboxParams{ID: dbID(c.OutboxID), DocumentID: session.DocumentID, VersionID: dbID(c.VersionID), ScanJobID: dbID(c.ScanJobID), BidID: dbID(c.BidID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.VersionID, 2, c.OccurredAt); err != nil {
			return err
		}
		result = application.EvidenceUploadResult{UploadID: c.UploadID, DocumentID: c.DocumentID, DocumentVersionID: c.VersionID, ScanJobID: c.ScanJobID, ScanState: "QUARANTINED", EvidenceExpiresAt: c.EvidenceExpiresAt}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) SatisfyRequirement(ctx context.Context, a application.WorkflowAuthority, c application.SatisfyRequirementCommit) (result application.SatisfyRequirementResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, workspaceVersion, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "SATISFY_REQUIREMENT_WITH_EVIDENCE", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			evidence, err := q.GetRequirementEvidence(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			requirement, err := q.GetBidRequirement(txCtx, dbID(c.RequirementID))
			if err != nil {
				return err
			}
			result = application.SatisfyRequirementResult{Evidence: mapRequirementEvidence(evidence.ID, evidence.RequirementID, evidence.BidID, evidence.DocumentVersionID, evidence.EvidenceLocation, evidence.Justification, evidence.AddedBy, evidence.AddedAt, evidence.ExpiresAt), Requirement: mapRequirement(requirement.ID, requirement.BidID, requirement.SourceVersionID, requirement.SourceLocation, requirement.Text, requirement.Mandatory, requirement.Category, requirement.Status, requirement.OwnerID, requirement.DueAt, requirement.Version, requirement.CreatedAt, requirement.UpdatedAt), WorkspaceVersion: workspaceVersion, Replay: true}
			return nil
		}
		locked, err := q.LockRequirementForEvidenceSatisfaction(txCtx, capturesql.LockRequirementForEvidenceSatisfactionParams{DocumentVersionID: dbID(c.DocumentVersionID), RequirementID: dbID(c.RequirementID), BidID: dbID(c.BidID), ExpectedRequirementVersion: c.ExpectedRequirementVersion, ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrRequirementEvidenceConflict
		}
		if err != nil {
			return err
		}
		if locked.ExpiresAt.Valid && !locked.ExpiresAt.Time.After(c.OccurredAt) {
			return application.ErrEvidenceExpired
		}
		if err = q.InsertRequirementEvidence(txCtx, capturesql.InsertRequirementEvidenceParams{ID: dbID(c.EvidenceID), RequirementID: locked.ID, BidID: locked.BidID, DocumentVersionID: locked.DocumentVersionID, EvidenceLocation: c.EvidenceLocation, Justification: c.Justification, ActorID: dbID(a.ActorID), AddedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		affected, err := q.SatisfyRequirement(txCtx, capturesql.SatisfyRequirementParams{UpdatedAt: dbTime(c.OccurredAt), ID: locked.ID, ExpectedVersion: c.ExpectedRequirementVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrRequirementEvidenceConflict
		}
		newWorkspaceVersion := c.ExpectedWorkspaceVersion + 1
		affected, err = q.AdvanceWorkspaceForRequirementChange(txCtx, capturesql.AdvanceWorkspaceForRequirementChangeParams{UpdatedAt: dbTime(c.OccurredAt), ID: locked.BidID, ExpectedVersion: c.ExpectedWorkspaceVersion})
		if err != nil {
			return err
		}
		if affected != 1 {
			return application.ErrRequirementEvidenceConflict
		}
		newRequirementVersion := c.ExpectedRequirementVersion + 1
		if err = q.InsertRequirementSatisfiedAudit(txCtx, capturesql.InsertRequirementSatisfiedAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), RequirementID: locked.ID, OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: locked.BidID, EvidenceID: dbID(c.EvidenceID), DocumentVersionID: locked.DocumentVersionID, RequirementVersion: newRequirementVersion, WorkspaceVersion: newWorkspaceVersion}); err != nil {
			return err
		}
		if err = q.InsertRequirementSatisfiedOutbox(txCtx, capturesql.InsertRequirementSatisfiedOutboxParams{ID: dbID(c.OutboxID), BidID: locked.BidID, WorkspaceVersion: newWorkspaceVersion, RequirementID: locked.ID, EvidenceID: dbID(c.EvidenceID), DocumentVersionID: locked.DocumentVersionID, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.EvidenceID, newWorkspaceVersion, c.OccurredAt); err != nil {
			return err
		}
		evidence, err := q.GetRequirementEvidence(txCtx, dbID(c.EvidenceID))
		if err != nil {
			return err
		}
		requirement, err := q.GetBidRequirement(txCtx, locked.ID)
		if err != nil {
			return err
		}
		result = application.SatisfyRequirementResult{Evidence: mapRequirementEvidence(evidence.ID, evidence.RequirementID, evidence.BidID, evidence.DocumentVersionID, evidence.EvidenceLocation, evidence.Justification, evidence.AddedBy, evidence.AddedAt, evidence.ExpiresAt), Requirement: mapRequirement(requirement.ID, requirement.BidID, requirement.SourceVersionID, requirement.SourceLocation, requirement.Text, requirement.Mandatory, requirement.Category, requirement.Status, requirement.OwnerID, requirement.DueAt, requirement.Version, requirement.CreatedAt, requirement.UpdatedAt), WorkspaceVersion: newWorkspaceVersion}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListRequirementEvidence(ctx context.Context, a application.WorkflowAuthority, bidID, requirementID application.ID) (result []application.RequirementEvidence, err error) {
	if err = s.validRead(ctx, a, 100); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := capturesql.New(db).ListRequirementEvidence(txCtx, capturesql.ListRequirementEvidenceParams{RequirementID: dbID(requirementID), BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		result = make([]application.RequirementEvidence, len(rows))
		for i, row := range rows {
			result[i] = mapRequirementEvidence(row.ID, row.RequirementID, row.BidID, row.DocumentVersionID, row.EvidenceLocation, row.Justification, row.AddedBy, row.AddedAt, row.ExpiresAt)
		}
		return nil
	})
	return result, err
}
func mapEvidenceSession(row capturesql.GetEvidenceUploadSessionRow) application.EvidenceUploadSession {
	var sum [32]byte
	copy(sum[:], row.ExpectedSha256)
	return application.EvidenceUploadSession{ID: appID(row.ID), DocumentID: appID(row.DocumentID), BidID: appID(row.BidID), CompletedVersionID: appID(row.CompletedVersionID), ObjectKey: row.ObjectKey, MediaType: row.ExpectedMediaType, SizeBytes: row.ExpectedSizeBytes, SHA256: sum, State: row.State, ExpiresAt: row.ExpiresAt.Time.UTC(), EvidenceExpiresAt: timePointer(row.EvidenceExpiresAt)}
}
func dbNullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return dbTime(value.UTC())
}
func mapRequirementEvidence(id, requirementID, bidID, versionID pgtype.UUID, location, justification string, addedBy pgtype.UUID, addedAt, expiresAt pgtype.Timestamptz) application.RequirementEvidence {
	return application.RequirementEvidence{ID: appID(id), RequirementID: appID(requirementID), BidID: appID(bidID), DocumentVersionID: appID(versionID), EvidenceLocation: location, Justification: justification, AddedBy: appID(addedBy), AddedAt: addedAt.Time.UTC(), ExpiresAt: timePointer(expiresAt)}
}
