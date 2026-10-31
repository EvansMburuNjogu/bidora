package persistence

import (
	app "bidora/backend/internal/opportunity/application"
	qsql "bidora/backend/internal/opportunity/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *WorkflowStore) FindBidArchiveReplay(ctx context.Context, a app.WorkflowAuthority, bid app.ID, key, request [32]byte) (result app.BidArchiveResult, found bool, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		r, e := q.GetBidArchiveIdempotency(tc, qsql.GetBidArchiveIdempotencyParams{ActorID: dbID(a.ActorID), KeyHash: key[:]})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		found = true
		if !bytes.Equal(r.RequestHash, request[:]) {
			return app.ErrBidArchiveConflict
		}
		if r.State != "COMPLETED" || r.ResponseStatus == nil || *r.ResponseStatus != 201 || !r.ResponseRef.Valid {
			return errors.New("archive_pending")
		}
		v, e := q.GetBidArchiveEvent(tc, r.ResponseRef)
		if e != nil {
			return e
		}
		result = app.BidArchiveResult{Archive: mapBidArchive(v), Replay: true}
		return nil
	})
	return
}
func (s *WorkflowStore) CommitBidArchive(ctx context.Context, a app.WorkflowAuthority, c app.BidArchiveCommit) (result app.BidArchiveResult, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		q := qsql.New(db)
		replay, ref, _, e := claimWorkflow(tc, q, c.CommandID, a.ActorID, "ARCHIVE_BID", c.KeyHash, c.RequestHash, c.OccurredAt)
		if errors.Is(e, app.ErrWorkflowConflict) {
			return app.ErrBidArchiveConflict
		}
		if e != nil {
			return e
		}
		if replay {
			v, e := q.GetBidArchiveEvent(tc, dbID(ref))
			if e != nil {
				return e
			}
			result = app.BidArchiveResult{Archive: mapBidArchive(v), Replay: true}
			return nil
		}
		x, e := q.LockBidForArchive(tc, qsql.LockBidForArchiveParams{ObservedAt: dbTime(c.OccurredAt), BidID: dbID(c.BidID)})
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrBidArchiveNotFound
		}
		if e != nil {
			return e
		}
		eligible := x.State == "NO_BID" || (x.State == "SUBMITTED" && x.HasSubmission && (x.CurrentOutcome == "WON" || x.CurrentOutcome == "LOST" || x.CurrentOutcome == "DISQUALIFIED" || x.CurrentOutcome == "CANCELLED" || x.CurrentOutcome == "WITHDRAWN"))
		if !eligible || x.ArchivedAt.Valid || x.Version != c.ExpectedWorkspaceVersion || x.OpenReviews > 0 || x.PendingUploads > 0 || x.PendingJobs > 0 {
			return app.ErrBidArchiveConflict
		}
		next := x.Version + 1
		n, e := q.ArchiveBidWorkspace(tc, qsql.ArchiveBidWorkspaceParams{ArchivedAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID), Reason: &c.Reason, BidID: dbID(c.BidID), ExpectedVersion: x.Version})
		if e != nil || n != 1 {
			return app.ErrBidArchiveConflict
		}
		if e = q.InsertBidArchiveEvent(tc, qsql.InsertBidArchiveEventParams{ID: dbID(c.ArchiveID), BidID: dbID(c.BidID), WorkspaceVersion: next, ActorID: dbID(a.ActorID), Reason: c.Reason, ArchivedAt: dbTime(c.OccurredAt)}); e != nil {
			return e
		}
		if e = q.InsertBidArchiveAudit(tc, qsql.InsertBidArchiveAuditParams{ID: dbID(c.AuditID), ActorID: dbID(a.ActorID), BidID: dbID(c.BidID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(c.CorrelationID), WorkspaceVersion: next}); e != nil {
			return e
		}
		if e = q.InsertBidArchiveOutbox(tc, qsql.InsertBidArchiveOutboxParams{ID: dbID(c.OutboxID), BidID: dbID(c.BidID), WorkspaceVersion: next, ArchiveID: dbID(c.ArchiveID), OccurredAt: dbTime(c.OccurredAt), ActorID: dbID(a.ActorID)}); e != nil {
			return e
		}
		if e = completeWorkflow(tc, q, c.CommandID, c.ArchiveID, next, c.OccurredAt); e != nil {
			return e
		}
		v, e := q.GetBidArchiveEvent(tc, dbID(c.ArchiveID))
		if e != nil {
			return e
		}
		result = app.BidArchiveResult{Archive: mapBidArchive(v)}
		return nil
	})
	return
}
func (s *WorkflowStore) GetBidArchive(ctx context.Context, a app.WorkflowAuthority, bid app.ID) (result app.BidArchive, err error) {
	if err = s.valid(ctx, a); err != nil {
		return
	}
	err = s.runner.WithinReadOnlyTransaction(ctx, func(tc context.Context, db postgres.DBTX) error {
		v, e := qsql.New(db).GetBidArchiveEventByBid(tc, dbID(bid))
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrBidArchiveNotFound
		}
		if e != nil {
			return e
		}
		result = mapBidArchive(v)
		return nil
	})
	return
}
func mapBidArchive(v qsql.BidArchiveEvent) app.BidArchive {
	return app.BidArchive{ID: appID(v.ID), BidID: appID(v.BidID), ArchivedBy: appID(v.ArchivedBy), WorkspaceVersion: v.WorkspaceVersion, Reason: v.Reason, PolicyVersion: v.PolicyVersion, ArchivedAt: v.ArchivedAt.Time.UTC()}
}
