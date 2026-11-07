package application

import (
	platformpolicy "bidora/backend/internal/platformadmin/domain"
	"context"
	"errors"
	"regexp"
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
type HistoryFilters struct {
	Category, Status string
	From, To         *time.Time
}

var historyCategoryPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,79}$`)

func (f HistoryFilters) Valid() bool {
	if f.Category != "" && !historyCategoryPattern.MatchString(f.Category) {
		return false
	}
	switch f.Status {
	case "", "NOT_ATTEMPTED", "PENDING", "SMTP_ACCEPTED", "FAILED", "AMBIGUOUS":
	default:
		return false
	}
	return !(f.From != nil && f.From.IsZero() || f.To != nil && f.To.IsZero() || f.From != nil && f.To != nil && !f.From.Before(*f.To))
}

type AttemptHistoryRow struct {
	Recipient, Subject, Message *string
	ID                          domain.ID // pagination only: never returned in the row response
	Category                    string
	AttemptNumber               int32
	StartedAt                   time.Time
	Disposition, FailureCode    *string // absent result is not a delivery outcome
	FinishedAt                  *time.Time
}
type HistoryPage struct {
	Rows []AttemptHistoryRow
	Next *HistoryCursor
}

type PlatformHistoryTx interface {
	ResolvePlatformAuthority(context.Context, platformapp.AuthorityQuery) (platformapp.AuthorityFacts, error)
	ListPlatformAttempts(context.Context, int32, *HistoryCursor, HistoryFilters) ([]AttemptHistoryRow, error)
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
func (s *PlatformHistoryService) List(ctx context.Context, subject HistorySubject, limit int32, cursor *HistoryCursor, selected ...HistoryFilters) (out HistoryPage, err error) {
	var filters HistoryFilters
	if len(selected) > 1 {
		return out, ErrHistoryInvalid
	}
	if len(selected) == 1 {
		filters = selected[0]
	}
	if !filters.Valid() {
		return out, ErrHistoryInvalid
	}
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
		if !facts.Grants("platform.integration.failure.read") || facts.UserID != platformapp.ID(subject.UserID) || facts.ContextGeneration != subject.ContextGeneration || !platformpolicy.AdministratorAssurance(facts.AssuranceLevel, facts.AssuranceMethod) || facts.AssuredAt.IsZero() || facts.AssuredAt.After(at) || facts.AssuredAt.Before(at.Add(-15*time.Minute)) {
			return ErrHistoryForbidden
		}
		rows, x := tx.ListPlatformAttempts(c, limit+1, cursor, filters)
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
