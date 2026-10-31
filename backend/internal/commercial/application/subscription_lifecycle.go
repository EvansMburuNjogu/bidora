package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

const (
	CapabilitySubscriptionRead   = "tenderpreneur.organization.subscription.read"
	CapabilitySubscriptionManage = "tenderpreneur.organization.subscription.manage"
)

var ErrSubscriptionAccessDenied = errors.New("commercial_subscription_access_denied")
var ErrSubscriptionNotFound = errors.New("commercial_subscription_not_found")
var ErrSubscriptionLifecycleConflict = errors.New("commercial_subscription_lifecycle_conflict")
var ErrSubscriptionUnavailable = errors.New("commercial_subscription_unavailable")

type UsageSummary struct {
	BidWorkspaces, DocumentVersions, AICredits, AICreditsReserved, StorageBytes int64
	ConcurrentAIJobs                                                            int32
	Version                                                                     int64
}

type SubscriptionSummary struct {
	SubscriptionID domain.ID
	Subscription   domain.Subscription
	Usage          UsageSummary
}

type SubscriptionAccess struct {
	ActorID, TenantID domain.ID
	ContextGeneration int64
}

type SubscriptionLifecycleInput struct {
	SubscriptionAccess
	ExpectedVersion int64
	IdempotencyKey  string
	CorrelationID   domain.ID
}

type SubscriptionLifecycleRecord struct {
	RequestHash [32]byte
	Summary     SubscriptionSummary
}

type SubscriptionLifecycleCommit struct {
	CommandID, TransitionID, AuditID, OutboxID, CorrelationID, ActorID domain.ID
	Operation, EventType                                               string
	KeyHash, RequestHash                                               [32]byte
	Before, After                                                      SubscriptionSummary
	OccurredAt                                                         time.Time
}

type SubscriptionTx interface {
	Authorize(context.Context, SubscriptionAccess, string) (bool, error)
	Read(context.Context, domain.ID, bool) (SubscriptionSummary, bool, error)
	GetLifecycleCommand(context.Context, domain.ID, string, [32]byte) (SubscriptionLifecycleRecord, bool, error)
	CommitLifecycle(context.Context, SubscriptionLifecycleCommit) error
	AppendLifecycleEvidence(context.Context, SubscriptionLifecycleCommit) error
}
type SubscriptionUnitOfWork interface {
	WithinSubscription(context.Context, func(context.Context, SubscriptionTx) error) error
}

type SubscriptionLifecycleService struct {
	uow   SubscriptionUnitOfWork
	ids   TrialIDGenerator
	clock TrialClock
}

func NewSubscriptionLifecycleService(uow SubscriptionUnitOfWork, ids TrialIDGenerator, clock TrialClock) (*SubscriptionLifecycleService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_subscription_lifecycle_service_invalid")
	}
	return &SubscriptionLifecycleService{uow: uow, ids: ids, clock: clock}, nil
}

func (s *SubscriptionLifecycleService) Summary(ctx context.Context, access SubscriptionAccess) (result SubscriptionSummary, err error) {
	if invalidSubscriptionAccess(ctx, access) {
		return result, ErrSubscriptionAccessDenied
	}
	err = s.uow.WithinSubscription(ctx, func(txCtx context.Context, tx SubscriptionTx) error {
		allowed, e := tx.Authorize(txCtx, access, CapabilitySubscriptionRead)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrSubscriptionAccessDenied
		}
		result, allowed, e = tx.Read(txCtx, access.TenantID, false)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrSubscriptionNotFound
		}
		return nil
	})
	return result, err
}

func (s *SubscriptionLifecycleService) ScheduleCancellation(ctx context.Context, input SubscriptionLifecycleInput) (SubscriptionSummary, error) {
	return s.change(ctx, input, "SCHEDULE_CANCELLATION", "commercial.subscription.cancellation_scheduled.v1", func(v *domain.Subscription, _ time.Time) error { return v.ScheduleCancellation() })
}
func (s *SubscriptionLifecycleService) WithdrawCancellation(ctx context.Context, input SubscriptionLifecycleInput) (SubscriptionSummary, error) {
	return s.change(ctx, input, "WITHDRAW_CANCELLATION", "commercial.subscription.cancellation_withdrawn.v1", func(v *domain.Subscription, at time.Time) error { return v.WithdrawCancellation(at) })
}
func (s *SubscriptionLifecycleService) change(ctx context.Context, input SubscriptionLifecycleInput, operation, event string, mutate func(*domain.Subscription, time.Time) error) (result SubscriptionSummary, err error) {
	if invalidSubscriptionAccess(ctx, input.SubscriptionAccess) || input.ExpectedVersion < 1 || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 || input.CorrelationID.IsZero() {
		return result, ErrSubscriptionRequestInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrSubscriptionUnavailable
	}
	keyHash := sha256.Sum256([]byte("commercial-subscription:" + operation + ":" + input.IdempotencyKey))
	body, _ := json.Marshal(struct {
		ActorID, TenantID                  domain.ID
		ContextGeneration, ExpectedVersion int64
		Operation                          string
	}{input.ActorID, input.TenantID, input.ContextGeneration, input.ExpectedVersion, operation})
	requestHash := sha256.Sum256(append([]byte("commercial-subscription-lifecycle:v1:"), body...))
	err = s.uow.WithinSubscription(ctx, func(txCtx context.Context, tx SubscriptionTx) error {
		allowed, e := tx.Authorize(txCtx, input.SubscriptionAccess, CapabilitySubscriptionManage)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrSubscriptionAccessDenied
		}
		replay, found, e := tx.GetLifecycleCommand(txCtx, input.TenantID, operation, keyHash)
		if e != nil {
			return e
		}
		if found {
			if replay.RequestHash != requestHash {
				return ErrSubscriptionLifecycleConflict
			}
			result = replay.Summary
			return nil
		}
		before, found, e := tx.Read(txCtx, input.TenantID, true)
		if e != nil {
			return e
		}
		if !found {
			return ErrSubscriptionNotFound
		}
		if before.Subscription.Version != input.ExpectedVersion {
			return ErrSubscriptionLifecycleConflict
		}
		after := before
		if e = mutate(&after.Subscription, at); e != nil {
			return ErrSubscriptionLifecycleConflict
		}
		ids, e := trialIDs(s.ids, 4)
		if e != nil {
			return e
		}
		commit := SubscriptionLifecycleCommit{CommandID: ids[0], TransitionID: ids[1], AuditID: ids[2], OutboxID: ids[3], CorrelationID: input.CorrelationID, ActorID: input.ActorID, Operation: operation, EventType: event, KeyHash: keyHash, RequestHash: requestHash, Before: before, After: after, OccurredAt: at}
		if e = tx.CommitLifecycle(txCtx, commit); e != nil {
			return e
		}
		if e = tx.AppendLifecycleEvidence(txCtx, commit); e != nil {
			return e
		}
		result = after
		return nil
	})
	if err != nil {
		return SubscriptionSummary{}, err
	}
	return result, nil
}
func invalidSubscriptionAccess(ctx context.Context, access SubscriptionAccess) bool {
	return ctx == nil || access.ActorID.IsZero() || access.TenantID.IsZero() || access.ContextGeneration < 1
}
