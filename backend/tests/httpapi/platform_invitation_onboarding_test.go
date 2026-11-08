package httpapi_test

import (
	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	platformapp "bidora/backend/internal/platformadmin/application"
	"bidora/backend/internal/platformadmin/domain"
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type onboardingFake struct {
	calls        int
	err          error
	token, email string
}

func (s *onboardingFake) Preview(_ context.Context, token string) (platformapp.InvitationPreview, error) {
	s.calls++
	s.token = token
	return platformapp.InvitationPreview{Email: "invited@example.test", Role: domain.PlatformOwner, ExpiresAt: time.Now().Add(time.Hour)}, s.err
}
func (s *onboardingFake) SetUpNewAccount(_ context.Context, token string, _ []byte, _, _ string, _ platformapp.ID, _ identityapp.RegistrationPassword) (string, error) {
	s.calls++
	s.token = token
	return "invited@example.test", s.err
}

type onboardingPasswords struct{}

func (onboardingPasswords) ValidateNew([]byte) error    { return nil }
func (onboardingPasswords) Hash([]byte) (string, error) { return "test-only", nil }

type onboardingAttempt struct{ err error }
type onboardingReservation struct{}

func (onboardingReservation) Finish(context.Context, bool) error { return nil }
func (a onboardingAttempt) Begin(context.Context, attempt.Scopes) (identityapp.AttemptReservation, error) {
	if a.err != nil {
		return nil, a.err
	}
	return onboardingReservation{}, nil
}
func TestInvitationSetupHTTPKeepsCanonicalMFAAndDoesNotIssueAdministratorSession(t *testing.T) {
	for _, mode := range []string{"preview", "setup", "origin", "expired", "existing", "throttled", "storage-unavailable"} {
		t.Run(mode, func(t *testing.T) {
			service := &onboardingFake{}
			login := &loginUseCaseFake{result: identityapp.LoginResult{MFARequired: true, MFAToken: strings.Repeat("B", 43), MFAExpiresAt: time.Now().Add(5 * time.Minute)}}
			protection, _ := browser.NewProtection([]string{"https://app.example.com"})
			keys, _ := attempt.NewDeriver([]byte("0123456789abcdef0123456789abcdef"))
			handler, _ := httpapi.NewLoginHandler(login, protection, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}, keys)
			attempts := onboardingAttempt{}
			switch mode {
			case "expired":
				service.err = platformapp.ErrInvitationInvalid
			case "existing":
				service.err = platformapp.ErrInvitedAccountExists
			case "throttled":
				attempts.err = identityapp.ErrAttemptDenied
			case "storage-unavailable":
				attempts.err = errors.New("unavailable")
			}
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
				return c.Next()
			})
			if e := httpapi.RegisterPlatformInvitationOnboarding(app, service, handler, onboardingPasswords{}, attempts, keys); e != nil {
				t.Fatal(e)
			}
			path := "setup"
			if mode == "preview" {
				path = mode
			}
			payload := map[string]string{"token": strings.Repeat("A", 43)}
			if path == "setup" {
				payload["password"] = "safe-test-password"
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/v1/auth/administrator-invitation/"+path, strings.NewReader(string(body)))
			req.Header.Set("Origin", "https://app.example.com")
			req.Header.Set("Content-Type", "application/json")
			if mode == "origin" {
				req.Header.Set("Origin", "https://attacker.invalid")
			}
			response, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			defer response.Body.Close()
			expected := map[string]int{"preview": 200, "setup": 202, "origin": 403, "expired": 400, "existing": 409, "throttled": 429, "storage-unavailable": 503}[mode]
			if response.StatusCode != expected {
				t.Fatalf("status %d, want %d", response.StatusCode, expected)
			}
			if mode == "setup" {
				if login.calls != 1 || login.input.Credentials.Email != "invited@example.test" {
					t.Fatal("login bypassed or mailbox not pinned")
				}
				cookies := response.Cookies()
				if len(cookies) != 1 || cookies[0].Name != "__Host-bidora_login_code" || !cookies[0].HttpOnly || !cookies[0].Secure {
					t.Fatal("setup issued an access session or unsafe challenge")
				}
			} else if login.calls != 0 || len(response.Cookies()) != 0 {
				t.Fatal("failed/preview request issued credentials")
			}
			if response.Header.Get("Cache-Control") != "private, no-store" {
				t.Fatal("invitation response cacheable")
			}
		})
	}
}
