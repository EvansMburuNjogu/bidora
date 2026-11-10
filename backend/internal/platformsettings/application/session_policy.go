package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"bidora/backend/internal/identity/securitypolicy"
	"bidora/backend/internal/platformsettings/domain"
)

// SessionPolicy configures bounded login/session controls. Mandatory verified
// email, administrator assurance, CSRF and recent sensitive-action checks remain.
type SessionPolicy struct {
	Security        securitypolicy.Controls
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
	ApplySessionPolicy(context.Context, SessionPolicySave, time.Time) (int64, error)
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
	return idle && absolute && p.AbsoluteSeconds >= p.IdleSeconds && p.Version > 0 && securitypolicy.ValidControls(p.Security)
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
		if !canReadInformation(auth, at) || !auth.EffectiveOwner || !recent(ctx, auth, at) {
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
		if in.Policy.Security.StepUpSeconds == 0 {
			in.Policy.Security.StepUpSeconds = current.Security.StepUpSeconds
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
		// Persist tighter bounds atomically; revoke only invalid or excess sessions.
		// A later relaxation cannot resurrect a previously expired session.
		before, _ := json.Marshal(current.Security)
		after, _ := json.Marshal(in.Policy.Security)
		var oldFields, newFields map[string]any
		_ = json.Unmarshal(before, &oldFields)
		_ = json.Unmarshal(after, &newFields)
		for _, field := range []string{"failed_login_attempts", "failure_window_seconds", "lockout_seconds", "minimum_password_bytes", "password_reset_seconds", "allow_multiple_sessions", "admin_idle_seconds", "admin_absolute_seconds", "step_up_seconds"} {
			in.Changes = append(in.Changes, settingChange(field, fmt.Sprint(oldFields[field]), fmt.Sprint(newFields[field]))...)
		}
		revoked, e := tx.ApplySessionPolicy(c, in, at)
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
