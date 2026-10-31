package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrSessionRevocationConflict = errors.New("session_revocation_conflict")
var ErrSessionAdministrationLimit = errors.New("session_administration_limit")
var ErrSessionAdministrationInvalid = errors.New("session_administration_invalid")

type RevokeOtherSessionInput struct {
	Principal                      SessionPrincipal
	TargetSessionID, CorrelationID ID
	IdempotencyKey                 string
	SessionAuthenticatedAt         time.Time
}
type RevokeOtherSessionCommit struct {
	CommandID, AuditID, OutboxID, TargetSessionID, CorrelationID ID
	Principal                                                    SessionPrincipal
	KeyHash, RequestHash                                         [32]byte
	OccurredAt                                                   time.Time
}
type RevokeOtherSessionResult struct{ Replay, Revoked bool }
type RevokeAllSessionsCommit struct {
	AuditID, OutboxID, CorrelationID ID
	Principal                        SessionPrincipal
	OccurredAt                       time.Time
}
type RevokeAllSessionsResult struct {
	AuthEpoch    int64
	RevokedCount int32
}
type SessionAdministrationStore interface {
	RevokeOtherSession(context.Context, RevokeOtherSessionCommit) (RevokeOtherSessionResult, error)
	RevokeAllSessions(context.Context, RevokeAllSessionsCommit) (RevokeAllSessionsResult, error)
}
type SessionAdministrationService struct {
	store SessionAdministrationStore
	ids   LoginIDGenerator
	clock LoginClock
}

func NewSessionAdministrationService(s SessionAdministrationStore, ids LoginIDGenerator, c LoginClock) (*SessionAdministrationService, error) {
	if s == nil || ids == nil || c == nil {
		return nil, errors.New("session_administration_service_invalid")
	}
	return &SessionAdministrationService{s, ids, c}, nil
}
func (s *SessionAdministrationService) RevokeOther(ctx context.Context, in RevokeOtherSessionInput) (RevokeOtherSessionResult, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validSessionAdminPrincipal(in.Principal) || in.TargetSessionID.IsZero() || in.CorrelationID.IsZero() || len(in.IdempotencyKey) < 1 || len(in.IdempotencyKey) > 200 {
		return RevokeOtherSessionResult{}, safe("session_administration_invalid", nil)
	}
	if !freshSession(in.SessionAuthenticatedAt, now) {
		return RevokeOtherSessionResult{}, safe("session_administration_fresh_auth_required", nil)
	}
	if in.TargetSessionID == in.Principal.SessionID {
		return RevokeOtherSessionResult{}, safe("session_administration_current_requires_logout", nil)
	}
	ids, e := newIdentityIDs(s.ids, 3)
	if e != nil {
		return RevokeOtherSessionResult{}, safe("session_administration_unavailable", e)
	}
	raw, _ := json.Marshal(struct {
		User, Current, Target ID
		Epoch                 int64
	}{in.Principal.UserID, in.Principal.SessionID, in.TargetSessionID, in.Principal.AuthEpoch})
	v, e := s.store.RevokeOtherSession(ctx, RevokeOtherSessionCommit{CommandID: ids[0], AuditID: ids[1], OutboxID: ids[2], TargetSessionID: in.TargetSessionID, CorrelationID: in.CorrelationID, Principal: in.Principal, KeyHash: sha256.Sum256([]byte("session-revoke-other:v1:" + in.IdempotencyKey)), RequestHash: sha256.Sum256(raw), OccurredAt: now})
	if errors.Is(e, ErrSessionRevocationConflict) {
		return v, safe("session_administration_conflict", e)
	}
	if e != nil {
		return v, safe("session_administration_unavailable", e)
	}
	return v, nil
}
func (s *SessionAdministrationService) RevokeAll(ctx context.Context, p SessionPrincipal, correlation ID, authenticatedAt time.Time) (RevokeAllSessionsResult, error) {
	now := s.clock.Now().UTC()
	if ctx == nil || !validSessionAdminPrincipal(p) || correlation.IsZero() {
		return RevokeAllSessionsResult{}, safe("session_administration_invalid", nil)
	}
	if !freshSession(authenticatedAt, now) {
		return RevokeAllSessionsResult{}, safe("session_administration_fresh_auth_required", nil)
	}
	ids, e := newIdentityIDs(s.ids, 2)
	if e != nil {
		return RevokeAllSessionsResult{}, safe("session_administration_unavailable", e)
	}
	v, e := s.store.RevokeAllSessions(ctx, RevokeAllSessionsCommit{AuditID: ids[0], OutboxID: ids[1], CorrelationID: correlation, Principal: p, OccurredAt: now})
	if errors.Is(e, ErrSessionAdministrationLimit) {
		return v, safe("session_administration_limit", e)
	}
	if errors.Is(e, ErrSessionAdministrationInvalid) {
		return v, safe("session_administration_invalid", e)
	}
	if e != nil {
		return v, safe("session_administration_unavailable", e)
	}
	return v, nil
}
func validSessionAdminPrincipal(p SessionPrincipal) bool {
	return !p.SessionID.IsZero() && !p.UserID.IsZero() && p.AuthEpoch > 0 && !p.AbsoluteExpiresAt.IsZero()
}
func freshSession(a, n time.Time) bool {
	if a.IsZero() || n.IsZero() {
		return false
	}
	age := n.Sub(a.UTC())
	return age >= 0 && age <= 15*time.Minute
}
func newIdentityIDs(g LoginIDGenerator, n int) ([]ID, error) {
	r := make([]ID, n)
	for i := range r {
		v, e := g.NewID()
		if e != nil || v.IsZero() {
			return nil, e
		}
		r[i] = v
	}
	return r, nil
}
