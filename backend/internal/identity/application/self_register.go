package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"bidora/backend/internal/identity/email"
)

var ErrIdentityAlreadyExists = errors.New("identity_already_exists")

type RegistrationPassword interface {
	ValidateNew([]byte) error
	Hash([]byte) (string, error)
}
type SelfRegistrationCommitter interface {
	CommitSelfRegistration(context.Context, SelfRegistrationCommit) error
}
type SelfRegistrationCommit struct {
	UserID, AuditID, CorrelationID ID
	Email                          email.Address
	FirstName, LastName            string
	PasswordPHC                    string
	CreatedAt                      time.Time
}

func (SelfRegistrationCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED self registration commit]")
}

type SelfRegistrationInput struct {
	FirstName, LastName string
	Email               string
	Password            []byte
	CorrelationID       ID
}
type SelfRegistrationResult struct {
	Accepted bool
	Created  bool
	UserID   ID
}

func (SelfRegistrationResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED self registration result]")
}

type SelfRegistrationService struct {
	passwords RegistrationPassword
	ids       LoginIDGenerator
	clock     LoginClock
	committer SelfRegistrationCommitter
}

func NewSelfRegistrationService(passwords RegistrationPassword, ids LoginIDGenerator, clock LoginClock, committer SelfRegistrationCommitter) (*SelfRegistrationService, error) {
	if passwords == nil || ids == nil || clock == nil || committer == nil {
		return nil, errors.New("self_registration_service_invalid")
	}
	return &SelfRegistrationService{passwords, ids, clock, committer}, nil
}
func (s *SelfRegistrationService) Register(ctx context.Context, input SelfRegistrationInput) (SelfRegistrationResult, error) {
	if ctx == nil || input.CorrelationID.IsZero() {
		return SelfRegistrationResult{}, safe("registration_invalid", nil)
	}
	if s == nil || s.passwords == nil || s.ids == nil || s.clock == nil || s.committer == nil {
		return SelfRegistrationResult{}, safe("registration_unavailable", nil)
	}
	address, err := email.Parse(input.Email)
	if err != nil || !ValidPersonalName(input.FirstName) || !ValidPersonalName(input.LastName) || s.passwords.ValidateNew(input.Password) != nil {
		return SelfRegistrationResult{}, safe("registration_invalid", nil)
	}
	phc, err := s.passwords.Hash(input.Password)
	if err != nil || phc == "" {
		return SelfRegistrationResult{}, safe("registration_unavailable", err)
	}
	userID, err := s.ids.NewID()
	if err != nil || userID.IsZero() {
		return SelfRegistrationResult{}, safe("registration_unavailable", err)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return SelfRegistrationResult{}, safe("registration_unavailable", err)
	}
	createdAt := s.clock.Now().UTC()
	if createdAt.IsZero() {
		return SelfRegistrationResult{}, safe("registration_unavailable", nil)
	}
	err = s.committer.CommitSelfRegistration(ctx, SelfRegistrationCommit{UserID: userID, AuditID: auditID, CorrelationID: input.CorrelationID, Email: address, FirstName: input.FirstName, LastName: input.LastName, PasswordPHC: phc, CreatedAt: createdAt})
	if errors.Is(err, ErrIdentityAlreadyExists) {
		return SelfRegistrationResult{Accepted: true}, nil
	}
	if err != nil {
		return SelfRegistrationResult{}, safe("registration_unavailable", err)
	}
	return SelfRegistrationResult{Accepted: true, Created: true, UserID: userID}, nil
}

// ValidPersonalName enforces the approved display-only registration policy.
// Spelling is preserved exactly; names are not normalized or used as authority.
func ValidPersonalName(value string) bool {
	if !utf8.ValidString(value) || len(value) < 1 || len(value) > 100 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
