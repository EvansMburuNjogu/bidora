package application

import (
	"context"
	"errors"
	"time"

	"bidora/backend/internal/identity/session"
)

type AuthenticationMethod string

const (
	AuthenticationMethodPassword AuthenticationMethod = "PASSWORD"
	AuthenticationMethodGoogle   AuthenticationMethod = "GOOGLE"
)

type AuthenticatedSessionInput struct {
	UserID, CorrelationID           ID
	AuthEpoch                       int64
	Method                          AuthenticationMethod
	AssuranceLevel, AssuranceMethod string
	AssuredAt                       time.Time
}
type SessionIssuer struct {
	tokens    SessionTokenGenerator
	ids       LoginIDGenerator
	clock     LoginClock
	committer SessionCommitter
}

func NewSessionIssuer(tokens SessionTokenGenerator, ids LoginIDGenerator, clock LoginClock, committer SessionCommitter) (*SessionIssuer, error) {
	if tokens == nil || ids == nil || clock == nil || committer == nil {
		return nil, errors.New("session_issuer_invalid")
	}
	return &SessionIssuer{tokens, ids, clock, committer}, nil
}
func (s *SessionIssuer) Issue(ctx context.Context, input AuthenticatedSessionInput) (LoginResult, error) {
	if ctx == nil || input.UserID.IsZero() || input.CorrelationID.IsZero() || input.AuthEpoch < 1 || input.Method != AuthenticationMethodPassword && input.Method != AuthenticationMethodGoogle {
		return LoginResult{}, safe("authentication_context_required", nil)
	}
	if s == nil || s.tokens == nil || s.ids == nil || s.clock == nil || s.committer == nil {
		return LoginResult{}, safe("authentication_unavailable", nil)
	}
	pair, err := s.tokens.GenerateSessionPair()
	if err != nil || pair.Session.IsZero() || pair.CSRF.IsZero() {
		return LoginResult{}, safe("authentication_unavailable", err)
	}
	sessionID, err := s.ids.NewID()
	if err != nil || sessionID.IsZero() {
		return LoginResult{}, safe("authentication_unavailable", err)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return LoginResult{}, safe("authentication_unavailable", err)
	}
	createdAt := s.clock.Now().UTC()
	idle, absolute, err := session.Expiries(createdAt)
	if err != nil {
		return LoginResult{}, safe("authentication_unavailable", err)
	}
	level, method, assured := input.AssuranceLevel, input.AssuranceMethod, input.AssuredAt.UTC()
	if level == "" {
		level, method, assured = "AAL1", "PRIMARY_ONLY", createdAt
	}
	if (level != "AAL1" && level != "AAL2") || assured.IsZero() {
		return LoginResult{}, safe("authentication_context_required", nil)
	}
	commit := SessionCommit{SessionID: sessionID, AuditID: auditID, UserID: input.UserID, CorrelationID: input.CorrelationID, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), AuthEpoch: input.AuthEpoch, AuthenticationMethod: input.Method, AssuranceLevel: level, AssuranceMethod: method, AssuredAt: assured, CreatedAt: createdAt, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if err = s.committer.CommitLoginSession(ctx, commit); err != nil {
		return LoginResult{}, safe("authentication_unavailable", err)
	}
	return LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, nil
}
