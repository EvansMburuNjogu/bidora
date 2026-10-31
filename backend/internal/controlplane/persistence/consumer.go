package persistence

import (
	controlapp "bidora/backend/internal/controlplane/application"
	controlsql "bidora/backend/internal/controlplane/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
)

type ConsumerTx struct{ queries *controlsql.Queries }

func NewConsumerTx(db postgres.DBTX) (*ConsumerTx, error) {
	if db == nil {
		return nil, errors.New("control_consumer_tx_invalid")
	}
	return &ConsumerTx{controlsql.New(db)}, nil
}
func (t *ConsumerTx) InsertInbox(ctx context.Context, r controlapp.InboxReceipt) error {
	if ctx == nil || r.ID.IsZero() || r.EventID.IsZero() || r.AggregateID.IsZero() || r.TenantID.IsZero() || r.Consumer == "" || r.EventType == "" || r.AggregateType == "" || r.SchemaVersion < 1 || r.AggregateVersion < 1 || r.AssignmentGeneration < 1 || r.RequestHash == [32]byte{} || r.OccurredAt.IsZero() || r.ProcessedAt.IsZero() {
		return errors.New("control_inbox_receipt_invalid")
	}
	return t.queries.InsertControlInbox(ctx, controlsql.InsertControlInboxParams{ID: controlID(r.ID), Consumer: r.Consumer, EventID: controlID(r.EventID), EventType: r.EventType, SchemaVersion: r.SchemaVersion, AggregateType: r.AggregateType, AggregateID: controlID(r.AggregateID), AggregateVersion: r.AggregateVersion, TenantID: controlID(r.TenantID), AssignmentGeneration: r.AssignmentGeneration, RequestHash: r.RequestHash[:], OccurredAt: controlTime(r.OccurredAt), ProcessedAt: controlTime(r.ProcessedAt)})
}
