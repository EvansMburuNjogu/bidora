package httpapi_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

type loginUseCaseFake struct {
	result   identityapp.LoginResult
	err      error
	input    identityapp.LoginInput
	password []byte
	calls    int
}

func (l *loginUseCaseFake) Login(_ context.Context, input identityapp.LoginInput) (identityapp.LoginResult, error) {
	l.calls++
	l.input = input
	l.password = append([]byte(nil), input.Credentials.Password...)
	return l.result, l.err
}

type loginIPFake struct {
	address   netip.Addr
	err       error
	peer      netip.AddrPort
	forwarded string
	calls     int
}

func (r *loginIPFake) Resolve(peer netip.AddrPort, forwarded string) (netip.Addr, error) {
	r.calls++
	r.peer = peer
	r.forwarded = forwarded
	return r.address, r.err
}

type loginKeyerFake struct {
	key     attempt.Key
	err     error
	address netip.Addr
	calls   int
}

func (k *loginKeyerFake) Network(address netip.Addr) (attempt.Key, error) {
	k.calls++
	k.address = address
	return k.key, k.err
}
func loginPair(t *testing.T) session.Pair {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 17})
	if err != nil {
		t.Fatal(err)
	}
	return pair
}
func loginHandlerApp(t *testing.T, usecase *loginUseCaseFake, ips httpapi.LoginClientIPResolver, keys httpapi.LoginNetworkKeyer) *fiber.App {
	t.Helper()
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	handler, err := httpapi.NewLoginHandler(usecase, protection, ips, keys)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{BodyLimit: httpapi.LoginBodyLimit})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post("/v1/auth/login", handler.Handle)
	return app
}
func replaceLoginBody(request *http.Request, body string) {
	request.Body = io.NopCloser(strings.NewReader(body))
	request.ContentLength = int64(len(body))
}
func loginRequest(body string) *http.Request {
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://app.example.com")
	return req
}
func defaultLoginDeps(t *testing.T) (*loginUseCaseFake, *loginIPFake, *loginKeyerFake) {
	t.Helper()
	pair := loginPair(t)
	usecase := &loginUseCaseFake{result: identityapp.LoginResult{Session: pair.Session, CSRF: pair.CSRF, IdleExpiresAt: time.Now().Add(time.Minute), AbsoluteExpiresAt: time.Now().Add(12 * time.Hour)}}
	ips := &loginIPFake{address: netip.MustParseAddr("198.51.100.7")}
	var key attempt.Key
	key[31] = 1
	return usecase, ips, &loginKeyerFake{key: key}
}

func TestLoginHandlerUsesTrustedProxyResolutionAndPrivacySafeNetworkScopes(t *testing.T) {
	resolver, err := browser.NewClientIPResolver([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("0123456789abcdef0123456789abcdef")
	deriver, err := attempt.NewDeriver(secret)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		peer      string
		forwarded string
		client    netip.Addr
		status    int
	}{
		{name: "untrusted peer ignores spoofed header", peer: "192.0.2.10:4567", forwarded: "203.0.113.99", client: netip.MustParseAddr("192.0.2.10"), status: http.StatusNoContent},
		{name: "trusted chain walks right to left", peer: "10.0.0.1:4567", forwarded: "203.0.113.99, 198.51.100.7, 10.2.0.4", client: netip.MustParseAddr("198.51.100.7"), status: http.StatusNoContent},
		{name: "trusted malformed chain fails closed", peer: "10.0.0.1:4567", forwarded: "203.0.113.99,,10.2.0.4", status: http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usecase, _, _ := defaultLoginDeps(t)
			app := loginHandlerApp(t, usecase, resolver, deriver)
			request := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(request)
			request.Header.SetMethod(http.MethodPost)
			request.SetRequestURI(httpapi.LoginPath)
			request.Header.SetContentType("application/json")
			request.Header.Set("Origin", "https://app.example.com")
			request.Header.Set("X-Forwarded-For", tc.forwarded)
			request.SetBodyString(`{"email":"Pilot@Example.com","password":"correct horse battery"}`)
			peer, peerErr := net.ResolveTCPAddr("tcp", tc.peer)
			if peerErr != nil {
				t.Fatal(peerErr)
			}
			requestCtx := &fasthttp.RequestCtx{}
			requestCtx.Init(request, peer, nil)
			app.Handler()(requestCtx)
			if requestCtx.Response.StatusCode() != tc.status {
				t.Fatal("trusted proxy HTTP status drifted", requestCtx.Response.StatusCode())
			}
			if tc.status != http.StatusNoContent {
				if usecase.calls != 0 {
					t.Fatal("malformed trusted chain crossed login boundary")
				}
				return
			}
			expected, keyErr := deriver.Network(tc.client)
			if keyErr != nil {
				t.Fatal(keyErr)
			}
			if usecase.calls != 1 || usecase.input.Credentials.NetworkScope.IsZero() || usecase.input.Credentials.NetworkScope != expected {
				t.Fatal("login did not receive the HMAC-only canonical network scope")
			}
			other, _ := deriver.Network(netip.MustParseAddr("198.51.100.200"))
			if usecase.input.Credentials.NetworkScope == other {
				t.Fatal("network scope is not bound to the resolved client")
			}
		})
	}
}

func TestLoginHandlerCreatesApprovedCookiesWithoutResponseBody(t *testing.T) {
	usecase, ips, keys := defaultLoginDeps(t)
	app := loginHandlerApp(t, usecase, ips, keys)
	req := loginRequest(`{"email":"Pilot@Example.com","password":"correct horse battery"}`)
	req.Header.Set("X-Forwarded-For", "203.0.113.4")
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "existing-session-is-ignored"})
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 204 || res.ContentLength > 0 || usecase.calls != 1 || usecase.input.Credentials.Email != "Pilot@Example.com" || string(usecase.password) != "correct horse battery" || usecase.input.Credentials.NetworkScope != keys.key || usecase.input.CorrelationID != httpIdentityIDFromString(t, "019c0000-0000-4000-8000-000000000001") || ips.calls != 1 || keys.address != ips.address {
		t.Fatal("login binding drifted", res.StatusCode, usecase.input)
	}
	cookies := res.Cookies()
	if len(cookies) != 2 {
		t.Fatal("login cookie count drifted", len(cookies))
	}
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
	}
	sessionCookie, csrfCookie := byName[session.CookieName], byName[session.CSRFCookieName]
	if sessionCookie == nil || csrfCookie == nil || !sessionCookie.Secure || !sessionCookie.HttpOnly || sessionCookie.Path != "/" || !csrfCookie.Secure || csrfCookie.HttpOnly || csrfCookie.Path != "/" || sessionCookie.SameSite != http.SameSiteLaxMode || csrfCookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("login cookie policy drifted", cookies)
	}
}
func httpIdentityIDFromString(t *testing.T, value string) identityapp.ID {
	t.Helper()
	parsed, err := uuid.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return identityapp.ID(parsed)
}
func TestLoginHandlerRejectsBoundaryFailuresBeforeAuthentication(t *testing.T) {
	cases := []struct {
		name          string
		mutate        func(*http.Request)
		ipErr, keyErr error
		status        int
	}{{"origin", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, nil, nil, 403}, {"query", func(r *http.Request) { r.URL.RawQuery = "secret=value" }, nil, nil, 400}, {"media", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, nil, nil, 415}, {"unknown", func(r *http.Request) {
		replaceLoginBody(r, `{"email":"a","password":"b","extra":1}`)
	}, nil, nil, 400}, {"duplicate", func(r *http.Request) {
		replaceLoginBody(r, `{"email":"a","email":"b","password":"c"}`)
	}, nil, nil, 400}, {"missing", func(r *http.Request) { replaceLoginBody(r, `{"email":"a"}`) }, nil, nil, 400}, {"surrogate", func(r *http.Request) {
		replaceLoginBody(r, `{"email":"a","password":"\ud800"}`)
	}, nil, nil, 400}, {"ip", func(*http.Request) {}, errors.New("ip unavailable"), nil, 503}, {"key", func(*http.Request) {}, nil, errors.New("key unavailable"), 503}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usecase, ips, keys := defaultLoginDeps(t)
			ips.err = tc.ipErr
			keys.err = tc.keyErr
			app := loginHandlerApp(t, usecase, ips, keys)
			req := loginRequest(`{"email":"a","password":"password-value"}`)
			tc.mutate(req)
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status || usecase.calls != 0 {
				t.Fatal("invalid login crossed use case", res.StatusCode, usecase.calls)
			}
		})
	}
}
func TestLoginHandlerMapsGenericAuthenticationFailures(t *testing.T) {
	for _, tc := range []struct {
		cause  string
		status int
		code   string
		retry  string
	}{{"credentials_invalid", 401, "CREDENTIALS_INVALID", ""}, {"attempt_denied", 429, "ATTEMPT_DENIED", "900"}, {"secret-canary redis diagnostic", 503, "AUTHENTICATION_UNAVAILABLE", ""}} {
		t.Run(tc.cause, func(t *testing.T) {
			usecase, ips, keys := defaultLoginDeps(t)
			usecase.err = errors.New(tc.cause)
			app := loginHandlerApp(t, usecase, ips, keys)
			res, err := app.Test(loginRequest(`{"email":"a","password":"password-value"}`))
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status || res.Header.Get("Retry-After") != tc.retry || len(res.Cookies()) != 0 {
				t.Fatal("login failure mapping drifted", res.StatusCode)
			}
			bodyBytes, err := io.ReadAll(res.Body)
			if err != nil || strings.Contains(string(bodyBytes), "secret-canary") || !strings.Contains(string(bodyBytes), tc.code) {
				t.Fatal("login failure leaked diagnostic", string(bodyBytes), err)
			}
		})
	}
}
func TestLoginHandlerConstructionFailsClosed(t *testing.T) {
	usecase, ips, keys := defaultLoginDeps(t)
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	for _, tc := range []struct {
		name string
		l    httpapi.LoginUseCase
		p    *browser.Protection
		i    httpapi.LoginClientIPResolver
		k    httpapi.LoginNetworkKeyer
	}{{"login", nil, protection, ips, keys}, {"origin", usecase, nil, ips, keys}, {"ip", usecase, protection, nil, keys}, {"key", usecase, protection, ips, nil}} {
		if handler, err := httpapi.NewLoginHandler(tc.l, tc.p, tc.i, tc.k); err == nil || handler != nil {
			t.Fatal("invalid login handler accepted", tc.name)
		}
	}
}
