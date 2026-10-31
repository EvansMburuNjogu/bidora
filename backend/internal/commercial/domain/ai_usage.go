package domain

import (
	"errors"
	"time"
)

type AIUsageKind string
type AIReservationStatus string

const (
	AIUsageDocumentPDF  AIUsageKind         = "DOCUMENT_PDF"
	AIUsageDocumentDOCX AIUsageKind         = "DOCUMENT_DOCX"
	AIUsageGeneration   AIUsageKind         = "GENERATION_TEXT"
	AIReserved          AIReservationStatus = "RESERVED"
	AISettled           AIReservationStatus = "SETTLED"
	AIReleased          AIReservationStatus = "RELEASED"
)

var ErrAIUsageInvalid = errors.New("commercial_ai_usage_invalid")
var ErrAIUsageLimitExceeded = errors.New("commercial_ai_usage_limit_exceeded")
var ErrAIUsageConflict = errors.New("commercial_ai_usage_conflict")
var ErrAIUsageAccessUnavailable = errors.New("commercial_ai_usage_access_unavailable")

type AIUsageCounters struct {
	CreditLimit, ConsumedCredits, ReservedCredits int64
	ConcurrentJobLimit, ConcurrentJobs            int32
	PeriodStart, PeriodEnd                        time.Time
	GraceEndsAt                                   *time.Time
	Version                                       int64
}
type AIUsageReservation struct {
	ID, CheckpointID, TenantID, SubscriptionID, UsagePeriodID, SourceOperationID, RequesterUserID ID
	Kind                                                                                          AIUsageKind
	AssignmentGeneration, EstimatedCredits, ActualCredits                                         int64
	Status                                                                                        AIReservationStatus
	Version                                                                                       int64
	ReservedAt                                                                                    time.Time
	CompletedAt                                                                                   *time.Time
}

func ReserveAIUsage(state, lastReconciled SubscriptionState, counters AIUsageCounters, credits int64, at time.Time) (AIUsageCounters, error) {
	at = at.UTC()
	if credits < 1 || at.IsZero() || counters.CreditLimit < 1 || counters.ConcurrentJobLimit < 1 || counters.ConsumedCredits < 0 || counters.ReservedCredits < 0 || counters.ConcurrentJobs < 0 || counters.Version < 1 || counters.PeriodStart.IsZero() || !counters.PeriodEnd.After(counters.PeriodStart) {
		return counters, ErrAIUsageInvalid
	}
	if at.Before(counters.PeriodStart) || !aiUsageAccess(state, lastReconciled, counters, at) {
		return counters, ErrAIUsageAccessUnavailable
	}
	if counters.ConsumedCredits > counters.CreditLimit || counters.ReservedCredits > counters.CreditLimit-counters.ConsumedCredits || credits > counters.CreditLimit-counters.ConsumedCredits-counters.ReservedCredits || counters.ConcurrentJobs >= counters.ConcurrentJobLimit {
		return counters, ErrAIUsageLimitExceeded
	}
	counters.ReservedCredits += credits
	counters.ConcurrentJobs++
	counters.Version++
	return counters, nil
}
func SettleAIUsage(counters AIUsageCounters, reservation AIUsageReservation, actual int64, at time.Time) (AIUsageCounters, AIUsageReservation, error) {
	at = at.UTC()
	if actual < 1 || at.IsZero() || reservation.Status != AIReserved || reservation.Version != 1 || reservation.EstimatedCredits < 1 || counters.Version < 1 || counters.ReservedCredits < reservation.EstimatedCredits || counters.ConcurrentJobs < 1 {
		return counters, reservation, ErrAIUsageConflict
	}
	remainingReserved := counters.ReservedCredits - reservation.EstimatedCredits
	if counters.ConsumedCredits > counters.CreditLimit || remainingReserved > counters.CreditLimit-counters.ConsumedCredits || actual > counters.CreditLimit-counters.ConsumedCredits-remainingReserved {
		return counters, reservation, ErrAIUsageLimitExceeded
	}
	counters.ReservedCredits = remainingReserved
	counters.ConsumedCredits += actual
	counters.ConcurrentJobs--
	counters.Version++
	reservation.ActualCredits = actual
	reservation.Status = AISettled
	reservation.Version = 2
	reservation.CompletedAt = &at
	return counters, reservation, nil
}
func ReleaseAIUsage(counters AIUsageCounters, reservation AIUsageReservation, at time.Time) (AIUsageCounters, AIUsageReservation, error) {
	at = at.UTC()
	if at.IsZero() || reservation.Status != AIReserved || reservation.Version != 1 || reservation.EstimatedCredits < 1 || counters.Version < 1 || counters.ReservedCredits < reservation.EstimatedCredits || counters.ConcurrentJobs < 1 {
		return counters, reservation, ErrAIUsageConflict
	}
	counters.ReservedCredits -= reservation.EstimatedCredits
	counters.ConcurrentJobs--
	counters.Version++
	reservation.Status = AIReleased
	reservation.Version = 2
	reservation.CompletedAt = &at
	return counters, reservation, nil
}
func AIUsageAccessAllowed(state, last SubscriptionState, counters AIUsageCounters, at time.Time) bool {
	return aiUsageAccess(state, last, counters, at.UTC())
}

func aiUsageAccess(state, last SubscriptionState, counters AIUsageCounters, at time.Time) bool {
	effective := state
	if state == ReviewRequired {
		effective = last
	}
	if effective == PastDue {
		return counters.GraceEndsAt != nil && at.Before(counters.GraceEndsAt.UTC())
	}
	return (effective == TrialActive || effective == Active) && at.Before(counters.PeriodEnd)
}
func ValidAIUsageKind(kind AIUsageKind) bool {
	return kind == AIUsageDocumentPDF || kind == AIUsageDocumentDOCX || kind == AIUsageGeneration
}
