// Package persistence owns tenant-local idempotency, audit and outbox writes.
package persistence

import (
	"bidora/backend/internal/postgres"
	sql "bidora/backend/internal/tenantevidence/persistence/sqlc"
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

type ID [16]byte
type Command struct {
	RequestHash     [32]byte
	ResponseRef     ID
	ResponseVersion int64
	ResponseStatus  int32
}
type Evidence struct {
	AuditID, OutboxID, ActorID, TargetID, CorrelationID ID
	Action, TargetType, AggregateType, EventType        string
	AggregateVersion                                    int64
	Summary, Payload                                    []byte
	At                                                  time.Time
}
type Tx struct{ q *sql.Queries }

func New(db postgres.DBTX) (*Tx, error) {
	if db == nil {
		return nil, errors.New("tenant_evidence_invalid")
	}
	return &Tx{sql.New(db)}, nil
}
func (t *Tx) GetCommand(ctx context.Context, actor ID, operation string, key, request [32]byte) (Command, bool, error) {
	row, err := t.q.GetTenantCommand(ctx, sql.GetTenantCommandParams{ActorID: id(actor), Operation: operation, KeyHash: key[:]})
	if errors.Is(err, pgx.ErrNoRows) {
		return Command{}, false, nil
	}
	if err != nil {
		return Command{}, false, err
	}
	if len(row.RequestHash) != 32 || !bytes.Equal(row.RequestHash, request[:]) || !row.ResponseRef.Valid || row.ResponseVersion == nil || row.ResponseStatus == nil {
		return Command{}, false, errors.New("tenant_command_conflict")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	return Command{hash, ID(row.ResponseRef.Bytes), *row.ResponseVersion, *row.ResponseStatus}, true, nil
}
func (t *Tx) Complete(ctx context.Context, idValue, actor ID, operation string, key, request [32]byte, status int32, response ID, version int64, at time.Time) error {
	return t.q.InsertCompletedTenantCommand(ctx, sql.InsertCompletedTenantCommandParams{ID: id(idValue), ActorID: id(actor), Operation: operation, KeyHash: key[:], RequestHash: request[:], ResponseStatus: &status, ResponseRef: id(response), ResponseVersion: &version, At: ts(at)})
}
func (t *Tx) Append(ctx context.Context, e Evidence) error {
	policy := int64(1)
	if err := t.q.InsertTenantAudit(ctx, sql.InsertTenantAuditParams{ID: id(e.AuditID), ActorID: id(e.ActorID), Action: e.Action, TargetType: e.TargetType, TargetID: id(e.TargetID), At: ts(e.At), CorrelationID: id(e.CorrelationID), ChangeSummary: e.Summary, PolicyVersion: &policy}); err != nil {
		return err
	}
	return t.q.InsertTenantOutbox(ctx, sql.InsertTenantOutboxParams{ID: id(e.OutboxID), AggregateType: e.AggregateType, AggregateID: id(e.TargetID), AggregateVersion: e.AggregateVersion, EventType: e.EventType, Payload: e.Payload, At: ts(e.At)})
}
func id(v ID) pgtype.UUID               { return pgtype.UUID{Bytes: v, Valid: v != ID{}} }
func ts(v time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: v.UTC(), Valid: !v.IsZero()} }
