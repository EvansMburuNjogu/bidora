// Package application defines verifier-only attestation issuance. It does not
// inspect external resources and is not an authoritative checkpoint verifier.
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
	ErrInvalid     = errors.New("provisioning_attestation_invalid")
	ErrUnavailable = errors.New("provisioning_attestation_unavailable")
)

type Issuance struct {
	AttestationID, JobID, OperationID, TenantID uuid.UUID
	ReservationID, ClusterID                    uuid.UUID
	ReservationGeneration                       int64
	LeaseTokenHash, VerificationDigest          [32]byte
	NextCheckpoint, VerificationKind            string
}

func (Issuance) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED provisioning attestation]") }
func (Issuance) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED provisioning attestation]")
}
func (v Issuance) Valid() bool {
	if v.AttestationID == uuid.Nil || v.JobID == uuid.Nil || v.OperationID == uuid.Nil || v.TenantID == uuid.Nil || v.ReservationID == uuid.Nil || v.ClusterID == uuid.Nil || v.ReservationGeneration < 1 || v.LeaseTokenHash == [32]byte{} || v.VerificationDigest == [32]byte{} {
		return false
	}
	switch v.NextCheckpoint {
	case "DATABASE_RESERVED":
		return v.VerificationKind == "PLACEMENT"
	case "DATABASE_CREATED":
		return v.VerificationKind == "DATABASE_OWNERSHIP"
	case "MIGRATIONS_APPLIED":
		return v.VerificationKind == "MIGRATION_LEDGER"
	case "RUNTIME_GRANTS_APPLIED":
		return v.VerificationKind == "RUNTIME_GRANTS"
	case "COMPATIBILITY_VERIFIED":
		return v.VerificationKind == "SCHEMA_COMPATIBILITY"
	default:
		return false
	}
}

type Receipt struct {
	AttestationID uuid.UUID
	ExpiresAt     time.Time
}

func (Receipt) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED provisioning attestation receipt]")
}
func (Receipt) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED provisioning attestation receipt]")
}

type Issuer interface {
	Issue(context.Context, Issuance) (Receipt, error)
}

// CapacityObservation is an assertion from a separately authenticated verifier,
// not proof of physical availability on its own. An unimplemented deployment-
// owned inspector must establish the cluster facts before issuance.
type CapacityObservation struct {
	ObservationID, ClusterID     uuid.UUID
	QuotaVersion, ClusterVersion int64
	AvailableSlots               int32
	EvidenceDigest               [32]byte
}

func (CapacityObservation) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[REDACTED capacity observation]")
}
func (CapacityObservation) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED capacity observation]")
}
func (v CapacityObservation) Valid() bool {
	return v.ObservationID != uuid.Nil && v.ClusterID != uuid.Nil && v.QuotaVersion > 0 && v.ClusterVersion > 0 && v.AvailableSlots >= 0 && v.EvidenceDigest != [32]byte{}
}

type CapacityReceipt struct {
	ObservationID uuid.UUID
	ExpiresAt     time.Time
}

func (CapacityReceipt) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED capacity receipt]") }
func (CapacityReceipt) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED capacity receipt]")
}

type CapacityIssuer interface {
	RecordCapacity(context.Context, CapacityObservation) (CapacityReceipt, error)
}
