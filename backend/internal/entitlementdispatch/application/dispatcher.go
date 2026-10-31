// Package application routes fixed control AI-entitlement acknowledgements to
// server-selected tenant databases. Event routing facts never grant authority.
package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	entitlementapp "bidora/backend/internal/entitlementcheckpoint/application"
	tenancyapp "bidora/backend/internal/tenancy/application"
	"github.com/google/uuid"
)

var ErrDispatchInvalid = errors.New("ai_entitlement_acknowledgement_dispatch_invalid")
var ErrDispatchUnavailable = errors.New("ai_entitlement_acknowledgement_dispatch_unavailable")
var ErrDispatchHeld = errors.New("ai_entitlement_acknowledgement_dispatch_held")

const maximumDispatchClaims int32 = 9

var dispatchRetryDelays = [...]time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour}

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

type Result struct {
	Processed bool
	TenantID  ID
}

type Record struct {
	ID, TenantID, ActorID, AggregateID ID
	AggregateType, EventType           string
	AggregateVersion                   int64
	SchemaVersion                      int32
	Classification                     string
	Payload                            []byte
	OccurredAt                         time.Time
	Attempt                            int32
}

type HoldCategory string

type HoldReason string

const (
	HoldStaleAssignment   HoldCategory = "STALE_ASSIGNMENT"
	HoldIntegrityConflict HoldCategory = "INTEGRITY_CONFLICT"
	HoldPoisonEvent       HoldCategory = "POISON_EVENT"
	HoldDeliveryExhausted HoldCategory = "DELIVERY_EXHAUSTED"

	ReasonStaleAssignment              HoldReason = "STALE_ASSIGNMENT"
	ReasonGenerationAhead              HoldReason = "GENERATION_AHEAD_OF_REGISTRY"
	ReasonInvalidEnvelope              HoldReason = "INVALID_ENVELOPE"
	ReasonRegistryUnavailable          HoldReason = "REGISTRY_UNAVAILABLE"
	ReasonCurrentAssignmentUnavailable HoldReason = "CURRENT_ASSIGNMENT_TEMPORARILY_UNAVAILABLE"
	ReasonEndpointUnavailable          HoldReason = "ENDPOINT_UNAVAILABLE"
	ReasonTenantApplyUnavailable       HoldReason = "TENANT_APPLY_UNAVAILABLE"
)

type Hold struct {
	EventID, TenantID                   ID
	EnvelopeHash                        [32]byte
	Category                            HoldCategory
	Reason                              HoldReason
	EventGeneration, ObservedGeneration *int64
	ExpectedAttempt                     int32
	HeldAt                              time.Time
}

type Outbox interface {
	Claim(context.Context, ID, time.Time, time.Time) (Record, bool, error)
	MarkPublished(context.Context, ID, int32, time.Time) error
	Defer(context.Context, ID, int32, time.Time) error
	Hold(context.Context, Hold) error
}

type AssignmentResolver interface {
	AssessAIEntitlementAcknowledgementAssignment(context.Context, tenancyapp.ID, int64) (tenancyapp.AIEntitlementAcknowledgementRoutingAssessment, error)
}

type AcknowledgementApplier interface {
	Apply(context.Context, entitlementapp.Acknowledgement) (entitlementapp.Result, error)
}

type EndpointResolver interface {
	ResolveAIEntitlementAcknowledgementEndpoint(tenancyapp.Assignment) (AcknowledgementApplier, error)
}

type Clock interface{ Now() time.Time }

type Dispatcher struct {
	outbox      Outbox
	assignments AssignmentResolver
	endpoints   EndpointResolver
	clock       Clock
	observer    Observer
}

func NewDispatcher(outbox Outbox, assignments AssignmentResolver, endpoints EndpointResolver, clock Clock, observer Observer) (*Dispatcher, error) {
	if outbox == nil || assignments == nil || endpoints == nil || clock == nil || observer == nil {
		return nil, ErrDispatchInvalid
	}
	return &Dispatcher{outbox: outbox, assignments: assignments, endpoints: endpoints, clock: clock, observer: observer}, nil
}

func (d *Dispatcher) ProcessNext(ctx context.Context, previousTenant ID) (result Result, err error) {
	if d == nil || ctx == nil || d.outbox == nil || d.assignments == nil || d.endpoints == nil || d.clock == nil || d.observer == nil {
		return result, ErrDispatchInvalid
	}
	claimedAt := d.clock.Now().UTC()
	if claimedAt.IsZero() {
		return result, ErrDispatchUnavailable
	}
	record, found, err := d.outbox.Claim(ctx, previousTenant, claimedAt, claimedAt.Add(AcknowledgementClaimLease))
	if err != nil || !found {
		if err != nil {
			d.observer.Observe(Observation{Signal: "CLAIM", Result: "UNAVAILABLE"})
			return result, dispatchUnavailable(err)
		}
		d.observer.Observe(Observation{Signal: "CLAIM", Result: "EMPTY"})
		return result, nil
	}
	result = Result{Processed: true, TenantID: record.TenantID}
	d.observer.Observe(Observation{Signal: "CLAIM", Result: "CLAIMED"})
	acknowledgement, parseErr := parseRecord(record)
	if parseErr != nil {
		return result, d.hold(ctx, record, HoldPoisonEvent, ReasonInvalidEnvelope, nil, nil, parseErr)
	}
	eventGeneration := acknowledgement.AssignmentGeneration
	assessment, routeErr := d.assignments.AssessAIEntitlementAcknowledgementAssignment(ctx, tenancyapp.ID(acknowledgement.TenantID), eventGeneration)
	if routeErr != nil {
		return result, d.retryOrHold(ctx, record, ReasonRegistryUnavailable, &eventGeneration, nil, routeErr)
	}
	observedGeneration := assessment.CurrentGeneration
	switch assessment.Status {
	case tenancyapp.AIEntitlementStaleAssignment:
		return result, d.hold(ctx, record, HoldStaleAssignment, ReasonStaleAssignment, &eventGeneration, &observedGeneration, ErrDispatchHeld)
	case tenancyapp.AIEntitlementGenerationAheadOfRegistry:
		return result, d.hold(ctx, record, HoldIntegrityConflict, ReasonGenerationAhead, &eventGeneration, &observedGeneration, ErrDispatchHeld)
	case tenancyapp.AIEntitlementCurrentAssignmentUnavailable:
		return result, d.retryOrHold(ctx, record, ReasonCurrentAssignmentUnavailable, &eventGeneration, &observedGeneration, ErrDispatchUnavailable)
	case tenancyapp.AIEntitlementExactCurrentAssignment:
	default:
		return result, d.hold(ctx, record, HoldPoisonEvent, ReasonInvalidEnvelope, &eventGeneration, &observedGeneration, ErrDispatchInvalid)
	}
	endpoint, endpointErr := d.endpoints.ResolveAIEntitlementAcknowledgementEndpoint(assessment.Assignment)
	if endpointErr != nil || endpoint == nil {
		if endpointErr == nil {
			endpointErr = ErrDispatchUnavailable
		}
		return result, d.retryOrHold(ctx, record, ReasonEndpointUnavailable, &eventGeneration, &observedGeneration, endpointErr)
	}
	if _, applyErr := endpoint.Apply(ctx, acknowledgement); applyErr != nil {
		return result, d.retryOrHold(ctx, record, ReasonTenantApplyUnavailable, &eventGeneration, &observedGeneration, applyErr)
	}
	publishedAt := d.clock.Now().UTC()
	if publishedAt.IsZero() || publishedAt.Before(record.OccurredAt.UTC()) {
		return result, ErrDispatchUnavailable
	}
	if err = d.outbox.MarkPublished(ctx, record.ID, record.Attempt, publishedAt); err != nil {
		return result, dispatchUnavailable(err)
	}
	d.observer.Observe(Observation{Signal: "DISPOSITION", Result: "PUBLISHED"})
	return result, nil
}

func (d *Dispatcher) retryOrHold(ctx context.Context, record Record, reason HoldReason, eventGeneration, observedGeneration *int64, cause error) error {
	if ctx.Err() != nil {
		return dispatchUnavailable(errors.Join(cause, ctx.Err()))
	}
	if record.Attempt >= maximumDispatchClaims {
		return d.hold(ctx, record, HoldDeliveryExhausted, reason, eventGeneration, observedGeneration, cause)
	}
	if record.Attempt < 1 || int(record.Attempt) > len(dispatchRetryDelays) {
		return d.hold(ctx, record, HoldPoisonEvent, ReasonInvalidEnvelope, eventGeneration, observedGeneration, ErrDispatchInvalid)
	}
	retryAt := d.clock.Now().UTC()
	if retryAt.IsZero() {
		return dispatchUnavailable(cause)
	}
	if err := d.outbox.Defer(ctx, record.ID, record.Attempt, retryAt.Add(dispatchRetryDelays[record.Attempt-1])); err != nil {
		return dispatchUnavailable(errors.Join(cause, err))
	}
	d.observer.Observe(Observation{Signal: "DISPOSITION", Result: "DEFERRED_" + string(reason)})
	return dispatchUnavailable(cause)
}

func (d *Dispatcher) hold(ctx context.Context, record Record, category HoldCategory, reason HoldReason, eventGeneration, observedGeneration *int64, cause error) error {
	if ctx.Err() != nil {
		return dispatchUnavailable(errors.Join(cause, ctx.Err()))
	}
	heldAt := d.clock.Now().UTC()
	if heldAt.IsZero() || heldAt.Before(record.OccurredAt.UTC()) {
		return dispatchUnavailable(cause)
	}
	hold := Hold{EventID: record.ID, TenantID: record.TenantID, EnvelopeHash: fingerprintRecord(record), Category: category, Reason: reason, EventGeneration: eventGeneration, ObservedGeneration: observedGeneration, ExpectedAttempt: record.Attempt, HeldAt: heldAt}
	if err := d.outbox.Hold(ctx, hold); err != nil {
		return dispatchUnavailable(errors.Join(cause, err))
	}
	d.observer.Observe(Observation{Signal: "DISPOSITION", Result: "HELD_" + string(category)})
	return ErrDispatchHeld
}

func fingerprintRecord(record Record) [32]byte {
	envelope := struct {
		ID, TenantID, ActorID, AggregateID       ID
		AggregateType, EventType, Classification string
		AggregateVersion                         int64
		SchemaVersion                            int32
		Payload                                  []byte
		OccurredAt                               string
	}{record.ID, record.TenantID, record.ActorID, record.AggregateID, record.AggregateType, record.EventType, record.Classification, record.AggregateVersion, record.SchemaVersion, record.Payload, record.OccurredAt.UTC().Format(time.RFC3339Nano)}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return sha256.Sum256([]byte("BIDORA_AI_ACKNOWLEDGEMENT_ENVELOPE_V1_INVALID"))
	}
	return sha256.Sum256(append([]byte("BIDORA_AI_ACKNOWLEDGEMENT_ENVELOPE_V1\x00"), encoded...))
}

func parseRecord(record Record) (entitlementapp.Acknowledgement, error) {
	if record.ID.IsZero() || record.TenantID.IsZero() || record.ActorID.IsZero() || record.AggregateID.IsZero() || record.AggregateVersion != 1 || record.SchemaVersion != 1 || record.Classification != "INTERNAL" || record.Attempt < 1 || record.OccurredAt.IsZero() || len(record.Payload) == 0 || len(record.Payload) > 4096 {
		return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
	}
	switch record.EventType {
	case "commercial.ai_usage.reserved.v1":
		if record.AggregateType != "commercial_ai_usage_reservation" {
			return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
		}
		var payload reservedPayload
		if err := decodeStrict(record.Payload, &payload); err != nil || payload.Status != "RESERVED" || payload.ExecutionID != "" || payload.ActualCredits != 0 || payload.UsageVersion < 2 || payload.EstimatedCredits < 1 || !validUsageKind(payload.Kind) {
			return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
		}
		checkpointID, tenantID, reservationID, correlationID, err := parseIDs(payload.CheckpointID, payload.TenantID, payload.ReservationID, payload.CorrelationID)
		if err != nil || ID(tenantID) != record.TenantID || ID(reservationID) != record.AggregateID || payload.AssignmentGeneration < 1 {
			return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
		}
		return entitlementapp.Acknowledgement{CheckpointID: checkpointID, TenantID: tenantID, EventID: entitlementapp.ID(record.ID), AggregateID: reservationID, ReservationID: reservationID, CorrelationID: correlationID, Disposition: entitlementapp.DispositionReserved, EventType: record.EventType, AggregateVersion: record.AggregateVersion, AssignmentGeneration: payload.AssignmentGeneration, EventOccurredAt: record.OccurredAt.UTC()}, nil
	case "commercial.ai_usage.denied.v1":
		if record.AggregateType != "ai_usage_checkpoint" {
			return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
		}
		var payload deniedPayload
		if err := decodeStrict(record.Payload, &payload); err != nil || payload.Disposition != "DENIED" || payload.EstimatedCredits < 1 || !validUsageKind(payload.Kind) || !validDenial(payload.DenialCode) {
			return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
		}
		checkpointID, tenantID, correlationID, err := parseThreeIDs(payload.CheckpointID, payload.TenantID, payload.CorrelationID)
		if err != nil || ID(tenantID) != record.TenantID || ID(checkpointID) != record.AggregateID || payload.AssignmentGeneration < 1 {
			return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
		}
		return entitlementapp.Acknowledgement{CheckpointID: checkpointID, TenantID: tenantID, EventID: entitlementapp.ID(record.ID), AggregateID: checkpointID, CorrelationID: correlationID, Disposition: entitlementapp.DispositionDenied, DenialCode: payload.DenialCode, EventType: record.EventType, AggregateVersion: record.AggregateVersion, AssignmentGeneration: payload.AssignmentGeneration, EventOccurredAt: record.OccurredAt.UTC()}, nil
	default:
		return entitlementapp.Acknowledgement{}, ErrDispatchInvalid
	}
}

type reservedPayload struct {
	CheckpointID, TenantID, ReservationID, CorrelationID, ExecutionID, Kind, Status string
	AssignmentGeneration, EstimatedCredits, ActualCredits, UsageVersion             int64
}
type deniedPayload struct {
	CheckpointID, TenantID, CorrelationID, Disposition, DenialCode, Kind string
	AssignmentGeneration, EstimatedCredits                               int64
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrDispatchInvalid
	}
	return nil
}
func parseIDs(values ...string) (entitlementapp.ID, entitlementapp.ID, entitlementapp.ID, entitlementapp.ID, error) {
	if len(values) != 4 {
		return entitlementapp.ID{}, entitlementapp.ID{}, entitlementapp.ID{}, entitlementapp.ID{}, ErrDispatchInvalid
	}
	parsed := make([]entitlementapp.ID, 4)
	for index, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return entitlementapp.ID{}, entitlementapp.ID{}, entitlementapp.ID{}, entitlementapp.ID{}, ErrDispatchInvalid
		}
		parsed[index] = entitlementapp.ID(id)
	}
	return parsed[0], parsed[1], parsed[2], parsed[3], nil
}
func parseThreeIDs(values ...string) (entitlementapp.ID, entitlementapp.ID, entitlementapp.ID, error) {
	if len(values) != 3 {
		return entitlementapp.ID{}, entitlementapp.ID{}, entitlementapp.ID{}, ErrDispatchInvalid
	}
	parsed := make([]entitlementapp.ID, 3)
	for index, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return entitlementapp.ID{}, entitlementapp.ID{}, entitlementapp.ID{}, ErrDispatchInvalid
		}
		parsed[index] = entitlementapp.ID(id)
	}
	return parsed[0], parsed[1], parsed[2], nil
}
func validUsageKind(value string) bool {
	return value == "DOCUMENT_PDF" || value == "DOCUMENT_DOCX" || value == "GENERATION_TEXT"
}
func validDenial(value string) bool {
	return value == "LIMIT_EXCEEDED" || value == "ACCESS_UNAVAILABLE" || value == "SUBSCRIPTION_UNAVAILABLE"
}

type unavailableError struct{ cause error }

func (e *unavailableError) Error() string                 { return ErrDispatchUnavailable.Error() }
func (e *unavailableError) Unwrap() error                 { return e.cause }
func (e *unavailableError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
func dispatchUnavailable(cause error) error               { return &unavailableError{cause: cause} }
