package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/identity/verification"
)

var ErrVerificationNotFound = errors.New("verification_not_found")

type EmailVerificationCommitter interface {
	ConsumeEmailVerification(context.Context, [32]byte, ID, ID, time.Time) (EmailVerificationResult, error)
}
type EmailVerificationResult struct {
	UserID     ID
	AuthEpoch  int64
	VerifiedAt time.Time
}

func (EmailVerificationResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED email verification result]")
}

type EmailVerificationInput struct {
	Token         string
	CorrelationID ID
}
type EmailVerificationService struct {
	committer EmailVerificationCommitter
	ids       LoginIDGenerator
	clock     LoginClock
}

func NewEmailVerificationService(committer EmailVerificationCommitter, ids LoginIDGenerator, clock LoginClock) (*EmailVerificationService, error) {
	if committer == nil || ids == nil || clock == nil {
		return nil, errors.New("email_verification_service_invalid")
	}
	return &EmailVerificationService{committer, ids, clock}, nil
}
func (s *EmailVerificationService) Verify(ctx context.Context, input EmailVerificationInput) (EmailVerificationResult, error) {
	if ctx == nil || input.CorrelationID.IsZero() {
		return EmailVerificationResult{}, safe("verification_invalid", nil)
	}
	if s == nil || s.committer == nil || s.ids == nil || s.clock == nil {
		return EmailVerificationResult{}, safe("verification_unavailable", nil)
	}
	tokenHash, err := verification.HashPresented(input.Token)
	if err != nil {
		return EmailVerificationResult{}, safe("verification_invalid", nil)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return EmailVerificationResult{}, safe("verification_unavailable", err)
	}
	at := s.clock.Now().UTC()
	if at.IsZero() {
		return EmailVerificationResult{}, safe("verification_unavailable", nil)
	}
	result, err := s.committer.ConsumeEmailVerification(ctx, tokenHash, auditID, input.CorrelationID, at)
	if errors.Is(err, ErrVerificationNotFound) {
		return EmailVerificationResult{}, safe("verification_invalid", nil)
	}
	if err != nil {
		return EmailVerificationResult{}, safe("verification_unavailable", err)
	}
	if result.UserID.IsZero() || result.AuthEpoch < 1 || result.VerifiedAt.IsZero() {
		return EmailVerificationResult{}, safe("verification_unavailable", nil)
	}
	return EmailVerificationResult{result.UserID, result.AuthEpoch, result.VerifiedAt.UTC()}, nil
}
