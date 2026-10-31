package persistence

import (
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *WorkflowStore) FindProposalPackageReplay(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, keyHash, requestHash [32]byte) (result application.ProposalPackageResult, found bool, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, false, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, err := q.GetProposalPackageIdempotency(txCtx, capturesql.GetProposalPackageIdempotencyParams{ActorID: dbID(a.ActorID), KeyHash: keyHash[:]})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		found = true
		if !bytes.Equal(row.RequestHash, requestHash[:]) {
			return application.ErrProposalPackageConflict
		}
		if row.State != "COMPLETED" || row.ResponseStatus == nil || *row.ResponseStatus != 201 || !row.ResponseRef.Valid {
			return errors.New("proposal_package_idempotency_pending")
		}
		manifest, err := q.GetProposalPackageManifest(txCtx, capturesql.GetProposalPackageManifestParams{ID: row.ResponseRef, BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		mapped, err := mapProposalPackage(manifest)
		if err != nil {
			return err
		}
		result = application.ProposalPackageResult{Manifest: mapped, Replay: true}
		return nil
	})
	return result, found, err
}
func (s *WorkflowStore) CommitProposalPackage(ctx context.Context, a application.WorkflowAuthority, c application.ProposalPackageCommit) (result application.ProposalPackageResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "PREPARE_PROPOSAL_PACKAGE", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetProposalPackageManifest(txCtx, capturesql.GetProposalPackageManifestParams{ID: dbID(ref), BidID: dbID(c.BidID)})
			if err != nil {
				return err
			}
			manifest, err := mapProposalPackage(row)
			if err != nil {
				return err
			}
			result = application.ProposalPackageResult{Manifest: manifest, Replay: true}
			return nil
		}
		locked, err := q.LockProposalForPackagePreparation(txCtx, capturesql.LockProposalForPackagePreparationParams{BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrProposalPackageConflict
		} else if err != nil {
			return err
		}
		if appID(locked.ProposalID) != c.ProposalID || locked.ProposalVersion != c.ExpectedProposalVersion || locked.WorkspaceVersion != c.ExpectedWorkspaceVersion || locked.Revision != c.ProposalRevision {
			return application.ErrProposalPackageConflict
		}
		if !locked.LetterheadVersionID.Valid || appID(locked.LetterheadVersionID) != c.LetterheadVersionID || locked.LetterheadVersionNumber == nil || *locked.LetterheadVersionNumber != c.LetterheadVersionNumber || locked.LetterheadTemplateKey == nil || *locked.LetterheadTemplateKey != c.LetterheadTemplateKey || len(locked.LetterheadContentHash) != 32 || !bytes.Equal(locked.LetterheadContentHash, c.LetterheadContentHash[:]) {
			return application.ErrProposalPackageConflict
		}
		blockers, err := json.Marshal(c.ReadinessBlockers)
		if err != nil {
			return err
		}
		if err = q.InsertProposalPackageManifest(txCtx, capturesql.InsertProposalPackageManifestParams{ID: dbID(c.ManifestID), BidID: dbID(c.BidID), ProposalID: dbID(c.ProposalID), ProposalVersionID: dbID(c.ProposalVersionID), ProposalRevision: c.ProposalRevision, PdfSha256: c.PDFSHA256[:], PdfSize: c.PDFSize, DocxSha256: c.DOCXSHA256[:], DocxSize: c.DOCXSize, ReadinessObservedAt: dbTime(c.ReadinessObservedAt), ReadinessReady: c.ReadinessReady, ReadinessBlockers: blockers, ManifestSha256: c.ManifestSHA256[:], ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt), LetterheadVersionID: dbID(c.LetterheadVersionID), LetterheadVersionNumber: &c.LetterheadVersionNumber, LetterheadTemplateKey: &c.LetterheadTemplateKey, LetterheadContentHash: c.LetterheadContentHash[:]}); err != nil {
			return err
		}
		if err = q.InsertProposalPackageAudit(txCtx, capturesql.InsertProposalPackageAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), ManifestID: dbID(c.ManifestID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), LetterheadVersionID: dbID(c.LetterheadVersionID), LetterheadVersionNumber: c.LetterheadVersionNumber, LetterheadTemplateKey: c.LetterheadTemplateKey}); err != nil {
			return err
		}
		if err = q.InsertProposalPackageOutbox(txCtx, capturesql.InsertProposalPackageOutboxParams{ID: dbID(c.OutboxID), ManifestID: dbID(c.ManifestID), BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), LetterheadVersionID: dbID(c.LetterheadVersionID), LetterheadVersionNumber: c.LetterheadVersionNumber, LetterheadTemplateKey: c.LetterheadTemplateKey, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.ManifestID, 2, c.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetProposalPackageManifest(txCtx, capturesql.GetProposalPackageManifestParams{ID: dbID(c.ManifestID), BidID: dbID(c.BidID)})
		if err != nil {
			return err
		}
		manifest, err := mapProposalPackage(row)
		if err != nil {
			return err
		}
		result = application.ProposalPackageResult{Manifest: manifest}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetProposalPackageManifest(ctx context.Context, a application.WorkflowAuthority, bidID, manifestID application.ID) (result application.ProposalPackageManifest, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetProposalPackageManifest(txCtx, capturesql.GetProposalPackageManifestParams{ID: dbID(manifestID), BidID: dbID(bidID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrProposalPackageNotFound
		} else if err != nil {
			return err
		}
		result, err = mapProposalPackage(row)
		return err
	})
	return result, err
}
func (s *WorkflowStore) ListProposalPackageManifests(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, cursor application.ReadCursor, limit int32) (result []application.ProposalPackageManifest, err error) {
	if err = s.validRead(ctx, a, limit); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if _, err := q.ValidateProposalPackageBidRead(txCtx, dbID(bidID)); errors.Is(err, pgx.ErrNoRows) {
			return application.ErrProposalPackageNotFound
		} else if err != nil {
			return err
		}
		cursorAt, cursorID := readCursor(cursor)
		rows, err := q.ListProposalPackageManifests(txCtx, capturesql.ListProposalPackageManifestsParams{BidID: dbID(bidID), CursorAt: cursorAt, CursorID: cursorID, PageLimit: limit})
		if err != nil {
			return err
		}
		result = make([]application.ProposalPackageManifest, len(rows))
		for i, row := range rows {
			result[i], err = mapProposalPackage(row)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}
func mapProposalPackage(row capturesql.BidProposalPackageManifest) (application.ProposalPackageManifest, error) {
	var blockers []string
	if err := json.Unmarshal(row.ReadinessBlockers, &blockers); err != nil {
		return application.ProposalPackageManifest{}, err
	}
	var pdf, docx, manifest [32]byte
	if len(row.PdfSha256) != 32 || len(row.DocxSha256) != 32 || len(row.ManifestSha256) != 32 {
		return application.ProposalPackageManifest{}, errors.New("proposal_package_hash_invalid")
	}
	copy(pdf[:], row.PdfSha256)
	copy(docx[:], row.DocxSha256)
	copy(manifest[:], row.ManifestSha256)
	var letterheadID *application.ID
	var letterheadHash *[32]byte
	if row.LetterheadVersionID.Valid && row.LetterheadVersionNumber != nil && *row.LetterheadVersionNumber > 0 && row.LetterheadTemplateKey != nil && *row.LetterheadTemplateKey == "STANDARD_PROPOSAL" && len(row.LetterheadContentHash) == 32 {
		id := appID(row.LetterheadVersionID)
		hash := [32]byte{}
		copy(hash[:], row.LetterheadContentHash)
		letterheadID = &id
		letterheadHash = &hash
	} else if row.LetterheadVersionID.Valid || row.LetterheadVersionNumber != nil || row.LetterheadTemplateKey != nil || len(row.LetterheadContentHash) != 0 {
		return application.ProposalPackageManifest{}, errors.New("proposal_package_letterhead_binding_invalid")
	}
	return application.ProposalPackageManifest{ID: appID(row.ID), BidID: appID(row.BidID), ProposalID: appID(row.ProposalID), ProposalVersionID: appID(row.ProposalVersionID), CreatedBy: appID(row.CreatedBy), LetterheadVersionID: letterheadID, LetterheadVersionNumber: row.LetterheadVersionNumber, LetterheadTemplateKey: row.LetterheadTemplateKey, LetterheadContentHash: letterheadHash, ProposalRevision: row.ProposalRevision, Status: row.PackageStatus, PDFRenderProfile: row.PdfRenderProfile, DOCXRenderProfile: row.DocxRenderProfile, PDFSHA256: pdf, DOCXSHA256: docx, ManifestSHA256: manifest, PDFSize: row.PdfSize, DOCXSize: row.DocxSize, ReadinessObservedAt: row.ReadinessObservedAt.Time.UTC(), ReadinessReady: row.ReadinessReady, ReadinessBlockers: blockers, ManifestVersion: row.ManifestVersion, CreatedAt: row.CreatedAt.Time.UTC()}, nil
}
