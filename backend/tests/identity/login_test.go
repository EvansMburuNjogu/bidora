package identity_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/session"
)

type authServiceFake struct {
	result identityapp.Authentication
	err    error
	calls  int
	order  *[]string
}

func (a *authServiceFake) Authenticate(context.Context, identityapp.AuthenticateInput) (identityapp.Authentication, error) {
	a.calls++
	if a.order != nil {
		*a.order = append(*a.order, "authenticate")
	}
	return a.result, a.err
}

type rehasherFake struct {
	err      error
	calls    int
	user     identityapp.ID
	version  int64
	password []byte
	order    *[]string
}

func (r *rehasherFake) RehashPassword(_ context.Context, user identityapp.ID, version int64, value []byte) error {
	r.calls++
	r.user = user
	r.version = version
	r.password = append([]byte(nil), value...)
	if r.order != nil {
		*r.order = append(*r.order, "rehash")
	}
	return r.err
}

type tokenGeneratorFake struct {
	pair  session.Pair
	err   error
	calls int
	order *[]string
}

func (g *tokenGeneratorFake) GenerateSessionPair() (session.Pair, error) {
	g.calls++
	if g.order != nil {
		*g.order = append(*g.order, "tokens")
	}
	return g.pair, g.err
}

type loginIDsFake struct {
	id    identityapp.ID
	err   error
	calls int
	order *[]string
}

func (g *loginIDsFake) NewID() (identityapp.ID, error) {
	g.calls++
	if g.order != nil {
		*g.order = append(*g.order, "id")
	}
	return g.id, g.err
}

type loginClockFake struct {
	at    time.Time
	order *[]string
}

func (c loginClockFake) Now() time.Time {
	if c.order != nil {
		*c.order = append(*c.order, "clock")
	}
	return c.at
}

type committerFake struct {
	commit identityapp.SessionCommit
	err    error
	calls  int
	order  *[]string
}

func (c *committerFake) CommitLoginSession(_ context.Context, commit identityapp.SessionCommit) error {
	c.calls++
	c.commit = commit
	if c.order != nil {
		*c.order = append(*c.order, "commit")
	}
	return c.err
}
func sessionPair(t *testing.T) session.Pair {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&sessionReader{next: 11})
	if err != nil {
		t.Fatal(err)
	}
	return pair
}
func loginService(t *testing.T, auth *authServiceFake, rehash *rehasherFake, tokens *tokenGeneratorFake, ids *loginIDsFake, clock loginClockFake, commit *committerFake) *identityapp.LoginService {
	t.Helper()
	service, err := identityapp.NewLoginService(auth, rehash, tokens, ids, clock, commit)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
func validAuthentication() identityapp.Authentication {
	return identityapp.Authentication{UserID: identityID(9), AuthEpoch: 4, CredentialVersion: 3}
}
func validLoginInput() identityapp.LoginInput {
	return identityapp.LoginInput{Credentials: loginInput("user@example.com", []byte("synthetic password")), CorrelationID: identityID(8)}
}

func TestLoginCommitsSessionBeforeReturningSecrets(t *testing.T) {
	order := []string{}
	auth := &authServiceFake{result: validAuthentication(), order: &order}
	rehash := &rehasherFake{order: &order}
	tokens := &tokenGeneratorFake{pair: sessionPair(t), order: &order}
	ids := &loginIDsFake{id: identityID(7), order: &order}
	clock := loginClockFake{at: time.Date(2026, 10, 18, 10, 0, 0, 0, time.FixedZone("source", 3600)), order: &order}
	committer := &committerFake{order: &order}
	service := loginService(t, auth, rehash, tokens, ids, clock, committer)
	result, err := service.Login(context.Background(), validLoginInput())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "authenticate,tokens,id,id,clock,commit" || rehash.calls != 0 || committer.calls != 1 {
		t.Fatal("login order drifted", order)
	}
	if committer.commit.SessionID != identityID(7) || committer.commit.AuditID != identityID(7) || committer.commit.UserID != identityID(9) || committer.commit.CorrelationID != identityID(8) || committer.commit.AuthEpoch != 4 || committer.commit.AuthenticationMethod != identityapp.AuthenticationMethodPassword || committer.commit.SessionHash != tokens.pair.Session.Hash() || committer.commit.CSRFHash != tokens.pair.CSRF.Hash() {
		t.Fatal("session commit mapping drifted")
	}
	if result.Session.RevealForCookie() != tokens.pair.Session.RevealForCookie() || result.CSRF.RevealForBootstrap() != tokens.pair.CSRF.RevealForBootstrap() || !result.IdleExpiresAt.Equal(time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)) || !result.AbsoluteExpiresAt.Equal(time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)) {
		t.Fatal("login result drifted")
	}
	if strings.Contains(fmt.Sprintf("%#v", result), result.Session.RevealForCookie()) || strings.Contains(fmt.Sprintf("%#v", committer.commit), result.CSRF.RevealForBootstrap()) {
		t.Fatal("login values leaked through formatting")
	}
}

func TestLoginRehashesBeforeGeneratingSession(t *testing.T) {
	order := []string{}
	authResult := validAuthentication()
	authResult.NeedsPasswordRehash = true
	auth := &authServiceFake{result: authResult, order: &order}
	rehash := &rehasherFake{order: &order}
	tokens := &tokenGeneratorFake{pair: sessionPair(t), order: &order}
	ids := &loginIDsFake{id: identityID(7), order: &order}
	committer := &committerFake{order: &order}
	service := loginService(t, auth, rehash, tokens, ids, loginClockFake{at: time.Now(), order: &order}, committer)
	input := validLoginInput()
	if _, err := service.Login(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "authenticate,rehash,tokens,id,id,clock,commit" || rehash.user != identityID(9) || rehash.version != 3 || string(rehash.password) != "synthetic password" {
		t.Fatal("rehash orchestration drifted", order)
	}
}

func TestLoginStopsBeforeReturningSecretsAtEveryFailure(t *testing.T) {
	cause := errors.New("secret-canary login dependency")
	cases := []struct {
		name  string
		setup func(*authServiceFake, *rehasherFake, *tokenGeneratorFake, *loginIDsFake, *loginClockFake, *committerFake)
	}{
		{"authenticate", func(a *authServiceFake, _ *rehasherFake, _ *tokenGeneratorFake, _ *loginIDsFake, _ *loginClockFake, _ *committerFake) {
			a.err = errors.New("credentials_invalid")
		}},
		{"invalid authentication", func(a *authServiceFake, _ *rehasherFake, _ *tokenGeneratorFake, _ *loginIDsFake, _ *loginClockFake, _ *committerFake) {
			a.result = identityapp.Authentication{}
		}},
		{"rehash", func(a *authServiceFake, r *rehasherFake, _ *tokenGeneratorFake, _ *loginIDsFake, _ *loginClockFake, _ *committerFake) {
			a.result.NeedsPasswordRehash = true
			r.err = cause
		}},
		{"tokens", func(_ *authServiceFake, _ *rehasherFake, g *tokenGeneratorFake, _ *loginIDsFake, _ *loginClockFake, _ *committerFake) {
			g.err = cause
		}},
		{"zero tokens", func(_ *authServiceFake, _ *rehasherFake, g *tokenGeneratorFake, _ *loginIDsFake, _ *loginClockFake, _ *committerFake) {
			g.pair = session.Pair{}
		}},
		{"id", func(_ *authServiceFake, _ *rehasherFake, _ *tokenGeneratorFake, g *loginIDsFake, _ *loginClockFake, _ *committerFake) {
			g.err = cause
		}},
		{"zero id", func(_ *authServiceFake, _ *rehasherFake, _ *tokenGeneratorFake, g *loginIDsFake, _ *loginClockFake, _ *committerFake) {
			g.id = identityapp.ID{}
		}},
		{"clock", func(_ *authServiceFake, _ *rehasherFake, _ *tokenGeneratorFake, _ *loginIDsFake, c *loginClockFake, _ *committerFake) {
			c.at = time.Time{}
		}},
		{"commit", func(_ *authServiceFake, _ *rehasherFake, _ *tokenGeneratorFake, _ *loginIDsFake, _ *loginClockFake, c *committerFake) {
			c.err = cause
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := &authServiceFake{result: validAuthentication()}
			rehash := &rehasherFake{}
			tokens := &tokenGeneratorFake{pair: sessionPair(t)}
			ids := &loginIDsFake{id: identityID(7)}
			clock := loginClockFake{at: time.Now()}
			committer := &committerFake{}
			tc.setup(auth, rehash, tokens, ids, &clock, committer)
			service := loginService(t, auth, rehash, tokens, ids, clock, committer)
			result, err := service.Login(context.Background(), validLoginInput())
			if err == nil || result != (identityapp.LoginResult{}) {
				t.Fatal("failed login returned secrets", result, err)
			}
			if tc.name != "authenticate" && err.Error() != "authentication_unavailable" {
				t.Fatal("unsafe failure code", err)
			}
			if strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("dependency diagnostic leaked")
			}
		})
	}
}

func TestLoginRejectsInvalidConfigurationAndContext(t *testing.T) {
	auth := &authServiceFake{result: validAuthentication()}
	rehash := &rehasherFake{}
	tokens := &tokenGeneratorFake{pair: sessionPair(t)}
	ids := &loginIDsFake{id: identityID(7)}
	clock := loginClockFake{at: time.Now()}
	committer := &committerFake{}
	builders := []func() (*identityapp.LoginService, error){func() (*identityapp.LoginService, error) {
		return identityapp.NewLoginService(nil, rehash, tokens, ids, clock, committer)
	}, func() (*identityapp.LoginService, error) {
		return identityapp.NewLoginService(auth, nil, tokens, ids, clock, committer)
	}, func() (*identityapp.LoginService, error) {
		return identityapp.NewLoginService(auth, rehash, nil, ids, clock, committer)
	}, func() (*identityapp.LoginService, error) {
		return identityapp.NewLoginService(auth, rehash, tokens, nil, clock, committer)
	}, func() (*identityapp.LoginService, error) {
		return identityapp.NewLoginService(auth, rehash, tokens, ids, nil, committer)
	}, func() (*identityapp.LoginService, error) {
		return identityapp.NewLoginService(auth, rehash, tokens, ids, clock, nil)
	}}
	for _, build := range builders {
		if service, err := build(); err == nil || err.Error() != "login_service_invalid" || service != nil {
			t.Fatal("invalid login service accepted")
		}
	}
	service := loginService(t, auth, rehash, tokens, ids, clock, committer)
	if _, err := service.Login(nil, validLoginInput()); err == nil || err.Error() != "authentication_context_required" {
		t.Fatal("nil login context accepted")
	}
	input := validLoginInput()
	input.CorrelationID = identityapp.ID{}
	if _, err := service.Login(context.Background(), input); err == nil || err.Error() != "authentication_context_required" || auth.calls != 0 {
		t.Fatal("missing correlation crossed authentication")
	}
}
