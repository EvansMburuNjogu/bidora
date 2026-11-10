package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/identity/session"
)

type CredentialAuthenticator interface {
	Authenticate(context.Context, AuthenticateInput) (Authentication, error)
}
type PasswordRehasher interface {
	RehashPassword(context.Context, ID, int64, []byte) error
}
type SessionTokenGenerator interface{ GenerateSessionPair() (session.Pair, error) }
type LoginIDGenerator interface{ NewID() (ID, error) }
type LoginClock interface{ Now() time.Time }

// SessionCommitter must insert the session and security audit in one control transaction.
type SessionCommitter interface {
	CommitLoginSession(context.Context, SessionCommit) error
}

type SessionCommit struct {
	SessionID, AuditID, UserID, CorrelationID              ID
	SessionHash, CSRFHash                                  [32]byte
	AuthEpoch                                              int64
	AuthenticationMethod                                   AuthenticationMethod
	AssuranceLevel, AssuranceMethod                        string
	CreatedAt, IdleExpiresAt, AbsoluteExpiresAt, AssuredAt time.Time
}

func (SessionCommit) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED session commit]") }

type LoginResult struct {
	SessionID                        ID // Internal committed session identity; never serialized by login handlers.
	Session                          session.SessionToken
	CSRF                             session.CSRFToken
	IdleExpiresAt, AbsoluteExpiresAt time.Time
	MFARequired                      bool
	MFAToken                         string
	MFAExpiresAt                     time.Time
}

func (LoginResult) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED login result]") }

type LoginInput struct {
	Credentials   AuthenticateInput
	CorrelationID ID
}

type MFALoginGate interface {
	BeginLogin(context.Context, ID, int64, AuthenticationMethod) (MFALoginGateResult, error)
}
type LoginService struct {
	auth     CredentialAuthenticator
	rehasher PasswordRehasher
	issuer   *SessionIssuer
	mfa      MFALoginGate
}

func NewLoginService(auth CredentialAuthenticator, rehasher PasswordRehasher, tokens SessionTokenGenerator, ids LoginIDGenerator, clock LoginClock, committer SessionCommitter) (*LoginService, error) {
	if auth == nil || rehasher == nil || tokens == nil || ids == nil || clock == nil || committer == nil {
		return nil, errors.New("login_service_invalid")
	}
	issuer, err := NewSessionIssuer(tokens, ids, clock, committer)
	if err != nil {
		return nil, errors.New("login_service_invalid")
	}
	return &LoginService{auth: auth, rehasher: rehasher, issuer: issuer}, nil
}
func (s *LoginService) EnableMFA(g MFALoginGate) error {
	if s == nil || g == nil {
		return errors.New("login_mfa_invalid")
	}
	s.mfa = g
	return nil
}
func (s *LoginService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	if ctx == nil {
		return LoginResult{}, safe("authentication_context_required", nil)
	}
	if s == nil || s.auth == nil || s.rehasher == nil || s.issuer == nil {
		return LoginResult{}, safe("authentication_unavailable", nil)
	}
	if input.CorrelationID.IsZero() {
		return LoginResult{}, safe("authentication_context_required", nil)
	}
	authentication, err := s.auth.Authenticate(ctx, input.Credentials)
	if err != nil {
		return LoginResult{}, err
	}
	if authentication.UserID.IsZero() || authentication.AuthEpoch < 1 || authentication.CredentialVersion < 1 {
		return LoginResult{}, safe("authentication_unavailable", nil)
	}
	if authentication.NeedsPasswordRehash {
		if err = s.rehasher.RehashPassword(ctx, authentication.UserID, authentication.CredentialVersion, input.Credentials.Password); err != nil {
			return LoginResult{}, safe("authentication_unavailable", err)
		}
	}
	if s.mfa != nil {
		challenge, e := s.mfa.BeginLogin(ctx, authentication.UserID, authentication.AuthEpoch, AuthenticationMethodPassword)
		if e != nil {
			return LoginResult{}, e
		}
		if challenge.Required {
			return LoginResult{MFARequired: true, MFAToken: challenge.Token, MFAExpiresAt: challenge.ExpiresAt}, nil
		}
	}
	return s.issuer.Issue(ctx, AuthenticatedSessionInput{UserID: authentication.UserID, CorrelationID: input.CorrelationID, AuthEpoch: authentication.AuthEpoch, Method: AuthenticationMethodPassword})
}

type SecureSessionTokenGenerator struct{}

func (SecureSessionTokenGenerator) GenerateSessionPair() (session.Pair, error) {
	return session.GeneratePair()
}
