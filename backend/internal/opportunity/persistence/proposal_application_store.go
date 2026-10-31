package persistence

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
)

func (s *WorkflowStore) ApplyProposalSuggestion(ctx context.Context, a application.WorkflowAuthority, c application.ApplyProposalSuggestionCommit) (result application.ProposalSaveResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, responseWorkspaceVersion, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "APPLY_AI_PROPOSAL_SUGGESTION", c.KeyHash, c.RequestHash, c.OccurredAt)
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
		locked, err := q.LockProposalSuggestionForApply(txCtx, capturesql.LockProposalSuggestionForApplyParams{SuggestionID: dbID(c.SuggestionID), ExpectedSuggestionVersion: c.ExpectedSuggestionVersion, BidID: dbID(c.BidID), ExpectedProposalVersion: c.ExpectedProposalVersion, ExpectedWorkspaceVersion: c.ExpectedWorkspaceVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrProposalConflict
		}
		if err != nil {
			return err
		}
		revision := locked.ProposalVersion + 1
		workspaceVersion := locked.WorkspaceVersion + 1
		responses, err := q.ListCurrentResponseVersionsForProposal(txCtx, locked.BidID)
		if err != nil {
			return err
		}
		if len(responses) == 0 || len(responses) > 1000 {
			return application.ErrProposalConflict
		}
		if err = q.InsertAppliedProposalVersion(txCtx, capturesql.InsertAppliedProposalVersionParams{ID: dbID(c.ProposalVersionID), ProposalID: locked.ProposalID, Revision: revision, WorkspaceVersion: workspaceVersion, OrganizationLegalName: locked.OrganizationLegalName, Title: locked.Title, Recipient: locked.Recipient, LetterheadLines: locked.LetterheadLines, Introduction: c.Introduction, Closing: c.Closing, ChangeNote: c.ReviewNote, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt), SuggestionID: locked.SuggestionID}); err != nil {
			return err
		}
		for i, row := range responses {
			if err = q.InsertProposalResponseVersion(txCtx, capturesql.InsertProposalResponseVersionParams{ProposalVersionID: dbID(c.ProposalVersionID), RequirementID: row.RequirementID, ResponseVersionID: row.ResponseVersionID, SourceDocumentVersionID: row.SourceDocumentVersionID, RequirementText: row.RequirementText, Category: row.Category, Mandatory: row.Mandatory, Ordinal: int32(i + 1)}); err != nil {
				return err
			}
		}
		affected, err := q.AdvanceProposal(txCtx, capturesql.AdvanceProposalParams{RevisionID: dbID(c.ProposalVersionID), NewVersion: revision, ActorID: dbID(a.ActorID), UpdatedAt: dbTime(c.OccurredAt), ID: locked.ProposalID, ExpectedVersion: c.ExpectedProposalVersion})
		if err != nil || affected != 1 {
			return application.ErrProposalConflict
		}
		affected, err = q.AdvanceWorkspaceForProposal(txCtx, capturesql.AdvanceWorkspaceForProposalParams{UpdatedAt: dbTime(c.OccurredAt), ID: locked.BidID, ExpectedVersion: c.ExpectedWorkspaceVersion})
		if err != nil || affected != 1 {
			return application.ErrProposalConflict
		}
		reviewNote := c.ReviewNote
		affected, err = q.ApplyProposalDraftSuggestion(txCtx, capturesql.ApplyProposalDraftSuggestionParams{ActorID: dbID(a.ActorID), ReviewedAt: dbTime(c.OccurredAt), ProposalVersionID: dbID(c.ProposalVersionID), ReviewNote: &reviewNote, ID: locked.SuggestionID, ExpectedVersion: c.ExpectedSuggestionVersion})
		if err != nil || affected != 1 {
			return application.ErrProposalConflict
		}
		if err = q.InsertProposalAudit(txCtx, capturesql.InsertProposalAuditParams{ID: dbID(c.ProposalAuditID), ActorID: dbID(a.ActorID), ProposalID: locked.ProposalID, OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: locked.BidID, ProposalVersionID: dbID(c.ProposalVersionID), Revision: revision, ResponseCount: int32(len(responses))}); err != nil {
			return err
		}
		if err = q.InsertProposalOutbox(txCtx, capturesql.InsertProposalOutboxParams{ID: dbID(c.ProposalOutboxID), ProposalID: locked.ProposalID, Revision: revision, BidID: locked.BidID, ProposalVersionID: dbID(c.ProposalVersionID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = q.InsertProposalSuggestionAppliedAudit(txCtx, capturesql.InsertProposalSuggestionAppliedAuditParams{ID: dbID(c.ReviewAuditID), ActorID: dbID(a.ActorID), SuggestionID: locked.SuggestionID, OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: locked.BidID, ProposalVersionID: dbID(c.ProposalVersionID)}); err != nil {
			return err
		}
		if err = q.InsertProposalSuggestionAppliedOutbox(txCtx, capturesql.InsertProposalSuggestionAppliedOutboxParams{ID: dbID(c.ReviewOutboxID), SuggestionID: locked.SuggestionID, SuggestionVersion: c.ExpectedSuggestionVersion + 1, BidID: locked.BidID, ProposalVersionID: dbID(c.ProposalVersionID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.ProposalVersionID, workspaceVersion, c.OccurredAt); err != nil {
			return err
		}
		doc, err := loadProposalVersion(txCtx, q, c.BidID, c.ProposalVersionID)
		if err != nil {
			return err
		}
		doc.WorkspaceVersion = workspaceVersion
		result = application.ProposalSaveResult{Proposal: doc}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetProposalApprovalReadiness(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, at time.Time) (result application.ProposalApprovalReadiness, err error) {
	if err = s.validRead(ctx, a, 1); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetProposalApprovalReadiness(txCtx, capturesql.GetProposalApprovalReadinessParams{ObservedAt: dbTime(at), BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		result = application.ProposalApprovalReadiness{BidID: appID(row.BidID), ProposalID: appID(row.ProposalID), ProposalVersionID: appID(row.CurrentRevisionID), WorkspaceState: row.WorkspaceState, WorkspaceVersion: row.WorkspaceVersion, RequirementCount: row.RequirementCount, OpenReviewThreadCount: row.OpenReviewThreadCount, UnassignedReviewThreadCount: row.UnassignedReviewThreadCount, OverdueReviewThreadCount: row.OverdueReviewThreadCount, MandatoryUnsatisfied: row.MandatoryUnsatisfied, OpenUnassigned: row.OpenUnassigned, OpenOverdue: row.OpenOverdue, MandatoryResponseMissing: row.MandatoryResponseMissing, ExpiredEvidenceCount: row.ExpiredEvidenceCount, ObservedAt: at.UTC(), Blockers: []string{}}
		if row.ProposalVersion != nil {
			result.ProposalVersion = *row.ProposalVersion
		}
		if row.WorkspaceState != "BID" {
			result.Blockers = append(result.Blockers, "BID_DECISION_REQUIRED")
		}
		if !row.ProposalID.Valid {
			result.Blockers = append(result.Blockers, "MISSING_PROPOSAL")
		} else if row.WorkspaceVersionAtCreation == nil || *row.WorkspaceVersionAtCreation != row.WorkspaceVersion {
			result.Blockers = append(result.Blockers, "PROPOSAL_OUTDATED")
		}
		if row.OpenReviewThreadCount > 0 {
			result.Blockers = append(result.Blockers, "OPEN_REVIEW_THREADS")
		}
		if row.UnassignedReviewThreadCount > 0 {
			result.Blockers = append(result.Blockers, "OPEN_REVIEWS_UNASSIGNED")
		}
		if row.OverdueReviewThreadCount > 0 {
			result.Blockers = append(result.Blockers, "REVIEWS_OVERDUE")
		}
		if row.MandatoryUnsatisfied > 0 {
			result.Blockers = append(result.Blockers, "MANDATORY_REQUIREMENTS_OPEN")
		}
		if row.OpenUnassigned > 0 {
			result.Blockers = append(result.Blockers, "OPEN_REQUIREMENTS_UNASSIGNED")
		}
		if row.OpenOverdue > 0 {
			result.Blockers = append(result.Blockers, "REQUIREMENTS_OVERDUE")
		}
		if row.MandatoryResponseMissing > 0 {
			result.Blockers = append(result.Blockers, "MANDATORY_RESPONSES_MISSING")
		}
		if row.ExpiredEvidenceCount > 0 {
			result.Blockers = append(result.Blockers, "REQUIREMENT_EVIDENCE_EXPIRED")
		}
		result.Ready = len(result.Blockers) == 0
		return nil
	})
	return result, err
}
