package application

import (
	"bidora/backend/internal/commercial/domain"
	"context"
	"time"
)

type PaymentCallbackTx interface {
	QueueUntrustedPaymentHint(context.Context, domain.ID, domain.ID, domain.ID, domain.ID, domain.ID, time.Time) error
}
type PaymentCallbackUnitOfWork interface {
	WithinPaymentCallback(context.Context, func(context.Context, PaymentCallbackTx) error) error
}
type PaymentCallbackService struct {
	uow   PaymentCallbackUnitOfWork
	ids   InvoiceIDGenerator
	clock InvoiceClock
}

func NewPaymentCallbackService(uow PaymentCallbackUnitOfWork, ids InvoiceIDGenerator, clock InvoiceClock) (*PaymentCallbackService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrPaymentRequestInvalid
	}
	return &PaymentCallbackService{uow, ids, clock}, nil
}

// Receive accepts only untrusted correlation hints. It never reads or accepts a
// payment status/amount, creates an order, settles money or grants entitlements.
// Unknown references intentionally have the same acknowledgement as known ones.
func (s *PaymentCallbackService) Receive(ctx context.Context, merchant, tracking, correlation domain.ID) error {
	if ctx == nil || merchant.IsZero() || tracking.IsZero() || correlation.IsZero() {
		return ErrPaymentRequestInvalid
	}
	ids, e := paymentIDs(s.ids, 2)
	if e != nil {
		return e
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return ErrPaymentUnavailable
	}
	return s.uow.WithinPaymentCallback(ctx, func(c context.Context, t PaymentCallbackTx) error {
		return t.QueueUntrustedPaymentHint(c, merchant, tracking, correlation, ids[0], ids[1], at)
	})
}
