package application

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/notification/domain"
	platformapp "bidora/backend/internal/platformadmin/application"
)

var ErrHistoryInvalid = errors.New("platform_notification_history_invalid")
var ErrHistoryForbidden = errors.New("platform_notification_history_forbidden")
var ErrHistoryUnavailable = errors.New("platform_notification_history_unavailable")

type HistoryCursor struct {
	StartedAt time.Time
	ID        domain.ID
}
type HistorySubject struct {
	UserID, SessionID, CorrelationID domain.ID
	ContextGeneration                int64
}
type AttemptHistoryRow struct {
	ID                       domain.ID // pagination only: never returned in the row response
	Category                 string
	AttemptNumber            int32
	StartedAt                time.Time
	Disposition, FailureCode *string // absent result is not a delivery outcome
	FinishedAt               *time.Time
}
type HistoryPage struct {
	Rows []AttemptHistoryRow
	Next *HistoryCursor
}

type PlatformHistoryTx interface {
	ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error)
	ListPlatformAttempts(context.Context, int32, *HistoryCursor) ([]AttemptHistoryRow, error)
	AppendHistoryReadAudit(context.Context, domain.ID, HistorySubject, time.Time, int, bool) error
}
type PlatformHistoryUnitOfWork interface {
	WithinPlatformHistory(context.Context, func(context.Context, PlatformHistoryTx) error) error
}
type PlatformHistoryService struct {
	uow   PlatformHistoryUnitOfWork
	ids   IDGenerator
	clock Clock
}

func NewPlatformHistoryService(uow PlatformHistoryUnitOfWork, ids IDGenerator, clock Clock) (*PlatformHistoryService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("platform_notification_history_service_invalid")
	}
	return &PlatformHistoryService{uow: uow, ids: ids, clock: clock}, nil
}
func (s *PlatformHistoryService) List(ctx context.Context, subject HistorySubject, limit int32, cursor *HistoryCursor) (out HistoryPage, err error) {
	if ctx == nil || s == nil || s.uow == nil || subject.UserID.IsZero() || subject.SessionID.IsZero() || subject.CorrelationID.IsZero() || subject.ContextGeneration < 1 || limit < 1 || limit > 100 || cursor != nil && (cursor.ID.IsZero() || cursor.StartedAt.IsZero()) {
		return out, ErrHistoryInvalid
	}
	at := s.clock.Now().UTC()
	audit, e := s.ids.NewID()
	if at.IsZero() || e != nil || audit.IsZero() {
		return out, ErrHistoryUnavailable
	}
	err = s.uow.WithinPlatformHistory(ctx, func(c context.Context, tx PlatformHistoryTx) error {
		facts, x := tx.ResolvePlatformAuthority(c, platformapp.AuthorityQuery{UserID: platformapp.ID(subject.UserID), SessionID: platformapp.ID(subject.SessionID), ObservedAt: at})
		if errors.Is(x, platformapp.ErrAuthorityNotFound) {
			return ErrHistoryForbidden
		}
		if x != nil {
			return x
		}
		if !facts.Grants("platform.integration.failure.read") || facts.UserID != platformapp.ID(subject.UserID) || facts.ContextGeneration != subject.ContextGeneration || facts.AssuranceLevel != "AAL2" || facts.AssuranceMethod != "TOTP" || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) {
			return ErrHistoryForbidden
		}
		rows, x := tx.ListPlatformAttempts(c, limit+1, cursor)
		if x != nil {
			return x
		}
		if len(rows) > int(limit) {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			out.Next = &HistoryCursor{StartedAt: last.StartedAt, ID: last.ID}
		}
		out.Rows = rows
		return tx.AppendHistoryReadAudit(c, audit, subject, at, len(rows), out.Next != nil)
	})
	if err != nil {
		if errors.Is(err, ErrHistoryForbidden) {
			return HistoryPage{}, ErrHistoryForbidden
		}
		return HistoryPage{}, ErrHistoryUnavailable
	}
	return out, nil
}
