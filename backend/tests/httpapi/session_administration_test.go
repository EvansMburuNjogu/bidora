package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type sessionInventoryHTTPFake struct {
	result identityapp.SessionInventory
	err    error
	calls  int
}

func (f *sessionInventoryHTTPFake) Get(context.Context, identityapp.SessionPrincipal) (identityapp.SessionInventory, error) {
	f.calls++
	return f.result, f.err
}

type sessionAdministrationHTTPFake struct {
	otherInput identityapp.RevokeOtherSessionInput
	allUser    identityapp.SessionPrincipal
	otherErr   error
	allErr     error
	otherCalls int
	allCalls   int
}

func (f *sessionAdministrationHTTPFake) RevokeOther(_ context.Context, input identityapp.RevokeOtherSessionInput) (identityapp.RevokeOtherSessionResult, error) {
	f.otherCalls++
	f.otherInput = input
	return identityapp.RevokeOtherSessionResult{Revoked: f.otherErr == nil}, f.otherErr
}
func (f *sessionAdministrationHTTPFake) RevokeAll(_ context.Context, principal identityapp.SessionPrincipal, _ identityapp.ID, _ time.Time) (identityapp.RevokeAllSessionsResult, error) {
	f.allCalls++
	f.allUser = principal
	return identityapp.RevokeAllSessionsResult{AuthEpoch: principal.AuthEpoch + 1}, f.allErr
}

func sessionAdministrationHTTPHarness(t *testing.T, inventory *sessionInventoryHTTPFake, administration *sessionAdministrationHTTPFake) (*fiber.App, string, string, identityapp.SessionPrincipal) {
	t.Helper()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 41})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	principal := identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 4, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(12 * time.Hour), AssuranceLevel: "AAL1", AssuranceMethod: "PRIMARY_ONLY", AssuredAt: now}
	validator := &sessionValidatorFake{principal: principal}
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
	if err != nil {
		t.Fatal(err)
	}
	inventoryHandler, err := httpapi.NewSessionInventoryHandler(inventory, protection)
	if err != nil {
		t.Fatal(err)
	}
	administrationHandler, err := httpapi.NewSessionAdministrationHandler(administration)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Get(httpapi.SessionInventoryPath, middleware.Handle, inventoryHandler.Get)
	app.Delete(httpapi.SessionRevokePath, middleware.NoTouch, administrationHandler.RevokeOther)
	app.Post(httpapi.SessionRevokeAllPath, middleware.NoTouch, administrationHandler.RevokeAll)
	return app, pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap(), principal
}

func sessionAdministrationRequest(method, path, sessionRaw, csrfRaw, origin string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Origin", origin)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionRaw})
	if csrfRaw != "" {
		request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfRaw})
		request.Header.Set(session.CSRFHeader, csrfRaw)
	}
	return request
}

func TestSessionInventoryReturnsCurrentAccountSessionsWithoutSensitiveMetadata(t *testing.T) {
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
	current, other := httpIdentityID(1), httpIdentityID(3)
	inventory := &sessionInventoryHTTPFake{result: identityapp.SessionInventory{Sessions: []identityapp.SessionInventoryItem{{SessionID: current, Current: true, CreatedAt: now.Add(-time.Hour), LastSeenAt: now, IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(11 * time.Hour)}, {SessionID: other, CreatedAt: now.Add(-2 * time.Hour), LastSeenAt: now.Add(-time.Minute), IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(10 * time.Hour)}}, ActiveSessionCount: 2, OtherSessionCount: 1, CurrentSessionLogoutAvailable: true, OtherSessionRevocationAvailable: true, RevokeAllAvailable: true, RevocationPolicyConfigured: true, ObservedAt: now}}
	administration := &sessionAdministrationHTTPFake{}
	app, sessionRaw, csrfRaw, _ := sessionAdministrationHTTPHarness(t, inventory, administration)
	response, err := app.Test(sessionAdministrationRequest(http.MethodGet, httpapi.SessionInventoryPath, sessionRaw, csrfRaw, "https://app.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "private, no-store" || inventory.calls != 1 {
		t.Fatal("session inventory response failed", response.StatusCode, readErr)
	}
	for _, forbidden := range []string{`"user_id"`, `"token"`, `"csrf"`, `"email"`, `"ip_address"`, `"user_agent"`, `"device"`} {
		if strings.Contains(string(body), forbidden) {
			t.Fatal("session inventory exposed sensitive metadata", forbidden, string(body))
		}
	}
	var envelope struct {
		Data struct {
			Sessions []struct {
				Current bool `json:"current"`
			} `json:"sessions"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &envelope); err != nil || len(envelope.Data.Sessions) != 2 || !envelope.Data.Sessions[0].Current || envelope.Data.Sessions[1].Current {
		t.Fatal("current-session inventory marker drifted", err)
	}
}

func TestSessionInventoryEnforcesOriginAndSessionBoundCSRFCookie(t *testing.T) {
	for _, tc := range []struct{ name, origin, csrf string }{{"origin", "https://evil.example", "valid"}, {"csrf_missing", "https://app.example.com", ""}, {"csrf_mismatch", "https://app.example.com", "mismatch"}} {
		t.Run(tc.name, func(t *testing.T) {
			inventory, administration := &sessionInventoryHTTPFake{}, &sessionAdministrationHTTPFake{}
			app, sessionRaw, csrfRaw, _ := sessionAdministrationHTTPHarness(t, inventory, administration)
			csrf := tc.csrf
			if csrf == "valid" {
				csrf = csrfRaw
			}
			response, err := app.Test(sessionAdministrationRequest(http.MethodGet, httpapi.SessionInventoryPath, sessionRaw, csrf, tc.origin))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusForbidden || inventory.calls != 0 {
				t.Fatal("invalid inventory browser binding crossed use case", response.StatusCode)
			}
		})
	}
}

func TestSessionAdministrationRevokesOtherAndAllWithBrowserProtection(t *testing.T) {
	inventory, administration := &sessionInventoryHTTPFake{}, &sessionAdministrationHTTPFake{}
	app, sessionRaw, csrfRaw, principal := sessionAdministrationHTTPHarness(t, inventory, administration)
	target := "00000000-0000-0000-0000-000000000003"
	otherRequest := sessionAdministrationRequest(http.MethodDelete, "/v1/auth/sessions/"+target, sessionRaw, csrfRaw, "https://app.example.com")
	otherRequest.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-4000-8000-000000000010")
	otherResponse, err := app.Test(otherRequest)
	if err != nil {
		t.Fatal(err)
	}
	otherResponse.Body.Close()
	if otherResponse.StatusCode != http.StatusNoContent || administration.otherCalls != 1 || administration.otherInput.Principal.UserID != principal.UserID || administration.otherInput.TargetSessionID == principal.SessionID || len(otherResponse.Cookies()) != 0 {
		t.Fatal("revoke-other HTTP behavior drifted", otherResponse.StatusCode)
	}
	allResponse, err := app.Test(sessionAdministrationRequest(http.MethodPost, httpapi.SessionRevokeAllPath, sessionRaw, csrfRaw, "https://app.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	allResponse.Body.Close()
	if allResponse.StatusCode != http.StatusNoContent || administration.allCalls != 1 || administration.allUser.UserID != principal.UserID || len(allResponse.Cookies()) != 2 {
		t.Fatal("revoke-all HTTP behavior drifted", allResponse.StatusCode)
	}
	for _, cookie := range allResponse.Cookies() {
		if cookie.Value != "" || cookie.MaxAge >= 0 || !cookie.Secure {
			t.Fatal("revoke-all did not clear secure browser cookies", cookie)
		}
	}
}

func TestSessionAdministrationRejectsInvalidOriginCSRFAndCurrentSession(t *testing.T) {
	for _, tc := range []struct{ name, origin, csrf string }{{"origin", "https://evil.example", "valid"}, {"csrf_missing", "https://app.example.com", ""}, {"csrf_mismatch", "https://app.example.com", "mismatch"}} {
		t.Run(tc.name, func(t *testing.T) {
			inventory, administration := &sessionInventoryHTTPFake{}, &sessionAdministrationHTTPFake{}
			app, sessionRaw, csrfRaw, _ := sessionAdministrationHTTPHarness(t, inventory, administration)
			csrf := tc.csrf
			if csrf == "valid" {
				csrf = csrfRaw
			}
			request := sessionAdministrationRequest(http.MethodPost, httpapi.SessionRevokeAllPath, sessionRaw, csrf, tc.origin)
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusForbidden || administration.allCalls != 0 {
				t.Fatal("invalid browser binding crossed session administration", response.StatusCode)
			}
		})
	}
	inventory := &sessionInventoryHTTPFake{}
	administration := &sessionAdministrationHTTPFake{otherErr: errors.New("session_administration_current_requires_logout")}
	app, sessionRaw, csrfRaw, principal := sessionAdministrationHTTPHarness(t, inventory, administration)
	request := sessionAdministrationRequest(http.MethodDelete, "/v1/auth/sessions/00000000-0000-0000-0000-000000000001", sessionRaw, csrfRaw, "https://app.example.com")
	request.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-4000-8000-000000000010")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusConflict || administration.otherCalls != 1 || administration.otherInput.TargetSessionID != principal.SessionID {
		t.Fatal("current session was not directed to logout", response.StatusCode)
	}
}
