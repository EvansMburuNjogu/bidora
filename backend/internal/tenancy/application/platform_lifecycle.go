package application

import (
	platformapp "bidora/backend/internal/platformadmin/application"
	policy "bidora/backend/internal/platformadmin/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrPlatformTenantConflict = errors.New("platform_tenant_conflict")

type PlatformTenantCommand struct {
	Subject                           PlatformDirectorySubject
	TenantID, CorrelationID           ID
	Operation, Reason, IdempotencyKey string
	ExpectedVersion                   int64
}
type PlatformTenantReceipt struct {
	TenantID ID
	State    string
	Version  int64
	Replay   bool
}
type PlatformTenantCommandRecord struct {
	RequestHash [32]byte
	Receipt     PlatformTenantReceipt
}
type PlatformTenantLifecycleTx interface {
	ResolveAuthority(context.Context, PlatformDirectorySubject) (platformapp.AuthorityFacts, error)
	LockCommand(context.Context, string) error
	FindCommand(context.Context, ID, [32]byte) (PlatformTenantCommandRecord, bool, error)
	LockTenant(context.Context, ID) (PlatformTenantReceipt, error)
	ReadyForReactivation(context.Context, ID) (bool, error)
	Apply(context.Context, PlatformTenantCommand, PlatformTenantReceipt, [32]byte, [32]byte) error
}
type PlatformTenantLifecycleStore interface {
	WithinLifecycle(context.Context, func(context.Context, PlatformTenantLifecycleTx) error) error
}
type PlatformTenantLifecycleService struct{ store PlatformTenantLifecycleStore }

func NewPlatformTenantLifecycleService(store PlatformTenantLifecycleStore) (*PlatformTenantLifecycleService, error) {
	if store == nil {
		return nil, ErrPlatformDirectoryInvalid
	}
	return &PlatformTenantLifecycleService{store}, nil
}
func (s *PlatformTenantLifecycleService) Change(ctx context.Context, in PlatformTenantCommand) (out PlatformTenantReceipt, err error) {
	p := in.Subject
	if ctx == nil || p.ActorID.IsZero() || p.SessionID.IsZero() || p.PlatformContextGeneration < 1 || p.ObservedAt.IsZero() || in.TenantID.IsZero() || in.CorrelationID.IsZero() || in.ExpectedVersion < 1 || !utf8.ValidString(in.Reason) || strings.TrimSpace(in.Reason) != in.Reason || len(in.Reason) < 1 || len(in.Reason) > 2000 || strings.IndexFunc(in.Reason, func(r rune) bool { return r < 32 || r == 127 }) >= 0 || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 200 || (in.Operation != "SUSPEND" && in.Operation != "REACTIVATE") {
		return out, ErrPlatformDirectoryInvalid
	}
	key := sha256.Sum256([]byte("platform-tenant:v1:" + in.IdempotencyKey))
	raw, _ := json.Marshal(struct {
		Tenant              ID
		Operation, Reason   string
		Version, Generation int64
	}{in.TenantID, in.Operation, in.Reason, in.ExpectedVersion, p.PlatformContextGeneration})
	hash := sha256.Sum256(raw)
	err = s.store.WithinLifecycle(ctx, func(c context.Context, tx PlatformTenantLifecycleTx) error {
		a, e := tx.ResolveAuthority(c, p)
		if e != nil {
			return ErrPlatformDirectoryForbidden
		}
		capability := "platform.tenant.suspend"
		if in.Operation == "REACTIVATE" {
			capability = "platform.tenant.reactivate"
		}
		if a.UserID != platformapp.ID(p.ActorID) || !a.Grants(capability) {
			return ErrPlatformDirectoryForbidden
		}
		if a.ContextGeneration != p.PlatformContextGeneration {
			return ErrPlatformDirectoryContextStale
		}
		if !policy.AdministratorAssurance(a.AssuranceLevel, a.AssuranceMethod) || a.AssuredAt.IsZero() || a.AssuredAt.After(p.ObservedAt) || a.AssuredAt.Before(p.ObservedAt.Add(-15*time.Minute)) {
			return ErrPlatformDirectoryAssuranceRequired
		}
		if e = tx.LockCommand(c, "tenant:"+hex.EncodeToString(p.ActorID[:])+":"+hex.EncodeToString(key[:])); e != nil {
			return e
		}
		record, found, e := tx.FindCommand(c, p.ActorID, key)
		if e != nil {
			return e
		}
		if found {
			if record.RequestHash != hash {
				return ErrPlatformTenantConflict
			}
			out = record.Receipt
			out.Replay = true
			return nil
		}
		current, e := tx.LockTenant(c, in.TenantID)
		if e != nil {
			return e
		}
		if current.Version != in.ExpectedVersion {
			return ErrPlatformTenantConflict
		}
		state := "SUSPENDED"
		if in.Operation == "SUSPEND" {
			if current.State != "ACTIVE" {
				return ErrPlatformTenantConflict
			}
		} else {
			if current.State != "SUSPENDED" {
				return ErrPlatformTenantConflict
			}
			ready, e := tx.ReadyForReactivation(c, in.TenantID)
			if e != nil {
				return e
			}
			if !ready {
				return ErrPlatformTenantConflict
			}
			state = "ACTIVE"
		}
		out = PlatformTenantReceipt{TenantID: in.TenantID, State: state, Version: current.Version + 1}
		return tx.Apply(c, in, out, key, hash)
	})
	if err != nil {
		out = PlatformTenantReceipt{}
	}
	return out, err
}
