package persistence

import (
	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	q "bidora/backend/internal/commercial/persistence/sqlc"
	"bidora/backend/internal/postgres"
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

func (s *Store) WithinPaymentCallback(ctx context.Context, fn func(context.Context, app.PaymentCallbackTx) error) error {
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		t, e := newTrialTx(db)
		if e != nil {
			return e
		}
		return fn(c, t)
	})
}
func (t *trialTx) QueueUntrustedPaymentHint(ctx context.Context, merchant, tracking, correlation, audit, event domain.ID, at time.Time) error {
	p, e := t.commercial.LockPesaPalCallbackPayment(ctx, commercialID(merchant))
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if p.State != "PROVIDER_CREATE_PENDING" && p.State != "AWAITING_CUSTOMER" && p.State != "PROVIDER_PENDING" && p.State != "SETTLED" {
		return nil
	}
	if p.ReconciliationAttempts >= 9 {
		return nil
	}
	if p.ProviderOrderReference != nil && *p.ProviderOrderReference != uuid.UUID(tracking).String() {
		return nil
	}
	inserted := int64(0)
	if p.ProviderOrderReference == nil {
		count, e := t.commercial.CountPaymentCallbackHints(ctx, p.ID)
		if e != nil {
			return e
		}
		if count >= 3 {
			latest, e := t.commercial.LatestPaymentCallbackHint(ctx, p.ID)
			if e != nil {
				return e
			}
			if latest.Bytes != [16]byte(tracking) {
				return nil
			}
		} else {
			inserted, e = t.commercial.InsertPaymentCallbackHint(ctx, q.InsertPaymentCallbackHintParams{PaymentID: p.ID, TrackingID: commercialID(tracking), At: commercialTime(at)})
			if e != nil {
				return e
			}
		}
	}
	queued, e := t.commercial.WakePesaPalReconciliation(ctx, q.WakePesaPalReconciliationParams{At: commercialTime(at), ID: p.ID, ExpectedVersion: p.Version})
	if e != nil {
		return e
	}
	if inserted == 0 && queued == 0 {
		return nil
	}
	if e = t.commercial.AppendPaymentCallbackAudit(ctx, q.AppendPaymentCallbackAuditParams{ID: commercialID(audit), TenantID: p.TenantID, PaymentID: p.ID, At: commercialTime(at), CorrelationID: commercialID(correlation)}); e != nil {
		return e
	}
	return t.commercial.AppendPaymentCallbackOutbox(ctx, q.AppendPaymentCallbackOutboxParams{ID: commercialID(event), TenantID: p.TenantID, PaymentID: p.ID, Version: p.Version + queued, At: commercialTime(at)})
}
