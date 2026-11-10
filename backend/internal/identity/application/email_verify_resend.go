package application

import (
	"context"
	"errors"

	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/email"
)

var ErrVerificationResendAttemptDenied = errors.New("verification_resend_attempt_denied")

type EmailVerificationResendInput struct {
	Email         string
	CorrelationID ID
	NetworkScope  attempt.Key
}

type EmailVerificationResendResult struct{ Accepted, VerificationIssued bool }

type EmailVerificationResendService struct {
	credentials  CredentialReader
	attempts     RegistrationAttemptController
	keyer        RegistrationAttemptKeyer
	verification VerificationIssueUseCase
}

func NewEmailVerificationResendService(credentials CredentialReader, attempts RegistrationAttemptController, keyer RegistrationAttemptKeyer, verification VerificationIssueUseCase) (*EmailVerificationResendService, error) {
	if credentials == nil || attempts == nil || keyer == nil || verification == nil {
		return nil, errors.New("email_verification_resend_service_invalid")
	}
	return &EmailVerificationResendService{credentials: credentials, attempts: attempts, keyer: keyer, verification: verification}, nil
}

// Resend returns the same accepted result for unknown, malformed, verified,
// disabled and ambiguously delivered targets. All requests consume the approved
// registration identity/network limits before any account lookup.
func (s *EmailVerificationResendService) Resend(ctx context.Context, input EmailVerificationResendInput) (EmailVerificationResendResult, error) {
	if ctx == nil || input.CorrelationID.IsZero() || input.NetworkScope.IsZero() {
		return EmailVerificationResendResult{}, safe("verification_resend_invalid", nil)
	}
	if s == nil || s.credentials == nil || s.attempts == nil || s.keyer == nil || s.verification == nil {
		return EmailVerificationResendResult{}, safe("verification_resend_unavailable", nil)
	}
	address, parseErr := email.Parse(input.Email)
	lookup := input.Email
	if parseErr == nil {
		lookup = address.Lookup()
	}
	if lookup == "" {
		lookup = "\x00"
	}
	identityScope, err := s.keyer.RegistrationIdentity(lookup)
	if err != nil {
		return EmailVerificationResendResult{}, safe("verification_resend_unavailable", err)
	}
	if err = s.attempts.Allow(ctx, attempt.Scopes{Identity: identityScope, Network: input.NetworkScope}); err != nil {
		if errors.Is(err, ErrRegistrationAttemptDenied) {
			return EmailVerificationResendResult{}, safe("verification_resend_attempt_denied", err)
		}
		return EmailVerificationResendResult{}, safe("verification_resend_unavailable", err)
	}
	accepted := EmailVerificationResendResult{Accepted: true}
	if parseErr != nil {
		return accepted, nil
	}
	record, err := s.credentials.LookupPasswordCredential(ctx, address.Lookup())
	if errors.Is(err, ErrCredentialNotFound) {
		return accepted, nil
	}
	if err != nil {
		return EmailVerificationResendResult{}, safe("verification_resend_unavailable", err)
	}
	if record.UserID.IsZero() || record.EmailVerifiedAt != nil || record.DisabledAt != nil {
		return accepted, nil
	}
	// Delivery ambiguity is intentionally concealed. Issuance invalidates the
	// previous active challenge atomically before the delivery attempt.
	_, issueErr := s.verification.Issue(ctx, EmailVerificationIssueInput{UserID: record.UserID, CorrelationID: input.CorrelationID})
	accepted.VerificationIssued = issueErr == nil
	return accepted, nil
}
