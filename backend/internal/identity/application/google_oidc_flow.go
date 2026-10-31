package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"

	"bidora/backend/internal/identity/googleoidc"
)

type GoogleTransactionStore interface {
	Create(context.Context, string, googleoidc.TransactionMaterial) error
	Consume(context.Context, string) (googleoidc.TransactionMaterial, error)
}
type GoogleCodeExchange interface {
	Exchange(context.Context, string, string) (googleoidc.ExchangeResult, error)
}
type GoogleTokenVerification interface {
	Verify(context.Context, string, string) (googleoidc.Identity, error)
}
type VerifiedGoogleAuthenticator interface {
	AuthenticateVerified(context.Context, VerifiedGoogleAuthenticationInput) (GoogleIdentityResult, error)
}
type GoogleOIDCFlow struct {
	authorization googleoidc.AuthorizationConfig
	transactions  GoogleTransactionStore
	exchanger     GoogleCodeExchange
	verifier      GoogleTokenVerification
	authenticator VerifiedGoogleAuthenticator
	random        io.Reader
}
type GoogleAuthorizationStart struct {
	request        googleoidc.AuthorizationRequest
	browserBinding string
}

func (GoogleAuthorizationStart) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google authorization start]")
}
func (r GoogleAuthorizationStart) URLForRedirect() string          { return r.request.URLForRedirect() }
func (r GoogleAuthorizationStart) BrowserBindingForCookie() string { return r.browserBinding }
func NewGoogleOIDCFlow(config googleoidc.AuthorizationConfig, transactions GoogleTransactionStore, exchanger GoogleCodeExchange, verifier GoogleTokenVerification, authenticator VerifiedGoogleAuthenticator) (*GoogleOIDCFlow, error) {
	return NewGoogleOIDCFlowWithRandom(config, transactions, exchanger, verifier, authenticator, rand.Reader)
}
func NewGoogleOIDCFlowWithRandom(config googleoidc.AuthorizationConfig, transactions GoogleTransactionStore, exchanger GoogleCodeExchange, verifier GoogleTokenVerification, authenticator VerifiedGoogleAuthenticator, random io.Reader) (*GoogleOIDCFlow, error) {
	if transactions == nil || exchanger == nil || verifier == nil || authenticator == nil || random == nil {
		return nil, errors.New("google_oidc_flow_config_invalid")
	}
	probe, err := googleoidc.GenerateTransactionWithRandom(strings.NewReader(strings.Repeat("x", googleoidc.TransactionSecretBytes*4)))
	if err != nil {
		return nil, errors.New("google_oidc_flow_config_invalid")
	}
	if _, err = googleoidc.BuildAuthorizationRequest(config, probe); err != nil {
		return nil, errors.New("google_oidc_flow_config_invalid")
	}
	return &GoogleOIDCFlow{authorization: config, transactions: transactions, exchanger: exchanger, verifier: verifier, authenticator: authenticator, random: random}, nil
}
func (f *GoogleOIDCFlow) Start(ctx context.Context) (GoogleAuthorizationStart, error) {
	if f == nil || f.transactions == nil || f.random == nil || ctx == nil {
		return GoogleAuthorizationStart{}, safe("google_authentication_unavailable", nil)
	}
	transaction, err := googleoidc.GenerateTransactionWithRandom(f.random)
	if err != nil {
		return GoogleAuthorizationStart{}, safe("google_authentication_unavailable", err)
	}
	request, err := googleoidc.BuildAuthorizationRequest(f.authorization, transaction)
	if err != nil {
		return GoogleAuthorizationStart{}, safe("google_authentication_unavailable", err)
	}
	if err = f.transactions.Create(ctx, transaction.StateForAuthorization(), transaction.MaterialForStore()); err != nil {
		return GoogleAuthorizationStart{}, safe("google_authentication_unavailable", err)
	}
	return GoogleAuthorizationStart{request: request, browserBinding: transaction.BrowserBindingForCookie()}, nil
}

type GoogleCallbackInput struct {
	State, BrowserBinding, Code string
	CorrelationID               ID
}

func (GoogleCallbackInput) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google callback input]")
}
func (f *GoogleOIDCFlow) Callback(ctx context.Context, input GoogleCallbackInput) (GoogleIdentityResult, error) {
	if f == nil || f.transactions == nil || f.exchanger == nil || f.verifier == nil || f.authenticator == nil {
		return GoogleIdentityResult{}, safe("google_authentication_unavailable", nil)
	}
	if ctx == nil || input.CorrelationID.IsZero() {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	material, err := f.transactions.Consume(ctx, input.State)
	if err != nil {
		return GoogleIdentityResult{}, googleFlowError(err)
	}
	if !material.IsSignIn() {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	matches, err := googleoidc.MatchesBrowserBinding(input.BrowserBinding, material.BrowserBindingHash)
	if err != nil || !matches {
		return GoogleIdentityResult{}, safe("google_authentication_invalid", nil)
	}
	exchange, err := f.exchanger.Exchange(ctx, input.Code, material.Verifier)
	if err != nil {
		return GoogleIdentityResult{}, googleFlowError(err)
	}
	identity, err := f.verifier.Verify(ctx, exchange.IDToken, material.Nonce)
	if err != nil {
		return GoogleIdentityResult{}, googleFlowError(err)
	}
	result, err := f.authenticator.AuthenticateVerified(ctx, VerifiedGoogleAuthenticationInput{Identity: identity, CorrelationID: input.CorrelationID})
	if err != nil {
		return GoogleIdentityResult{}, err
	}
	return result, nil
}
func googleFlowError(err error) error {
	if err != nil && strings.HasSuffix(err.Error(), "_unavailable") {
		return safe("google_authentication_unavailable", err)
	}
	return safe("google_authentication_invalid", nil)
}
