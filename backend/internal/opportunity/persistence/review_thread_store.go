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

func (s *WorkflowStore) CommitReviewThread(ctx context.Context, a application.WorkflowAuthority, c application.ReviewThreadCommit) (result application.ReviewThreadResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		commandType := "REVIEW_THREAD_" + c.Action
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, commandType, c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			thread, err := loadReviewThread(txCtx, q, c.BidID, ref)
			if err != nil {
				return err
			}
			result = application.ReviewThreadResult{Thread: thread, Replay: true}
			return nil
		}
		version := int64(1)
		action := "ReviewThreadCreated"
		eventType := "bid.review_thread.created.v1"
		kind := "COMMENT"
		if c.Action == "CREATE" {
			if _, err = q.LockReviewTarget(txCtx, capturesql.LockReviewTargetParams{BidID: dbID(c.BidID), TargetType: c.TargetType, TargetID: dbID(c.TargetID)}); errors.Is(err, pgx.ErrNoRows) {
				return application.ErrReviewThreadConflict
			} else if err != nil {
				return err
			}
			if err = q.InsertReviewThread(txCtx, capturesql.InsertReviewThreadParams{ID: dbID(c.ThreadID), BidID: dbID(c.BidID), TargetType: c.TargetType, TargetID: dbID(c.TargetID), Subject: c.ThreadSubject, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
				return err
			}
		} else {
			locked, err := q.LockReviewThread(txCtx, capturesql.LockReviewThreadParams{ID: dbID(c.ThreadID), BidID: dbID(c.BidID), ExpectedVersion: c.ExpectedThreadVersion})
			if errors.Is(err, pgx.ErrNoRows) {
				return application.ErrReviewThreadConflict
			}
			if err != nil {
				return err
			}
			if locked.Status != "OPEN" {
				return application.ErrReviewThreadConflict
			}
			version = locked.Version + 1
			if c.Action == "RESOLVE" {
				action = "ReviewThreadResolved"
				eventType = "bid.review_thread.resolved.v1"
				kind = "RESOLUTION_NOTE"
			} else if c.Action == "COMMENT" {
				action = "ReviewCommentAdded"
				eventType = "bid.review_comment.added.v1"
			} else {
				return application.ErrReviewThreadConflict
			}
		}
		if err = q.InsertReviewComment(txCtx, capturesql.InsertReviewCommentParams{ID: dbID(c.CommentID), ThreadID: dbID(c.ThreadID), ThreadVersion: version, CommentKind: kind, Body: c.Body, ActorID: dbID(a.ActorID), CreatedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if c.Action == "COMMENT" {
			affected, err := q.AdvanceReviewThread(txCtx, capturesql.AdvanceReviewThreadParams{ID: dbID(c.ThreadID), ExpectedVersion: c.ExpectedThreadVersion})
			if err != nil || affected != 1 {
				return application.ErrReviewThreadConflict
			}
		} else if c.Action == "RESOLVE" {
			affected, err := q.ResolveReviewThread(txCtx, capturesql.ResolveReviewThreadParams{ActorID: dbID(a.ActorID), ResolvedAt: dbTime(c.OccurredAt), ID: dbID(c.ThreadID), ExpectedVersion: c.ExpectedThreadVersion})
			if err != nil || affected != 1 {
				return application.ErrReviewThreadConflict
			}
		}
		if err = q.InsertReviewAudit(txCtx, capturesql.InsertReviewAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: action, ThreadID: dbID(c.ThreadID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: dbID(c.BidID), ThreadVersion: version}); err != nil {
			return err
		}
		if err = q.InsertReviewOutbox(txCtx, capturesql.InsertReviewOutboxParams{ID: dbID(c.OutboxID), ThreadID: dbID(c.ThreadID), ThreadVersion: version, EventType: eventType, BidID: dbID(c.BidID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.ThreadID, version, c.OccurredAt); err != nil {
			return err
		}
		thread, err := loadReviewThread(txCtx, q, c.BidID, c.ThreadID)
		if err != nil {
			return err
		}
		result = application.ReviewThreadResult{Thread: thread}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) AssignReviewThread(ctx context.Context, a application.WorkflowAuthority, c application.ReviewAssignmentCommit) (result application.ReviewThreadResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return result, err
	}
	err = s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		replay, ref, _, err := claimWorkflow(txCtx, q, c.CommandID, a.ActorID, "ASSIGN_REVIEW_THREAD", c.KeyHash, c.RequestHash, c.OccurredAt)
		if err != nil {
			return err
		}
		if replay {
			thread, err := loadReviewThread(txCtx, q, c.BidID, ref)
			if err != nil {
				return err
			}
			result = application.ReviewThreadResult{Thread: thread, Replay: true}
			return nil
		}
		locked, err := q.LockReviewThread(txCtx, capturesql.LockReviewThreadParams{ID: dbID(c.ThreadID), BidID: dbID(c.BidID), ExpectedVersion: c.ExpectedThreadVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrReviewThreadConflict
		}
		if err != nil {
			return err
		}
		if locked.Status != "OPEN" || locked.Version >= 200 {
			return application.ErrReviewThreadConflict
		}
		newVersion := locked.Version + 1
		affected, err := q.UpdateReviewAssignment(txCtx, capturesql.UpdateReviewAssignmentParams{AssignedTo: dbID(c.AssigneeID), DueAt: dbNullableTime(c.DueAt), ID: locked.ID, ExpectedVersion: c.ExpectedThreadVersion})
		if err != nil || affected != 1 {
			return application.ErrReviewThreadConflict
		}
		if err = q.InsertReviewAssignment(txCtx, capturesql.InsertReviewAssignmentParams{ID: dbID(c.AssignmentID), ThreadID: locked.ID, ThreadVersion: newVersion, AssignedTo: dbID(c.AssigneeID), DueAt: dbNullableTime(c.DueAt), Reason: c.Reason, ActorID: dbID(a.ActorID), AssignedAt: dbTime(c.OccurredAt)}); err != nil {
			return err
		}
		if err = q.InsertReviewAudit(txCtx, capturesql.InsertReviewAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), Action: "ReviewThreadAssigned", ThreadID: locked.ID, OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), BidID: locked.BidID, ThreadVersion: newVersion}); err != nil {
			return err
		}
		if err = q.InsertReviewOutbox(txCtx, capturesql.InsertReviewOutboxParams{ID: dbID(c.OutboxID), ThreadID: locked.ID, ThreadVersion: newVersion, EventType: "bid.review_thread.assigned.v1", BidID: locked.BidID, OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); err != nil {
			return err
		}
		if err = completeWorkflow(txCtx, q, c.CommandID, c.ThreadID, newVersion, c.OccurredAt); err != nil {
			return err
		}
		thread, err := loadReviewThread(txCtx, q, c.BidID, c.ThreadID)
		if err != nil {
			return err
		}
		result = application.ReviewThreadResult{Thread: thread}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListReviewAssignments(ctx context.Context, a application.WorkflowAuthority, bidID, threadID application.ID) (result []application.ReviewAssignment, err error) {
	if err = s.validRead(ctx, a, 100); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if _, err := q.GetReviewThread(txCtx, capturesql.GetReviewThreadParams{ID: dbID(threadID), BidID: dbID(bidID)}); err != nil {
			return err
		}
		rows, err := q.ListReviewAssignmentHistory(txCtx, dbID(threadID))
		if err != nil {
			return err
		}
		result = make([]application.ReviewAssignment, len(rows))
		for i, row := range rows {
			result[i] = application.ReviewAssignment{ID: appID(row.ID), ThreadID: appID(row.ThreadID), AssignedTo: appID(row.AssignedTo), AssignedBy: appID(row.AssignedBy), ThreadVersion: row.ThreadVersion, DueAt: timePointer(row.DueAt), Reason: row.AssignmentReason, AssignedAt: row.AssignedAt.Time.UTC()}
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetReviewDashboard(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, observedAt time.Time) (result application.ReviewDashboard, err error) {
	if err = s.validRead(ctx, a, 101); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if _, err := q.ValidateReviewDashboardBid(txCtx, dbID(bidID)); errors.Is(err, pgx.ErrNoRows) {
			return application.ErrReviewDashboardNotFound
		} else if err != nil {
			return err
		}
		counts, err := q.GetBidReviewDashboardCounts(txCtx, capturesql.GetBidReviewDashboardCountsParams{ObservedAt: dbTime(observedAt), BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		rows, err := q.ListBidReviewerWorkloads(txCtx, capturesql.ListBidReviewerWorkloadsParams{ObservedAt: dbTime(observedAt), BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		if len(rows) > 100 {
			return application.ErrReviewWorkloadReadLimit
		}
		result = application.ReviewDashboard{BidID: bidID, TotalCount: counts.TotalCount, OpenCount: counts.OpenCount, ResolvedCount: counts.ResolvedCount, UnassignedCount: counts.UnassignedCount, OverdueCount: counts.OverdueCount, ObservedAt: observedAt, Reviewers: make([]application.ReviewerWorkload, len(rows))}
		for i, row := range rows {
			result.Reviewers[i] = application.ReviewerWorkload{ReviewerID: appID(row.AssignedTo), OpenCount: row.OpenCount, OverdueCount: row.OverdueCount, BidCount: 1, EarliestDueAt: timePointer(row.EarliestDueAt)}
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) GetTenantReviewWorkloads(ctx context.Context, a application.WorkflowAuthority, observedAt time.Time) (result application.TenantReviewWorkloads, err error) {
	if err = s.validRead(ctx, a, 101); err != nil {
		return result, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		counts, err := q.GetTenantReviewWorkloadCounts(txCtx, dbTime(observedAt))
		if err != nil {
			return err
		}
		rows, err := q.ListTenantReviewerWorkloads(txCtx, dbTime(observedAt))
		if err != nil {
			return err
		}
		if len(rows) > 100 {
			return application.ErrReviewWorkloadReadLimit
		}
		result = application.TenantReviewWorkloads{OpenCount: counts.OpenCount, UnassignedCount: counts.UnassignedCount, OverdueCount: counts.OverdueCount, BidCount: counts.BidCount, ObservedAt: observedAt, Reviewers: make([]application.ReviewerWorkload, len(rows))}
		for i, row := range rows {
			result.Reviewers[i] = application.ReviewerWorkload{ReviewerID: appID(row.AssignedTo), OpenCount: row.OpenCount, OverdueCount: row.OverdueCount, BidCount: row.BidCount, EarliestDueAt: timePointer(row.EarliestDueAt)}
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListReviewerOpenWork(ctx context.Context, a application.WorkflowAuthority, reviewerID application.ID) (result []application.ReviewerOpenWork, err error) {
	if err = s.valid(ctx, a); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := capturesql.New(db).ListReviewerOpenWork(txCtx, dbID(reviewerID))
		if err != nil {
			return err
		}
		if len(rows) > 200 {
			return application.ErrReviewWorkloadReadLimit
		}
		result = make([]application.ReviewerOpenWork, len(rows))
		for i, row := range rows {
			result[i] = application.ReviewerOpenWork{ThreadID: appID(row.ID), BidID: appID(row.BidID), TargetID: appID(row.TargetID), TargetType: row.TargetType, Subject: row.Subject, ThreadVersion: row.Version, DueAt: timePointer(row.DueAt), CreatedAt: row.CreatedAt.Time.UTC()}
		}
		return nil
	})
	return result, err
}
func (s *WorkflowStore) ListReviewThreads(ctx context.Context, a application.WorkflowAuthority, bidID application.ID) (result []application.ReviewThread, err error) {
	if err = s.validRead(ctx, a, 100); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		count, err := q.CountBidReviewThreads(txCtx, dbID(bidID))
		if err != nil {
			return err
		}
		if count > 100 {
			return application.ErrReviewThreadReadLimit
		}
		rows, err := q.ListReviewThreads(txCtx, dbID(bidID))
		if err != nil {
			return err
		}
		result = make([]application.ReviewThread, len(rows))
		totalComments := 0
		for i, row := range rows {
			result[i], err = mapReviewThread(txCtx, q, row.ID, row.BidID, row.TargetType, row.TargetID, row.Subject, row.Status, row.Version, row.CreatedBy, row.CreatedAt, row.ResolvedBy, row.ResolvedAt, row.AssignedTo, row.DueAt)
			if err != nil {
				return err
			}
			totalComments += len(result[i].Comments)
			if totalComments > 1000 {
				return application.ErrReviewThreadReadLimit
			}
		}
		return nil
	})
	return result, err
}
func loadReviewThread(ctx context.Context, q *capturesql.Queries, bidID, threadID application.ID) (application.ReviewThread, error) {
	row, err := q.GetReviewThread(ctx, capturesql.GetReviewThreadParams{ID: dbID(threadID), BidID: dbID(bidID)})
	if err != nil {
		return application.ReviewThread{}, err
	}
	return mapReviewThread(ctx, q, row.ID, row.BidID, row.TargetType, row.TargetID, row.Subject, row.Status, row.Version, row.CreatedBy, row.CreatedAt, row.ResolvedBy, row.ResolvedAt, row.AssignedTo, row.DueAt)
}
func mapReviewThread(ctx context.Context, q *capturesql.Queries, id, bidID pgtype.UUID, targetType string, targetID pgtype.UUID, subject, status string, version int64, createdBy pgtype.UUID, createdAt pgtype.Timestamptz, resolvedBy pgtype.UUID, resolvedAt pgtype.Timestamptz, assignedTo pgtype.UUID, dueAt pgtype.Timestamptz) (application.ReviewThread, error) {
	count, err := q.CountReviewComments(ctx, id)
	if err != nil {
		return application.ReviewThread{}, err
	}
	if count > 200 {
		return application.ReviewThread{}, application.ErrReviewThreadReadLimit
	}
	rows, err := q.ListReviewComments(ctx, id)
	if err != nil {
		return application.ReviewThread{}, err
	}
	comments := make([]application.ReviewComment, len(rows))
	for i, row := range rows {
		comments[i] = application.ReviewComment{ID: appID(row.ID), ThreadID: appID(row.ThreadID), CreatedBy: appID(row.CreatedBy), ThreadVersion: row.ThreadVersion, Kind: row.CommentKind, Body: row.Body, CreatedAt: row.CreatedAt.Time.UTC()}
	}
	return application.ReviewThread{ID: appID(id), BidID: appID(bidID), TargetID: appID(targetID), CreatedBy: appID(createdBy), ResolvedBy: appID(resolvedBy), TargetType: targetType, Subject: subject, Status: status, Version: version, CreatedAt: createdAt.Time.UTC(), ResolvedAt: timePointer(resolvedAt), AssignedTo: appID(assignedTo), DueAt: timePointer(dueAt), Comments: comments}, nil
}
