package persistence

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	app "bidora/backend/internal/entitlementcheckpoint/application"
	entitlementsql "bidora/backend/internal/entitlementcheckpoint/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type transactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}
type Store struct{ runner transactionRunner }

func NewStore(runner transactionRunner) (*Store, error) {
	if runner == nil {
		return nil, errors.New("ai_entitlement_store_invalid")
	}
	return &Store{runner}, nil
}
func (s *Store) WithinCheckpoint(ctx context.Context, fn func(context.Context, app.Tx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("ai_entitlement_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error { return fn(txCtx, &tx{q: entitlementsql.New(db)}) })
}

type tx struct{ q *entitlementsql.Queries }

// BindTransaction exposes the Entitlement owner adapter to an outer
// same-database workflow transaction; callers receive only the application Tx.
func BindTransaction(db postgres.DBTX) (app.Tx, error) {
	if db == nil {
		return nil, errors.New("ai_entitlement_transaction_invalid")
	}
	return &tx{q: entitlementsql.New(db)}, nil
}

func (t *tx) FindBySource(ctx context.Context, id app.ID) (app.Checkpoint, bool, error) {
	row, err := t.q.GetAIEntitlementCheckpointBySource(ctx, dbID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Checkpoint{}, false, nil
	}
	if err != nil {
		return app.Checkpoint{}, false, err
	}
	value, err := mapCheckpoint(row)
	return value, err == nil, err
}
func (t *tx) Lock(ctx context.Context, id app.ID) (app.Checkpoint, bool, error) {
	row, err := t.q.LockAIEntitlementCheckpoint(ctx, dbID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Checkpoint{}, false, nil
	}
	if err != nil {
		return app.Checkpoint{}, false, err
	}
	value, err := mapCheckpoint(row)
	return value, err == nil, err
}
func (t *tx) InboxSeen(ctx context.Context, id app.ID) (bool, error) {
	return t.q.AIEntitlementInboxSeen(ctx, dbID(id))
}
func (t *tx) InsertRequest(ctx context.Context, c app.RequestCommit) error {
	in := c.Input
	if err := t.q.InsertAIEntitlementCheckpoint(ctx, entitlementsql.InsertAIEntitlementCheckpointParams{ID: dbID(c.CheckpointID), TenantID: dbID(in.TenantID), TargetType: in.TargetType, TargetID: dbID(in.TargetID), SourceOperationID: dbID(in.SourceOperationID), RequestHash: c.RequestHash[:], RequesterUserID: dbID(in.RequesterUserID), AssignmentGeneration: in.AssignmentGeneration, UsageKind: in.UsageKind, EstimatedCredits: in.EstimatedCredits, At: dbTime(c.OccurredAt)}); err != nil {
		return err
	}
	if err := t.q.InsertAIEntitlementAudit(ctx, entitlementsql.InsertAIEntitlementAuditParams{ID: dbID(c.AuditID), ActorID: dbID(in.RequesterUserID), Action: "AIEntitlementReservationRequested", CheckpointID: dbID(c.CheckpointID), OccurredAt: dbTime(c.OccurredAt), CorrelationID: dbID(in.CorrelationID), State: string(app.Pending), Version: 1}); err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		CheckpointID, TenantID, TargetID, SourceOperationID, RequesterUserID, CorrelationID string
		TargetType, UsageKind                                                               string
		AssignmentGeneration, EstimatedCredits                                              int64
		RequestHash                                                                         string
	}{uuid.UUID(c.CheckpointID).String(), uuid.UUID(in.TenantID).String(), uuid.UUID(in.TargetID).String(), uuid.UUID(in.SourceOperationID).String(), uuid.UUID(in.RequesterUserID).String(), uuid.UUID(in.CorrelationID).String(), in.TargetType, in.UsageKind, in.AssignmentGeneration, in.EstimatedCredits, hex.EncodeToString(c.RequestHash[:])})
	if err != nil {
		return err
	}
	return t.q.InsertAIEntitlementOutbox(ctx, entitlementsql.InsertAIEntitlementOutboxParams{ID: dbID(c.OutboxID), ActorID: dbID(in.RequesterUserID), CheckpointID: dbID(c.CheckpointID), CheckpointVersion: 1, EventType: "tenant.ai_entitlement.reserve_requested.v1", Payload: payload, OccurredAt: dbTime(c.OccurredAt)})
}
func (t *tx) ApplyAcknowledgement(ctx context.Context, c app.AckCommit) error {
	a := c.Acknowledgement
	if err := t.q.InsertAIEntitlementInbox(ctx, entitlementsql.InsertAIEntitlementInboxParams{ID: dbID(c.InboxID), EventID: dbID(a.EventID), EventType: a.EventType, AggregateID: dbID(a.AggregateID), AggregateVersion: a.AggregateVersion, EventOccurredAt: dbTime(a.EventOccurredAt), ProcessedAt: dbTime(c.ProcessedAt)}); err != nil {
		return err
	}
	var affected int64
	var err error
	var reservation pgtype.UUID
	var denial *string
	eventType, action := "tenant.ai_entitlement.reserved.v1", "AIEntitlementReserved"
	if a.Disposition == app.DispositionReserved {
		reservation = dbID(a.ReservationID)
		affected, err = t.q.AcknowledgeAIEntitlementReserved(ctx, entitlementsql.AcknowledgeAIEntitlementReservedParams{ReservationID: reservation, At: dbTime(c.ProcessedAt), ID: dbID(c.Before.ID)})
	} else {
		denial = &a.DenialCode
		eventType, action = "tenant.ai_entitlement.denied.v1", "AIEntitlementDenied"
		affected, err = t.q.AcknowledgeAIEntitlementDenied(ctx, entitlementsql.AcknowledgeAIEntitlementDeniedParams{DenialCode: denial, At: dbTime(c.ProcessedAt), ID: dbID(c.Before.ID)})
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		return app.ErrConflict
	}
	if err = t.q.InsertAIEntitlementTransition(ctx, entitlementsql.InsertAIEntitlementTransitionParams{ID: dbID(c.TransitionID), CheckpointID: dbID(c.Before.ID), ToState: string(c.After.State), SourceEventID: dbID(a.EventID), SourceEventType: a.EventType, SourceAggregateVersion: a.AggregateVersion, ReservationID: reservation, DenialCode: denial, OccurredAt: dbTime(c.ProcessedAt)}); err != nil {
		return err
	}
	if err = t.q.InsertAIEntitlementAudit(ctx, entitlementsql.InsertAIEntitlementAuditParams{ID: dbID(c.AuditID), ActorID: dbID(c.Before.RequesterUserID), Action: action, CheckpointID: dbID(c.Before.ID), OccurredAt: dbTime(c.ProcessedAt), CorrelationID: dbID(a.CorrelationID), State: string(c.After.State), Version: c.After.Version}); err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		CheckpointID, TargetID, State, ReservationID, DenialCode string
		Version                                                  int64
	}{uuid.UUID(c.Before.ID).String(), uuid.UUID(c.Before.TargetID).String(), string(c.After.State), optionalID(c.After.ReservationID), c.After.DenialCode, c.After.Version})
	if err != nil {
		return err
	}
	return t.q.InsertAIEntitlementOutbox(ctx, entitlementsql.InsertAIEntitlementOutboxParams{ID: dbID(c.OutboxID), ActorID: dbID(c.Before.RequesterUserID), CheckpointID: dbID(c.Before.ID), CheckpointVersion: c.After.Version, EventType: eventType, Payload: payload, OccurredAt: dbTime(c.ProcessedAt)})
}
func (t *tx) FindCompletionRequest(ctx context.Context, checkpointID app.ID) (app.CompletionRequest, bool, error) {
	row, err := t.q.GetAIEntitlementCompletionRequest(ctx, dbID(checkpointID))
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CompletionRequest{}, false, nil
	}
	if err != nil {
		return app.CompletionRequest{}, false, err
	}
	request, err := mapCompletionRequest(row.ID, row.CheckpointID, row.ReservationID, row.ExecutionID, row.Operation, row.ActualCredits, row.CorrelationID, row.RequestedAt)
	return request, err == nil, err
}
func (t *tx) FindPendingCompletion(ctx context.Context) (app.CompletionRequest, bool, error) {
	row, err := t.q.FindPendingAIEntitlementCompletion(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.CompletionRequest{}, false, nil
	}
	if err != nil {
		return app.CompletionRequest{}, false, err
	}
	request, err := mapCompletionRequest(row.ID, row.CheckpointID, row.ReservationID, row.ExecutionID, row.Operation, row.ActualCredits, row.CorrelationID, row.RequestedAt)
	if err != nil {
		return app.CompletionRequest{}, false, err
	}
	request.Checkpoint = app.Checkpoint{ID: app.ID(row.CheckpointID.Bytes), TenantID: app.ID(row.TenantID.Bytes), RequesterUserID: app.ID(row.RequesterUserID.Bytes), AssignmentGeneration: row.AssignmentGeneration, EstimatedCredits: row.EstimatedCredits, UsageKind: row.UsageKind, State: app.State(row.State), ReservationID: app.ID(row.ReservationID.Bytes), Version: row.Version}
	return request, true, nil
}
func (t *tx) InsertCompletionRequest(ctx context.Context, c app.CompletionRequestCommit) error {
	var actual *int64
	if c.Input.Operation == app.CompletionSettle {
		value := c.Input.ActualCredits
		actual = &value
	}
	if err := t.q.InsertAIEntitlementCompletionRequest(ctx, entitlementsql.InsertAIEntitlementCompletionRequestParams{ID: dbID(c.RequestID), CheckpointID: dbID(c.Input.CheckpointID), ReservationID: dbID(c.Input.ReservationID), ExecutionID: dbID(c.Input.ExecutionID), Operation: string(c.Input.Operation), ActualCredits: actual, CorrelationID: dbID(c.Input.CorrelationID), RequestedAt: dbTime(c.RequestedAt)}); err != nil {
		return err
	}
	if err := t.q.InsertAIEntitlementCompletionReconciliation(ctx, entitlementsql.InsertAIEntitlementCompletionReconciliationParams{RequestID: dbID(c.RequestID), UpdatedAt: dbTime(c.RequestedAt)}); err != nil {
		return err
	}
	affected, err := t.q.MarkAIEntitlementCompletionPending(ctx, entitlementsql.MarkAIEntitlementCompletionPendingParams{State: string(c.After.State), At: dbTime(c.RequestedAt), ID: dbID(c.Input.CheckpointID), ReservationID: dbID(c.Input.ReservationID)})
	if err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return app.ErrConflict
	}
	action, eventType := "AIEntitlementSettlementRequested", "tenant.ai_entitlement.settle_requested.v1"
	if c.Input.Operation == app.CompletionRelease {
		action, eventType = "AIEntitlementReleaseRequested", "tenant.ai_entitlement.release_requested.v1"
	}
	if err = t.q.InsertAIEntitlementAudit(ctx, entitlementsql.InsertAIEntitlementAuditParams{ID: dbID(c.AuditID), ActorID: dbID(c.Before.RequesterUserID), Action: action, CheckpointID: dbID(c.Before.ID), OccurredAt: dbTime(c.RequestedAt), CorrelationID: dbID(c.Input.CorrelationID), State: string(c.After.State), Version: c.After.Version}); err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		CheckpointID, ReservationID, ExecutionID, Operation string
		ActualCredits, AssignmentGeneration                 int64
	}{uuid.UUID(c.Before.ID).String(), uuid.UUID(c.Input.ReservationID).String(), uuid.UUID(c.Input.ExecutionID).String(), string(c.Input.Operation), c.Input.ActualCredits, c.Before.AssignmentGeneration})
	if err != nil {
		return err
	}
	return t.q.InsertAIEntitlementOutbox(ctx, entitlementsql.InsertAIEntitlementOutboxParams{ID: dbID(c.OutboxID), ActorID: dbID(c.Before.RequesterUserID), CheckpointID: dbID(c.Before.ID), CheckpointVersion: c.After.Version, EventType: eventType, Payload: payload, OccurredAt: dbTime(c.RequestedAt)})
}
func (t *tx) ApplyCompletionAcknowledgement(ctx context.Context, c app.CompletionAcknowledgementCommit) error {
	var actual *int64
	if c.Request.Operation == app.CompletionSettle {
		value := c.Request.ActualCredits
		actual = &value
	}
	if err := t.q.InsertAIEntitlementCompletionAcknowledgement(ctx, entitlementsql.InsertAIEntitlementCompletionAcknowledgementParams{ID: dbID(c.AcknowledgementID), RequestID: dbID(c.Request.ID), CheckpointID: dbID(c.Request.CheckpointID), ReservationID: dbID(c.Request.ReservationID), ExecutionID: dbID(c.Request.ExecutionID), Operation: string(c.Request.Operation), FinalState: string(c.After.State), ActualCredits: actual, AcknowledgedAt: dbTime(c.AcknowledgedAt)}); err != nil {
		return err
	}
	affected, err := t.q.MarkAIEntitlementCompletionFinal(ctx, entitlementsql.MarkAIEntitlementCompletionFinalParams{State: string(c.After.State), At: dbTime(c.AcknowledgedAt), ID: dbID(c.Request.CheckpointID), ExpectedState: string(c.Before.State), ReservationID: dbID(c.Request.ReservationID)})
	if err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return app.ErrConflict
	}
	action, eventType := "AIEntitlementSettled", "tenant.ai_entitlement.settled.v1"
	if c.Request.Operation == app.CompletionRelease {
		action, eventType = "AIEntitlementReleased", "tenant.ai_entitlement.released.v1"
	}
	if err = t.q.InsertAIEntitlementAudit(ctx, entitlementsql.InsertAIEntitlementAuditParams{ID: dbID(c.AuditID), ActorID: dbID(c.Before.RequesterUserID), Action: action, CheckpointID: dbID(c.Before.ID), OccurredAt: dbTime(c.AcknowledgedAt), CorrelationID: dbID(c.Request.CorrelationID), State: string(c.After.State), Version: c.After.Version}); err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		CheckpointID, ReservationID, ExecutionID, State string
		ActualCredits                                   int64
	}{uuid.UUID(c.Before.ID).String(), uuid.UUID(c.Request.ReservationID).String(), uuid.UUID(c.Request.ExecutionID).String(), string(c.After.State), c.Request.ActualCredits})
	if err != nil {
		return err
	}
	return t.q.InsertAIEntitlementOutbox(ctx, entitlementsql.InsertAIEntitlementOutboxParams{ID: dbID(c.OutboxID), ActorID: dbID(c.Before.RequesterUserID), CheckpointID: dbID(c.Before.ID), CheckpointVersion: c.After.Version, EventType: eventType, Payload: payload, OccurredAt: dbTime(c.AcknowledgedAt)})
}
func mapCompletionRequest(id, checkpointID, reservationID, executionID pgtype.UUID, operation string, actual *int64, correlationID pgtype.UUID, requestedAt pgtype.Timestamptz) (app.CompletionRequest, error) {
	credits := int64(0)
	if actual != nil {
		credits = *actual
	}
	request := app.CompletionRequest{ID: app.ID(id.Bytes), CheckpointID: app.ID(checkpointID.Bytes), ReservationID: app.ID(reservationID.Bytes), ExecutionID: app.ID(executionID.Bytes), CorrelationID: app.ID(correlationID.Bytes), Operation: app.CompletionOperation(operation), ActualCredits: credits, RequestedAt: requestedAt.Time.UTC()}
	if !requestedAt.Valid || request.ID.IsZero() || request.CheckpointID.IsZero() || request.ReservationID.IsZero() || request.ExecutionID.IsZero() || request.CorrelationID.IsZero() {
		return app.CompletionRequest{}, errors.New("ai_entitlement_completion_request_invalid")
	}
	return request, nil
}

func mapCheckpoint(row entitlementsql.BidoraEntitlementAiUsageCheckpoint) (app.Checkpoint, error) {
	if len(row.RequestHash) != 32 || !row.CreatedAt.Valid || !row.UpdatedAt.Valid {
		return app.Checkpoint{}, errors.New("ai_entitlement_checkpoint_row_invalid")
	}
	var hash [32]byte
	copy(hash[:], row.RequestHash)
	denial := ""
	if row.DenialCode != nil {
		denial = *row.DenialCode
	}
	return app.Checkpoint{ID: app.ID(row.ID.Bytes), TenantID: app.ID(row.TenantID.Bytes), TargetType: row.TargetType, TargetID: app.ID(row.TargetID.Bytes), SourceOperationID: app.ID(row.SourceOperationID.Bytes), RequestHash: hash, RequesterUserID: app.ID(row.RequesterUserID.Bytes), AssignmentGeneration: row.AssignmentGeneration, UsageKind: row.UsageKind, EstimatedCredits: row.EstimatedCredits, State: app.State(row.State), ReservationID: app.ID(row.ReservationID.Bytes), DenialCode: denial, Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC()}, nil
}
func dbID(id app.ID) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()} }
func dbTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
func optionalID(id app.ID) string {
	if id.IsZero() {
		return ""
	}
	return uuid.UUID(id).String()
}
