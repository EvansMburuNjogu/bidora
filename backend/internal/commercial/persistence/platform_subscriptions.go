package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"time"

	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	q "bidora/backend/internal/commercial/persistence/sqlc"
	pa "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlatformSubscriptionStore struct {
	runner    transactionRunner
	authority PlanAuthorityFactory
}

func NewPlatformSubscriptionStore(runner transactionRunner, authority PlanAuthorityFactory) (*PlatformSubscriptionStore, error) {
	if runner == nil || authority == nil {
		return nil, errors.New("platform_subscription_store_invalid")
	}
	return &PlatformSubscriptionStore{runner, authority}, nil
}
func (s *PlatformSubscriptionStore) WithinPlatformSubscription(ctx context.Context, fn func(context.Context, app.PlatformSubscriptionTx) error) error {
	if s == nil || ctx == nil || fn == nil {
		return errors.New("platform_subscription_transaction_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		authority, e := s.authority(db)
		if e != nil {
			return e
		}
		subscription, e := newTrialTx(db)
		if e != nil {
			return e
		}
		return fn(c, &platformSubscriptionTx{q: q.New(db), authority: authority, subscription: subscription})
	})
}

type platformSubscriptionTx struct {
	q            *q.Queries
	authority    PlanAuthorityTx
	subscription *trialTx
}

func (t *platformSubscriptionTx) ResolvePlatformAuthority(c context.Context, in pa.AuthorityQuery) (pa.AuthorityFacts, error) {
	return t.authority.Resolve(c, in)
}
func (t *platformSubscriptionTx) ReadPlatformSubscription(c context.Context, tenant domain.ID, lock bool) (app.SubscriptionSummary, bool, error) {
	return t.subscription.Read(c, tenant, lock)
}
func (t *platformSubscriptionTx) ListPlatformSubscriptionInventory(c context.Context, after domain.ID, limit int32, state string) ([]app.PlatformSubscriptionInventoryRow, error) {
	return t.ListFilteredPlatformSubscriptionInventory(c, after, limit, state, "", "")
}
func (t *platformSubscriptionTx) ListFilteredPlatformSubscriptionInventory(c context.Context, after domain.ID, limit int32, state, plan, cadence string) ([]app.PlatformSubscriptionInventoryRow, error) {
	rows, e := t.q.ListPlatformSubscriptionInventory(c, q.ListPlatformSubscriptionInventoryParams{AfterID: pgtype.UUID{Bytes: after, Valid: true}, RowLimit: limit, StateFilter: state, PlanFilter: plan, CadenceFilter: cadence})
	if e != nil {
		return nil, e
	}
	result := make([]app.PlatformSubscriptionInventoryRow, 0, len(rows))
	for _, v := range rows {
		if !v.ID.Valid || !v.TenantID.Valid || !v.UpdatedAt.Valid || v.HasCurrentTerm != v.CommercialPeriodEnd.Valid {
			return nil, errors.New("platform_subscription_inventory_invalid")
		}
		row := app.PlatformSubscriptionInventoryRow{TenantName: v.TenantName, SubscriptionID: domain.ID(v.ID.Bytes), TenantID: domain.ID(v.TenantID.Bytes), State: domain.SubscriptionState(v.EffectiveState), Version: v.Version, PlanKey: domain.PlanKey(v.PlanKey), Cadence: domain.BillingCadence(v.Cadence), HasCurrentTerm: v.HasCurrentTerm, UpdatedAt: v.UpdatedAt.Time.UTC()}
		if v.CommercialPeriodEnd.Valid {
			at := v.CommercialPeriodEnd.Time.UTC()
			row.PeriodEnd = &at
		}
		if v.CancelAt.Valid {
			at := v.CancelAt.Time.UTC()
			row.CancelAt = &at
		}
		result = append(result, row)
	}
	return result, nil
}
func (t *platformSubscriptionTx) AppendPlatformSubscriptionInventoryRead(c context.Context, audit, actor, correlation domain.ID, count int32, state string, at time.Time) error {
	summary, _ := json.Marshal(struct {
		Rows  int32  `json:"rows_returned"`
		State string `json:"state_filter"`
	}{count, state})
	return t.q.InsertPlatformSubscriptionInventoryRead(c, q.InsertPlatformSubscriptionInventoryReadParams{ID: planUUID(audit), ActorID: planUUID(actor), At: planTime(at), CorrelationID: planUUID(correlation), Summary: summary})
}
func (t *platformSubscriptionTx) GetPlatformSubscriptionCommand(c context.Context, actor domain.ID, op string, key [32]byte) (app.PlatformSubscriptionCommand, bool, error) {
	v, e := t.q.GetPlatformSubscriptionCommand(c, q.GetPlatformSubscriptionCommandParams{ActorID: planUUID(actor), Operation: op, KeyHash: key[:]})
	if errors.Is(e, pgx.ErrNoRows) {
		return app.PlatformSubscriptionCommand{}, false, nil
	}
	if e != nil {
		return app.PlatformSubscriptionCommand{}, false, e
	}
	if len(v.RequestHash) != 32 || !v.TenantID.Valid || !v.SubscriptionID.Valid {
		return app.PlatformSubscriptionCommand{}, false, errors.New("platform_subscription_command_invalid")
	}
	var hash [32]byte
	copy(hash[:], v.RequestHash)
	receipt := app.PlatformSubscriptionReceipt{TenantID: domain.ID(v.TenantID.Bytes), SubscriptionID: domain.ID(v.SubscriptionID.Bytes), State: domain.SubscriptionState(v.ResultState), Version: v.ResultVersion}
	if v.ResultCancelAt.Valid {
		at := v.ResultCancelAt.Time.UTC()
		receipt.CancelAt = &at
	}
	return app.PlatformSubscriptionCommand{RequestHash: hash, Receipt: receipt}, true, nil
}
func (t *platformSubscriptionTx) CommitPlatformSubscription(c context.Context, in app.PlatformSubscriptionCommit) error {
	before, after := in.Before, in.After
	if after.SubscriptionID.IsZero() || after.SubscriptionID != before.SubscriptionID || after.Subscription.TenantID != before.Subscription.TenantID || after.Subscription.Version != before.Subscription.Version+1 {
		return errors.New("platform_subscription_commit_invalid")
	}
	var rows int64
	var e error
	action, event := "PlatformSubscriptionCancellationScheduled", "commercial.subscription.cancellation_scheduled.v1"
	switch in.Operation {
	case "SCHEDULE_CANCELLATION":
		if after.Subscription.CancelAt == nil {
			return errors.New("platform_subscription_cancel_at_missing")
		}
		rows, e = t.q.ScheduleSubscriptionCancellation(c, q.ScheduleSubscriptionCancellationParams{CancelAt: planTime(*after.Subscription.CancelAt), UpdatedAt: planTime(in.At), ID: planUUID(after.SubscriptionID), ExpectedVersion: before.Subscription.Version})
	case "WITHDRAW_CANCELLATION":
		if after.Subscription.CancelAt != nil {
			return errors.New("platform_subscription_withdraw_invalid")
		}
		action, event = "PlatformSubscriptionCancellationWithdrawn", "commercial.subscription.cancellation_withdrawn.v1"
		rows, e = t.q.WithdrawSubscriptionCancellation(c, q.WithdrawSubscriptionCancellationParams{UpdatedAt: planTime(in.At), ID: planUUID(after.SubscriptionID), ExpectedVersion: before.Subscription.Version})
	default:
		return errors.New("platform_subscription_operation_invalid")
	}
	if e != nil {
		return e
	}
	if rows != 1 {
		return app.ErrPlatformSubscriptionConflict
	}
	if e = t.q.InsertSubscriptionTransition(c, q.InsertSubscriptionTransitionParams{ID: planUUID(in.TransitionID), SubscriptionID: planUUID(after.SubscriptionID), FromState: string(before.Subscription.State), ToState: string(after.Subscription.State), Action: "PLATFORM_" + in.Operation, SubscriptionVersion: after.Subscription.Version, ActorUserID: planUUID(in.ActorID), OccurredAt: planTime(in.At)}); e != nil {
		return e
	}
	var cancel pgtype.Timestamptz
	if after.Subscription.CancelAt != nil {
		cancel = planTime(*after.Subscription.CancelAt)
	}
	if e = t.q.InsertPlatformSubscriptionCommand(c, q.InsertPlatformSubscriptionCommandParams{ActorID: planUUID(in.ActorID), Operation: in.Operation, KeyHash: in.KeyHash[:], RequestHash: in.RequestHash[:], TenantID: planUUID(after.Subscription.TenantID), SubscriptionID: planUUID(after.SubscriptionID), ResultState: string(after.Subscription.State), ResultVersion: after.Subscription.Version, ResultCancelAt: cancel, At: planTime(in.At)}); e != nil {
		return e
	}
	summary, _ := json.Marshal(struct {
		TenantID       string     `json:"tenant_id"`
		SubscriptionID string     `json:"subscription_id"`
		Version        int64      `json:"version"`
		State          string     `json:"state"`
		CancelAt       *time.Time `json:"cancel_at"`
	}{uuid.UUID(after.Subscription.TenantID).String(), uuid.UUID(after.SubscriptionID).String(), after.Subscription.Version, string(after.Subscription.State), after.Subscription.CancelAt})
	if e = t.q.InsertPlatformSubscriptionAudit(c, q.InsertPlatformSubscriptionAuditParams{ID: planUUID(in.AuditID), ActorID: planUUID(in.ActorID), Action: action, SubscriptionID: planUUID(after.SubscriptionID), At: planTime(in.At), CorrelationID: planUUID(in.CorrelationID), Summary: summary}); e != nil {
		return e
	}
	// Keep the established commercial subscription event v1 payload wire shape.
	cancelAt := ""
	if after.Subscription.CancelAt != nil {
		cancelAt = after.Subscription.CancelAt.UTC().Format(time.RFC3339Nano)
	}
	payload, _ := json.Marshal(struct {
		TenantID, SubscriptionID, State, CancelAt string
		Version                                   int64
	}{uuid.UUID(after.Subscription.TenantID).String(), uuid.UUID(after.SubscriptionID).String(), string(after.Subscription.State), cancelAt, after.Subscription.Version})
	return t.q.InsertPlatformSubscriptionOutbox(c, q.InsertPlatformSubscriptionOutboxParams{ID: planUUID(in.OutboxID), ActorID: planUUID(in.ActorID), SubscriptionID: planUUID(after.SubscriptionID), Version: after.Subscription.Version, EventType: event, Payload: payload, At: planTime(in.At)})
}
func (t *platformSubscriptionTx) AppendPlatformSubscriptionRead(c context.Context, audit, actor, correlation, subscription domain.ID, at time.Time) error {
	return t.q.InsertPlatformSubscriptionAudit(c, q.InsertPlatformSubscriptionAuditParams{ID: planUUID(audit), ActorID: planUUID(actor), Action: "PlatformSubscriptionRead", SubscriptionID: planUUID(subscription), At: planTime(at), CorrelationID: planUUID(correlation), Summary: []byte(`{}`)})
}
