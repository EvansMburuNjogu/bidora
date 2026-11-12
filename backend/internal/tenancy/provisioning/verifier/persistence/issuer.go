// Package persistence implements the verifier's fixed-function issuance boundary.
// It is not composed into a worker or API runtime.
package persistence

import (
	"context"

	"bidora/backend/internal/postgres"
	app "bidora/backend/internal/tenancy/provisioning/verifier/application"
	q "bidora/backend/internal/tenancy/provisioning/verifier/persistence/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Runner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type Issuer struct{ runner Runner }

func NewIssuer(runner Runner) (*Issuer, error) {
	if runner == nil {
		return nil, app.ErrInvalid
	}
	return &Issuer{runner: runner}, nil
}
func id(v uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: v, Valid: v != uuid.Nil} }

// Issue persists an assertion through the verifier-only database function.
// Its caller must independently inspect the authoritative target first; this
// method cannot determine whether the supplied digest represents true facts.
func (s *Issuer) Issue(ctx context.Context, input app.Issuance) (receipt app.Receipt, err error) {
	if s == nil || s.runner == nil || ctx == nil || !input.Valid() {
		return receipt, app.ErrInvalid
	}
	err = s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		if c == nil || db == nil {
			return app.ErrUnavailable
		}
		row, e := q.New(db).IssueProvisioningAttestation(c, q.IssueProvisioningAttestationParams{AttestationID: id(input.AttestationID), JobID: id(input.JobID), OperationID: id(input.OperationID), TenantID: id(input.TenantID), ReservationID: id(input.ReservationID), ClusterID: id(input.ClusterID), ReservationGeneration: input.ReservationGeneration, LeaseTokenHash: input.LeaseTokenHash[:], NextCheckpoint: input.NextCheckpoint, VerificationKind: input.VerificationKind, VerificationDigest: input.VerificationDigest[:]})
		if e != nil {
			return e
		}
		if !row.AttestationID.Valid || uuid.UUID(row.AttestationID.Bytes) != input.AttestationID || !row.ExpiresAt.Valid {
			return app.ErrUnavailable
		}
		receipt = app.Receipt{AttestationID: uuid.UUID(row.AttestationID.Bytes), ExpiresAt: row.ExpiresAt.Time.UTC()}
		return nil
	})
	if err != nil {
		return app.Receipt{}, app.ErrUnavailable
	}
	return receipt, nil
}

// RecordCapacity records only a verifier assertion. A deployment-owned
// inspector must independently determine actual available capacity first.
func (s *Issuer) RecordCapacity(ctx context.Context, input app.CapacityObservation) (receipt app.CapacityReceipt, err error) {
	if s == nil || s.runner == nil || ctx == nil || !input.Valid() {
		return receipt, app.ErrInvalid
	}
	err = s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		if c == nil || db == nil {
			return app.ErrUnavailable
		}
		row, e := q.New(db).RecordProvisioningCapacity(c, q.RecordProvisioningCapacityParams{ObservationID: id(input.ObservationID), ClusterID: id(input.ClusterID), QuotaVersion: input.QuotaVersion, ClusterVersion: input.ClusterVersion, AvailableSlots: input.AvailableSlots, EvidenceDigest: input.EvidenceDigest[:]})
		if e != nil {
			return e
		}
		if !row.ObservationID.Valid || uuid.UUID(row.ObservationID.Bytes) != input.ObservationID || !row.ExpiresAt.Valid {
			return app.ErrUnavailable
		}
		receipt = app.CapacityReceipt{ObservationID: uuid.UUID(row.ObservationID.Bytes), ExpiresAt: row.ExpiresAt.Time.UTC()}
		return nil
	})
	if err != nil {
		return app.CapacityReceipt{}, app.ErrUnavailable
	}
	return receipt, nil
}

var _ app.Issuer = (*Issuer)(nil)
var _ app.CapacityIssuer = (*Issuer)(nil)
