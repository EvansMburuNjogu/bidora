// Package persistence implements the dedicated worker's control-plane job
// transactions. It is not composed into either API runtime.
package persistence

import (
	"context"
	"errors"

	"bidora/backend/internal/postgres"
	app "bidora/backend/internal/tenancy/provisioning/application"
	q "bidora/backend/internal/tenancy/provisioning/persistence/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Runner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type Store struct{ runner Runner }

func New(runner Runner) (*Store, error) {
	if runner == nil {
		return nil, app.ErrInvalid
	}
	return &Store{runner: runner}, nil
}
func pgid(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil} }
func (s *Store) Claim(ctx context.Context, executor, token, event uuid.UUID, seconds int32) (lease app.Lease, found bool, err error) {
	err = s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		row, e := q.New(db).ClaimProvisioningJob(c, q.ClaimProvisioningJobParams{LeaseToken: pgid(token), EventID: pgid(event), LeaseSeconds: seconds})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if !row.ID.Valid || !row.OperationID.Valid || !row.TenantID.Valid || !row.LeaseUntil.Valid || !row.ExecutorID.Valid || uuid.UUID(row.ExecutorID.Bytes) != executor {
			return app.ErrUnavailable
		}
		lease = app.Lease{JobID: uuid.UUID(row.ID.Bytes), OperationID: uuid.UUID(row.OperationID.Bytes), TenantID: uuid.UUID(row.TenantID.Bytes), Checkpoint: row.Checkpoint, Attempts: row.Attempts, ExpiresAt: row.LeaseUntil.Time.UTC(), Token: token, ExecutorID: executor}
		found = true
		return nil
	})
	if err != nil {
		return app.Lease{}, false, app.ErrUnavailable
	}
	return lease, found, nil
}
func (s *Store) Renew(ctx context.Context, l app.Lease, seconds int32) error {
	return safeStoreError(s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		renewed, e := q.New(db).RenewProvisioningLease(c, q.RenewProvisioningLeaseParams{JobID: pgid(l.JobID), OperationID: pgid(l.OperationID), TenantID: pgid(l.TenantID), LeaseToken: pgid(l.Token), LeaseSeconds: seconds})
		if e != nil {
			return e
		}
		if !renewed {
			return app.ErrStaleLease
		}
		return nil
	}))
}
func (s *Store) Reserve(ctx context.Context, l app.Lease, reservation uuid.UUID) (result app.PlacementReservation, err error) {
	if s == nil || s.runner == nil || ctx == nil || reservation == uuid.Nil {
		return result, app.ErrInvalid
	}
	err = s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		if c == nil || db == nil {
			return app.ErrUnavailable
		}
		row, e := q.New(db).ReserveProvisioningPlacement(c, q.ReserveProvisioningPlacementParams{ReservationID: pgid(reservation), JobID: pgid(l.JobID), OperationID: pgid(l.OperationID), TenantID: pgid(l.TenantID), LeaseToken: pgid(l.Token)})
		if e != nil {
			return e
		}
		if !row.ReservationID.Valid || uuid.UUID(row.ReservationID.Bytes) != reservation || !row.ClusterID.Valid || row.Generation < 1 || len(row.TargetDigest) != 32 {
			return app.ErrUnavailable
		}
		result = app.PlacementReservation{ID: uuid.UUID(row.ReservationID.Bytes), ClusterID: uuid.UUID(row.ClusterID.Bytes), Generation: row.Generation}
		copy(result.TargetDigest[:], row.TargetDigest)
		if result.TargetDigest == [32]byte{} {
			return app.ErrUnavailable
		}
		return nil
	})
	if err != nil {
		return app.PlacementReservation{}, app.ErrUnavailable
	}
	return result, nil
}

func (s *Store) Advance(context.Context, app.Lease, app.CheckpointEvidence, uuid.UUID) error {
	// No database advance function exists until independently authorized
	// verifier attestations can be consumed atomically.
	return app.ErrUnavailable
}
func (s *Store) Hold(ctx context.Context, l app.Lease, reason string, event uuid.UUID) error {
	return safeStoreError(s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		held, e := q.New(db).HoldProvisioningJobForReview(c, q.HoldProvisioningJobForReviewParams{JobID: pgid(l.JobID), OperationID: pgid(l.OperationID), TenantID: pgid(l.TenantID), LeaseToken: pgid(l.Token), FailureCode: reason, EventID: pgid(event)})
		if e != nil {
			return e
		}
		if !held {
			return app.ErrStaleLease
		}
		return nil
	}))
}
func (s *Store) HoldExpired(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	// Expired-lease recovery needs a separately authorized operator boundary.
	return false, app.ErrUnavailable
}

func safeStoreError(err error) error {
	if err == nil || errors.Is(err, app.ErrStaleLease) {
		return err
	}
	return app.ErrUnavailable
}

var _ app.Store = (*Store)(nil)
