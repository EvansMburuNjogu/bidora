package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/identity/googleoidc"
)

var ErrGoogleLinkRequired = errors.New("google_link_required")
var ErrGoogleAuthenticationDenied = errors.New("google_authentication_denied")

type GoogleIdentityResolver interface {
	ResolveGoogleIdentity(context.Context, GoogleIdentityCommit) (GoogleIdentityResult, error)
}
type GoogleIdentityCommit struct {
	UserID, ExternalIdentityID, AuditID, CorrelationID ID
	Identity                                           googleoidc.Identity
	AuthenticatedAt                                    time.Time
}

func (GoogleIdentityCommit) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google identity commit]")
}

type GoogleIdentityResult struct {
	UserID    ID
	AuthEpoch int64
	Created   bool
}

func (GoogleIdentityResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google identity result]")
}

type GoogleAuthenticationInput struct {
	Claims        googleoidc.Claims
	ExpectedNonce string
	CorrelationID ID
}
type VerifiedGoogleAuthenticationInput struct {
	Identity      googleoidc.Identity
	CorrelationID ID
}
type GoogleAuthenticationService struct {
	validator *googleoidc.Validator
	resolver  GoogleIdentityResolver
	ids       LoginIDGenerator
	clock     LoginClock
}

func NewGoogleAuthenticationService(validator *googleoidc.Validator, resolver GoogleIdentityResolver, ids LoginIDGenerator, clock LoginClock) (*GoogleAuthenticationService, error) {
	if validator == nil || resolver == nil || ids == nil || clock == nil {
		return nil, errors.New("google_authentication_service_invalid")
	}
	return &GoogleAuthenticationService{validator, resolver, ids, clock}, nil
}
func (s *GoogleAuthenticationService) Authenticate(ctx context.Context, input GoogleAuthenticationInput) (GoogleIdentityResult, error) {
	if ctx == nil || input.CorrelationID.IsZero() {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	if s == nil || s.validator == nil || s.resolver == nil || s.ids == nil || s.clock == nil {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", nil)
	}
	authenticatedAt := s.clock.Now().UTC()
	if authenticatedAt.IsZero() {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", nil)
	}
	identity, err := s.validator.Validate(input.Claims, input.ExpectedNonce, authenticatedAt)
	if err != nil {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	return s.authenticateIdentity(ctx, identity, input.CorrelationID, authenticatedAt)
}
func (s *GoogleAuthenticationService) AuthenticateVerified(ctx context.Context, input VerifiedGoogleAuthenticationInput) (GoogleIdentityResult, error) {
	if ctx == nil || input.CorrelationID.IsZero() || input.Identity.Subject == "" || input.Identity.Email.IsZero() || input.Identity.IssuedAt.IsZero() || input.Identity.ExpiresAt.IsZero() {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	if s == nil || s.resolver == nil || s.ids == nil || s.clock == nil {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", nil)
	}
	authenticatedAt := s.clock.Now().UTC()
	if authenticatedAt.IsZero() || input.Identity.IssuedAt.After(authenticatedAt.Add(2*time.Minute)) || !input.Identity.ExpiresAt.After(authenticatedAt.Add(-2*time.Minute)) {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	return s.authenticateIdentity(ctx, input.Identity, input.CorrelationID, authenticatedAt)
}
func (s *GoogleAuthenticationService) authenticateIdentity(ctx context.Context, identity googleoidc.Identity, correlationID ID, authenticatedAt time.Time) (GoogleIdentityResult, error) {
	userID, err := s.ids.NewID()
	if err != nil || userID.IsZero() {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", err)
	}
	externalID, err := s.ids.NewID()
	if err != nil || externalID.IsZero() {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", err)
	}
	auditID, err := s.ids.NewID()
	if err != nil || auditID.IsZero() {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", err)
	}
	result, err := s.resolver.ResolveGoogleIdentity(ctx, GoogleIdentityCommit{UserID: userID, ExternalIdentityID: externalID, AuditID: auditID, CorrelationID: correlationID, Identity: identity, AuthenticatedAt: authenticatedAt})
	if errors.Is(err, ErrGoogleLinkRequired) || errors.Is(err, ErrGoogleAuthenticationDenied) {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	if err != nil {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", err)
	}
	if result.UserID.IsZero() || result.AuthEpoch < 1 {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", nil)
	}
	return result, nil
}
