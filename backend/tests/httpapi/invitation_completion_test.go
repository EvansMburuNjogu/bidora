package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	platform "bidora/backend/internal/platformadmin/application"
	"github.com/gofiber/fiber/v3"
)

type completionIDs struct{ n byte }

func (c *completionIDs) NewID() (app.ID, error) { c.n++; return app.ID{c.n}, nil }

type completionSessions struct{ commits int }

func (c *completionSessions) CommitLoginSession(context.Context, app.SessionCommit) error {
	c.commits++
	return nil
}

type completionInvitation struct {
	httpapi.PlatformAdministrationUseCase
	calls int
	input platform.AcceptInvitationInput
	err   error
}

func (c *completionInvitation) AcceptInvitation(_ context.Context, input platform.AcceptInvitationInput) (platform.AcceptInvitationResult, error) {
	c.calls++
	c.input = input
	return platform.AcceptInvitationResult{AuthEpoch: 2}, c.err
}

func TestInvitationAcceptancePreparesAtomicContinuationFromActualEmailProof(t *testing.T) {
	for _, tc := range []struct {
		name, level, method string
		age                 time.Duration
		failure             error
		status, calls       int
	}{
		{"ordinary email proof", "AAL1", "EMAIL_OTP", 3 * time.Minute, nil, 204, 1},
		{"administrator email proof", "AAL2", "EMAIL_OTP", 3 * time.Minute, nil, 204, 1},
		{"primary only", "AAL1", "PRIMARY_ONLY", 0, nil, 403, 0},
		{"expired proof", "AAL1", "EMAIL_OTP", 16 * time.Minute, nil, 403, 0},
		{"future proof", "AAL1", "EMAIL_OTP", -time.Minute, nil, 403, 0},
		{"transaction fails", "AAL1", "EMAIL_OTP", time.Minute, platform.ErrInvitationInvalid, 400, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			pair, _ := session.GeneratePair()
			validator := &sessionValidatorFake{principal: app.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), AuthEpoch: 1, CSRFHash: pair.CSRF.Hash(), AssuranceLevel: tc.level, AssuranceMethod: tc.method, AssuredAt: now.Add(-tc.age), IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour)}}
			protection, _ := browser.NewProtection([]string{"https://app.example.com"})
			middleware, _ := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{now})
			usecase := &completionInvitation{err: tc.failure}
			commits := &completionSessions{}
			issuer, _ := app.NewSessionIssuer(app.SecureSessionTokenGenerator{}, &completionIDs{}, browserClockFake{now}, commits)
			handler, _ := httpapi.NewPlatformAdministrationHandler(usecase, browserClockFake{now})
			handler.EnableSessionContinuation(issuer)
			server := fiber.New()
			server.Use(func(c fiber.Ctx) error {
				c.Set("X-Request-ID", "00000000-0000-4000-8000-000000000001")
				return c.Next()
			})
			if err := httpapi.RegisterPlatformInvitationRoutes(server, middleware, handler); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("POST", "/v1/platform/administrator-invitations/accept", strings.NewReader(`{"token":"`+strings.Repeat("A", 43)+`"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://app.example.com")
			request.Header.Set("Idempotency-Key", "00000000-0000-4000-8000-000000000002")
			request.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
			request.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
			request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			response, err := server.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status || usecase.calls != tc.calls {
				t.Fatalf("status=%d calls=%d, want %d/%d", response.StatusCode, usecase.calls, tc.status, tc.calls)
			}
			if commits.commits != 0 {
				t.Fatal("replacement was committed outside invitation transaction")
			}
			if tc.calls > 0 {
				prepared := usecase.input.Session
				if prepared == nil || !prepared.AssuredAt.Equal(validator.principal.AssuredAt) || prepared.SessionHash == [32]byte{} || prepared.CSRFHash == [32]byte{} {
					t.Fatal("actual proof timestamp/session preparation lost")
				}
			}
			if tc.status == 204 {
				if len(response.Cookies()) != 2 {
					t.Fatal("missing final session cookies")
				}
			} else if len(response.Cookies()) != 0 {
				t.Fatal(errors.New("failure exposed replacement credentials"))
			}
		})
	}
}
