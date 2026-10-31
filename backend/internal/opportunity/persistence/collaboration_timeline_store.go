package persistence

import (
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *WorkflowStore) ListCollaborationTimeline(ctx context.Context, a application.WorkflowAuthority, bidID application.ID, cursor application.ReadCursor, limit int32) (result []application.CollaborationActivity, err error) {
	if err = s.validRead(ctx, a, limit); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		q := capturesql.New(db)
		if _, err := q.ValidateReviewDashboardBid(txCtx, dbID(bidID)); errors.Is(err, pgx.ErrNoRows) {
			return application.ErrCollaborationTimelineNotFound
		} else if err != nil {
			return err
		}
		cursorAt, cursorID := readCursor(cursor)
		rows, err := q.ListBidCollaborationTimeline(txCtx, capturesql.ListBidCollaborationTimelineParams{CursorAt: cursorAt, CursorID: cursorID, PageLimit: limit, BidID: dbID(bidID)})
		if err != nil {
			return err
		}
		result = make([]application.CollaborationActivity, len(rows))
		for i, row := range rows {
			result[i] = application.CollaborationActivity{ID: appID(row.EventID), ActorID: appID(row.ActorID), TargetID: appID(row.TargetID), RelatedID: appID(row.RelatedID), EventType: row.EventType, TargetType: row.TargetType, DetailCode: row.DetailCode, Version: row.EventVersion, OccurredAt: row.OccurredAt.Time.UTC(), DueAt: timePointer(row.DueAt)}
		}
		return nil
	})
	return result, err
}
