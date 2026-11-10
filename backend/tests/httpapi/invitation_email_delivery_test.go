package httpapi_test

import (
	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"io"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

type invitedCredentialFake struct{ state string }

func (f invitedCredentialFake) LookupInvitationIdentity(context.Context, string) (app.RecoveryAccountFacts, error) {
	if f.state == "CREATE" {
		return app.RecoveryAccountFacts{}, app.ErrCredentialNotFound
	}
	return app.RecoveryAccountFacts{EmailVerified: f.state == "SIGN_IN" || f.state == "GOOGLE_SIGN_IN", PasswordCredentialPresent: f.state != "GOOGLE_SIGN_IN", GoogleIdentityPresent: f.state == "GOOGLE_SIGN_IN"}, nil
}

type invitedResendFake struct {
	calls int
	sent  bool
}

func (f *invitedResendFake) Resend(context.Context, app.EmailVerificationResendInput) (app.EmailVerificationResendResult, error) {
	f.calls++
	return app.EmailVerificationResendResult{Accepted: true, VerificationIssued: f.sent}, nil
}
func TestInvitedExistingAccountActuallyRequestsVerificationAndNeverOverwritesPassword(t *testing.T) {
	for _, tc := range []struct {
		state string
		sent  bool
		want  int
	}{{"VERIFY", true, 202}, {"VERIFY", false, 503}, {"SIGN_IN", true, 409}, {"CREATE", false, 503}} {
		t.Run(tc.state+map[bool]string{true: "-sent", false: "-not-sent"}[tc.sent], func(t *testing.T) {
			protection, _ := browser.NewProtection([]string{"https://app.example.com"})
			register := &registrationUseCaseFake{result: app.OpenRegistrationResult{Accepted: true, Created: true, VerificationIssued: false}}
			h, _ := httpapi.NewRegistrationHandler(register, protection, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}, &registrationNetworkKeyerFake{key: attempt.Key{1}})
			resend := &invitedResendFake{sent: tc.sent}
			h.EnableInvitationRegistration(&invitationRegistrationFake{})
			h.EnableInvitationVerification(invitedCredentialFake{state: tc.state}, resend)
			server := fiber.New()
			server.Post(httpapi.InvitationRegistrationPath, h.HandleInvitation)
			req := httptest.NewRequest("POST", httpapi.InvitationRegistrationPath, strings.NewReader(`{"token":"test-token","password":"another password"}`))
			req.Header.Set("Origin", "https://app.example.com")
			req.Header.Set("Content-Type", "application/json")
			response, err := server.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)
			if response.StatusCode != tc.want {
				t.Fatal("false verification outcome", response.StatusCode, string(body))
			}
			if tc.state != "CREATE" && register.calls != 0 {
				t.Fatal("existing password was overwritten")
			}
			if tc.state == "VERIFY" && resend.calls != 1 {
				t.Fatal("verification email was not attempted")
			}
			if tc.want == 503 && !strings.Contains(string(body), "VERIFICATION_DELIVERY_UNCONFIRMED") {
				t.Fatal(errors.New("delivery failure hidden"))
			}
		})
	}
}
