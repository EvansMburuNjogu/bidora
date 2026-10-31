package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrTrialStartInvalid = errors.New("commercial_trial_start_invalid")
var ErrTrialStartIneligible = errors.New("commercial_trial_start_ineligible")
var ErrTrialStartConflict = errors.New("commercial_trial_start_conflict")
var ErrTrialStartUnavailable = errors.New("commercial_trial_start_unavailable")

type TrialStartInput struct {
	TenantID, InitiatingAdminID, ExecutionID, CorrelationID domain.ID
	IdempotencyKey                                          string
	ContextGeneration                                       int64
}

type TrialStartResult struct {
	SubscriptionID domain.ID
	State          domain.SubscriptionState
	Version        int64
	TrialEndsAt    time.Time
	Replay         bool
}

type TrialTenantFacts struct {
	Active, Tenderpreneur, ProvisioningComplete bool
}

type TrialCommandRecord struct {
	RequestHash     [32]byte
	SubscriptionID  domain.ID
	ResponseVersion int64
	State           domain.SubscriptionState
	TrialEndsAt     time.Time
}

type TrialStartCommit struct {
	CommandID, SubscriptionID, TermID, UsagePeriodID, AuditID, OutboxID domain.ID
	InitiatingAdminID, ExecutionID, CorrelationID                       domain.ID
	Subscription                                                        domain.Subscription
	KeyHash, RequestHash                                                [32]byte
	OccurredAt                                                          time.Time
}

type TrialStartTx interface {
	LockTenant(context.Context, domain.ID) (TrialTenantFacts, error)
	LockIdentity(context.Context, domain.ID) (bool, error)
	LockMembership(context.Context, domain.ID, domain.ID, int64) (domain.ID, bool, error)
	CapabilityAllowed(context.Context, domain.ID) (bool, error)
	GetCommand(context.Context, domain.ID, [32]byte) (TrialCommandRecord, bool, error)
	SubscriptionExists(context.Context, domain.ID) (bool, error)
	TrialMarkerExists(context.Context, domain.ID) (bool, error)
	InsertTrial(context.Context, TrialStartCommit) error
	AppendTrialEvidence(context.Context, TrialStartCommit) error
}

type TrialStartUnitOfWork interface {
	WithinTrialStart(context.Context, func(context.Context, TrialStartTx) error) error
}

type TrialIDGenerator interface{ NewID() (domain.ID, error) }
type TrialClock interface{ Now() time.Time }

type TrialStartService struct {
	uow   TrialStartUnitOfWork
	ids   TrialIDGenerator
	clock TrialClock
}

func NewTrialStartService(uow TrialStartUnitOfWork, ids TrialIDGenerator, clock TrialClock) (*TrialStartService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("commercial_trial_start_service_invalid")
	}
	return &TrialStartService{uow: uow, ids: ids, clock: clock}, nil
}

func (s *TrialStartService) Start(ctx context.Context, input TrialStartInput) (result TrialStartResult, err error) {
	if ctx == nil || input.TenantID.IsZero() || input.InitiatingAdminID.IsZero() || input.CorrelationID.IsZero() || input.ContextGeneration < 1 || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 128 {
		return result, ErrTrialStartInvalid
	}
	request, _ := json.Marshal(struct {
		TenantID, InitiatingAdminID domain.ID
		ContextGeneration           int64
	}{input.TenantID, input.InitiatingAdminID, input.ContextGeneration})
	keyHash := sha256.Sum256([]byte("commercial-start-trial:v1:" + input.IdempotencyKey))
	requestHash := sha256.Sum256(append([]byte("commercial-start-trial:v1:"), request...))
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return result, ErrTrialStartUnavailable
	}
	err = s.uow.WithinTrialStart(ctx, func(txCtx context.Context, tx TrialStartTx) error {
		tenant, txErr := tx.LockTenant(txCtx, input.TenantID)
		if txErr != nil {
			return txErr
		}
		if !tenant.Active || !tenant.Tenderpreneur || !tenant.ProvisioningComplete {
			return ErrTrialStartIneligible
		}
		identityEligible, txErr := tx.LockIdentity(txCtx, input.InitiatingAdminID)
		if txErr != nil {
			return txErr
		}
		if !identityEligible {
			return ErrTrialStartIneligible
		}
		membershipID, active, txErr := tx.LockMembership(txCtx, input.InitiatingAdminID, input.TenantID, input.ContextGeneration)
		if txErr != nil {
			return txErr
		}
		if !active || membershipID.IsZero() {
			return ErrTrialStartIneligible
		}
		allowed, txErr := tx.CapabilityAllowed(txCtx, membershipID)
		if txErr != nil {
			return txErr
		}
		if !allowed {
			return ErrTrialStartIneligible
		}
		replay, found, txErr := tx.GetCommand(txCtx, input.TenantID, keyHash)
		if txErr != nil {
			return txErr
		}
		if found {
			if replay.RequestHash != requestHash || replay.SubscriptionID.IsZero() || replay.ResponseVersion != 1 || replay.State != domain.TrialActive || replay.TrialEndsAt.IsZero() {
				return ErrTrialStartConflict
			}
			result = TrialStartResult{SubscriptionID: replay.SubscriptionID, State: replay.State, Version: replay.ResponseVersion, TrialEndsAt: replay.TrialEndsAt, Replay: true}
			return nil
		}
		exists, txErr := tx.SubscriptionExists(txCtx, input.TenantID)
		if txErr != nil {
			return txErr
		}
		marked, txErr := tx.TrialMarkerExists(txCtx, input.TenantID)
		if txErr != nil {
			return txErr
		}
		if exists || marked {
			return ErrTrialStartIneligible
		}
		subscription, txErr := domain.StartTrial(input.TenantID, domain.TrialEligibility{TenantActive: true, ProvisioningComplete: true, EffectiveOrganizationAdmin: true}, at)
		if txErr != nil {
			return ErrTrialStartIneligible
		}
		idCount := 6
		if input.ExecutionID.IsZero() {
			idCount = 7
		}
		ids, txErr := trialIDs(s.ids, idCount)
		if txErr != nil {
			return txErr
		}
		executionID := input.ExecutionID
		if executionID.IsZero() {
			executionID = ids[6]
		}
		commit := TrialStartCommit{CommandID: ids[0], SubscriptionID: ids[1], TermID: ids[2], UsagePeriodID: ids[3], AuditID: ids[4], OutboxID: ids[5], InitiatingAdminID: input.InitiatingAdminID, ExecutionID: executionID, CorrelationID: input.CorrelationID, Subscription: subscription, KeyHash: keyHash, RequestHash: requestHash, OccurredAt: at}
		if txErr = tx.InsertTrial(txCtx, commit); txErr != nil {
			return txErr
		}
		if txErr = tx.AppendTrialEvidence(txCtx, commit); txErr != nil {
			return txErr
		}
		result = TrialStartResult{SubscriptionID: commit.SubscriptionID, State: subscription.State, Version: subscription.Version, TrialEndsAt: *subscription.TrialEndsAt}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrTrialStartIneligible) || errors.Is(err, ErrTrialStartConflict) {
			return TrialStartResult{}, err
		}
		return TrialStartResult{}, ErrTrialStartUnavailable
	}
	return result, nil
}

func trialIDs(generator TrialIDGenerator, count int) ([]domain.ID, error) {
	ids := make([]domain.ID, count)
	for index := range ids {
		id, err := generator.NewID()
		if err != nil || id.IsZero() {
			return nil, ErrTrialStartUnavailable
		}
		ids[index] = id
	}
	return ids, nil
}
