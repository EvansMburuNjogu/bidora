package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// OperationScope is obtained from the server-owned control-plane lease, never
// from browser input. The deployment provisioner must independently validate and
// serialize the operation against current authority; a Go value or possession of
// a lease token alone is not authorization for a privileged resource operation.
type OperationScope struct {
	JobID, OperationID, TenantID uuid.UUID
	LeaseToken, ExecutorID       uuid.UUID
	LeaseExpiresAt               time.Time
}

func (s OperationScope) Valid() bool {
	return s.JobID != uuid.Nil && s.OperationID != uuid.Nil && s.TenantID != uuid.Nil && s.LeaseToken != uuid.Nil && s.ExecutorID != uuid.Nil && !s.LeaseExpiresAt.IsZero()
}
func (OperationScope) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED provisioning scope]") }
func (OperationScope) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED provisioning scope]")
}

// PlacementRef and SecretRef are server-generated opaque locators. They never
// grant authority by themselves and must not appear in browser responses or
// ordinary logs. The concrete provider validates syntax and ownership.
type PlacementRef string

func (PlacementRef) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED placement reference]") }
func (PlacementRef) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED placement reference]")
}

func validOpaqueReference(value string, max int) bool {
	if len(value) < 1 || len(value) > max || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

type SecretRef string

func (SecretRef) Format(s fmt.State, _ rune)   { fmt.Fprint(s, "[REDACTED secret reference]") }
func (SecretRef) MarshalJSON() ([]byte, error) { return json.Marshal("[REDACTED secret reference]") }

// EffectOutcome is intentionally not a boolean. Confirmed requires an
// authoritative ownership/state observation, not just a successful request or
// provider acknowledgement. Unknown effect and ownership conflicts must be
// reconciled by an authorized operator, not retried blindly. Any transport or
// implementation error after dispatch is ambiguous unless the adapter proves
// that the effect was not performed; callers must never infer non-execution.
type EffectOutcome string

const (
	EffectConfirmed         EffectOutcome = "CONFIRMED"
	EffectNotPerformed      EffectOutcome = "NOT_PERFORMED"
	EffectUnavailable       EffectOutcome = "UNAVAILABLE"
	EffectAmbiguous         EffectOutcome = "AMBIGUOUS"
	EffectOwnershipConflict EffectOutcome = "OWNERSHIP_CONFLICT"
)

func (o EffectOutcome) Valid() bool {
	switch o {
	case EffectConfirmed, EffectNotPerformed, EffectUnavailable, EffectAmbiguous, EffectOwnershipConflict:
		return true
	}
	return false
}

type PlacementResult struct {
	Outcome   EffectOutcome
	Reference PlacementRef
	// EvidenceDigest commits to safe canonical ownership/placement evidence;
	// it is not, independently, proof of resource state.
	EvidenceDigest [32]byte
}

func (r PlacementResult) Valid() bool {
	if !r.Outcome.Valid() {
		return false
	}
	if r.Outcome == EffectConfirmed {
		return validOpaqueReference(string(r.Reference), 500) && r.EvidenceDigest != [32]byte{}
	}
	return r.Reference == "" && r.EvidenceDigest == [32]byte{}
}
func (PlacementResult) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED placement result]") }
func (PlacementResult) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED placement result]")
}

type DatabaseResult struct {
	Outcome        EffectOutcome
	EvidenceDigest [32]byte
}

func (r DatabaseResult) Valid() bool {
	if !r.Outcome.Valid() {
		return false
	}
	if r.Outcome == EffectConfirmed {
		return r.EvidenceDigest != [32]byte{}
	}
	return r.EvidenceDigest == [32]byte{}
}
func (DatabaseResult) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED database result]") }
func (DatabaseResult) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED database result]")
}

// DatabaseProvisioner is a deployment-owned privileged boundary. Reserve must
// enforce server-selected capacity and durable operation ownership. Inspect and
// EnsureDatabase must reconcile the exact operation-owned target. Implementations
// must serialize/fence external effects by durable operation identity as well as
// checking current lease authority; a control DB lease cannot cancel in-flight
// CREATE DATABASE. No automatic deletion or reassignment is authorized here.
type DatabaseProvisioner interface {
	Reserve(context.Context, OperationScope) (PlacementResult, error)
	Inspect(context.Context, OperationScope, PlacementRef) (DatabaseResult, error)
	EnsureDatabase(context.Context, OperationScope, PlacementRef) (DatabaseResult, error)
}

type CredentialPurpose string

const (
	TenantMigratorCredential CredentialPurpose = "TENANT_MIGRATOR"
	TenantRuntimeCredential  CredentialPurpose = "TENANT_RUNTIME"
)

func (p CredentialPurpose) Valid() bool {
	return p == TenantMigratorCredential || p == TenantRuntimeCredential
}

// SecretReferenceResult contains a reference and version only, never secret
// material. A confirmed reference must be bound by the secret provider to the
// exact operation, tenant and purpose, and independently auditable.
type SecretReferenceResult struct {
	Outcome   EffectOutcome
	Reference SecretRef
	Version   string
}

func (r SecretReferenceResult) Valid() bool {
	if !r.Outcome.Valid() {
		return false
	}
	if r.Outcome == EffectConfirmed {
		return validOpaqueReference(string(r.Reference), 500) && validOpaqueReference(r.Version, 128)
	}
	return r.Reference == "" && r.Version == ""
}
func (SecretReferenceResult) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED secret result]") }
func (SecretReferenceResult) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED secret result]")
}

// CredentialAccess is an opaque provider-owned capability, not a serializable
// credential. Use supplies short-lived borrowed material only to an authorized
// migration or tenant-pool adapter. The callback must not retain or serialize
// it; the provider must clear its mutable buffers after the callback returns.
// This contract deliberately defines no ambient credentials.
type CredentialAccess interface {
	CredentialPurpose() CredentialPurpose
	Use(func(username string, password []byte) error) error
}

// SecretManager is deployment-owned. Ensure and Inspect must be idempotent and
// operation-bound, and verify caller authority separately from locator presence.
// WithCredential must authenticate/authorize the caller and purpose, resolve the
// exact version only for the duration of use, audit access without secret values,
// and prevent use after callback return as far as the concrete provider permits.
// A callback must not retain or serialize secret material. Implementations must
// surface ambiguous effects explicitly and must not silently rotate credentials.
type SecretManager interface {
	EnsureCredentialReference(context.Context, OperationScope, CredentialPurpose) (SecretReferenceResult, error)
	InspectReference(context.Context, OperationScope, CredentialPurpose, SecretRef, string) (SecretReferenceResult, error)
	WithCredential(context.Context, OperationScope, CredentialPurpose, SecretRef, string, func(CredentialAccess) error) error
}
