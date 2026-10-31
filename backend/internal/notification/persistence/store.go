// Package persistence implements notification evidence ports against the control database.
package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	notificationsql "bidora/backend/internal/notification/persistence/sqlc"
	"bidora/backend/internal/postgres"
	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type transactionRunner interface {
	WithinTransaction(context.Context, func(context.Context, postgres.DBTX) error) error
}

type Store struct{ runner transactionRunner }

func NewStore(runner transactionRunner) (*Store, error) {
	if runner == nil {
		return nil, errors.New("notification_store_invalid")
	}
	return &Store{runner: runner}, nil
}

func (s *Store) WithinNotification(ctx context.Context, fn func(context.Context, app.Tx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("notification_unit_of_work_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(txCtx context.Context, db postgres.DBTX) error {
		if db == nil {
			return errors.New("notification_tx_invalid")
		}
		return fn(txCtx, &tx{q: notificationsql.New(db)})
	})
}

type tx struct{ q *notificationsql.Queries }

func (t *tx) GetNotificationByIdempotencyHash(ctx context.Context, hash [32]byte) (domain.Notification, bool, error) {
	row, err := t.q.GetNotificationByIdempotencyHash(ctx, hash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Notification{}, false, nil
	}
	if err != nil {
		return domain.Notification{}, false, err
	}
	return notification(row), true, nil
}
func (t *tx) GetNotification(ctx context.Context, id domain.ID) (domain.Notification, bool, error) {
	row, err := t.q.GetNotification(ctx, uuid(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Notification{}, false, nil
	}
	if err != nil {
		return domain.Notification{}, false, err
	}
	return notification(row), true, nil
}
func (t *tx) InsertNotification(ctx context.Context, n domain.Notification) error {
	return t.q.InsertNotification(ctx, notificationsql.InsertNotificationParams{
		ID: uuid(n.ID), TenantID: nullableUUID(n.TenantID), ActorID: nullableUUID(n.ActorID), RecipientUserID: nullableUUID(n.RecipientUserID),
		Category: n.Category, TemplateKey: n.TemplateKey, TemplateVersion: n.TemplateVersion, SourceType: n.SourceType, SourceID: uuid(n.SourceID),
		RecipientFingerprint: n.RecipientFingerprint[:], ContentFingerprint: n.ContentFingerprint[:], FingerprintKeyVersion: n.FingerprintKeyVersion,
		IdempotencyKeyHash: n.IdempotencyKeyHash[:], CorrelationID: uuid(n.CorrelationID), CreatedAt: timestamp(n.CreatedAt),
	})
}
func (t *tx) IsActiveSMTPVersion(ctx context.Context, versionID domain.ID) (bool, error) {
	return t.q.IsActiveSMTPVersion(ctx, uuid(versionID))
}
func (t *tx) CanStartAttempt(ctx context.Context, notificationID domain.ID, number int32) (bool, error) {
	progress, err := t.q.GetDeliveryAttemptProgress(ctx, uuid(notificationID))
	if err != nil {
		return false, err
	}
	return progress.UnfinishedCount == 0 && progress.AcceptedCount == 0 && number == progress.AttemptCount+1, nil
}
func (t *tx) GetAttemptByNumber(ctx context.Context, notificationID domain.ID, number int32) (domain.Attempt, bool, error) {
	row, err := t.q.GetDeliveryAttemptByNumber(ctx, notificationsql.GetDeliveryAttemptByNumberParams{NotificationID: uuid(notificationID), AttemptNumber: number})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attempt{}, false, nil
	}
	if err != nil {
		return domain.Attempt{}, false, err
	}
	return domain.Attempt{ID: id(row.ID), NotificationID: id(row.NotificationID), Number: row.AttemptNumber, SMTPVersionID: id(row.SmtpVersionID), CorrelationID: id(row.CorrelationID), StartedAt: row.StartedAt.Time}, true, nil
}
func (t *tx) GetAttempt(ctx context.Context, attemptID domain.ID) (domain.Attempt, bool, error) {
	row, err := t.q.GetDeliveryAttempt(ctx, uuid(attemptID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attempt{}, false, nil
	}
	if err != nil {
		return domain.Attempt{}, false, err
	}
	return domain.Attempt{ID: id(row.ID), NotificationID: id(row.NotificationID), Number: row.AttemptNumber, SMTPVersionID: id(row.SmtpVersionID), CorrelationID: id(row.CorrelationID), StartedAt: row.StartedAt.Time}, true, nil
}
func (t *tx) InsertAttempt(ctx context.Context, a domain.Attempt) error {
	return t.q.InsertDeliveryAttempt(ctx, notificationsql.InsertDeliveryAttemptParams{ID: uuid(a.ID), NotificationID: uuid(a.NotificationID), AttemptNumber: a.Number, SmtpVersionID: uuid(a.SMTPVersionID), CorrelationID: uuid(a.CorrelationID), StartedAt: timestamp(a.StartedAt)})
}
func (t *tx) GetResult(ctx context.Context, attemptID domain.ID) (domain.Result, bool, error) {
	row, err := t.q.GetDeliveryAttemptResult(ctx, uuid(attemptID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Result{}, false, nil
	}
	if err != nil {
		return domain.Result{}, false, err
	}
	failure := ""
	if row.FailureCode != nil {
		failure = *row.FailureCode
	}
	return domain.Result{ID: id(row.ID), AttemptID: id(row.AttemptID), Disposition: domain.Disposition(row.Disposition), FailureCode: failure, CorrelationID: id(row.CorrelationID), FinishedAt: row.FinishedAt.Time}, true, nil
}
func (t *tx) InsertResult(ctx context.Context, r domain.Result) error {
	var failure *string
	if r.FailureCode != "" {
		value := r.FailureCode
		failure = &value
	}
	return t.q.InsertDeliveryAttemptResult(ctx, notificationsql.InsertDeliveryAttemptResultParams{ID: uuid(r.ID), AttemptID: uuid(r.AttemptID), Disposition: string(r.Disposition), FailureCode: failure, CorrelationID: uuid(r.CorrelationID), FinishedAt: timestamp(r.FinishedAt)})
}
func (t *tx) AppendEvidence(ctx context.Context, e app.Evidence) error {
	summary, err := json.Marshal(struct {
		Category, Disposition, FailureCode string
		TemplateVersion, AggregateVersion  int64
		AttemptNumber                      int32
	}{e.Category, e.Disposition, e.FailureCode, e.TemplateVersion, e.AggregateVersion, e.AttemptNumber})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		NotificationID, AttemptID, Category, Disposition, FailureCode string
		TemplateVersion, Version                                      int64
		AttemptNumber                                                 int32
	}{idString(e.NotificationID), idString(e.AttemptID), e.Category, e.Disposition, e.FailureCode, e.TemplateVersion, e.AggregateVersion, e.AttemptNumber})
	if err != nil {
		return err
	}
	if err = t.q.InsertNotificationAudit(ctx, notificationsql.InsertNotificationAuditParams{ID: uuid(e.AuditID), TenantID: nullableUUID(e.TenantID), ActorID: nullableUUID(e.ActorID), Action: e.Action, TargetID: uuid(e.NotificationID), OccurredAt: timestamp(e.OccurredAt), CorrelationID: uuid(e.CorrelationID), ChangeSummary: summary}); err != nil {
		return err
	}
	return t.q.InsertNotificationOutbox(ctx, notificationsql.InsertNotificationOutboxParams{ID: uuid(e.OutboxID), TenantID: nullableUUID(e.TenantID), ActorID: nullableUUID(e.ActorID), AggregateID: uuid(e.NotificationID), AggregateVersion: e.AggregateVersion, EventType: e.EventType, Payload: payload, OccurredAt: timestamp(e.OccurredAt)})
}

func notification(row notificationsql.NotificationNotification) domain.Notification {
	return domain.Notification{ID: id(row.ID), TenantID: id(row.TenantID), ActorID: id(row.ActorID), RecipientUserID: id(row.RecipientUserID), Category: row.Category, TemplateKey: row.TemplateKey, TemplateVersion: row.TemplateVersion, SourceType: row.SourceType, SourceID: id(row.SourceID), RecipientFingerprint: hash(row.RecipientFingerprint), ContentFingerprint: hash(row.ContentFingerprint), FingerprintKeyVersion: row.FingerprintKeyVersion, IdempotencyKeyHash: hash(row.IdempotencyKeyHash), CorrelationID: id(row.CorrelationID), CreatedAt: row.CreatedAt.Time}
}
func uuid(value domain.ID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}
func nullableUUID(value domain.ID) pgtype.UUID { return uuid(value) }
func id(value pgtype.UUID) domain.ID {
	if !value.Valid {
		return domain.ID{}
	}
	return domain.ID(value.Bytes)
}
func hash(value []byte) (out [32]byte) { copy(out[:], value); return }
func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}
func idString(value domain.ID) string {
	if value.IsZero() {
		return ""
	}
	return googleuuid.UUID(value).String()
}
