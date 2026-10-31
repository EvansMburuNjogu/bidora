// Package application contains identity use cases independent of Fiber, pgx and
// provider SDKs. It does not create sessions or authorize tenant operations.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/email"
	"bidora/backend/internal/identity/password"
)

type ID [16]byte

func (id ID) IsZero() bool { return id == ID{} }

var ErrCredentialNotFound = errors.New("credential_not_found")
var ErrAttemptDenied = errors.New("attempt_denied")

type CredentialRecord struct {
	UserID                       ID
	PasswordPHC                  string
	EmailVerifiedAt, DisabledAt  *time.Time
	AuthEpoch, CredentialVersion int64
}

func (CredentialRecord) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED credential record]") }

type CredentialReader interface {
	LookupPasswordCredential(context.Context, string) (CredentialRecord, error)
}
type AttemptKeyer interface {
	Identity(string) (attempt.Key, error)
}
type AttemptReservation interface {
	Finish(context.Context, bool) error
}
type AttemptController interface {
	Begin(context.Context, attempt.Scopes) (AttemptReservation, error)
}
type PasswordVerifier interface {
	ValidatePHC(string) error
	Verify([]byte, string) (password.Verification, error)
}

type Authenticator struct {
	reader   CredentialReader
	attempts AttemptController
	keyer    AttemptKeyer
	verifier PasswordVerifier
	dummyPHC string
}
type AuthenticateInput struct {
	Email        string
	Password     []byte
	NetworkScope attempt.Key
}
type Authentication struct {
	UserID                       ID
	AuthEpoch, CredentialVersion int64
	NeedsPasswordRehash          bool
}

func NewAuthenticator(reader CredentialReader, attempts AttemptController, keyer AttemptKeyer, verifier PasswordVerifier, dummyPHC string) (*Authenticator, error) {
	if reader == nil || attempts == nil || keyer == nil || verifier == nil || dummyPHC == "" {
		return nil, errors.New("authenticator_invalid")
	}
	if err := verifier.ValidatePHC(dummyPHC); err != nil {
		return nil, errors.New("authenticator_invalid")
	}
	return &Authenticator{reader, attempts, keyer, verifier, dummyPHC}, nil
}
func (a *Authenticator) Authenticate(ctx context.Context, input AuthenticateInput) (Authentication, error) {
	if ctx == nil {
		return Authentication{}, safe("authentication_context_required", nil)
	}
	if a == nil || a.reader == nil || a.attempts == nil || a.keyer == nil || a.verifier == nil || a.dummyPHC == "" {
		return Authentication{}, safe("authentication_unavailable", nil)
	}
	address, addressErr := email.Parse(input.Email)
	lookup := input.Email
	if addressErr == nil {
		lookup = address.Lookup()
	}
	if lookup == "" {
		lookup = "\x00"
	}
	identityScope, err := a.keyer.Identity(lookup)
	if err != nil {
		return Authentication{}, safe("authentication_unavailable", err)
	}
	scopes := attempt.Scopes{Identity: identityScope, Network: input.NetworkScope}
	if !scopes.Valid() {
		return Authentication{}, safe("authentication_context_required", nil)
	}
	reservation, err := a.attempts.Begin(ctx, scopes)
	if err != nil {
		if errors.Is(err, ErrAttemptDenied) {
			return Authentication{}, safe("credentials_invalid", nil)
		}
		return Authentication{}, safe("authentication_unavailable", err)
	}
	if reservation == nil {
		return Authentication{}, safe("authentication_unavailable", nil)
	}
	record := CredentialRecord{}
	phc := a.dummyPHC
	lookupErr := addressErr
	if addressErr == nil {
		var err error
		record, err = a.reader.LookupPasswordCredential(ctx, address.Lookup())
		lookupErr = err
		if err == nil {
			phc = record.PasswordPHC
		} else if !errors.Is(err, ErrCredentialNotFound) {
			lookupErr = err
		}
	}
	verification, verifyErr := a.verifier.Verify(input.Password, phc)
	validRecord := lookupErr == nil && !record.UserID.IsZero() && record.PasswordPHC != "" && record.EmailVerifiedAt != nil && record.DisabledAt == nil && record.AuthEpoch > 0 && record.CredentialVersion > 0
	success := verifyErr == nil && verification.Match && validRecord
	if err := reservation.Finish(ctx, success); err != nil {
		return Authentication{}, safe("authentication_unavailable", err)
	}
	if verifyErr != nil {
		return Authentication{}, safe("authentication_unavailable", verifyErr)
	}
	if lookupErr != nil && !errors.Is(lookupErr, ErrCredentialNotFound) && addressErr == nil {
		return Authentication{}, safe("authentication_unavailable", lookupErr)
	}
	if !success {
		return Authentication{}, safe("credentials_invalid", nil)
	}
	return Authentication{record.UserID, record.AuthEpoch, record.CredentialVersion, verification.NeedsRehash}, nil
}

type operationError struct {
	code  string
	cause error
}

func safe(code string, cause error) error               { return &operationError{code, cause} }
func (e *operationError) Error() string                 { return e.code }
func (e *operationError) Unwrap() error                 { return e.cause }
func (e *operationError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.code) }
