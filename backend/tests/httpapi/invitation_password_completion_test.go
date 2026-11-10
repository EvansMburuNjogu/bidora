package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

func TestInvitationRegistrationClassifiesIdentityWithoutChangingExistingCredentials(t *testing.T) {
	for _, state := range []string{"CREATE", "VERIFY", "SIGN_IN", "GOOGLE_SIGN_IN"} {
		t.Run(state, func(t *testing.T) {
			protection, _ := browser.NewProtection([]string{"https://app.example.com"})
			registration := &registrationUseCaseFake{}
			handler, _ := httpapi.NewRegistrationHandler(registration, protection, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}, &registrationNetworkKeyerFake{key: attempt.Key{1}})
			handler.EnableInvitationRegistration(&invitationRegistrationFake{})
			resend := &invitedResendFake{}
			handler.EnableInvitationVerification(invitedCredentialFake{state: state}, resend)
			server := fiber.New()
			server.Post(httpapi.InvitationDetailsPath, handler.InvitationDetails)
			request := httptest.NewRequest("POST", httpapi.InvitationDetailsPath, strings.NewReader(`{"token":"test-token"}`))
			request.Header.Set("Origin", "https://app.example.com")
			request.Header.Set("Content-Type", "application/json")
			response, err := server.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			var body struct {
				Data struct {
					Setup string `json:"setup_state"`
				}
			}
			if err = json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 || body.Data.Setup != state || registration.calls != 0 || resend.calls != 0 {
				t.Fatalf("metadata mutated credentials or misclassified identity: status=%d setup=%s", response.StatusCode, body.Data.Setup)
			}
		})
	}
}

func TestInvitationRegistrationRejectsRetiredRecoverySetupAndIdentityOverrides(t *testing.T) {
	for _, extra := range []string{`,"purpose":"PASSWORD_SETUP"`, `,"purpose":"COMPLETE_SETUP","code":"001234"`, `,"email":"other@example.test"`, `,"role":"PLATFORM_OWNER"`} {
		protection, _ := browser.NewProtection([]string{"https://app.example.com"})
		registration := &registrationUseCaseFake{}
		handler, _ := httpapi.NewRegistrationHandler(registration, protection, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}, &registrationNetworkKeyerFake{key: attempt.Key{1}})
		handler.EnableInvitationRegistration(&invitationRegistrationFake{})
		handler.EnableInvitationVerification(invitedCredentialFake{state: "SIGN_IN"}, &invitedResendFake{})
		server := fiber.New()
		server.Post(httpapi.InvitationRegistrationPath, handler.HandleInvitation)
		request := httptest.NewRequest("POST", httpapi.InvitationRegistrationPath, strings.NewReader(`{"token":"test-token"`+extra+`}`))
		request.Header.Set("Origin", "https://app.example.com")
		request.Header.Set("Content-Type", "application/json")
		response, err := server.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 || registration.calls != 0 {
			t.Fatalf("retired setup or identity override accepted: %d", response.StatusCode)
		}
	}
}

var _ interface {
	LookupInvitationIdentity(context.Context, string) (app.RecoveryAccountFacts, error)
} = invitedCredentialFake{}
