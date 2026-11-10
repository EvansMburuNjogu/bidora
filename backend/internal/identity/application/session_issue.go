package application

import (
	"bidora/backend/internal/identity/securitypolicy"
	"context"
	"errors"
	"time"
)

type AuthenticationMethod string

const (
	AuthenticationMethodPassword            AuthenticationMethod = "PASSWORD"
	AuthenticationMethodGoogle              AuthenticationMethod = "GOOGLE"
	AuthenticationMethodSessionContinuation AuthenticationMethod = "SESSION_CONTINUATION"
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
	if ctx == nil || input.UserID.IsZero() || input.CorrelationID.IsZero() || input.AuthEpoch < 1 || input.Method != AuthenticationMethodPassword && input.Method != AuthenticationMethodGoogle && input.Method != AuthenticationMethodSessionContinuation {
		return LoginResult{}, safe("authentication_context_required", nil)
	}
	if s == nil || s.tokens == nil || s.ids == nil || s.clock == nil || s.committer == nil {
		return LoginResult{}, safe("authentication_unavailable", nil)
	}
	administrator := false
	if classifier, ok := s.committer.(interface {
		SessionAdministrator(context.Context, ID) (bool, error)
	}); ok {
		var err error
		administrator, err = classifier.SessionAdministrator(ctx, input.UserID)
		if err != nil {
			return LoginResult{}, safe("authentication_unavailable", err)
		}
	}
	commit, result, err := s.Prepare(ctx, input, administrator)
	if err != nil {
		return LoginResult{}, err
	}
	if err = s.committer.CommitLoginSession(ctx, commit); err != nil {
		return LoginResult{}, safe("authentication_unavailable", err)
	}
	return result, nil
}

// Prepare creates credentials without committing them. Invitation promotion uses
// its existing transaction to commit the replacement session with the role grant.
func (s *SessionIssuer) Prepare(ctx context.Context, input AuthenticatedSessionInput, administrator bool) (SessionCommit, LoginResult, error) {
	if ctx == nil || s == nil || s.tokens == nil || s.ids == nil || s.clock == nil || input.UserID.IsZero() || input.AuthEpoch < 1 || input.CorrelationID.IsZero() || input.Method != AuthenticationMethodPassword && input.Method != AuthenticationMethodGoogle && input.Method != AuthenticationMethodSessionContinuation {
		return SessionCommit{}, LoginResult{}, safe("authentication_context_required", nil)
	}
	pair, err := s.tokens.GenerateSessionPair()
	if err != nil || pair.Session.IsZero() || pair.CSRF.IsZero() {
		return SessionCommit{}, LoginResult{}, safe("authentication_unavailable", err)
	}
	sessionID, err := s.ids.NewID()
	if err != nil || sessionID.IsZero() {
		return SessionCommit{}, LoginResult{}, safe("authentication_unavailable", err)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return SessionCommit{}, LoginResult{}, safe("authentication_unavailable", err)
	}
	createdAt := s.clock.Now().UTC()
	idle, absolute, err := securitypolicy.Expiries(ctx, createdAt, administrator)
	if err != nil {
		return SessionCommit{}, LoginResult{}, safe("authentication_unavailable", err)
	}
	level, method, assured := input.AssuranceLevel, input.AssuranceMethod, input.AssuredAt.UTC()
	if level == "" {
		level, method, assured = "AAL1", "PRIMARY_ONLY", createdAt
	}
	if (level != "AAL1" && level != "AAL2") || assured.IsZero() || assured.After(createdAt) {
		return SessionCommit{}, LoginResult{}, safe("authentication_context_required", nil)
	}
	commit := SessionCommit{SessionID: sessionID, AuditID: auditID, UserID: input.UserID, CorrelationID: input.CorrelationID, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), AuthEpoch: input.AuthEpoch, AuthenticationMethod: input.Method, AssuranceLevel: level, AssuranceMethod: method, AssuredAt: assured, CreatedAt: createdAt, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	return commit, LoginResult{SessionID: sessionID, Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, nil
}
