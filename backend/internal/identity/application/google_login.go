package application

import (
	"context"
	"errors"
	"fmt"
)

type GoogleCallbackAuthenticator interface {
	Callback(context.Context, GoogleCallbackInput) (GoogleIdentityResult, error)
}
type GoogleLoginService struct {
	flow   GoogleCallbackAuthenticator
	issuer *SessionIssuer
	mfa    MFALoginGate
}
type GoogleLoginResult struct {
	Identity GoogleIdentityResult
	Login    LoginResult
}

func (GoogleLoginResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google login result]")
}
func NewGoogleLoginService(flow GoogleCallbackAuthenticator, issuer *SessionIssuer) (*GoogleLoginService, error) {
	if flow == nil || issuer == nil {
		return nil, errors.New("google_login_service_invalid")
	}
	return &GoogleLoginService{flow: flow, issuer: issuer}, nil
}
func (s *GoogleLoginService) EnableMFA(g MFALoginGate) error {
	if s == nil || g == nil {
		return errors.New("google_login_mfa_invalid")
	}
	s.mfa = g
	return nil
}
func (s *GoogleLoginService) Login(ctx context.Context, input GoogleCallbackInput) (GoogleLoginResult, error) {
	if ctx == nil || input.CorrelationID.IsZero() {
		return GoogleLoginResult{}, safe("google_authentication_invalid", nil)
	}
	if s == nil || s.flow == nil || s.issuer == nil {
		return GoogleLoginResult{}, safe("google_authentication_unavailable", nil)
	}
	identity, err := s.flow.Callback(ctx, input)
	if err != nil {
		return GoogleLoginResult{}, err
	}
	if s.mfa != nil {
		challenge, e := s.mfa.BeginLogin(ctx, identity.UserID, identity.AuthEpoch, AuthenticationMethodGoogle)
		if e != nil {
			return GoogleLoginResult{}, e
		}
		if challenge.Required {
			return GoogleLoginResult{Identity: identity, Login: LoginResult{MFARequired: true, MFAToken: challenge.Token, MFAExpiresAt: challenge.ExpiresAt}}, nil
		}
	}
	login, err := s.issuer.Issue(ctx, AuthenticatedSessionInput{UserID: identity.UserID, CorrelationID: input.CorrelationID, AuthEpoch: identity.AuthEpoch, Method: AuthenticationMethodGoogle})
	if err != nil {
		return GoogleLoginResult{}, err
	}
	return GoogleLoginResult{Identity: identity, Login: login}, nil
}
