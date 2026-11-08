package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/identity/session"
)

var ErrSessionNotFound = errors.New("session_not_found")

type SessionRecord struct {
	AuthenticatedAt                  time.Time // Trusted stored session creation, never inferred from expiry.
	SessionID, UserID                ID
	CSRFHash                         [32]byte
	AuthEpoch                        int64
	IdleExpiresAt, AbsoluteExpiresAt time.Time
	AssuranceLevel, AssuranceMethod  string
	AssuredAt                        time.Time
}

func (SessionRecord) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED session record]") }

type SessionRepository interface {
	GetSession(context.Context, [32]byte, time.Time) (SessionRecord, error)
}
type SessionClock interface{ Now() time.Time }
type SessionPrincipal struct {
	AuthenticatedAt                  time.Time
	SessionID, UserID                ID
	CSRFHash                         [32]byte
	AuthEpoch                        int64
	IdleExpiresAt, AbsoluteExpiresAt time.Time
	AssuranceLevel, AssuranceMethod  string
	AssuredAt                        time.Time
}

func (SessionPrincipal) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED session principal]") }

type SessionValidator struct {
	repository SessionRepository
	clock      SessionClock
}

func NewSessionValidator(repository SessionRepository, clock SessionClock) (*SessionValidator, error) {
	if repository == nil || clock == nil {
		return nil, errors.New("session_validator_invalid")
	}
	return &SessionValidator{repository, clock}, nil
}
func (v *SessionValidator) Validate(ctx context.Context, rawToken string) (SessionPrincipal, error) {
	if ctx == nil {
		return SessionPrincipal{}, safe("session_invalid", nil)
	}
	if v == nil || v.repository == nil || v.clock == nil {
		return SessionPrincipal{}, safe("session_unavailable", nil)
	}
	tokenHash, err := session.HashPresentedSession(rawToken)
	if err != nil {
		return SessionPrincipal{}, safe("session_invalid", nil)
	}
	observedAt := v.clock.Now().UTC()
	if observedAt.IsZero() {
		return SessionPrincipal{}, safe("session_unavailable", nil)
	}
	record, err := v.repository.GetSession(ctx, tokenHash, observedAt)
	if errors.Is(err, ErrSessionNotFound) {
		return SessionPrincipal{}, safe("session_invalid", nil)
	}
	if err != nil {
		return SessionPrincipal{}, safe("session_unavailable", err)
	}
	if record.SessionID.IsZero() || record.UserID.IsZero() || record.CSRFHash == [32]byte{} || record.AuthEpoch < 1 || record.AuthenticatedAt.IsZero() || record.AuthenticatedAt.After(observedAt) || !record.IdleExpiresAt.After(observedAt) || !record.AbsoluteExpiresAt.After(observedAt) || record.IdleExpiresAt.After(record.AbsoluteExpiresAt) || (record.AssuranceLevel != "AAL1" && record.AssuranceLevel != "AAL2") || record.AssuredAt.IsZero() {
		return SessionPrincipal{}, safe("session_unavailable", nil)
	}
	return SessionPrincipal{AuthenticatedAt: record.AuthenticatedAt.UTC(), SessionID: record.SessionID, UserID: record.UserID, CSRFHash: record.CSRFHash, AuthEpoch: record.AuthEpoch, IdleExpiresAt: record.IdleExpiresAt.UTC(), AbsoluteExpiresAt: record.AbsoluteExpiresAt.UTC(), AssuranceLevel: record.AssuranceLevel, AssuranceMethod: record.AssuranceMethod, AssuredAt: record.AssuredAt.UTC()}, nil
}
