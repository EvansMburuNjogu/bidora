package application

import (
	"context"
	"errors"
	"time"
)

// RegistrationContinuationLifetime covers the one-hour registration/resend
// rate-limit window while keeping the browser binding time-limited.
const RegistrationContinuationLifetime = 90 * time.Minute

// RegistrationContinuationStore provides single-use browser binding. It is
// not proof of mailbox ownership and cannot independently authorize a session.
type RegistrationContinuationStore interface {
	Consume(context.Context, string, ID) (bool, error)
}

type RegistrationSessionCommitter interface {
	CommitRegistrationSession(context.Context, SessionCommit) error
}

type RegistrationContinuationService struct {
	store     RegistrationContinuationStore
	issuer    *SessionIssuer
	committer RegistrationSessionCommitter
}

func NewRegistrationContinuationService(store RegistrationContinuationStore, issuer *SessionIssuer, committer RegistrationSessionCommitter) (*RegistrationContinuationService, error) {
	if store == nil || issuer == nil || committer == nil {
		return nil, errors.New("registration_continuation_invalid")
	}
	return &RegistrationContinuationService{store: store, issuer: issuer, committer: committer}, nil
}

// Complete must only receive the result of successful email verification in
// this request. A missing binding is not an authentication failure: verification
// remains valid and the caller must use normal sign-in instead.
func (s *RegistrationContinuationService) Complete(ctx context.Context, secret string, verified EmailVerificationResult, correlation ID) (LoginResult, bool, error) {
	if s == nil || ctx == nil || verified.UserID.IsZero() || verified.AuthEpoch < 1 || correlation.IsZero() {
		return LoginResult{}, false, errors.New("registration_continuation_invalid")
	}
	if secret == "" {
		return LoginResult{}, false, nil
	}
	matched, err := s.store.Consume(ctx, secret, verified.UserID)
	if err != nil {
		return LoginResult{}, false, err
	}
	if !matched {
		return LoginResult{}, false, nil
	}
	commit, result, err := s.issuer.Prepare(ctx, AuthenticatedSessionInput{UserID: verified.UserID, CorrelationID: correlation, AuthEpoch: verified.AuthEpoch, Method: AuthenticationMethodPassword}, false)
	if err != nil {
		return LoginResult{}, false, err
	}
	if err = s.committer.CommitRegistrationSession(ctx, commit); err != nil {
		return LoginResult{}, false, err
	}
	return result, true, nil
}
