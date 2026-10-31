package application

import (
	"context"
	"errors"

	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/email"
)

var ErrRegistrationAttemptDenied = errors.New("registration_attempt_denied")

type RegistrationAttemptController interface {
	Allow(context.Context, attempt.Scopes) error
}
type RegistrationAttemptKeyer interface {
	RegistrationIdentity(string) (attempt.Key, error)
}
type VerificationIssueUseCase interface {
	Issue(context.Context, EmailVerificationIssueInput) (EmailVerificationIssueResult, error)
}
type OpenRegistrationInput struct {
	Registration SelfRegistrationInput
	NetworkScope attempt.Key
}
type OpenRegistrationResult struct{ Accepted, Created, VerificationIssued bool }
type OpenRegistrationService struct {
	registrations *SelfRegistrationService
	attempts      RegistrationAttemptController
	keyer         RegistrationAttemptKeyer
	verification  VerificationIssueUseCase
}

func NewOpenRegistrationService(registrations *SelfRegistrationService, attempts RegistrationAttemptController, keyer RegistrationAttemptKeyer, verification VerificationIssueUseCase) (*OpenRegistrationService, error) {
	if registrations == nil || attempts == nil || keyer == nil || verification == nil {
		return nil, errors.New("open_registration_service_invalid")
	}
	return &OpenRegistrationService{registrations, attempts, keyer, verification}, nil
}
func (s *OpenRegistrationService) Register(ctx context.Context, input OpenRegistrationInput) (OpenRegistrationResult, error) {
	if ctx == nil || input.Registration.CorrelationID.IsZero() || input.NetworkScope.IsZero() {
		return OpenRegistrationResult{}, safe("registration_context_required", nil)
	}
	if s == nil || s.registrations == nil || s.attempts == nil || s.keyer == nil || s.verification == nil {
		return OpenRegistrationResult{}, safe("registration_unavailable", nil)
	}
	address, parseErr := email.Parse(input.Registration.Email)
	lookup := input.Registration.Email
	if parseErr == nil {
		lookup = address.Lookup()
	}
	if lookup == "" {
		lookup = "\x00"
	}
	identityScope, err := s.keyer.RegistrationIdentity(lookup)
	if err != nil {
		return OpenRegistrationResult{}, safe("registration_unavailable", err)
	}
	if err = s.attempts.Allow(ctx, attempt.Scopes{Identity: identityScope, Network: input.NetworkScope}); err != nil {
		if errors.Is(err, ErrRegistrationAttemptDenied) {
			return OpenRegistrationResult{}, safe("registration_attempt_denied", nil)
		}
		return OpenRegistrationResult{}, safe("registration_unavailable", err)
	}
	registered, err := s.registrations.Register(ctx, input.Registration)
	if err != nil {
		return OpenRegistrationResult{}, err
	}
	result := OpenRegistrationResult{Accepted: registered.Accepted, Created: registered.Created}
	if !registered.Created {
		return result, nil
	}
	_, issueErr := s.verification.Issue(ctx, EmailVerificationIssueInput{UserID: registered.UserID, CorrelationID: input.Registration.CorrelationID})
	result.VerificationIssued = issueErr == nil
	return result, nil
}
