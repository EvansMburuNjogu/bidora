package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"bidora/backend/internal/identity/session"
)

var ErrPasswordChangeConflict = errors.New("password_change_conflict")

type PasswordReauthentication interface {
	Reauthenticate(context.Context, GoogleLinkReauthenticationInput) (GoogleLinkProof, error)
}
type PasswordChangeCommitter interface {
	CommitPasswordChange(context.Context, PasswordChangeCommit) error
}
type PasswordChangeInput struct {
	Principal     SessionPrincipal
	Credentials   AuthenticateInput
	NewPassword   []byte
	CorrelationID ID
}
type PasswordChangeCommit struct {
	UserID, CurrentSessionID, ReplacementSessionID, PasswordAuditID, LoginAuditID, CorrelationID ID
	ExpectedAuthEpoch, ExpectedCredentialVersion, NewAuthEpoch, NewCredentialVersion             int64
	PasswordPHC                                                                                  string
	SessionHash, CSRFHash                                                                        [32]byte
	ChangedAt, IdleExpiresAt, AbsoluteExpiresAt                                                  time.Time
}

func (PasswordChangeInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED password change input]")
}
func (PasswordChangeCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED password change commit]")
}

type PasswordChangeService struct {
	reauth    PasswordReauthentication
	password  RegistrationPassword
	tokens    SessionTokenGenerator
	ids       LoginIDGenerator
	clock     LoginClock
	committer PasswordChangeCommitter
}

func NewPasswordChangeService(reauth PasswordReauthentication, password RegistrationPassword, tokens SessionTokenGenerator, ids LoginIDGenerator, clock LoginClock, committer PasswordChangeCommitter) (*PasswordChangeService, error) {
	if reauth == nil || password == nil || tokens == nil || ids == nil || clock == nil || committer == nil {
		return nil, errors.New("password_change_service_invalid")
	}
	return &PasswordChangeService{reauth, password, tokens, ids, clock, committer}, nil
}
func (s *PasswordChangeService) Change(ctx context.Context, input PasswordChangeInput) (LoginResult, error) {
	if ctx == nil || input.Principal.UserID.IsZero() || input.Principal.SessionID.IsZero() || input.Principal.AuthEpoch < 1 || input.CorrelationID.IsZero() {
		return LoginResult{}, safe("password_change_invalid", nil)
	}
	if s == nil || s.reauth == nil || s.password == nil || s.tokens == nil || s.ids == nil || s.clock == nil || s.committer == nil {
		return LoginResult{}, safe("password_change_unavailable", nil)
	}
	if err := s.password.ValidateNew(input.NewPassword); err != nil {
		return LoginResult{}, safe("password_change_new_password_invalid", nil)
	}
	proof, err := s.reauth.Reauthenticate(ctx, GoogleLinkReauthenticationInput{Principal: input.Principal, Credentials: input.Credentials})
	if err != nil {
		if err.Error() == "attempt_denied" {
			return LoginResult{}, err
		}
		if err.Error() == "google_link_reauthentication_invalid" {
			return LoginResult{}, safe("password_change_credentials_invalid", nil)
		}
		return LoginResult{}, safe("password_change_unavailable", err)
	}
	if proof.AuthEpoch == math.MaxInt64 || proof.CredentialVersion == math.MaxInt64 {
		return LoginResult{}, safe("password_change_unavailable", nil)
	}
	phc, err := s.password.Hash(input.NewPassword)
	if err != nil {
		return LoginResult{}, safe("password_change_unavailable", err)
	}
	pair, err := s.tokens.GenerateSessionPair()
	if err != nil || pair.Session.IsZero() || pair.CSRF.IsZero() {
		return LoginResult{}, safe("password_change_unavailable", err)
	}
	ids := make([]ID, 3)
	for index := range ids {
		ids[index], err = s.ids.NewID()
		if err != nil || ids[index].IsZero() {
			return LoginResult{}, safe("password_change_unavailable", err)
		}
	}
	changedAt := s.clock.Now().UTC()
	idle, absolute, err := session.Expiries(changedAt)
	if changedAt.IsZero() || err != nil {
		return LoginResult{}, safe("password_change_unavailable", err)
	}
	commit := PasswordChangeCommit{UserID: proof.UserID, CurrentSessionID: proof.SessionID, ReplacementSessionID: ids[0], PasswordAuditID: ids[1], LoginAuditID: ids[2], CorrelationID: input.CorrelationID, ExpectedAuthEpoch: proof.AuthEpoch, ExpectedCredentialVersion: proof.CredentialVersion, NewAuthEpoch: proof.AuthEpoch + 1, NewCredentialVersion: proof.CredentialVersion + 1, PasswordPHC: phc, SessionHash: pair.Session.Hash(), CSRFHash: pair.CSRF.Hash(), ChangedAt: changedAt, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}
	if err = s.committer.CommitPasswordChange(ctx, commit); errors.Is(err, ErrPasswordChangeConflict) {
		return LoginResult{}, safe("password_change_invalid", nil)
	} else if err != nil {
		return LoginResult{}, safe("password_change_unavailable", err)
	}
	return LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: idle, AbsoluteExpiresAt: absolute}, nil
}
