package application

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/commercial/domain"
)

var ErrSubscriptionRequestInvalid = errors.New("commercial_subscription_request_invalid")
var ErrSubscriptionVersionConflict = errors.New("commercial_subscription_version_conflict")
var ErrSubscriptionTransitionConflict = errors.New("commercial_subscription_transition_conflict")
var ErrSubscriptionTrialIneligible = errors.New("commercial_subscription_trial_ineligible")
var ErrSubscriptionReconciliationRequired = errors.New("commercial_subscription_reconciliation_required")

type SubscriptionPolicyService struct{}

func NewSubscriptionPolicyService() *SubscriptionPolicyService { return &SubscriptionPolicyService{} }

func (*SubscriptionPolicyService) StartTrial(ctx context.Context, tenantID domain.ID, eligibility domain.TrialEligibility, at time.Time) (domain.Subscription, error) {
	if ctx == nil {
		return domain.Subscription{}, ErrSubscriptionRequestInvalid
	}
	value, err := domain.StartTrial(tenantID, eligibility, at)
	return value, mapSubscriptionError(err)
}

func (*SubscriptionPolicyService) NewPaymentPending(ctx context.Context, tenantID domain.ID, plan domain.PlanKey, cadence domain.BillingCadence) (domain.Subscription, error) {
	if ctx == nil {
		return domain.Subscription{}, ErrSubscriptionRequestInvalid
	}
	value, err := domain.NewPaymentPending(tenantID, plan, cadence)
	return value, mapSubscriptionError(err)
}

func (*SubscriptionPolicyService) BeginPayment(ctx context.Context, current domain.Subscription, expectedVersion int64, plan domain.PlanKey, cadence domain.BillingCadence) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.BeginPayment(plan, cadence) })
}

func (*SubscriptionPolicyService) CancelPendingPayment(ctx context.Context, current domain.Subscription, expectedVersion int64) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.CancelPendingPayment() })
}

func (*SubscriptionPolicyService) NewPaid(ctx context.Context, tenantID domain.ID, term domain.TermSnapshot, reconciled bool, at time.Time) (domain.Subscription, error) {
	if ctx == nil {
		return domain.Subscription{}, ErrSubscriptionRequestInvalid
	}
	value, err := domain.NewPaidSubscription(tenantID, term, reconciled, at)
	return value, mapSubscriptionError(err)
}

func (*SubscriptionPolicyService) ActivatePaid(ctx context.Context, current domain.Subscription, expectedVersion int64, term domain.TermSnapshot, reconciled bool, at time.Time) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.ActivatePaid(term, reconciled, at) })
}

func (*SubscriptionPolicyService) MarkRenewalUnpaid(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.MarkRenewalUnpaid(at) })
}

func (*SubscriptionPolicyService) SuspendOverdue(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.SuspendOverdue(at) })
}

func (*SubscriptionPolicyService) ResumePaid(ctx context.Context, current domain.Subscription, expectedVersion int64, term domain.TermSnapshot, reconciled bool, at time.Time) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.ResumePaid(term, reconciled, at) })
}

func (*SubscriptionPolicyService) ScheduleCancellation(ctx context.Context, current domain.Subscription, expectedVersion int64) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.ScheduleCancellation() })
}

func (*SubscriptionPolicyService) WithdrawCancellation(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.WithdrawCancellation(at) })
}

func (*SubscriptionPolicyService) EndAtBoundary(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.EndAtBoundary(at) })
}

func (*SubscriptionPolicyService) RequireReview(ctx context.Context, current domain.Subscription, expectedVersion int64, at time.Time) (domain.Subscription, error) {
	return mutate(ctx, current, expectedVersion, func(value *domain.Subscription) error { return value.RequireReview(at) })
}

func mutate(ctx context.Context, current domain.Subscription, expectedVersion int64, operation func(*domain.Subscription) error) (domain.Subscription, error) {
	if ctx == nil || operation == nil || expectedVersion < 1 {
		return domain.Subscription{}, ErrSubscriptionRequestInvalid
	}
	if current.Version != expectedVersion {
		return domain.Subscription{}, ErrSubscriptionVersionConflict
	}
	updated := current
	if err := operation(&updated); err != nil {
		return domain.Subscription{}, mapSubscriptionError(err)
	}
	return updated, nil
}

func mapSubscriptionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrReconciliationRequired):
		return ErrSubscriptionReconciliationRequired
	case errors.Is(err, domain.ErrSubscriptionConflict):
		return ErrSubscriptionTransitionConflict
	case errors.Is(err, domain.ErrTrialIneligible):
		return ErrSubscriptionTrialIneligible
	default:
		return ErrSubscriptionRequestInvalid
	}
}
