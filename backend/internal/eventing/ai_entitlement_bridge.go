// Package eventing provides the bounded tenant-to-control AI entitlement bridge.
package eventing

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	eventsql "bidora/backend/internal/eventing/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const ReservationSubject = "bidora.v1.tenant.ai_entitlement.reserve_requested"
const ReservationStream = "BIDORA_TENANT_AI_ENTITLEMENT"
const ReservationDurable = "control-commercial-ai-reservation-v1"
const OutboxLease = 2 * time.Minute

var ErrInvalid = errors.New("ai_entitlement_bridge_invalid")
var ErrUnavailable = errors.New("ai_entitlement_bridge_unavailable")

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID() (ID, error) }
type transactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}

type Envelope struct {
	EventID          string          `json:"event_id"`
	ActorID          string          `json:"actor_id"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int64           `json:"aggregate_version"`
	EventType        string          `json:"event_type"`
	SchemaVersion    int32           `json:"schema_version"`
	Payload          json.RawMessage `json:"payload"`
	OccurredAt       string          `json:"occurred_at"`
}
type Claimed struct {
	ID, LeaseID ID
	Attempts    int32
	Data        []byte
}

type Outbox struct{ runner transactionRunner }

func NewOutbox(r transactionRunner) (*Outbox, error) {
	if r == nil {
		return nil, ErrInvalid
	}
	return &Outbox{r}, nil
}
func (o *Outbox) Claim(ctx context.Context, lease ID, at time.Time) (Claimed, bool, error) {
	if ctx == nil || lease.IsZero() || at.IsZero() {
		return Claimed{}, false, ErrInvalid
	}
	var result Claimed
	err := o.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		row, e := eventsql.New(db).ClaimAIEntitlementOutbox(c, eventsql.ClaimAIEntitlementOutboxParams{LeaseID: dbID(lease), LeaseExpiresAt: dbTime(at.Add(OutboxLease)), ClaimedAt: dbTime(at)})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		if !row.ID.Valid || !row.ActorID.Valid || !row.AggregateID.Valid || !row.OccurredAt.Valid || row.Attempts < 1 || len(row.Payload) == 0 {
			return ErrInvalid
		}
		raw, e := json.Marshal(Envelope{EventID: uuid.UUID(row.ID.Bytes).String(), ActorID: uuid.UUID(row.ActorID.Bytes).String(), AggregateType: row.AggregateType, AggregateID: uuid.UUID(row.AggregateID.Bytes).String(), AggregateVersion: row.AggregateVersion, EventType: row.EventType, SchemaVersion: row.SchemaVersion, Payload: row.Payload, OccurredAt: row.OccurredAt.Time.UTC().Format(time.RFC3339Nano)})
		if e != nil {
			return e
		}
		result = Claimed{ID: ID(row.ID.Bytes), LeaseID: lease, Attempts: row.Attempts, Data: raw}
		return nil
	})
	if err != nil {
		return Claimed{}, false, err
	}
	return result, !result.ID.IsZero(), nil
}
func (o *Outbox) Published(ctx context.Context, c Claimed, at time.Time) error {
	return o.finish(ctx, c, at, true)
}
func (o *Outbox) Retry(ctx context.Context, c Claimed, at time.Time) error {
	return o.finish(ctx, c, at, false)
}
func (o *Outbox) finish(ctx context.Context, c Claimed, at time.Time, published bool) error {
	if ctx == nil || c.ID.IsZero() || c.LeaseID.IsZero() || c.Attempts < 1 || at.IsZero() {
		return ErrInvalid
	}
	return o.runner.WithinTransaction(ctx, func(x context.Context, db postgres.DBTX) error {
		q := eventsql.New(db)
		var n int64
		var e error
		if published {
			n, e = q.MarkAIEntitlementOutboxPublished(x, eventsql.MarkAIEntitlementOutboxPublishedParams{PublishedAt: dbTime(at), ID: dbID(c.ID), LeaseID: dbID(c.LeaseID), ExpectedAttempts: c.Attempts})
		} else if c.Attempts >= 9 {
			n, e = q.HoldAIEntitlementOutbox(x, eventsql.HoldAIEntitlementOutboxParams{ID: dbID(c.ID), LeaseID: dbID(c.LeaseID)})
		} else {
			delays := [...]time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour}
			delay := delays[c.Attempts-1]
			n, e = q.RetryAIEntitlementOutbox(x, eventsql.RetryAIEntitlementOutboxParams{NextAttemptAt: dbTime(at.Add(delay)), ID: dbID(c.ID), LeaseID: dbID(c.LeaseID), ExpectedAttempts: c.Attempts})
		}
		if e != nil {
			return e
		}
		if n != 1 {
			return ErrUnavailable
		}
		return nil
	})
}

type ReservationConsumer interface {
	Consume(context.Context, commercialapp.AIReservationRequestEvent) (commercialapp.AIReservationOutcome, error)
}

func DecodeReservation(raw []byte) (commercialapp.AIReservationRequestEvent, error) {
	var env Envelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&env) != nil || d.Decode(&struct{}{}) != io.EOF {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	eventID, e := uuid.Parse(env.EventID)
	if e != nil {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	actor, e := uuid.Parse(env.ActorID)
	if e != nil || actor == uuid.Nil {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	aggregate, e := uuid.Parse(env.AggregateID)
	if e != nil {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	occurred, e := time.Parse(time.RFC3339Nano, env.OccurredAt)
	if e != nil {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	var p struct {
		CheckpointID         string `json:"checkpoint_id"`
		TenantID             string `json:"tenant_id"`
		TargetID             string `json:"target_id"`
		SourceOperationID    string `json:"source_operation_id"`
		RequesterUserID      string `json:"requester_user_id"`
		CorrelationID        string `json:"correlation_id"`
		TargetType           string `json:"target_type"`
		UsageKind            string `json:"usage_kind"`
		RequestHash          string `json:"request_hash"`
		AssignmentGeneration int64  `json:"assignment_generation"`
		EstimatedCredits     int64  `json:"estimated_credits"`
	}
	d = json.NewDecoder(bytes.NewReader(env.Payload))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || d.Decode(&struct{}{}) != io.EOF {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	ids := make([]uuid.UUID, 6)
	for i, s := range []string{p.CheckpointID, p.TenantID, p.TargetID, p.SourceOperationID, p.RequesterUserID, p.CorrelationID} {
		ids[i], e = uuid.Parse(s)
		if e != nil || ids[i] == uuid.Nil {
			return commercialapp.AIReservationRequestEvent{}, ErrInvalid
		}
	}
	if actor != ids[4] || aggregate != ids[0] {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	hashBytes, e := hex.DecodeString(p.RequestHash)
	if e != nil || len(hashBytes) != 32 {
		return commercialapp.AIReservationRequestEvent{}, ErrInvalid
	}
	var hash [32]byte
	copy(hash[:], hashBytes)
	return commercialapp.AIReservationRequestEvent{EventID: domain.ID(eventID), AggregateID: domain.ID(aggregate), CheckpointID: domain.ID(ids[0]), TenantID: domain.ID(ids[1]), TargetID: domain.ID(ids[2]), SourceOperationID: domain.ID(ids[3]), RequesterUserID: domain.ID(ids[4]), CorrelationID: domain.ID(ids[5]), EventType: env.EventType, AggregateType: env.AggregateType, SchemaVersion: env.SchemaVersion, AggregateVersion: env.AggregateVersion, AssignmentGeneration: p.AssignmentGeneration, EstimatedCredits: p.EstimatedCredits, TargetType: p.TargetType, Kind: domain.AIUsageKind(p.UsageKind), RequestHash: hash, OccurredAt: occurred.UTC()}, nil
}
func dbID(id ID) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte(id), Valid: !id.IsZero()} }
func dbTime(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: v.UTC(), Valid: !v.IsZero()}
}
