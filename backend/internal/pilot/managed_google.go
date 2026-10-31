package pilot

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attemptredis"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/identity/oidcredis"
	identitypersistence "bidora/backend/internal/identity/persistence"
	app "bidora/backend/internal/platformsettings/application"
	persistence "bidora/backend/internal/platformsettings/persistence"
	"bidora/backend/internal/postgres"
)

type googleKeyTester struct{ transport *googleoidc.Transport }

func (t googleKeyTester) TestGoogle(ctx context.Context) error {
	keys, e := googleoidc.NewGoogleKeySet(t.transport)
	if e != nil {
		return e
	}
	_, e = keys.Key(ctx, "bidora-settings-probe")
	if errors.Is(e, googleoidc.ErrKeyNotFound) {
		return nil
	}
	return e
}

type managedGoogle struct {
	settings   *app.GoogleService
	deployment *GoogleConfig
	control    *postgres.Pool
	redis      *attemptredis.ClientRunner
	identity   *identityComposition
	prefix     string
	keys       *googleoidc.KeySet
	transport  *googleoidc.Transport
	proof      *identityapp.GoogleLinkReauthenticationService
}
type managedGoogleRequest struct {
	flow       *identityapp.GoogleOIDCFlow
	login      *identityapp.GoogleLoginService
	link       *identityapp.GoogleOIDCLinkFlow
	initiation *identityapp.GoogleLinkInitiationService
}

func (g *managedGoogle) request(ctx context.Context) (*managedGoogleRequest, error) {
	if g == nil || g.deployment == nil || g.settings == nil || g.transport == nil {
		return nil, errors.New("google_authentication_unavailable")
	}
	snapshot, e := g.settings.Runtime(ctx)
	if e != nil {
		return nil, e
	}
	c := snapshot.Configuration
	if !c.Enabled {
		return nil, errors.New("google_authentication_unavailable")
	}
	secret, e := g.settings.Secret(snapshot)
	if e != nil {
		return nil, e
	}
	defer func() {
		for i := range secret {
			secret[i] = 0
		}
	}()
	namespace := sha256.Sum256([]byte(g.prefix))
	transactions, e := oidcredis.New(g.redis, fmt.Sprintf("bidora:g:%x:v%d", namespace[:12], c.Version), g.deployment.TransactionTTL)
	if e != nil {
		return nil, e
	}
	verifier, e := googleoidc.NewTokenVerifier(c.ClientID, g.keys)
	if e != nil {
		return nil, e
	}
	validator, e := googleoidc.NewValidator(googleoidc.Config{ClientID: c.ClientID})
	if e != nil {
		return nil, e
	}
	guard := func(ctx context.Context, db postgres.DBTX, address string) (bool, error) {
		owner, e := persistence.NewGoogleSettingsTx(db)
		if e != nil {
			return false, e
		}
		if e = owner.LockGoogleForAuthentication(ctx); e != nil {
			return false, e
		}
		active, e := owner.ReadGoogle(ctx)
		if e != nil {
			return false, e
		}
		if !active.Configuration.AllowsVerifiedEmail(c.Version, address) {
			return false, identityapp.ErrGoogleAuthenticationDenied
		}
		return active.Configuration.AllowSignup, nil
	}
	resolver, e := identitypersistence.NewGoogleIdentityResolver(g.control)
	if e != nil {
		return nil, e
	}
	if e = resolver.EnablePolicy(guard); e != nil {
		return nil, e
	}
	authentication, e := identityapp.NewGoogleAuthenticationService(validator, resolver, identityIDs{}, systemClock{})
	if e != nil {
		return nil, e
	}
	exchanger, e := googleoidc.NewCodeExchanger(g.transport, googleoidc.ExchangeConfig{ClientID: c.ClientID, ClientSecret: string(secret), RedirectURI: g.deployment.CallbackURL})
	if e != nil {
		return nil, e
	}
	flow, e := identityapp.NewGoogleOIDCFlow(googleoidc.AuthorizationConfig{ClientID: c.ClientID, RedirectURI: g.deployment.CallbackURL}, transactions, exchanger, verifier, authentication)
	if e != nil {
		return nil, e
	}
	issuer, e := identityapp.NewSessionIssuer(identityapp.SecureSessionTokenGenerator{}, identityIDs{}, systemClock{}, g.identity.loginCommitter)
	if e != nil {
		return nil, e
	}
	login, e := identityapp.NewGoogleLoginService(flow, issuer)
	if e != nil {
		return nil, e
	}
	if e = login.EnableMFA(g.identity.mfaService); e != nil {
		return nil, e
	}
	linkCommitter, e := identitypersistence.NewGoogleLinkCommitter(g.control)
	if e != nil {
		return nil, e
	}
	if e = linkCommitter.EnablePolicy(guard); e != nil {
		return nil, e
	}
	linkService, e := identityapp.NewGoogleLinkService(identityapp.SecureSessionTokenGenerator{}, identityIDs{}, systemClock{}, linkCommitter)
	if e != nil {
		return nil, e
	}
	linkExchange, e := googleoidc.NewCodeExchanger(g.transport, googleoidc.ExchangeConfig{ClientID: c.ClientID, ClientSecret: string(secret), RedirectURI: g.deployment.LinkCallbackURL})
	if e != nil {
		return nil, e
	}
	link, e := identityapp.NewGoogleOIDCLinkFlow(googleoidc.AuthorizationConfig{ClientID: c.ClientID, RedirectURI: g.deployment.LinkCallbackURL}, transactions, linkExchange, verifier, linkService)
	if e != nil {
		return nil, e
	}
	initiation, e := identityapp.NewGoogleLinkInitiationService(g.proof, link)
	if e != nil {
		return nil, e
	}
	return &managedGoogleRequest{flow: flow, login: login, link: link, initiation: initiation}, nil
}
func (g *managedGoogle) Start(ctx context.Context) (identityapp.GoogleAuthorizationStart, error) {
	r, e := g.request(ctx)
	if e != nil {
		return identityapp.GoogleAuthorizationStart{}, e
	}
	return r.flow.Start(ctx)
}
func (g *managedGoogle) Login(ctx context.Context, in identityapp.GoogleCallbackInput) (identityapp.GoogleLoginResult, error) {
	r, e := g.request(ctx)
	if e != nil {
		return identityapp.GoogleLoginResult{}, e
	}
	return r.login.Login(ctx, in)
}

type managedGoogleLink struct{ google *managedGoogle }

func (g managedGoogleLink) Start(ctx context.Context, in identityapp.GoogleLinkReauthenticationInput) (identityapp.GoogleAuthorizationStart, error) {
	r, e := g.google.request(ctx)
	if e != nil {
		return identityapp.GoogleAuthorizationStart{}, e
	}
	return r.initiation.Start(ctx, in)
}
func (g managedGoogleLink) Callback(ctx context.Context, in identityapp.GoogleLinkCallbackInput) (identityapp.LoginResult, error) {
	r, e := g.google.request(ctx)
	if e != nil {
		return identityapp.LoginResult{}, e
	}
	return r.link.Callback(ctx, in)
}
func composeManagedGoogle(config Config, control *postgres.Pool, redis *attemptredis.ClientRunner, identity *identityComposition) (*httpapi.GoogleStartHandler, *httpapi.GoogleCallbackHandler, *httpapi.GoogleLinkStartHandler, *httpapi.GoogleLinkCallbackHandler, error) {
	if config.Google == nil {
		return nil, nil, nil, nil, nil
	}
	proof, e := identityapp.NewGoogleLinkReauthenticationService(identity.authenticator, identity.rehasher)
	if e != nil {
		return nil, nil, nil, nil, e
	}
	keys, e := googleoidc.NewGoogleKeySet(identity.googleTransport)
	if e != nil {
		return nil, nil, nil, nil, e
	}
	g := &managedGoogle{settings: identity.google, control: control, redis: redis, identity: identity, prefix: config.RedisPrefix, deployment: config.Google, transport: identity.googleTransport, keys: keys, proof: proof}
	start, callback, e := httpapi.NewGoogleHTTPHandlers(g, g, identity.protection, httpapi.GoogleHTTPConfig{SuccessRedirect: config.Google.SuccessRedirect, FailureRedirect: config.Google.FailureRedirect, TransactionTTL: config.Google.TransactionTTL})
	if e != nil {
		return nil, nil, nil, nil, e
	}
	linkStart, linkCallback, e := httpapi.NewGoogleLinkHTTPHandlers(managedGoogleLink{g}, managedGoogleLink{g}, identity.ipResolver, identity.deriver, httpapi.GoogleHTTPConfig{SuccessRedirect: config.Google.LinkSuccessRedirect, FailureRedirect: config.Google.LinkFailureRedirect, TransactionTTL: config.Google.TransactionTTL})
	return start, callback, linkStart, linkCallback, e
}
