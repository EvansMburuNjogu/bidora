package googleoidc

import (
	"errors"
	"fmt"
	"net/url"
)

const googleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"

type AuthorizationConfig struct{ ClientID, RedirectURI string }

func (AuthorizationConfig) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google OIDC authorization config]")
}

type AuthorizationRequest struct{ rawURL string }

func (AuthorizationRequest) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google OIDC authorization request]")
}
func (r AuthorizationRequest) URLForRedirect() string { return r.rawURL }
func BuildAuthorizationRequest(config AuthorizationConfig, transaction Transaction) (AuthorizationRequest, error) {
	if !boundedASCII(config.ClientID, 1, 255) || !validRedirectURI(config.RedirectURI) || transaction.IsZero() {
		return AuthorizationRequest{}, errors.New("google_oidc_authorization_invalid")
	}
	query := url.Values{"client_id": {config.ClientID}, "redirect_uri": {config.RedirectURI}, "response_type": {"code"}, "scope": {"openid email"}, "state": {transaction.StateForAuthorization()}, "nonce": {transaction.NonceForAuthorization()}, "code_challenge": {transaction.PKCEChallenge()}, "code_challenge_method": {"S256"}}
	return AuthorizationRequest{rawURL: googleAuthorizationEndpoint + "?" + query.Encode()}, nil
}
