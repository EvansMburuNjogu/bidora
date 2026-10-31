// Package application sequences durable notification delivery evidence around an external SMTP call.
package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"bidora/backend/internal/notification/domain"
	"github.com/google/uuid"
)

var ErrInvalid = errors.New("notification_request_invalid")
var ErrConflict = errors.New("notification_evidence_conflict")
var ErrUnavailable = errors.New("notification_evidence_unavailable")

type PrepareInput struct {
	TenantID, ActorID, RecipientUserID, SourceID, CorrelationID domain.ID
	Category, TemplateKey, SourceType, IdempotencyKey           string
	TemplateVersion                                             int64
	RecipientFingerprint, ContentFingerprint                    [32]byte
	FingerprintKeyVersion                                       int32
}

type StartAttemptInput struct {
	NotificationID, SMTPVersionID, CorrelationID domain.ID
	Number                                       int32
}

type FinishAttemptInput struct {
	AttemptID, CorrelationID domain.ID
	Disposition              domain.Disposition
	FailureCode              string
}

type Evidence struct {
	AuditID, OutboxID, TenantID, ActorID, CorrelationID, NotificationID, AttemptID domain.ID
	Action, EventType, Category, Disposition, FailureCode                          string
	AggregateVersion, TemplateVersion                                              int64
	AttemptNumber                                                                  int32
	OccurredAt                                                                     time.Time
}

type Tx interface {
	GetNotificationByIdempotencyHash(context.Context, [32]byte) (domain.Notification, bool, error)
	GetNotification(context.Context, domain.ID) (domain.Notification, bool, error)
	InsertNotification(context.Context, domain.Notification) error
	IsActiveSMTPVersion(context.Context, domain.ID) (bool, error)
	CanStartAttempt(context.Context, domain.ID, int32) (bool, error)
	GetAttemptByNumber(context.Context, domain.ID, int32) (domain.Attempt, bool, error)
	GetAttempt(context.Context, domain.ID) (domain.Attempt, bool, error)
	InsertAttempt(context.Context, domain.Attempt) error
	GetResult(context.Context, domain.ID) (domain.Result, bool, error)
	InsertResult(context.Context, domain.Result) error
	AppendEvidence(context.Context, Evidence) error
}

type UnitOfWork interface {
	WithinNotification(context.Context, func(context.Context, Tx) error) error
}
type IDGenerator interface{ NewID() (domain.ID, error) }
type Clock interface{ Now() time.Time }

type Service struct {
	uow   UnitOfWork
	ids   IDGenerator
	clock Clock
}

func NewService(uow UnitOfWork, ids IDGenerator, clock Clock) (*Service, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, errors.New("notification_service_invalid")
	}
	return &Service{uow: uow, ids: ids, clock: clock}, nil
}

func (s *Service) Prepare(ctx context.Context, in PrepareInput) (result domain.Notification, err error) {
	parsedKey, parseErr := uuid.Parse(in.IdempotencyKey)
	if ctx == nil || parseErr != nil || parsedKey == uuid.Nil {
		return result, ErrInvalid
	}
	keyHash := sha256.Sum256([]byte(parsedKey.String()))
	at := s.clock.Now().UTC()
	ids, err := s.newIDs(3)
	if err != nil {
		return result, err
	}
	candidate := domain.Notification{
		ID: ids[0], TenantID: in.TenantID, ActorID: in.ActorID, RecipientUserID: in.RecipientUserID,
		SourceID: in.SourceID, CorrelationID: in.CorrelationID, Category: in.Category,
		TemplateKey: in.TemplateKey, SourceType: in.SourceType, TemplateVersion: in.TemplateVersion,
		RecipientFingerprint: in.RecipientFingerprint, ContentFingerprint: in.ContentFingerprint,
		FingerprintKeyVersion: in.FingerprintKeyVersion, IdempotencyKeyHash: keyHash, CreatedAt: at,
	}
	if !candidate.Valid() {
		return result, ErrInvalid
	}
	err = s.uow.WithinNotification(ctx, func(txCtx context.Context, tx Tx) error {
		existing, found, e := tx.GetNotificationByIdempotencyHash(txCtx, keyHash)
		if e != nil {
			return e
		}
		if found {
			if !sameNotification(existing, candidate) {
				return ErrConflict
			}
			result = existing
			return nil
		}
		if e = tx.InsertNotification(txCtx, candidate); e != nil {
			return e
		}
		e = tx.AppendEvidence(txCtx, Evidence{AuditID: ids[1], OutboxID: ids[2], TenantID: candidate.TenantID, ActorID: candidate.ActorID, CorrelationID: candidate.CorrelationID, NotificationID: candidate.ID, Action: "NotificationPrepared", EventType: "notification.prepared.v1", Category: candidate.Category, AggregateVersion: 1, TemplateVersion: candidate.TemplateVersion, OccurredAt: at})
		if e == nil {
			result = candidate
		}
		return e
	})
	if err != nil {
		return domain.Notification{}, mapError(err)
	}
	return result, nil
}

func (s *Service) StartAttempt(ctx context.Context, in StartAttemptInput) (result domain.Attempt, err error) {
	at := s.clock.Now().UTC()
	candidate := domain.Attempt{NotificationID: in.NotificationID, SMTPVersionID: in.SMTPVersionID, CorrelationID: in.CorrelationID, Number: in.Number, StartedAt: at}
	if ctx == nil || candidate.NotificationID.IsZero() || candidate.SMTPVersionID.IsZero() || candidate.CorrelationID.IsZero() || candidate.Number < 1 || candidate.Number > 100 {
		return result, ErrInvalid
	}
	ids, err := s.newIDs(3)
	if err != nil {
		return result, err
	}
	candidate.ID = ids[0]
	err = s.uow.WithinNotification(ctx, func(txCtx context.Context, tx Tx) error {
		n, found, e := tx.GetNotification(txCtx, in.NotificationID)
		if e != nil {
			return e
		}
		if !found {
			return ErrConflict
		}
		existing, found, e := tx.GetAttemptByNumber(txCtx, in.NotificationID, in.Number)
		if e != nil {
			return e
		}
		if found {
			if existing.SMTPVersionID != in.SMTPVersionID {
				return ErrConflict
			}
			result = existing
			return nil
		}
		allowed, e := tx.CanStartAttempt(txCtx, in.NotificationID, in.Number)
		if e != nil {
			return e
		}
		if !allowed {
			return ErrConflict
		}
		active, e := tx.IsActiveSMTPVersion(txCtx, in.SMTPVersionID)
		if e != nil {
			return e
		}
		if !active {
			return ErrConflict
		}
		if e = tx.InsertAttempt(txCtx, candidate); e != nil {
			return e
		}
		e = tx.AppendEvidence(txCtx, Evidence{AuditID: ids[1], OutboxID: ids[2], TenantID: n.TenantID, ActorID: n.ActorID, CorrelationID: in.CorrelationID, NotificationID: n.ID, AttemptID: candidate.ID, Action: "NotificationDeliveryStarted", EventType: "notification.delivery.started.v1", Category: n.Category, AggregateVersion: int64(in.Number) * 2, TemplateVersion: n.TemplateVersion, AttemptNumber: in.Number, OccurredAt: at})
		if e == nil {
			result = candidate
		}
		return e
	})
	if err != nil {
		return domain.Attempt{}, mapError(err)
	}
	return result, nil
}

func (s *Service) FinishAttempt(ctx context.Context, in FinishAttemptInput) (result domain.Result, err error) {
	at := s.clock.Now().UTC()
	ids, err := s.newIDs(3)
	if err != nil {
		return result, err
	}
	candidate := domain.Result{ID: ids[0], AttemptID: in.AttemptID, CorrelationID: in.CorrelationID, Disposition: in.Disposition, FailureCode: in.FailureCode, FinishedAt: at}
	if ctx == nil || !candidate.Valid() {
		return result, ErrInvalid
	}
	err = s.uow.WithinNotification(ctx, func(txCtx context.Context, tx Tx) error {
		a, found, e := tx.GetAttempt(txCtx, in.AttemptID)
		if e != nil {
			return e
		}
		if !found {
			return ErrConflict
		}
		n, found, e := tx.GetNotification(txCtx, a.NotificationID)
		if e != nil {
			return e
		}
		if !found {
			return ErrConflict
		}
		existing, found, e := tx.GetResult(txCtx, a.ID)
		if e != nil {
			return e
		}
		if found {
			if existing.Disposition != in.Disposition || existing.FailureCode != in.FailureCode {
				return ErrConflict
			}
			result = existing
			return nil
		}
		if e = tx.InsertResult(txCtx, candidate); e != nil {
			return e
		}
		e = tx.AppendEvidence(txCtx, Evidence{AuditID: ids[1], OutboxID: ids[2], TenantID: n.TenantID, ActorID: n.ActorID, CorrelationID: in.CorrelationID, NotificationID: n.ID, AttemptID: a.ID, Action: "NotificationDeliveryFinished", EventType: "notification.delivery.finished.v1", Category: n.Category, Disposition: string(in.Disposition), FailureCode: in.FailureCode, AggregateVersion: int64(a.Number)*2 + 1, TemplateVersion: n.TemplateVersion, AttemptNumber: a.Number, OccurredAt: at})
		if e == nil {
			result = candidate
		}
		return e
	})
	if err != nil {
		return domain.Result{}, mapError(err)
	}
	return result, nil
}

func (s *Service) newIDs(count int) ([]domain.ID, error) {
	result := make([]domain.ID, count)
	for i := range result {
		id, err := s.ids.NewID()
		if err != nil || id.IsZero() {
			return nil, ErrUnavailable
		}
		result[i] = id
	}
	return result, nil
}

func sameNotification(a, b domain.Notification) bool {
	return a.TenantID == b.TenantID && a.ActorID == b.ActorID && a.RecipientUserID == b.RecipientUserID &&
		a.SourceID == b.SourceID && a.Category == b.Category && a.TemplateKey == b.TemplateKey &&
		a.SourceType == b.SourceType && a.TemplateVersion == b.TemplateVersion &&
		a.RecipientFingerprint == b.RecipientFingerprint && a.ContentFingerprint == b.ContentFingerprint &&
		a.FingerprintKeyVersion == b.FingerprintKeyVersion
}

func mapError(err error) error {
	if errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) || errors.Is(err, ErrUnavailable) {
		return err
	}
	return ErrUnavailable
}
