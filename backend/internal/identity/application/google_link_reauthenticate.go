package application

import (
	"context"
	"errors"
	"fmt"
	"math"
)

type GoogleLinkReauthenticationInput struct {
	Principal   SessionPrincipal
	Credentials AuthenticateInput
}

func (GoogleLinkReauthenticationInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google link reauthentication input]")
}

type GoogleLinkProof struct {
	UserID, SessionID            ID
	AuthEpoch, CredentialVersion int64
}

func (GoogleLinkProof) Format(s fmt.State, verb rune) { fmt.Fprint(s, "[REDACTED Google link proof]") }

type GoogleLinkReauthenticationService struct {
	auth     CredentialAuthenticator
	rehasher PasswordRehasher
}

func NewGoogleLinkReauthenticationService(auth CredentialAuthenticator, rehasher PasswordRehasher) (*GoogleLinkReauthenticationService, error) {
	if auth == nil || rehasher == nil {
		return nil, errors.New("google_link_reauthentication_service_invalid")
	}
	return &GoogleLinkReauthenticationService{auth: auth, rehasher: rehasher}, nil
}
func (s *GoogleLinkReauthenticationService) Reauthenticate(ctx context.Context, input GoogleLinkReauthenticationInput) (GoogleLinkProof, error) {
	principal := input.Principal
	if ctx == nil || principal.UserID.IsZero() || principal.SessionID.IsZero() || principal.AuthEpoch < 1 {
		return GoogleLinkProof{}, safe("google_link_reauthentication_invalid", nil)
	}
	if s == nil || s.auth == nil || s.rehasher == nil {
		return GoogleLinkProof{}, safe("google_link_reauthentication_unavailable", nil)
	}
	authentication, err := s.auth.Authenticate(ctx, input.Credentials)
	if err != nil {
		switch err.Error() {
		case "attempt_denied":
			return GoogleLinkProof{}, err
		case "authentication_unavailable":
			return GoogleLinkProof{}, safe("google_link_reauthentication_unavailable", err)
		default:
			return GoogleLinkProof{}, safe("google_link_reauthentication_invalid", nil)
		}
	}
	if authentication.UserID.IsZero() || authentication.UserID != principal.UserID || authentication.AuthEpoch != principal.AuthEpoch || authentication.CredentialVersion < 1 {
		return GoogleLinkProof{}, safe("google_link_reauthentication_invalid", nil)
	}
	credentialVersion := authentication.CredentialVersion
	if authentication.NeedsPasswordRehash {
		if credentialVersion == math.MaxInt64 {
			return GoogleLinkProof{}, safe("google_link_reauthentication_unavailable", nil)
		}
		if err = s.rehasher.RehashPassword(ctx, authentication.UserID, credentialVersion, input.Credentials.Password); err != nil {
			return GoogleLinkProof{}, safe("google_link_reauthentication_unavailable", err)
		}
		credentialVersion++
	}
	return GoogleLinkProof{UserID: principal.UserID, SessionID: principal.SessionID, AuthEpoch: principal.AuthEpoch, CredentialVersion: credentialVersion}, nil
}
