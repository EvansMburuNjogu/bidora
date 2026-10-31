package application

import (
	"context"
	"crypto/rand"
	"errors"
	"io"

	"bidora/backend/internal/identity/googleoidc"
)

type GoogleLinkProofIssuer interface {
	Reauthenticate(context.Context, GoogleLinkReauthenticationInput) (GoogleLinkProof, error)
}
type GoogleLinkAuthorizationStarter interface {
	Start(context.Context, GoogleLinkProof) (GoogleAuthorizationStart, error)
}
type GoogleLinkInitiationService struct {
	proof GoogleLinkProofIssuer
	flow  GoogleLinkAuthorizationStarter
}

func NewGoogleLinkInitiationService(proof GoogleLinkProofIssuer, flow GoogleLinkAuthorizationStarter) (*GoogleLinkInitiationService, error) {
	if proof == nil || flow == nil {
		return nil, errors.New("google_link_initiation_service_invalid")
	}
	return &GoogleLinkInitiationService{proof: proof, flow: flow}, nil
}
func (s *GoogleLinkInitiationService) Start(ctx context.Context, input GoogleLinkReauthenticationInput) (GoogleAuthorizationStart, error) {
	if s == nil || s.proof == nil || s.flow == nil {
		return GoogleAuthorizationStart{}, safe("google_link_unavailable", nil)
	}
	proof, err := s.proof.Reauthenticate(ctx, input)
	if err != nil {
		return GoogleAuthorizationStart{}, err
	}
	return s.flow.Start(ctx, proof)
}

type VerifiedGoogleLinker interface {
	LinkVerified(context.Context, GoogleLinkProof, googleoidc.Identity, ID) (LoginResult, error)
}
type GoogleOIDCLinkFlow struct {
	authorization googleoidc.AuthorizationConfig
	transactions  GoogleTransactionStore
	exchanger     GoogleCodeExchange
	verifier      GoogleTokenVerification
	linker        VerifiedGoogleLinker
	random        io.Reader
}

func NewGoogleOIDCLinkFlow(config googleoidc.AuthorizationConfig, transactions GoogleTransactionStore, exchanger GoogleCodeExchange, verifier GoogleTokenVerification, linker VerifiedGoogleLinker) (*GoogleOIDCLinkFlow, error) {
	return NewGoogleOIDCLinkFlowWithRandom(config, transactions, exchanger, verifier, linker, rand.Reader)
}
func NewGoogleOIDCLinkFlowWithRandom(config googleoidc.AuthorizationConfig, transactions GoogleTransactionStore, exchanger GoogleCodeExchange, verifier GoogleTokenVerification, linker VerifiedGoogleLinker, random io.Reader) (*GoogleOIDCLinkFlow, error) {
	if transactions == nil || exchanger == nil || verifier == nil || linker == nil || random == nil {
		return nil, errors.New("google_oidc_link_flow_config_invalid")
	}
	probe, err := googleoidc.GenerateTransactionWithRandom(&fixedLinkReader{})
	if err != nil {
		return nil, errors.New("google_oidc_link_flow_config_invalid")
	}
	if _, err = googleoidc.BuildAuthorizationRequest(config, probe); err != nil {
		return nil, errors.New("google_oidc_link_flow_config_invalid")
	}
	return &GoogleOIDCLinkFlow{authorization: config, transactions: transactions, exchanger: exchanger, verifier: verifier, linker: linker, random: random}, nil
}

type fixedLinkReader struct{}

func (*fixedLinkReader) Read(value []byte) (int, error) {
	for index := range value {
		value[index] = 1
	}
	return len(value), nil
}
func (f *GoogleOIDCLinkFlow) Start(ctx context.Context, proof GoogleLinkProof) (GoogleAuthorizationStart, error) {
	if ctx == nil || proof.UserID.IsZero() || proof.SessionID.IsZero() || proof.AuthEpoch < 1 || proof.CredentialVersion < 1 {
		return GoogleAuthorizationStart{}, safe("google_link_invalid", nil)
	}
	if f == nil || f.transactions == nil || f.random == nil {
		return GoogleAuthorizationStart{}, safe("google_link_unavailable", nil)
	}
	transaction, err := googleoidc.GenerateTransactionWithRandom(f.random)
	if err != nil {
		return GoogleAuthorizationStart{}, safe("google_link_unavailable", err)
	}
	material, err := transaction.MaterialForLink([16]byte(proof.UserID), [16]byte(proof.SessionID), proof.AuthEpoch, proof.CredentialVersion)
	if err != nil {
		return GoogleAuthorizationStart{}, safe("google_link_invalid", nil)
	}
	request, err := googleoidc.BuildAuthorizationRequest(f.authorization, transaction)
	if err != nil {
		return GoogleAuthorizationStart{}, safe("google_link_unavailable", err)
	}
	if err = f.transactions.Create(ctx, transaction.StateForAuthorization(), material); err != nil {
		return GoogleAuthorizationStart{}, safe("google_link_unavailable", err)
	}
	return GoogleAuthorizationStart{request: request, browserBinding: transaction.BrowserBindingForCookie()}, nil
}

type GoogleLinkCallbackInput struct {
	Principal                   SessionPrincipal
	State, BrowserBinding, Code string
	CorrelationID               ID
}

func (f *GoogleOIDCLinkFlow) Callback(ctx context.Context, input GoogleLinkCallbackInput) (LoginResult, error) {
	principal := input.Principal
	if ctx == nil || principal.UserID.IsZero() || principal.SessionID.IsZero() || principal.AuthEpoch < 1 || input.CorrelationID.IsZero() {
		return LoginResult{}, safe("google_link_invalid", nil)
	}
	if f == nil || f.transactions == nil || f.exchanger == nil || f.verifier == nil || f.linker == nil {
		return LoginResult{}, safe("google_link_unavailable", nil)
	}
	material, err := f.transactions.Consume(ctx, input.State)
	if err != nil {
		return LoginResult{}, googleLinkFlowError(err)
	}
	if !material.MatchesLink([16]byte(principal.UserID), [16]byte(principal.SessionID), principal.AuthEpoch) {
		return LoginResult{}, safe("google_link_invalid", nil)
	}
	matches, err := googleoidc.MatchesBrowserBinding(input.BrowserBinding, material.BrowserBindingHash)
	if err != nil || !matches {
		return LoginResult{}, safe("google_link_invalid", nil)
	}
	exchange, err := f.exchanger.Exchange(ctx, input.Code, material.Verifier)
	if err != nil {
		return LoginResult{}, googleLinkFlowError(err)
	}
	identity, err := f.verifier.Verify(ctx, exchange.IDToken, material.Nonce)
	if err != nil {
		return LoginResult{}, googleLinkFlowError(err)
	}
	return f.linker.LinkVerified(ctx, GoogleLinkProof{UserID: principal.UserID, SessionID: principal.SessionID, AuthEpoch: principal.AuthEpoch, CredentialVersion: material.CredentialVersion}, identity, input.CorrelationID)
}
func googleLinkFlowError(err error) error {
	if err != nil && hasUnavailableSuffix(err.Error()) {
		return safe("google_link_unavailable", err)
	}
	return safe("google_link_invalid", nil)
}
func hasUnavailableSuffix(value string) bool {
	const suffix = "_unavailable"
	return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
}
