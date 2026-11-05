package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"bidora/backend/internal/platformsettings/domain"
)

// SessionPolicy can only tighten ordinary sessions. The separate verified
// administrator profile and mandatory MFA/password/browser controls are fixed.
type SessionPolicy struct {
	IdleSeconds     int32
	AbsoluteSeconds int32
	Version         int64
}
type SessionPolicySave struct {
	Changes        []FieldChange
	Subject        Subject
	Policy         SessionPolicy
	IdempotencyKey string
	CorrelationID  domain.ID
}
type SessionPolicyTx interface {
	AuthorizeInformation(context.Context, Subject, time.Time) (Authority, error)
	ReadSessionPolicy(context.Context, bool) (SessionPolicy, error)
	FindSessionPolicyCommand(context.Context, domain.ID, [32]byte) (InformationCommand, bool, error)
	UpdateSessionPolicy(context.Context, SessionPolicySave, time.Time) (int64, error)
	RevokeOrdinarySessions(context.Context, time.Time) (int64, error)
	AppendSessionPolicyEvidence(context.Context, domain.ID, domain.ID, SessionPolicySave, int64, time.Time, [32]byte, [32]byte) error
}
type SessionPolicyUnitOfWork interface {
	WithinSessionPolicy(context.Context, bool, func(context.Context, SessionPolicyTx) error) error
}
type SessionPolicyService struct {
	uow   SessionPolicyUnitOfWork
	ids   IDGenerator
	clock Clock
}

func NewSessionPolicyService(uow SessionPolicyUnitOfWork, ids IDGenerator, clock Clock) (*SessionPolicyService, error) {
	if uow == nil || ids == nil || clock == nil {
		return nil, ErrInformationInvalid
	}
	return &SessionPolicyService{uow, ids, clock}, nil
}
func ValidSessionPolicy(p SessionPolicy) bool {
	idle := p.IdleSeconds == 300 || p.IdleSeconds == 600 || p.IdleSeconds == 900 || p.IdleSeconds == 1800
	absolute := p.AbsoluteSeconds == 3600 || p.AbsoluteSeconds == 14400 || p.AbsoluteSeconds == 28800 || p.AbsoluteSeconds == 43200
	return idle && absolute && p.AbsoluteSeconds >= p.IdleSeconds && p.Version > 0
}
func (s *SessionPolicyService) Read(ctx context.Context, subject Subject) (out SessionPolicy, err error) {
	if s == nil || !validInformationSubject(ctx, subject) {
		return out, ErrInformationInvalid
	}
	at := s.clock.Now().UTC()
	err = s.uow.WithinSessionPolicy(ctx, false, func(c context.Context, tx SessionPolicyTx) error {
		auth, e := tx.AuthorizeInformation(c, subject, at)
		if e != nil {
			return e
		}
		if !canReadInformation(auth, at) {
			return ErrInformationForbidden
		}
		out, e = tx.ReadSessionPolicy(c, false)
		return e
	})
	if err != nil && !errors.Is(err, ErrInformationForbidden) {
		return SessionPolicy{}, ErrInformationUnavailable
	}
	return out, err
}
func (s *SessionPolicyService) Save(ctx context.Context, in SessionPolicySave) (out SessionPolicy, err error) {
	if s == nil || !validInformationSubject(ctx, in.Subject) || !ValidSessionPolicy(in.Policy) || in.CorrelationID.IsZero() || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 128 {
		return out, ErrInformationInvalid
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return out, ErrInformationUnavailable
	}
	audit, e := s.ids.NewID()
	if e != nil || audit.IsZero() {
		return out, ErrInformationUnavailable
	}
	outbox, e := s.ids.NewID()
	if e != nil || outbox.IsZero() || outbox == audit {
		return out, ErrInformationUnavailable
	}
	encoded, e := json.Marshal(in.Policy)
	if e != nil {
		return out, ErrInformationInvalid
	}
	key := sha256.Sum256([]byte("platform-session-policy-key-v1\x00" + in.IdempotencyKey))
	hash := sha256.Sum256(append([]byte("platform-session-policy-v1\x00"), encoded...))
	err = s.uow.WithinSessionPolicy(ctx, true, func(c context.Context, tx SessionPolicyTx) error {
		auth, e := tx.AuthorizeInformation(c, in.Subject, at)
		if e != nil {
			return e
		}
		if !canReadInformation(auth, at) || !auth.EffectiveOwner {
			return ErrInformationForbidden
		}
		current, e := tx.ReadSessionPolicy(c, true)
		if e != nil {
			return e
		}
		old, found, e := tx.FindSessionPolicyCommand(c, in.Subject.ActorID, key)
		if e != nil {
			return e
		}
		if found {
			if old.RequestHash != hash || old.ResultVersion != current.Version {
				return ErrInformationConflict
			}
			out = current
			return nil
		}
		if current.Version != in.Policy.Version {
			return ErrInformationConflict
		}
		in.Changes = settingChange("idle_seconds", strconv.Itoa(int(current.IdleSeconds)), strconv.Itoa(int(in.Policy.IdleSeconds)))
		in.Changes = append(in.Changes, settingChange("absolute_seconds", strconv.Itoa(int(current.AbsoluteSeconds)), strconv.Itoa(int(in.Policy.AbsoluteSeconds)))...)
		affected, e := tx.UpdateSessionPolicy(c, in, at)
		if e != nil {
			return e
		}
		if affected != 1 {
			return ErrInformationConflict
		}
		// Revoke all ordinary sessions on every change, so relaxing a formerly
		// stricter policy cannot resurrect a session that expired under that policy.
		revoked, e := tx.RevokeOrdinarySessions(c, at)
		if e != nil {
			return e
		}
		if e = tx.AppendSessionPolicyEvidence(c, audit, outbox, in, revoked, at, key, hash); e != nil {
			return e
		}
		out = in.Policy
		out.Version++
		return nil
	})
	if err != nil && !errors.Is(err, ErrInformationForbidden) && !errors.Is(err, ErrInformationConflict) {
		return SessionPolicy{}, ErrInformationUnavailable
	}
	return out, err
}
