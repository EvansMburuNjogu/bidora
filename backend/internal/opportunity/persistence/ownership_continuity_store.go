package persistence

import (
	"bidora/backend/internal/opportunity/application"
	capturesql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"time"
)

func (s *WorkflowStore) ListOwnershipContinuityActivity(ctx context.Context, a application.WorkflowAuthority, observedAt time.Time) (result []application.OwnershipActivity, err error) {
	if err = s.valid(ctx, a); err != nil {
		return nil, err
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		rows, err := capturesql.New(db).ListOwnershipContinuityActivity(txCtx, dbTime(observedAt))
		if err != nil {
			return err
		}
		if len(rows) > 500 {
			return application.ErrOwnershipContinuityLimit
		}
		result = make([]application.OwnershipActivity, len(rows))
		for i, row := range rows {
			result[i] = application.OwnershipActivity{UserID: appID(row.UserID), OwnedBidCount: row.OwnedBidCount, OpenRequirementCount: row.OpenRequirementCount, OpenReviewCount: row.OpenReviewCount, OverdueReviewCount: row.OverdueReviewCount, LastActivityAt: timePointer(row.LastActivityAt)}
		}
		return nil
	})
	return result, err
}
