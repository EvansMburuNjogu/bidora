package httpapi_test

import (
	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	platform "bidora/backend/internal/platformadmin/application"
	invite "bidora/backend/internal/platformadmin/invitation"
	"bytes"
	"context"
	"errors"
	"github.com/gofiber/fiber/v3"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

type activationRegistration struct {
	registrationUseCaseFake
	denied       bool
	limitedEmail string
}

func (r *activationRegistration) AllowInvitationEnrollment(_ context.Context, email string, _ attempt.Key) error {
	r.limitedEmail = email
	if r.denied {
		return app.ErrRegistrationAttemptDenied
	}
	return nil
}

type activationUseCase struct {
	calls    int
	err      error
	password string
}

func (u *activationUseCase) ActivateInvitation(_ context.Context, in platform.ActivateInvitationInput) error {
	u.calls++
	u.password = string(in.Password)
	return u.err
}
func TestInvitationActivationHTTPRequiresOfferOriginQuotaAndIdempotencyWithoutLogin(t *testing.T) {
	token, _ := invite.Generate()
	for _, tc := range []struct {
		name, extra       string
		origin, key, deny bool
		failure           error
		status, calls     int
	}{
		{name: "activate", origin: true, key: true, status: 204, calls: 1},
		{name: "foreign origin", key: true, status: 403},
		{name: "missing key", origin: true, status: 400},
		{name: "email override", origin: true, key: true, extra: `,"email":"other@example.test"`, status: 400},
		{name: "rate limited", origin: true, key: true, deny: true, status: 429},
		{name: "protected account", origin: true, key: true, failure: platform.ErrInvitationInvalid, status: 409, calls: 1},
		{name: "unconfirmed", origin: true, key: true, failure: errors.New("database unavailable"), status: 503, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			protection, _ := browser.NewProtection([]string{"https://app.example.com"})
			ip := &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}
			key := bytes.Repeat([]byte{3}, 32)
			deriver, _ := attempt.NewDeriver(key)
			registration := &activationRegistration{denied: tc.deny}
			handler, _ := httpapi.NewRegistrationHandler(registration, protection, ip, &registrationNetworkKeyerFake{key: attempt.Key{1}})
			codes, _ := httpapi.NewEmailCodes(key, &shortCodeAttempts{}, deriver, ip)
			handler.EnableShortCodes(codes)
			handler.EnableInvitationRegistration(&invitationRegistrationFake{})
			activation := &activationUseCase{err: tc.failure}
			handler.EnableInvitationActivation(activation)
			server := fiber.New()
			server.Post(httpapi.InvitationActivationPath, handler.ActivateInvitation)
			request := httptest.NewRequest("POST", httpapi.InvitationActivationPath, strings.NewReader(`{"token":"`+token.RevealForDelivery()+`","password":"a secure test password"`+tc.extra+`}`))
			request.Header.Set("Content-Type", "application/json")
			if tc.origin {
				request.Header.Set("Origin", "https://app.example.com")
			}
			if tc.key {
				request.Header.Set("Idempotency-Key", "00000000-0000-4000-8000-000000000001")
			}
			response, err := server.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status || activation.calls != tc.calls || registration.calls != 0 || len(response.Cookies()) != 0 {
				t.Fatalf("activation boundary drift: status=%d calls=%d", response.StatusCode, activation.calls)
			}
			if tc.calls > 0 && (activation.password != "a secure test password" || registration.limitedEmail != "saved@example.test") {
				t.Fatal("password or quota was not bound to invitation snapshot")
			}
		})
	}
}
