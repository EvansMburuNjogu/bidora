package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *WorkflowStore) SaveProposal(ctx context.Context, a application.WorkflowAuthority, c application.ProposalCommit) (result application.ProposalSaveResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, responseWorkspaceVersion, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "SAVE_PROPOSAL", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			doc, err := loadProposalVersion(txCtx, q, c.BidID, ref)
			if err != nil {
				return err
			}
			doc.WorkspaceVersion = responseWorkspaceVersion
			result = application.ProposalSaveResult{Proposal: doc, Replay: true}
			return nil
		}
		if _, err = q.LockBidForProposal(txCtx, capturesql.LockBidForProposalParams{BidID: dbID(c.BidID), ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion}); errors.Is(err, pgx.ErrNoRows) {
			return application.ErrProposalConflict
		} else if err != nil {
			return err
		}
		proposalID := c.ProposalID
		revision := int64(1)
		expectedStoredVersion := int64(1)
		existing, lookupErr := q.GetProposalForUpdate(txCtx, dbID(c.BidID))
		if lookupErr == nil {
			if c.ExpectedProposalVersion < 1 || existing.Version != c.ExpectedProposalVersion {
				return application.ErrProposalConflict
			}
			proposalID = appID(existing.ID)
			revision = existing.Version + 1
			expectedStoredVersion = existing.Version
		} else if errors.Is(lookupErr, pgx.ErrNoRows) {
			if c.ExpectedProposalVersion != 0 {
				return application.ErrProposalConflict
			}
			if err = q.InsertProposal(txCtx, capturesql.InsertProposalParams{ID: dbID(proposalID), BidID: dbID(c.BidID), ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
				return err
			}
		} else {
			return lookupErr
		}
		responses, err := q.ListCurrentResponseVersionsForProposal(txCtx, dbID(c.BidID))
		if err != nil {
			return err
		}
		if len(responses) == 0 || len(responses) > 1000 {
			return application.ErrProposalConflict
		}
		if err = q.InsertProposalVersion(txCtx, capturesql.InsertProposalVersionParams{ID: dbID(c.ProposalVersionID), ProposalID: dbID(proposalID), Revision: revision, WorkspaceVersion: c.ExpectedWorkspaceVersion + 1, Title: c.Title, Recipient: c.Recipient, LetterheadLines: c.LetterheadJSON, Introduction: c.Introduction, Closing: c.Closing, ChangeNote: c.ChangeNote, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		for i, row := range responses {
			if err = q.InsertProposalResponseVersion(txCtx, capturesql.InsertProposalResponseVersionParams{ProposalVersionID: dbID(c.ProposalVersionID), RequirementID: row.RequirementID, ResponseVersionID: row.ResponseVersionID, SourceDocumentVersionID: row.SourceDocumentVersionID, RequirementText: row.RequirementText, Category: row.Category, Mandatory: row.Mandatory, Ordinal: int32(i + 1)}); err != nil {
				return err
			}
		}
		affected, err := q.AdvanceProposal(txCtx, capturesql.AdvanceProposalParams{RevisionID: dbID(c.ProposalVersionID), NewVersion: revision, ActorID: dbID(a.ActorID), UpdatedAt: dbTime(c.OccurredAt), ID: dbID(proposalID), ExpectedVersion: expectedStoredVersion})
		if err != nil || affected != 1 {
			return affectedErr(err)
		}
		affected, err = q.AdvanceWorkspaceForProposal(txCtx, capturesql.AdvanceWorkspaceForProposalParams{UpdatedAt: dbTime(c.OccurredAt), ID: dbID(c.BidID), ExpectedVersion: c.ExpectedWorkspaceVersion})
		if err != nil || affected != 1 {
			return application.ErrProposalConflict
		}
		if err = q.InsertProposalAudit(txCtx, capturesql.InsertProposalAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), ProposalID: dbID(proposalID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), Revision: revision, ResponseCount: int32(len(responses))}); err != nil {
			return err
		}
		if err = q.InsertProposalOutbox(txCtx, capturesql.InsertProposalOutboxParams{ID: dbID(c.OutboxID), ProposalID: dbID(proposalID), Revision: revision, BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.ProposalVersionID, c.ExpectedWorkspaceVersion+1, c.OccurredAt); err != nil {
			return err
		}
		doc, err := loadProposalVersion(txCtx, q, c.BidID, c.ProposalVersionID)
		if err != nil {
			return err
		}
		doc.WorkspaceVersion = c.ExpectedWorkspaceVersion + 1
		result = application.ProposalSaveResult{Proposal: doc}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetProposal(ctx context.Context, a application.WorkflowAuthority, bidID application.ID) (result application.ProposalDocument, err error) {
	if err = s.validRead(ctx, a, 1000); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		row, err := q.GetCurrentProposal(txCtx, dbID(bidID))
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrProposalNotFound
		}
		if err != nil {
			return err
		}
		result, err = proposalDocument(txCtx, q, row.ID, row.BidID, row.CurrentRevisionID, row.Version, row.Revision, row.Title, row.Recipient, row.LetterheadLines, row.Introduction, row.Closing, row.ChangeNote, row.CreatedBy, row.CreatedAt.Time, row.LegalName, row.WorkspaceVersion, row.LetterheadVersionID, row.LetterheadVersionNumber, row.LetterheadTemplateKey, row.LetterheadContentHash)
		return err
	})
	return result, err
}
func (s *WorkflowStore) GetProposalVersion(ctx context.Context, a application.WorkflowAuthority, bidID, versionID application.ID) (result application.ProposalDocument, err error) {
	if err = s.validRead(ctx, a, 1000); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		var loadErr error
		result, loadErr = loadProposalVersion(txCtx, capturesql.New(db), bidID, versionID)
		if errors.Is(loadErr, pgx.ErrNoRows) {
			return application.ErrProposalNotFound
		}
		return loadErr
	})
	return result, err
}
func (s *WorkflowStore) ListProposalHistory(ctx context.Context, a application.WorkflowAuthority, bidID application.ID) (result []application.ProposalHistoryItem, err error) {
	if err = s.validRead(ctx, a, 100); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := capturesql.New(db).ListProposalHistory(txCtx, dbID(bidID))
		if err != nil {
			return err
		}
		result = make([]application.ProposalHistoryItem, len(rows))
		for i, row := range rows {
			result[i] = application.ProposalHistoryItem{ProposalVersionID: appID(row.ID), Revision: row.Revision, Title: row.Title, ChangeNote: row.ChangeNote, CreatedBy: appID(row.CreatedBy), CreatedAt: row.CreatedAt.Time.UTC()}
		}
		return nil
	})
	return result, err
}
func loadProposalVersion(ctx context.Context, q *capturesql.Queries, bidID, versionID application.ID) (application.ProposalDocument, error) {
	row, err := q.GetProposalVersion(ctx, capturesql.GetProposalVersionParams{BidID: dbID(bidID), ProposalVersionID: dbID(versionID)})
	if err != nil {
		return application.ProposalDocument{}, err
	}
	return proposalDocument(ctx, q, row.ID, row.BidID, row.ProposalVersionID, row.Revision, row.Revision, row.Title, row.Recipient, row.LetterheadLines, row.Introduction, row.Closing, row.ChangeNote, row.CreatedBy, row.CreatedAt.Time, row.LegalName, row.WorkspaceVersion, row.LetterheadVersionID, row.LetterheadVersionNumber, row.LetterheadTemplateKey, row.LetterheadContentHash)
}
func proposalDocument(ctx context.Context, q *capturesql.Queries, proposalID, bidID, versionID pgtype.UUID, proposalVersion, revision int64, title, recipient string, letterhead []byte, introduction, closing, changeNote string, createdBy pgtype.UUID, createdAt time.Time, legalName string, workspaceVersion int64, letterheadVersionID pgtype.UUID, letterheadVersionNumber *int64, letterheadTemplateKey *string, letterheadHash []byte) (application.ProposalDocument, error) {
	lines, err := decodeLetterhead(letterhead)
	if err != nil {
		return application.ProposalDocument{}, err
	}
	rows, err := q.ListProposalSnapshotResponses(ctx, versionID)
	if err != nil {
		return application.ProposalDocument{}, err
	}
	responses := make([]application.ProposalResponseSnapshot, len(rows))
	for i, row := range rows {
		responses[i] = application.ProposalResponseSnapshot{RequirementID: appID(row.RequirementID), ResponseVersionID: appID(row.ResponseVersionID), RequirementText: row.RequirementText, Category: row.Category, Mandatory: row.Mandatory, Content: row.Content, ResponseRevision: int64(row.Revision), Ordinal: row.Ordinal, CreatedAt: row.CreatedAt.Time.UTC()}
	}
	var boundID *application.ID
	var boundHash *[32]byte
	if letterheadVersionID.Valid && letterheadVersionNumber != nil && *letterheadVersionNumber > 0 && letterheadTemplateKey != nil && *letterheadTemplateKey == "STANDARD_PROPOSAL" && len(letterheadHash) == 32 {
		id := appID(letterheadVersionID)
		hash := [32]byte{}
		copy(hash[:], letterheadHash)
		boundID = &id
		boundHash = &hash
	} else if letterheadVersionID.Valid || letterheadVersionNumber != nil || letterheadTemplateKey != nil || len(letterheadHash) != 0 {
		return application.ProposalDocument{}, errors.New("proposal_letterhead_binding_invalid")
	}
	return application.ProposalDocument{ProposalID: appID(proposalID), ProposalVersionID: appID(versionID), BidID: appID(bidID), LetterheadVersionID: boundID, LetterheadVersionNumber: letterheadVersionNumber, LetterheadTemplateKey: letterheadTemplateKey, LetterheadContentHash: boundHash, OrganizationLegalName: legalName, Title: title, Recipient: recipient, LetterheadLines: lines, Introduction: introduction, Closing: closing, ChangeNote: changeNote, ProposalVersion: proposalVersion, Revision: revision, WorkspaceVersion: workspaceVersion, CreatedBy: appID(createdBy), CreatedAt: createdAt.UTC(), Responses: responses}, nil
}
func decodeLetterhead(raw []byte) ([]string, error) {
	var v []string
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}
