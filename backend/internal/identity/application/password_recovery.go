package application

import (
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/verification"
	"context"
	"errors"
	"time"
)

var ErrPasswordRecoveryInvalid = errors.New("password_recovery_invalid")
var ErrRecoveryAttemptDenied = errors.New("recovery_attempt_denied")

type PasswordRecoveryIssueCommand struct {
	ChallengeID, AuditID, CorrelationID ID
	Email                               email.Address
	TokenHash                           [32]byte
	CreatedAt                           time.Time
}
type PasswordRecoveryIssuance struct {
	Issued    bool
	UserID    ID
	Recipient string
	ExpiresAt time.Time
}
type PasswordRecoveryIssuer interface {
	IssuePasswordRecovery(context.Context, PasswordRecoveryIssueCommand) (PasswordRecoveryIssuance, error)
}
type PasswordRecoveryDelivery interface {
	DeliverPasswordRecovery(context.Context, email.Address, verification.Token, time.Time) error
}
type PasswordRecoveryDeliveryRequest struct {
	ChallengeID, UserID, CorrelationID ID
	Recipient                          email.Address
	Token                              verification.Token
	ExpiresAt                          time.Time
}
type EvidencedPasswordRecoveryDelivery interface {
	DeliverPasswordRecoveryWithEvidence(context.Context, PasswordRecoveryDeliveryRequest) error
}
type PasswordRecoveryRequestService struct {
	issuer   PasswordRecoveryIssuer
	delivery PasswordRecoveryDelivery
	tokens   VerificationTokenGenerator
	ids      LoginIDGenerator
	clock    LoginClock
}

func NewPasswordRecoveryRequestService(issuer PasswordRecoveryIssuer, delivery PasswordRecoveryDelivery, tokens VerificationTokenGenerator, ids LoginIDGenerator, clock LoginClock) (*PasswordRecoveryRequestService, error) {
	if issuer == nil || delivery == nil || tokens == nil || ids == nil || clock == nil {
		return nil, errors.New("password_recovery_request_service_invalid")
	}
	return &PasswordRecoveryRequestService{issuer, delivery, tokens, ids, clock}, nil
}
func (s *PasswordRecoveryRequestService) Request(ctx context.Context, emailText string, correlationID ID) error {
	address, err := email.Parse(emailText)
	if ctx == nil || err != nil || correlationID.IsZero() {
		return safe("password_recovery_request_invalid", nil)
	}
	token, err := s.tokens.Generate()
	if err != nil {
		return safe("password_recovery_request_unavailable", err)
	}
	challengeID, err := s.ids.NewID()
	if err != nil {
		return safe("password_recovery_request_unavailable", err)
	}
	auditID, err := s.ids.NewID()
	if err != nil {
		return safe("password_recovery_request_unavailable", err)
	}
	at := s.clock.Now().UTC()
	issued, err := s.issuer.IssuePasswordRecovery(ctx, PasswordRecoveryIssueCommand{ChallengeID: challengeID, AuditID: auditID, CorrelationID: correlationID, Email: address, TokenHash: token.Hash(), CreatedAt: at})
	if err != nil {
		return safe("password_recovery_request_unavailable", err)
	}
	if !issued.Issued {
		return nil
	}
	recipient, err := email.Parse(issued.Recipient)
	if err != nil {
		return safe("password_recovery_request_unavailable", err)
	}
	if delivery, ok := s.delivery.(EvidencedPasswordRecoveryDelivery); ok {
		if issued.UserID.IsZero() {
			return safe("password_recovery_request_unavailable", nil)
		}
		_ = delivery.DeliverPasswordRecoveryWithEvidence(ctx, PasswordRecoveryDeliveryRequest{ChallengeID: challengeID, UserID: issued.UserID, CorrelationID: correlationID, Recipient: recipient, Token: token, ExpiresAt: issued.ExpiresAt})
	} else {
		_ = s.delivery.DeliverPasswordRecovery(ctx, recipient, token, issued.ExpiresAt)
	}
	return nil
}

type RecoveryAttemptController interface {
	Allow(context.Context, attempt.Scopes) error
}
type RecoveryAttemptKeyer interface {
	RecoveryIdentity(string) (attempt.Key, error)
}
type PasswordRecoveryRequester interface {
	Request(context.Context, string, ID) error
}
type PublicPasswordRecoveryInput struct {
	Email         string
	CorrelationID ID
	NetworkScope  attempt.Key
}
type PublicPasswordRecoveryService struct {
	recovery     PasswordRecoveryRequester
	attempts     RecoveryAttemptController
	keyer        RecoveryAttemptKeyer
	availability EmailLoginAvailability
}

func NewPublicPasswordRecoveryService(recovery PasswordRecoveryRequester, attempts RecoveryAttemptController, keyer RecoveryAttemptKeyer, availability ...EmailLoginAvailability) (*PublicPasswordRecoveryService, error) {
	if recovery == nil || attempts == nil || keyer == nil || len(availability) > 1 || len(availability) == 1 && availability[0] == nil {
		return nil, errors.New("public_password_recovery_service_invalid")
	}
	s := &PublicPasswordRecoveryService{recovery: recovery, attempts: attempts, keyer: keyer}
	if len(availability) == 1 {
		s.availability = availability[0]
	}
	return s, nil
}
func (s *PublicPasswordRecoveryService) Request(ctx context.Context, in PublicPasswordRecoveryInput) error {
	if ctx == nil || in.CorrelationID.IsZero() || in.NetworkScope.IsZero() {
		return safe("password_recovery_request_invalid", nil)
	}
	lookup := in.Email
	if address, err := email.Parse(in.Email); err == nil {
		lookup = address.Lookup()
	}
	if lookup == "" {
		lookup = "\x00"
	}
	identityScope, err := s.keyer.RecoveryIdentity(lookup)
	if err != nil {
		return safe("password_recovery_request_unavailable", err)
	}
	if err = s.attempts.Allow(ctx, attempt.Scopes{Identity: identityScope, Network: in.NetworkScope}); errors.Is(err, ErrRecoveryAttemptDenied) {
		return safe("password_recovery_attempt_denied", nil)
	} else if err != nil {
		return safe("password_recovery_request_unavailable", err)
	}
	// This checks the globally configured delivery dependency, not whether an
	// account exists. Do not acknowledge a request when no SMTP can send it.
	if s.availability != nil && !s.availability.EmailLoginAvailable(ctx) {
		return safe("password_recovery_request_unavailable", nil)
	}
	err = s.recovery.Request(ctx, in.Email, in.CorrelationID)
	if err != nil && err.Error() == "password_recovery_request_invalid" {
		return nil
	}
	return err
}

type PasswordRecoveryResetInput struct {
	Token         string
	NewPassword   []byte
	CorrelationID ID
}
type PasswordRecoveryResetCommit struct {
	AuditID, CorrelationID ID
	TokenHash              [32]byte
	PasswordPHC            string
	ResetAt                time.Time
}
type PasswordRecoveryResetCommitter interface {
	CommitPasswordRecoveryReset(context.Context, PasswordRecoveryResetCommit) error
}
type PasswordRecoveryResetService struct {
	committer PasswordRecoveryResetCommitter
	password  RegistrationPassword
	ids       LoginIDGenerator
	clock     LoginClock
}

func NewPasswordRecoveryResetService(committer PasswordRecoveryResetCommitter, password RegistrationPassword, ids LoginIDGenerator, clock LoginClock) (*PasswordRecoveryResetService, error) {
	if committer == nil || password == nil || ids == nil || clock == nil {
		return nil, errors.New("password_recovery_reset_service_invalid")
	}
	return &PasswordRecoveryResetService{committer, password, ids, clock}, nil
}
func (s *PasswordRecoveryResetService) Reset(ctx context.Context, in PasswordRecoveryResetInput) error {
	hash, err := verification.HashPresented(in.Token)
	if ctx == nil || err != nil || in.CorrelationID.IsZero() || s.password.ValidateNew(in.NewPassword) != nil {
		return safe("password_recovery_reset_invalid", nil)
	}
	phc, err := s.password.Hash(in.NewPassword)
	if err != nil {
		return safe("password_recovery_reset_unavailable", err)
	}
	auditID, err := s.ids.NewID()
	if err != nil {
		return safe("password_recovery_reset_unavailable", err)
	}
	err = s.committer.CommitPasswordRecoveryReset(ctx, PasswordRecoveryResetCommit{AuditID: auditID, CorrelationID: in.CorrelationID, TokenHash: hash, PasswordPHC: phc, ResetAt: s.clock.Now().UTC()})
	if errors.Is(err, ErrPasswordRecoveryInvalid) {
		return safe("password_recovery_reset_invalid", nil)
	}
	if err != nil {
		return safe("password_recovery_reset_unavailable", err)
	}
	return nil
}
