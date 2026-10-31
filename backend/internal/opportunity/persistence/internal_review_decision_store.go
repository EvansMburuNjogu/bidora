package persistence

import (
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *WorkflowStore) CommitInternalReviewDecision(ctx context.Context, a application.WorkflowAuthority, c application.InternalReviewDecisionCommit) (result application.InternalReviewDecisionResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "RECORD_INTERNAL_REVIEW_DECISION", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			row, err := q.GetInternalReviewDecision(txCtx, dbID(ref))
			if err != nil {
				return err
			}
			result = application.InternalReviewDecisionResult{Decision: mapInternalReviewDecision(row), Replay: true}
			return nil
		}
		locked, err := q.LockCurrentProposalVersionForInternalReview(txCtx, capturesql.LockCurrentProposalVersionForInternalReviewParams{BidID: dbID(c.BidID), ProposalVersionID: dbID(c.ProposalVersionID)})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrInternalReviewConflict
		} else if err != nil {
			return err
		}
		sequence, err := q.GetReviewerDecisionSequence(txCtx, capturesql.GetReviewerDecisionSequenceParams{ProposalVersionID: dbID(c.ProposalVersionID), ReviewerID: dbID(a.ActorID)})
		if err != nil {
			return err
		}
		if sequence != c.ExpectedPreviousSequence || sequence >= 100 {
			return application.ErrInternalReviewConflict
		}
		next := sequence + 1
		if err = q.InsertInternalReviewDecision(txCtx, capturesql.InsertInternalReviewDecisionParams{ID: dbID(c.DecisionID), BidID: dbID(c.BidID), ProposalID: locked.ProposalID, ProposalVersionID: dbID(c.ProposalVersionID), ReviewerID: dbID(a.ActorID), ReviewerSequence: next, Decision: c.Decision, Rationale: c.Rationale, DecidedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertInternalReviewDecisionAudit(txCtx, capturesql.InsertInternalReviewDecisionAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), ProposalVersionID: dbID(c.ProposalVersionID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), ReviewerSequence: next, Decision: c.Decision}); err != nil {
			return err
		}
		if err = q.InsertInternalReviewDecisionOutbox(txCtx, capturesql.InsertInternalReviewDecisionOutboxParams{ID: dbID(c.OutboxID), DecisionID: dbID(c.DecisionID), ProposalVersionID: dbID(c.ProposalVersionID), ReviewerSequence: next, BidID: dbID(c.BidID), ReviewerID: dbID(a.ActorID), Decision: c.Decision, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.DecisionID, int64(next), c.OccurredAt); err != nil {
			return err
		}
		row, err := q.GetInternalReviewDecision(txCtx, dbID(c.DecisionID))
		if err != nil {
			return err
		}
		result = application.InternalReviewDecisionResult{Decision: mapInternalReviewDecision(row)}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListInternalReviewDecisions(ctx context.Context, a application.WorkflowAuthority, bidID, proposalVersionID application.ID) (result []application.InternalReviewDecision, err error) {
	if err = s.valid(ctx, a); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if _, err := q.ValidateProposalVersionForInternalReviewRead(txCtx, capturesql.ValidateProposalVersionForInternalReviewReadParams{BidID: dbID(bidID), ProposalVersionID: dbID(proposalVersionID)}); errors.Is(err, pgx.ErrNoRows) {
			return application.ErrInternalReviewNotFound
		} else if err != nil {
			return err
		}
		rows, err := q.ListInternalReviewDecisionHistory(txCtx, capturesql.ListInternalReviewDecisionHistoryParams{BidID: dbID(bidID), ProposalVersionID: dbID(proposalVersionID)})
		if err != nil {
			return err
		}
		if len(rows) > 200 {
			return application.ErrInternalReviewReadLimit
		}
		result = make([]application.InternalReviewDecision, len(rows))
		for i, row := range rows {
			result[i] = mapInternalReviewDecision(row)
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetApprovalPolicyPreparation(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, observedAt time.Time) (result application.ApprovalPolicyPreparation, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		row, err := capturesql.New(db).GetApprovalPolicyPreparation(txCtx, dbID(bidID))
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrInternalReviewNotFound
		} else if err != nil {
			return err
		}
		blockers := []string{}
		if row.ReviewerCount == 0 {
			blockers = append(blockers, "NO_INTERNAL_REVIEW_DECISIONS")
		}
		if row.ReviewCompleteCount == 0 {
			blockers = append(blockers, "NO_REVIEW_COMPLETE")
		}
		if row.ChangesRequestedCount > 0 {
			blockers = append(blockers, "INTERNAL_CHANGES_REQUESTED")
		}
		if row.OpenReviewThreadCount > 0 {
			blockers = append(blockers, "OPEN_REVIEW_THREADS")
		}
		blockers = append(blockers, "APPROVAL_COMMAND_RECHECK_REQUIRED")
		result = application.ApprovalPolicyPreparation{BidID: bidID, ProposalVersionID: appID(row.ProposalVersionID), ReviewerCount: row.ReviewerCount, ReviewCompleteCount: row.ReviewCompleteCount, ChangesRequestedCount: row.ChangesRequestedCount, OpenReviewThreadCount: row.OpenReviewThreadCount, ApprovalPolicyConfigured: true, EligibleForApproval: false, Blockers: blockers, ObservedAt: observedAt}
		return nil
	})
	return result, err
}
func mapInternalReviewDecision(row capturesql.BidInternalReviewDecision) application.InternalReviewDecision {
	return application.InternalReviewDecision{ID: appID(row.ID), BidID: appID(row.BidID), ProposalVersionID: appID(row.ProposalVersionID), ReviewerID: appID(row.ReviewerID), ReviewerSequence: row.ReviewerSequence, Decision: row.Decision, Rationale: row.Rationale, DecidedAt: row.DecidedAt.Time.UTC()}
}
