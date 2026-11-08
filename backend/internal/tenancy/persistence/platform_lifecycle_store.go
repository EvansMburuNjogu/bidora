package persistence

import (
	platformapp "bidora/backend/internal/platformadmin/application"
	authority "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	app "bidora/backend/internal/tenancy/application"
	qsql "bidora/backend/internal/tenancy/persistence/sqlc"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type lifecycleRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type PlatformTenantLifecycleStore struct{ runner lifecycleRunner }

func NewPlatformTenantLifecycleStore(r lifecycleRunner) (*PlatformTenantLifecycleStore, error) {
	if r == nil {
		return nil, errors.New("tenant_lifecycle_store_invalid")
	}
	return &PlatformTenantLifecycleStore{r}, nil
}
func (s *PlatformTenantLifecycleStore) WithinLifecycle(ctx context.Context, use func(context.Context, app.PlatformTenantLifecycleTx) error) error {
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		a, e := authority.NewSettingsAuthorityTx(db)
		if e != nil {
			return e
		}
		return use(c, &lifecycleTx{qsql.New(db), a})
	})
}

type lifecycleTx struct {
	q         *qsql.Queries
	authority *authority.SettingsAuthorityTx
}

func (t *lifecycleTx) ResolveAuthority(c context.Context, s app.PlatformDirectorySubject) (platformapp.AuthorityFacts, error) {
	return t.authority.Resolve(c, platformapp.AuthorityQuery{UserID: platformapp.ID(s.ActorID), SessionID: platformapp.ID(s.SessionID), ObservedAt: s.ObservedAt})
}
func (t *lifecycleTx) LockCommand(c context.Context, key string) error {
	return t.q.LockPlatformTenantCommand(c, key)
}
func lifecycleUUID(id app.ID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
func (t *lifecycleTx) FindCommand(c context.Context, actor app.ID, key [32]byte) (app.PlatformTenantCommandRecord, bool, error) {
	r, e := t.q.GetPlatformTenantCommand(c, qsql.GetPlatformTenantCommandParams{ActorID: lifecycleUUID(actor), KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.PlatformTenantCommandRecord{}, false, nil
	}
	if e != nil {
		return app.PlatformTenantCommandRecord{}, false, e
	}
	if len(r.RequestHash) != 32 {
		return app.PlatformTenantCommandRecord{}, false, errors.New("tenant_command_corrupt")
	}
	var hash [32]byte
	copy(hash[:], r.RequestHash)
	return app.PlatformTenantCommandRecord{RequestHash: hash, Receipt: app.PlatformTenantReceipt{TenantID: app.ID(r.TenantID.Bytes), State: r.ResultState, Version: r.ResultVersion}}, true, nil
}
func (t *lifecycleTx) LockTenant(c context.Context, id app.ID) (app.PlatformTenantReceipt, error) {
	r, e := t.q.GetPlatformTenantForLifecycle(c, lifecycleUUID(id))
	if errors.Is(e, pgx.ErrNoRows) {
		return app.PlatformTenantReceipt{}, app.ErrPlatformTenantNotFound
	}
	if e != nil {
		return app.PlatformTenantReceipt{}, e
	}
	return app.PlatformTenantReceipt{TenantID: id, State: r.State, Version: r.Version}, nil
}
func (t *lifecycleTx) ReadyForReactivation(c context.Context, id app.ID) (bool, error) {
	return t.q.PlatformTenantReadyForReactivation(c, lifecycleUUID(id))
}
func (t *lifecycleTx) Apply(c context.Context, in app.PlatformTenantCommand, out app.PlatformTenantReceipt, key, hash [32]byte) error {
	at := pgtype.Timestamptz{Time: in.Subject.ObservedAt.UTC(), Valid: true}
	n, e := t.q.UpdatePlatformTenantLifecycle(c, qsql.UpdatePlatformTenantLifecycleParams{ID: lifecycleUUID(in.TenantID), State: out.State, At: at, ExpectedVersion: in.ExpectedVersion})
	if e != nil {
		return e
	}
	if n != 1 {
		return app.ErrPlatformTenantConflict
	}
	if e = t.q.InsertPlatformTenantCommand(c, qsql.InsertPlatformTenantCommandParams{ActorID: lifecycleUUID(in.Subject.ActorID), KeyHash: key[:], RequestHash: hash[:], TenantID: lifecycleUUID(in.TenantID), Operation: in.Operation, ResultState: out.State, ResultVersion: out.Version, Reason: in.Reason, At: at}); e != nil {
		return e
	}
	id, e := uuid.NewRandom()
	if e != nil {
		return e
	}
	summary, _ := json.Marshal(map[string]any{"reason": in.Reason, "state": out.State, "version": out.Version})
	action := "PlatformTenantSuspended"
	if in.Operation == "REACTIVATE" {
		action = "PlatformTenantReactivated"
	}
	return t.q.InsertPlatformTenantLifecycleAudit(c, qsql.InsertPlatformTenantLifecycleAuditParams{ID: pgtype.UUID{Bytes: id, Valid: true}, ActorID: lifecycleUUID(in.Subject.ActorID), Action: action, TenantID: lifecycleUUID(in.TenantID), At: at, CorrelationID: lifecycleUUID(in.CorrelationID), Summary: summary})
}
