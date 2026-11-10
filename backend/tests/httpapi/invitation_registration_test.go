package httpapi_test

import (
	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	platform "bidora/backend/internal/platformadmin/application"
	"context"
	"github.com/gofiber/fiber/v3"
	"io"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

type invitationRegistrationFake struct {
	err   error
	calls int
}

func (f *invitationRegistrationFake) RegistrationIdentity(context.Context, string) (platform.InvitationRegistrationIdentity, error) {
	f.calls++
	return platform.InvitationRegistrationIdentity{FirstName: "Invited", LastName: "Person", Email: "saved@example.test", Role: "PLATFORM_OWNER"}, f.err
}
func TestInvitationDetailsAreMaskedReadOnlyAndProofBound(t *testing.T) {
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	use := &registrationUseCaseFake{}
	h, _ := httpapi.NewRegistrationHandler(use, protection, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}, &registrationNetworkKeyerFake{key: attempt.Key{1}})
	resolver := &invitationRegistrationFake{}
	h.EnableInvitationRegistration(resolver)
	server := fiber.New()
	server.Post(httpapi.InvitationDetailsPath, h.InvitationDetails)
	for _, invalid := range []bool{false, true} {
		if invalid {
			resolver.err = platform.ErrInvitationInvalid
		}
		req := httptest.NewRequest("POST", httpapi.InvitationDetailsPath, strings.NewReader(`{"token":"test-token"}`))
		req.Header.Set("Origin", "https://app.example.com")
		req.Header.Set("Content-Type", "application/json")
		response, err := server.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if invalid {
			if response.StatusCode != 400 || strings.Contains(string(raw), "saved@example.test") {
				t.Fatal("invalid invitation leaked identity")
			}
		} else {
			if response.StatusCode != 200 || !strings.Contains(string(raw), "s***@e***.test") || !strings.Contains(string(raw), "PLATFORM_OWNER") || strings.Contains(string(raw), "saved@example.test") {
				t.Fatal("incorrect masked invitation details")
			}
		}
	}
	if use.calls != 0 {
		t.Fatal("reading details created account")
	}
}

func TestInvitationRegistrationUsesSavedIdentityAndRejectsOverrides(t *testing.T) {
	for _, tc := range []struct {
		name, body, origin string
		invalid            bool
		status             int
	}{
		{"password only", `{"token":"test-token","password":"synthetic password"}`, "https://app.example.com", false, 202},
		{"identity override", `{"token":"test-token","password":"synthetic password","email":"attacker@example.test"}`, "https://app.example.com", false, 400},
		{"name override", `{"token":"test-token","password":"synthetic password","first_name":"Attacker"}`, "https://app.example.com", false, 400},
		{"duplicate", `{"token":"a","token":"b","password":"synthetic password"}`, "https://app.example.com", false, 400},
		{"foreign origin", `{"token":"test-token","password":"synthetic password"}`, "https://foreign.invalid", false, 403},
		{"invalid invitation", `{"token":"test-token","password":"synthetic password"}`, "https://app.example.com", true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			protection, _ := browser.NewProtection([]string{"https://app.example.com"})
			use := &registrationUseCaseFake{result: app.OpenRegistrationResult{Accepted: true}}
			h, _ := httpapi.NewRegistrationHandler(use, protection, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}, &registrationNetworkKeyerFake{key: attempt.Key{1}})
			resolver := &invitationRegistrationFake{}
			if tc.invalid {
				resolver.err = platform.ErrInvitationInvalid
			}
			h.EnableInvitationRegistration(resolver)
			server := fiber.New()
			server.Post(httpapi.InvitationRegistrationPath, h.HandleInvitation)
			req := httptest.NewRequest("POST", httpapi.InvitationRegistrationPath, strings.NewReader(tc.body))
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Content-Type", "application/json")
			response, err := server.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatal("unexpected setup status", response.StatusCode)
			}
			if tc.status == 202 {
				r := use.input.Registration
				if use.calls != 1 || r.FirstName != "Invited" || r.LastName != "Person" || r.Email != "saved@example.test" || string(r.Password) != "synthetic password" {
					t.Fatal("invitation identity not used")
				}
			} else if use.calls != 0 {
				t.Fatal("invalid setup created an account")
			}
			if tc.name == "foreign origin" && resolver.calls != 0 {
				t.Fatal("foreign origin reached invitation lookup")
			}
		})
	}
}
