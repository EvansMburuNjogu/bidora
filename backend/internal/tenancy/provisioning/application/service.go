// Package application contains the fenced provisioning-job boundary. It does
// not create resources, select placement, activate subscriptions or start a worker.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalid     = errors.New("provisioning_job_invalid")
	ErrStaleLease  = errors.New("provisioning_lease_stale")
	ErrUnavailable = errors.New("provisioning_job_unavailable")
)

type Lease struct {
	JobID, OperationID, TenantID, Token, ExecutorID uuid.UUID
	Checkpoint                                      string
	Attempts                                        int32
	ExpiresAt                                       time.Time
}

func (Lease) Format(s fmt.State, _ rune)   { fmt.Fprint(s, "[REDACTED provisioning lease]") }
func (Lease) MarshalJSON() ([]byte, error) { return json.Marshal("[REDACTED provisioning lease]") }

type Store interface {
	Claim(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int32) (Lease, bool, error)
	Renew(context.Context, Lease, int32) error
	Reserve(context.Context, Lease, uuid.UUID) (PlacementReservation, error)
	Advance(context.Context, Lease, CheckpointEvidence, uuid.UUID) error
	Hold(context.Context, Lease, string, uuid.UUID) error
	HoldExpired(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

// CheckpointEvidence binds a trusted verifier's observation to the exact lease
// and operation. Digest is a purpose-separated digest of safe canonical proof;
// raw credentials, locators and provider responses are never stored here.
type CheckpointEvidence struct {
	JobID, OperationID, TenantID, LeaseToken uuid.UUID
	Checkpoint, Kind                         string
	Digest                                   [32]byte
}

func (CheckpointEvidence) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED checkpoint evidence]")
}
func (CheckpointEvidence) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED checkpoint evidence]")
}

// CheckpointVerifier must inspect authoritative operation-owned state. A digest
// is only a commitment to evidence, not independent proof of an external effect.
// No verifier is composed here and provider responses alone are insufficient.
type CheckpointVerifier interface {
	Verify(context.Context, Lease, string) (CheckpointEvidence, error)
}

type Service struct {
	store Store
}

func New(store Store) (*Service, error) {
	if store == nil {
		return nil, ErrInvalid
	}
	return &Service{store: store}, nil
}

// PlacementReservation is a stable control-plane identity, not proof that a
// physical database exists or capacity remains available.
type PlacementReservation struct {
	ID, ClusterID uuid.UUID
	Generation    int64
	TargetDigest  [32]byte
}

func (PlacementReservation) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED placement reservation]")
}
func (PlacementReservation) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED placement reservation]")
}

func validLease(l Lease) bool {
	return l.JobID != uuid.Nil && l.OperationID != uuid.Nil && l.TenantID != uuid.Nil && l.Token != uuid.Nil && l.ExecutorID != uuid.Nil && !l.ExpiresAt.IsZero()
}
func validDuration(seconds int32) bool { return seconds >= 30 && seconds <= 300 }

func (s *Service) Claim(ctx context.Context, executor uuid.UUID, seconds int32) (Lease, bool, error) {
	if s == nil || ctx == nil || executor == uuid.Nil || !validDuration(seconds) {
		return Lease{}, false, ErrInvalid
	}
	token, err := uuid.NewRandom()
	if err != nil {
		return Lease{}, false, ErrUnavailable
	}
	event, err := uuid.NewRandom()
	if err != nil {
		return Lease{}, false, ErrUnavailable
	}
	return s.store.Claim(ctx, executor, token, event, seconds)
}
func (s *Service) Renew(ctx context.Context, lease Lease, seconds int32) error {
	if s == nil || ctx == nil || !validLease(lease) || !validDuration(seconds) {
		return ErrInvalid
	}
	return s.store.Renew(ctx, lease, seconds)
}

// Reserve obtains the server-selected quota reservation. The stable operation-
// derived ID permits same-job replay without inventing a new target after an
// ambiguous response. A separate observer and broker must still confirm actual
// capacity and resource ownership before any checkpoint can advance.
func (s *Service) Reserve(ctx context.Context, lease Lease) (PlacementReservation, error) {
	if s == nil || ctx == nil || !validLease(lease) || lease.Checkpoint != "PLAN_SELECTED" {
		return PlacementReservation{}, ErrInvalid
	}
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("bidora:provisioning-reservation:v1:"+lease.JobID.String()))
	return s.store.Reserve(ctx, lease, id)
}

// Trial checkpoints only. Assignment and commercial activation require a
// separate transaction-bound access-readiness design and are intentionally not
// advanceable by this boundary.
var nextCheckpoint = map[string]string{
	"PLAN_SELECTED": "DATABASE_RESERVED", "DATABASE_RESERVED": "DATABASE_CREATED", "DATABASE_CREATED": "MIGRATIONS_APPLIED", "MIGRATIONS_APPLIED": "RUNTIME_GRANTS_APPLIED", "RUNTIME_GRANTS_APPLIED": "COMPATIBILITY_VERIFIED",
}

var evidenceKind = map[string]string{
	"DATABASE_RESERVED": "PLACEMENT", "DATABASE_CREATED": "DATABASE_OWNERSHIP", "MIGRATIONS_APPLIED": "MIGRATION_LEDGER", "RUNTIME_GRANTS_APPLIED": "RUNTIME_GRANTS", "COMPATIBILITY_VERIFIED": "SCHEMA_COMPATIBILITY",
}

func (s *Service) Advance(ctx context.Context, lease Lease, next string) error {
	if s == nil || ctx == nil || !validLease(lease) || nextCheckpoint[lease.Checkpoint] != next || next == "" {
		return ErrInvalid
	}
	// Advancement remains disabled until an independently authorized verifier
	// can issue an attestation consumed atomically by the database function.
	return ErrUnavailable
}
func (s *Service) Hold(ctx context.Context, lease Lease, reason string) error {
	if s == nil || ctx == nil || !validLease(lease) || !validFailure(reason) {
		return ErrInvalid
	}
	event, err := uuid.NewRandom()
	if err != nil {
		return ErrUnavailable
	}
	return s.store.Hold(ctx, lease, reason, event)
}
func (s *Service) HoldExpired(ctx context.Context, executor uuid.UUID) (bool, error) {
	if s == nil || ctx == nil || executor == uuid.Nil {
		return false, ErrInvalid
	}
	event, err := uuid.NewRandom()
	if err != nil {
		return false, ErrUnavailable
	}
	return s.store.HoldExpired(ctx, executor, event)
}
func validFailure(v string) bool {
	switch v {
	case "CAPACITY_UNAVAILABLE", "DATABASE_RESERVATION_FAILED", "DATABASE_CREATION_FAILED", "MIGRATION_FAILED", "RUNTIME_GRANT_FAILED", "COMPATIBILITY_FAILED", "ASSIGNMENT_FAILED", "SUBSCRIPTION_ACTIVATION_FAILED", "EFFECT_AMBIGUOUS", "INTERNAL_ERROR":
		return true
	}
	return false
}
