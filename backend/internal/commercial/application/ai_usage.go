package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrAIUsageRequestInvalid = errors.New("commercial_ai_usage_request_invalid")
var ErrAIUsageUnavailable = errors.New("commercial_ai_usage_unavailable")
var ErrAIUsageLimit = errors.New("commercial_ai_usage_limit")
var ErrAIUsageConflict = errors.New("commercial_ai_usage_conflict")
var ErrAIUsageAccessUnavailable = errors.New("commercial_ai_usage_access_unavailable")
var ErrAIUsageSubscriptionUnavailable = errors.New("commercial_ai_usage_subscription_unavailable")

type AIUsageReserveInput struct {
	CheckpointID, TenantID, RequesterUserID, SourceOperationID, CorrelationID domain.ID
	Kind                                                                      domain.AIUsageKind
	AssignmentGeneration, EstimatedCredits                                    int64
}
type AIUsageCompleteInput struct {
	ReservationID, ExecutionID, CorrelationID domain.ID
	ActualCredits                             int64
}
type AIUsageReleaseInput struct{ ReservationID, ExecutionID, CorrelationID domain.ID }
type AIUsageReservationCheck struct {
	ReservationID, CheckpointID, TenantID, RequesterUserID domain.ID
	Kind                                                   domain.AIUsageKind
	AssignmentGeneration, EstimatedCredits                 int64
}
type AIUsageResult struct {
	Reservation domain.AIUsageReservation
	Counters    domain.AIUsageCounters
	Replay      bool
}
type AIUsageCurrent struct {
	SubscriptionID, UsagePeriodID domain.ID
	State, LastReconciledState    domain.SubscriptionState
	Counters                      domain.AIUsageCounters
}
type AIUsageStored struct {
	Reservation                domain.AIUsageReservation
	Counters                   domain.AIUsageCounters
	OperationHash, RequestHash [32]byte
}
type AIUsageCompletionEvidence struct {
	ExecutionID   domain.ID
	Status        domain.AIReservationStatus
	ActualCredits int64
	OccurredAt    time.Time
}
type AIUsageCommit struct {
	TransitionID, AuditID, OutboxID, CorrelationID, ExecutionID domain.ID
	Before, After                                               AIUsageStored
	Operation, EventType                                        string
	OccurredAt                                                  time.Time
}

type AIUsageTx interface {
	LockCurrent(context.Context, domain.ID) (AIUsageCurrent, bool, error)
	FindReservation(context.Context, domain.ID, domain.ID) (AIUsageStored, bool, error)
	InsertReservation(context.Context, AIUsageStored, domain.ID, domain.ID, domain.ID, time.Time) error
	LockReservation(context.Context, domain.ID) (AIUsageStored, bool, error)
	FindReservationCompletion(context.Context, domain.ID) (AIUsageCompletionEvidence, bool, error)
	CompleteReservation(context.Context, AIUsageCommit) error
	AppendAIUsageEvidence(context.Context, AIUsageCommit) error
}
type AIUsageUnitOfWork interface {
	WithinAIUsage(context.Context, func(context.Context, AIUsageTx) error) error
}

type AIUsageService struct {
	uow   AIUsageUnitOfWork
	ids   TrialIDGenerator
	clock TrialClock
}

func NewAIUsageService(uow AIUsageUnitOfWork, ids TrialIDGenerator, clock TrialClock) (*AIUsageService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_ai_usage_service_invalid")
	}
	return &AIUsageService{uow, ids, clock}, nil
}
func (s *AIUsageService) Reserve(ctx context.Context, input AIUsageReserveInput) (result AIUsageResult, err error) {
	if ctx == nil || input.CheckpointID.IsZero() || input.TenantID.IsZero() || input.RequesterUserID.IsZero() || input.SourceOperationID.IsZero() || input.CorrelationID.IsZero() || input.AssignmentGeneration < 1 || !domain.ValidAIUsageKind(input.Kind) || input.EstimatedCredits < 1 {
		return result, ErrAIUsageRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrAIUsageUnavailable
	}
	err = s.uow.WithinAIUsage(ctx, func(txCtx context.Context, tx AIUsageTx) error {
		var reserveErr error
		result, reserveErr = ReserveAIUsageWithinTransaction(txCtx, tx, input, s.ids, at)
		return reserveErr
	})
	if err != nil {
		return AIUsageResult{}, err
	}
	return result, nil
}
func ReserveAIUsageWithinTransaction(ctx context.Context, tx AIUsageTx, input AIUsageReserveInput, generator TrialIDGenerator, at time.Time) (result AIUsageResult, err error) {
	if ctx == nil || tx == nil || generator == nil || at.IsZero() || input.CheckpointID.IsZero() || input.TenantID.IsZero() || input.RequesterUserID.IsZero() || input.SourceOperationID.IsZero() || input.CorrelationID.IsZero() || input.AssignmentGeneration < 1 || !domain.ValidAIUsageKind(input.Kind) || input.EstimatedCredits < 1 {
		return result, ErrAIUsageRequestInvalid
	}
	operationHash := sha256.Sum256(append([]byte("commercial-ai-operation:v1:"), input.SourceOperationID[:]...))
	encoded, _ := json.Marshal(struct {
		CheckpointID, TenantID, RequesterUserID, SourceOperationID domain.ID
		Kind                                                       domain.AIUsageKind
		AssignmentGeneration, EstimatedCredits                     int64
	}{input.CheckpointID, input.TenantID, input.RequesterUserID, input.SourceOperationID, input.Kind, input.AssignmentGeneration, input.EstimatedCredits})
	requestHash := sha256.Sum256(append([]byte("commercial-ai-reserve:v1:"), encoded...))
	existing, found, err := tx.FindReservation(ctx, input.TenantID, input.SourceOperationID)
	if err != nil {
		return result, err
	}
	if found {
		if existing.RequestHash != requestHash {
			return result, ErrAIUsageConflict
		}
		return AIUsageResult{Reservation: existing.Reservation, Counters: existing.Counters, Replay: true}, nil
	}
	current, found, err := tx.LockCurrent(ctx, input.TenantID)
	if err != nil {
		return result, err
	}
	if !found {
		return result, ErrAIUsageSubscriptionUnavailable
	}
	existing, found, err = tx.FindReservation(ctx, input.TenantID, input.SourceOperationID)
	if err != nil {
		return result, err
	}
	if found {
		if existing.RequestHash != requestHash {
			return result, ErrAIUsageConflict
		}
		return AIUsageResult{Reservation: existing.Reservation, Counters: existing.Counters, Replay: true}, nil
	}
	after, err := domain.ReserveAIUsage(current.State, current.LastReconciledState, current.Counters, input.EstimatedCredits, at)
	if err != nil {
		return result, mapAIUsageDomain(err)
	}
	ids, err := trialIDs(generator, 3)
	if err != nil {
		return result, err
	}
	reservation := domain.AIUsageReservation{ID: ids[0], CheckpointID: input.CheckpointID, TenantID: input.TenantID, SubscriptionID: current.SubscriptionID, UsagePeriodID: current.UsagePeriodID, SourceOperationID: input.SourceOperationID, RequesterUserID: input.RequesterUserID, Kind: input.Kind, AssignmentGeneration: input.AssignmentGeneration, EstimatedCredits: input.EstimatedCredits, Status: domain.AIReserved, Version: 1, ReservedAt: at}
	stored := AIUsageStored{Reservation: reservation, Counters: after, OperationHash: operationHash, RequestHash: requestHash}
	if err = tx.InsertReservation(ctx, stored, ids[1], ids[2], input.CorrelationID, at); err != nil {
		return result, err
	}
	return AIUsageResult{Reservation: reservation, Counters: after}, nil
}

func (s *AIUsageService) VerifyReserved(ctx context.Context, input AIUsageReservationCheck) error {
	if s == nil || ctx == nil || input.ReservationID.IsZero() || input.CheckpointID.IsZero() || input.TenantID.IsZero() || input.RequesterUserID.IsZero() || input.AssignmentGeneration < 1 || input.EstimatedCredits < 1 || !domain.ValidAIUsageKind(input.Kind) {
		return ErrAIUsageRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return ErrAIUsageUnavailable
	}
	return s.uow.WithinAIUsage(ctx, func(txCtx context.Context, tx AIUsageTx) error {
		stored, found, err := tx.LockReservation(txCtx, input.ReservationID)
		if err != nil {
			return err
		}
		reservation := stored.Reservation
		if !found || reservation.Status != domain.AIReserved || reservation.Version != 1 || reservation.ID != input.ReservationID || reservation.CheckpointID != input.CheckpointID || reservation.TenantID != input.TenantID || reservation.RequesterUserID != input.RequesterUserID || reservation.Kind != input.Kind || reservation.AssignmentGeneration != input.AssignmentGeneration || reservation.EstimatedCredits != input.EstimatedCredits {
			return ErrAIUsageConflict
		}
		current, accessible, err := tx.LockCurrent(txCtx, input.TenantID)
		if err != nil {
			return err
		}
		if !accessible || current.SubscriptionID != reservation.SubscriptionID || current.UsagePeriodID != reservation.UsagePeriodID || !domain.AIUsageAccessAllowed(current.State, current.LastReconciledState, current.Counters, at) {
			return ErrAIUsageAccessUnavailable
		}
		return nil
	})
}

func (s *AIUsageService) Settle(ctx context.Context, input AIUsageCompleteInput) (AIUsageResult, error) {
	if input.ActualCredits < 1 {
		return AIUsageResult{}, ErrAIUsageRequestInvalid
	}
	return s.complete(ctx, input.ReservationID, input.ExecutionID, input.CorrelationID, input.ActualCredits, false)
}
func (s *AIUsageService) Release(ctx context.Context, input AIUsageReleaseInput) (AIUsageResult, error) {
	return s.complete(ctx, input.ReservationID, input.ExecutionID, input.CorrelationID, 0, true)
}
func (s *AIUsageService) complete(ctx context.Context, reservationID, executionID, correlationID domain.ID, actual int64, release bool) (result AIUsageResult, err error) {
	if ctx == nil || reservationID.IsZero() || executionID.IsZero() || correlationID.IsZero() {
		return result, ErrAIUsageRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrAIUsageUnavailable
	}
	operation, event := "SETTLE", "commercial.ai_usage.settled.v1"
	if release {
		operation, event = "RELEASE", "commercial.ai_usage.released.v1"
	}
	err = s.uow.WithinAIUsage(ctx, func(txCtx context.Context, tx AIUsageTx) error {
		before, found, e := tx.LockReservation(txCtx, reservationID)
		if e != nil {
			return e
		}
		if !found {
			return ErrAIUsageUnavailable
		}
		if before.Reservation.Status != domain.AIReserved {
			if (release && before.Reservation.Status != domain.AIReleased) || (!release && (before.Reservation.Status != domain.AISettled || before.Reservation.ActualCredits != actual)) {
				return ErrAIUsageConflict
			}
			evidence, evidenceFound, evidenceErr := tx.FindReservationCompletion(txCtx, reservationID)
			if evidenceErr != nil {
				return evidenceErr
			}
			expectedStatus := domain.AISettled
			if release {
				expectedStatus = domain.AIReleased
			}
			if !evidenceFound || evidence.ExecutionID != executionID || evidence.Status != expectedStatus || evidence.ActualCredits != actual {
				return ErrAIUsageConflict
			}
			result = AIUsageResult{Reservation: before.Reservation, Counters: before.Counters, Replay: true}
			return nil
		}
		after := before
		if release {
			after.Counters, after.Reservation, e = domain.ReleaseAIUsage(before.Counters, before.Reservation, at)
		} else {
			after.Counters, after.Reservation, e = domain.SettleAIUsage(before.Counters, before.Reservation, actual, at)
		}
		if e != nil {
			return mapAIUsageDomain(e)
		}
		ids, e := trialIDs(s.ids, 3)
		if e != nil {
			return e
		}
		commit := AIUsageCommit{TransitionID: ids[0], AuditID: ids[1], OutboxID: ids[2], CorrelationID: correlationID, ExecutionID: executionID, Before: before, After: after, Operation: operation, EventType: event, OccurredAt: at}
		if e = tx.CompleteReservation(txCtx, commit); e != nil {
			return e
		}
		if e = tx.AppendAIUsageEvidence(txCtx, commit); e != nil {
			return e
		}
		result = AIUsageResult{Reservation: after.Reservation, Counters: after.Counters}
		return nil
	})
	if err != nil {
		return AIUsageResult{}, err
	}
	return result, nil
}
func mapAIUsageDomain(err error) error {
	switch {
	case errors.Is(err, domain.ErrAIUsageLimitExceeded):
		return ErrAIUsageLimit
	case errors.Is(err, domain.ErrAIUsageConflict):
		return ErrAIUsageConflict
	case errors.Is(err, domain.ErrAIUsageAccessUnavailable):
		return ErrAIUsageAccessUnavailable
	default:
		return ErrAIUsageUnavailable
	}
}
