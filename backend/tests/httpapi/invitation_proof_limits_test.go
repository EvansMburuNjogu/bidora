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
	"github.com/gofiber/fiber/v3"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func fiberAppForInvitationDetails(h *httpapi.RegistrationHandler) *fiber.App {
	server := fiber.New()
	server.Post(httpapi.InvitationDetailsPath, h.InvitationDetails)
	return server
}

type codeProofResolver struct{ calls int }

func (r *codeProofResolver) RegistrationIdentity(context.Context, string) (platform.InvitationRegistrationIdentity, error) {
	r.calls++
	return platform.InvitationRegistrationIdentity{}, platform.ErrInvitationInvalid
}
func TestShortInvitationProofAttemptsShareNonceAndFailClosed(t *testing.T) {
	token, err := invite.Generate()
	if err != nil {
		t.Fatal(err)
	}
	wire := token.RevealForDelivery()
	nonce := strings.Split(wire, ".")[0]
	key := bytes.Repeat([]byte{3}, 32)
	deriver, _ := attempt.NewDeriver(key)
	limits := &shortCodeAttempts{}
	codes, _ := httpapi.NewEmailCodes(key, limits, deriver, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")})
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	handler, _ := httpapi.NewRegistrationHandler(&registrationUseCaseFake{result: app.OpenRegistrationResult{Accepted: true}}, protection, &loginIPFake{address: netip.MustParseAddr("192.0.2.1")}, &registrationNetworkKeyerFake{key: attempt.Key{1}})
	resolver := &codeProofResolver{}
	handler.EnableInvitationRegistration(resolver)
	handler.EnableShortCodes(codes)
	server := fiberAppForInvitationDetails(handler)
	for _, code := range []string{"000000", "000001", "000002"} {
		if code == "000002" {
			limits.denied = true
		}
		req := httptest.NewRequest("POST", httpapi.InvitationDetailsPath, strings.NewReader(`{"token":"`+nonce+"."+code+`"}`))
		req.Header.Set("Origin", "https://app.example.com")
		req.Header.Set("Content-Type", "application/json")
		response, e := server.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		expected := 400
		if limits.denied {
			expected = 429
		}
		if response.StatusCode != expected {
			t.Fatal("proof attempt result wrong")
		}
	}
	if resolver.calls != 2 || len(limits.scopes) != 3 || limits.scopes[0].Identity != limits.scopes[1].Identity || limits.scopes[1].Identity != limits.scopes[2].Identity {
		t.Fatal("code guesses escaped proof limits")
	}
}
