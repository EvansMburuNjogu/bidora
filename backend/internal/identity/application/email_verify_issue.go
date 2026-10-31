package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
)

var ErrVerificationTargetUnavailable = errors.New("verification_target_unavailable")

type VerificationTokenGenerator interface {
	Generate() (verification.Token, error)
}
type SecureVerificationTokenGenerator struct{}

func (SecureVerificationTokenGenerator) Generate() (verification.Token, error) {
	return verification.Generate()
}

type EmailVerificationIssuer interface {
	IssueEmailVerification(context.Context, EmailVerificationChallenge) (EmailVerificationIssuance, error)
}
type EmailVerificationChallenge struct {
	ChallengeID, UserID, ActorID, AuditID, CorrelationID ID
	TokenHash                                            [32]byte
	CreatedAt, ExpiresAt                                 time.Time
}
type EmailVerificationIssuance struct {
	UserID    ID
	Recipient string
	ExpiresAt time.Time
}

func (EmailVerificationIssuance) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED email verification issuance]")
}

type EmailVerificationDelivery interface {
	DeliverEmailVerification(context.Context, email.Address, verification.Token, time.Time) error
}
type EmailVerificationDeliveryRequest struct {
	ChallengeID, UserID, ActorID, CorrelationID ID
	Recipient                                   email.Address
	Token                                       verification.Token
	ExpiresAt                                   time.Time
}
type EvidencedEmailVerificationDelivery interface {
	DeliverEmailVerificationWithEvidence(context.Context, EmailVerificationDeliveryRequest) error
}
type EmailVerificationIssueInput struct{ UserID, ActorID, CorrelationID ID }
type EmailVerificationIssueResult struct {
	UserID    ID
	ExpiresAt time.Time
}

func (EmailVerificationIssueResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED email verification issue result]")
}

type EmailVerificationIssueService struct {
	issuer   EmailVerificationIssuer
	delivery EmailVerificationDelivery
	tokens   VerificationTokenGenerator
	ids      LoginIDGenerator
	clock    LoginClock
}

func NewEmailVerificationIssueService(issuer EmailVerificationIssuer, delivery EmailVerificationDelivery, tokens VerificationTokenGenerator, ids LoginIDGenerator, clock LoginClock) (*EmailVerificationIssueService, error) {
	if issuer == nil || delivery == nil || tokens == nil || ids == nil || clock == nil {
		return nil, errors.New("email_verification_issue_service_invalid")
	}
	return &EmailVerificationIssueService{issuer, delivery, tokens, ids, clock}, nil
}
func (s *EmailVerificationIssueService) Issue(ctx context.Context, input EmailVerificationIssueInput) (EmailVerificationIssueResult, error) {
	if ctx == nil || input.UserID.IsZero() || input.CorrelationID.IsZero() {
		return EmailVerificationIssueResult{}, safe("verification_issuance_invalid", nil)
	}
	if s == nil || s.issuer == nil || s.delivery == nil || s.tokens == nil || s.ids == nil || s.clock == nil {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", nil)
	}
	token, err := s.tokens.Generate()
	if err != nil || token.IsZero() {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", err)
	}
	challengeID, err := s.ids.NewID()
	if err != nil || challengeID.IsZero() {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", err)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", err)
	}
	createdAt := s.clock.Now().UTC()
	expiresAt, err := verification.ExpiresAt(createdAt)
	if err != nil {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", err)
	}
	issued, err := s.issuer.IssueEmailVerification(ctx, EmailVerificationChallenge{ChallengeID: challengeID, UserID: input.UserID, ActorID: input.ActorID, AuditID: auditID, CorrelationID: input.CorrelationID, TokenHash: token.Hash(), CreatedAt: createdAt, ExpiresAt: expiresAt})
	if errors.Is(err, ErrVerificationTargetUnavailable) {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", nil)
	}
	if err != nil {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", err)
	}
	recipient, err := email.Parse(issued.Recipient)
	if err != nil || issued.UserID != input.UserID || !issued.ExpiresAt.Equal(expiresAt) {
		return EmailVerificationIssueResult{}, safe("verification_issuance_unavailable", err)
	}
	if delivery, ok := s.delivery.(EvidencedEmailVerificationDelivery); ok {
		err = delivery.DeliverEmailVerificationWithEvidence(ctx, EmailVerificationDeliveryRequest{ChallengeID: challengeID, UserID: input.UserID, ActorID: input.ActorID, CorrelationID: input.CorrelationID, Recipient: recipient, Token: token, ExpiresAt: expiresAt})
	} else {
		err = s.delivery.DeliverEmailVerification(ctx, recipient, token, expiresAt)
	}
	if err != nil {
		return EmailVerificationIssueResult{}, safe("verification_delivery_unavailable", err)
	}
	return EmailVerificationIssueResult{UserID: input.UserID, ExpiresAt: expiresAt}, nil
}
