package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	app "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	notificationsql "bidora/backend/internal/notification/persistence/sqlc"
	platformapp "bidora/backend/internal/platformadmin/application"
	platformpersistence "bidora/backend/internal/platformadmin/persistence"
	"bidora/backend/internal/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

type PlatformHistoryStore struct{ runner transactionRunner }

func NewPlatformHistoryStore(runner transactionRunner) (*PlatformHistoryStore, error) {
	if runner == nil {
		return nil, errors.New("platform_notification_history_store_invalid")
	}
	return &PlatformHistoryStore{runner}, nil
}
func (s *PlatformHistoryStore) WithinPlatformHistory(ctx context.Context, fn func(context.Context, app.PlatformHistoryTx) error) error {
	if ctx == nil || s == nil || s.runner == nil || fn == nil {
		return errors.New("platform_notification_history_tx_invalid")
	}
	return s.runner.WithinTransaction(ctx, func(c context.Context, db postgres.DBTX) error {
		authority, err := platformpersistence.NewSettingsAuthorityTx(db)
		if err != nil {
			return err
		}
		return fn(c, &platformHistoryTx{q: notificationsql.New(db), authority: authority})
	})
}

type platformHistoryTx struct {
	q         *notificationsql.Queries
	authority *platformpersistence.SettingsAuthorityTx
}

func (t *platformHistoryTx) ResolvePlatformAuthority(ctx context.Context, query platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error) {
	return t.authority.Resolve(ctx, query)
}
func (t *platformHistoryTx) ListPlatformAttempts(ctx context.Context, limit int32, cursor *app.HistoryCursor) ([]app.AttemptHistoryRow, error) {
	var at pgtype.Timestamptz
	var cursorID pgtype.UUID
	if cursor != nil {
		at = timestamp(cursor.StartedAt)
		cursorID = uuid(cursor.ID)
	}
	rows, err := t.q.ListPlatformNotificationAttempts(ctx, notificationsql.ListPlatformNotificationAttemptsParams{CursorAt: at, CursorID: cursorID, PageLimit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]app.AttemptHistoryRow, 0, len(rows))
	for _, row := range rows {
		if !row.ID.Valid || row.AttemptNumber < 1 || row.AttemptNumber > 100 || !row.StartedAt.Valid || row.StartedAt.Time.IsZero() || !platformNotificationCategory(row.Category) {
			return nil, errors.New("platform_notification_history_row_invalid")
		}
		item := app.AttemptHistoryRow{ID: id(row.ID), Category: row.Category, AttemptNumber: row.AttemptNumber, StartedAt: row.StartedAt.Time.UTC()}
		if row.ResultID.Valid || row.ResultCorrelationID.Valid || row.Disposition != nil || row.FailureCode != nil || row.FinishedAt.Valid {
			if !row.ResultID.Valid || !row.ResultCorrelationID.Valid || row.Disposition == nil || !row.FinishedAt.Valid || row.FinishedAt.Time.Before(row.StartedAt.Time) {
				return nil, errors.New("platform_notification_history_result_invalid")
			}
			code := ""
			if row.FailureCode != nil {
				code = *row.FailureCode
			}
			result := domain.Result{ID: id(row.ResultID), AttemptID: id(row.ID), CorrelationID: id(row.ResultCorrelationID), Disposition: domain.Disposition(*row.Disposition), FailureCode: code, FinishedAt: row.FinishedAt.Time}
			if !result.Valid() {
				return nil, errors.New("platform_notification_history_result_invalid")
			}
			disposition, finished := *row.Disposition, row.FinishedAt.Time.UTC()
			item.Disposition, item.FinishedAt = &disposition, &finished
			if code != "" {
				item.FailureCode = &code
			}
		}
		out = append(out, item)
	}
	return out, nil
}
func platformNotificationCategory(v string) bool {
	switch v {
	case "PLATFORM_INVITATION", "PLATFORM_SESSION_REVOCATION", "PLATFORM_IDENTITY_STATE_CHANGE", "PLATFORM_ROLE_CHANGE", "PLATFORM_TEMPLATE_TEST":
		return true
	}
	return false
}
func (t *platformHistoryTx) AppendHistoryReadAudit(ctx context.Context, auditID domain.ID, subject app.HistorySubject, at time.Time, count int, more bool) error {
	if auditID.IsZero() || subject.UserID.IsZero() || subject.CorrelationID.IsZero() || at.IsZero() || count < 0 || count > 100 {
		return errors.New("platform_notification_history_audit_invalid")
	}
	summary, err := json.Marshal(struct {
		ResultCount int  `json:"result_count"`
		HasMore     bool `json:"has_more"`
	}{count, more})
	if err != nil {
		return err
	}
	return t.q.InsertPlatformNotificationHistoryReadAudit(ctx, notificationsql.InsertPlatformNotificationHistoryReadAuditParams{ID: uuid(auditID), ActorID: uuid(subject.UserID), At: timestamp(at), CorrelationID: uuid(subject.CorrelationID), Summary: summary})
}
