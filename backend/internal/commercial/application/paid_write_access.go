package application

import (
	"bidora/backend/internal/commercial/domain"
	"context"
	"errors"
	"time"
)

var ErrPaidWriteAccessInvalid = errors.New("commercial_paid_write_access_invalid")
var ErrPaidWriteAccessDenied = errors.New("commercial_paid_write_access_denied")
var ErrPaidWriteAccessUnavailable = errors.New("commercial_paid_write_access_unavailable")

type PaidWriteAccessService struct {
	uow   SubscriptionUnitOfWork
	clock InvoiceClock
}

func NewPaidWriteAccessService(uow SubscriptionUnitOfWork, clock InvoiceClock) (*PaidWriteAccessService, error) {
	if uow == nil || clock == nil {
		return nil, errors.New("commercial_paid_write_access_service_invalid")
	}
	return &PaidWriteAccessService{uow: uow, clock: clock}, nil
}
func (s *PaidWriteAccessService) Require(ctx context.Context, tenant domain.ID) error {
	if ctx == nil || tenant.IsZero() {
		return ErrPaidWriteAccessInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return ErrPaidWriteAccessUnavailable
	}
	err := s.uow.WithinSubscription(ctx, func(c context.Context, tx SubscriptionTx) error {
		summary, found, e := tx.Read(c, tenant, false)
		if e != nil {
			return e
		}
		if !found || !paidWriteAllowed(summary.Subscription, at) {
			return ErrPaidWriteAccessDenied
		}
		return nil
	})
	if err == nil || errors.Is(err, ErrPaidWriteAccessDenied) {
		return err
	}
	return ErrPaidWriteAccessUnavailable
}
func paidWriteAllowed(s domain.Subscription, at time.Time) bool {
	if s.Access() != domain.FullAccess {
		return false
	}
	switch s.State {
	case domain.TrialActive:
		return s.TrialEndsAt != nil && at.Before(s.TrialEndsAt.UTC()) && at.Before(s.Term.CommercialPeriodEnd.UTC())
	case domain.Active:
		return at.Before(s.Term.CommercialPeriodEnd.UTC())
	case domain.PastDue:
		return s.GraceEndsAt != nil && at.Before(s.GraceEndsAt.UTC())
	default:
		return false
	}
}
