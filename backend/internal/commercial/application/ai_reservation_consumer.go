package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

const AIReservationConsumerName = "control.commercial.ai_reservation.v1"

var ErrAIReservationEventInvalid = errors.New("commercial_ai_reservation_event_invalid")
var ErrAIReservationEventConflict = errors.New("commercial_ai_reservation_event_conflict")
var ErrAIReservationConsumerUnavailable = errors.New("commercial_ai_reservation_consumer_unavailable")

type AIReservationRequestEvent struct {
	EventID, AggregateID, CheckpointID, TenantID, TargetID, SourceOperationID, RequesterUserID, CorrelationID domain.ID
	EventType, AggregateType, TargetType                                                                      string
	SchemaVersion                                                                                             int32
	AggregateVersion, AssignmentGeneration, EstimatedCredits                                                  int64
	Kind                                                                                                      domain.AIUsageKind
	RequestHash                                                                                               [32]byte
	OccurredAt                                                                                                time.Time
}
type AIReservationOutcome struct {
	EventID, CheckpointID, TenantID, SourceOperationID, RequesterUserID, ReservationID domain.ID
	RequestHash                                                                        [32]byte
	AssignmentGeneration                                                               int64
	Disposition, DenialCode                                                            string
	ProcessedAt                                                                        time.Time
	Replay                                                                             bool
}
type AIReservationConsumptionCommit struct {
	InboxID, OutcomeID, AuditID, OutboxID domain.ID
	Event                                 AIReservationRequestEvent
	Outcome                               AIReservationOutcome
	ProcessedAt                           time.Time
}
type AIReservationConsumerTx interface {
	AIUsageTx
	FindAIReservationOutcome(context.Context, domain.ID) (AIReservationOutcome, bool, error)
	FindAIReservationOutcomeByCheckpoint(context.Context, domain.ID) (AIReservationOutcome, bool, error)
	LockAIEntitlementAssignment(context.Context, domain.ID) (int64, bool, error)
	AIRequestCapabilityAllowed(context.Context, domain.ID, domain.ID, int64, string) (bool, error)
	RecordAIReservationConsumption(context.Context, AIReservationConsumptionCommit) error
}
type AIReservationConsumerUnitOfWork interface {
	WithinAIReservationConsumption(context.Context, func(context.Context, AIReservationConsumerTx) error) error
}
type AIReservationConsumer struct {
	uow   AIReservationConsumerUnitOfWork
	ids   TrialIDGenerator
	clock TrialClock
}

func NewAIReservationConsumer(uow AIReservationConsumerUnitOfWork, ids TrialIDGenerator, clock TrialClock) (*AIReservationConsumer, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrAIReservationEventInvalid
	}
	return &AIReservationConsumer{uow, ids, clock}, nil
}
func (s *AIReservationConsumer) Consume(ctx context.Context, event AIReservationRequestEvent) (result AIReservationOutcome, err error) {
	if ctx == nil || !validReservationEvent(event) {
		return result, ErrAIReservationEventInvalid
	}
	expectedHash := reservationEventHash(event)
	if expectedHash != event.RequestHash {
		return result, ErrAIReservationEventInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrAIReservationConsumerUnavailable
	}
	if at.Before(event.OccurredAt.UTC()) {
		return result, ErrAIReservationEventInvalid
	}
	err = s.uow.WithinAIReservationConsumption(ctx, func(txCtx context.Context, tx AIReservationConsumerTx) error {
		existing, found, e := tx.FindAIReservationOutcome(txCtx, event.EventID)
		if e != nil {
			return e
		}
		if found {
			if !sameReservationEvent(existing, event) {
				return ErrAIReservationEventConflict
			}
			existing.Replay = true
			result = existing
			return nil
		}
		generation, eligible, e := tx.LockAIEntitlementAssignment(txCtx, event.TenantID)
		if e != nil {
			return e
		}
		existing, found, e = tx.FindAIReservationOutcome(txCtx, event.EventID)
		if e != nil {
			return e
		}
		if found {
			if !sameReservationEvent(existing, event) {
				return ErrAIReservationEventConflict
			}
			existing.Replay = true
			result = existing
			return nil
		}
		existing, found, e = tx.FindAIReservationOutcomeByCheckpoint(txCtx, event.CheckpointID)
		if e != nil {
			return e
		}
		if found {
			return ErrAIReservationEventConflict
		}
		denial := ""
		if !eligible || generation != event.AssignmentGeneration {
			denial = "ACCESS_UNAVAILABLE"
		}
		capability := aiRequestCapability(event.TargetType)
		if denial == "" {
			allowed, e := tx.AIRequestCapabilityAllowed(txCtx, event.RequesterUserID, event.TenantID, event.AssignmentGeneration, capability)
			if e != nil {
				return e
			}
			if !allowed {
				denial = "ACCESS_UNAVAILABLE"
			}
		}
		outcome := AIReservationOutcome{EventID: event.EventID, CheckpointID: event.CheckpointID, TenantID: event.TenantID, SourceOperationID: event.SourceOperationID, RequesterUserID: event.RequesterUserID, RequestHash: event.RequestHash, AssignmentGeneration: event.AssignmentGeneration, ProcessedAt: at}
		if denial == "" {
			reserve, e := ReserveAIUsageWithinTransaction(txCtx, tx, AIUsageReserveInput{CheckpointID: event.CheckpointID, TenantID: event.TenantID, RequesterUserID: event.RequesterUserID, SourceOperationID: event.SourceOperationID, CorrelationID: event.CorrelationID, Kind: event.Kind, AssignmentGeneration: event.AssignmentGeneration, EstimatedCredits: event.EstimatedCredits}, s.ids, at)
			if e != nil {
				switch {
				case errors.Is(e, ErrAIUsageLimit):
					denial = "LIMIT_EXCEEDED"
				case errors.Is(e, ErrAIUsageAccessUnavailable):
					denial = "ACCESS_UNAVAILABLE"
				case errors.Is(e, ErrAIUsageSubscriptionUnavailable):
					denial = "SUBSCRIPTION_UNAVAILABLE"
				default:
					return e
				}
			} else {
				outcome.Disposition = "RESERVED"
				outcome.ReservationID = reserve.Reservation.ID
			}
		}
		idsNeeded := 2
		if denial != "" {
			outcome.Disposition = "DENIED"
			outcome.DenialCode = denial
			idsNeeded = 4
		}
		ids, e := trialIDs(s.ids, idsNeeded)
		if e != nil {
			return e
		}
		commit := AIReservationConsumptionCommit{InboxID: ids[0], OutcomeID: ids[1], Event: event, Outcome: outcome, ProcessedAt: at}
		if denial != "" {
			commit.AuditID, commit.OutboxID = ids[2], ids[3]
		}
		if e = tx.RecordAIReservationConsumption(txCtx, commit); e != nil {
			return e
		}
		result = outcome
		return nil
	})
	if err != nil {
		return AIReservationOutcome{}, mapReservationConsumerError(err)
	}
	return result, nil
}
func validReservationEvent(v AIReservationRequestEvent) bool {
	return !v.EventID.IsZero() && !v.AggregateID.IsZero() && v.AggregateID == v.CheckpointID && !v.CheckpointID.IsZero() && !v.TenantID.IsZero() && !v.TargetID.IsZero() && !v.SourceOperationID.IsZero() && !v.RequesterUserID.IsZero() && !v.CorrelationID.IsZero() && v.EventType == "tenant.ai_entitlement.reserve_requested.v1" && v.AggregateType == "ai_usage_checkpoint" && v.SchemaVersion == 1 && v.AggregateVersion == 1 && v.AssignmentGeneration > 0 && v.EstimatedCredits > 0 && validAIRequestTarget(v.TargetType) && domain.ValidAIUsageKind(v.Kind) && v.RequestHash != [32]byte{} && !v.OccurredAt.IsZero()
}
func validAIRequestTarget(v string) bool {
	return v == "TENDER_ANALYSIS" || v == "REQUIREMENT_REVIEW" || v == "RESPONSE_DRAFT" || v == "PROPOSAL_DRAFT"
}
func aiRequestCapability(target string) string {
	switch target {
	case "TENDER_ANALYSIS":
		return "tenderpreneur.opportunity.document.upload"
	case "REQUIREMENT_REVIEW":
		return "tenderpreneur.bid.requirement.manage"
	case "RESPONSE_DRAFT", "PROPOSAL_DRAFT":
		return "tenderpreneur.bid.response.edit"
	}
	return ""
}
func reservationEventHash(v AIReservationRequestEvent) [32]byte {
	encoded, _ := json.Marshal(struct {
		TenantID, TargetID, SourceOperationID, RequesterUserID domain.ID
		TargetType, UsageKind                                  string
		AssignmentGeneration, EstimatedCredits                 int64
	}{v.TenantID, v.TargetID, v.SourceOperationID, v.RequesterUserID, v.TargetType, string(v.Kind), v.AssignmentGeneration, v.EstimatedCredits})
	return sha256.Sum256(append([]byte("tenant-ai-entitlement:v1:"), encoded...))
}
func sameReservationEvent(o AIReservationOutcome, e AIReservationRequestEvent) bool {
	return o.EventID == e.EventID && o.CheckpointID == e.CheckpointID && o.TenantID == e.TenantID && o.SourceOperationID == e.SourceOperationID && o.RequesterUserID == e.RequesterUserID && o.AssignmentGeneration == e.AssignmentGeneration && o.RequestHash == e.RequestHash
}
func mapReservationConsumerError(err error) error {
	if errors.Is(err, ErrAIReservationEventInvalid) || errors.Is(err, ErrAIReservationEventConflict) {
		return err
	}
	if errors.Is(err, ErrAIUsageRequestInvalid) {
		return ErrAIReservationEventInvalid
	}
	if errors.Is(err, ErrAIUsageConflict) {
		return ErrAIReservationEventConflict
	}
	return ErrAIReservationConsumerUnavailable
}
