package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

type recoveryHTTPFake struct {
	requestCalls, resetCalls int
	lastReset                identityapp.PasswordRecoveryResetInput
}

func (f *recoveryHTTPFake) Request(context.Context, identityapp.PublicPasswordRecoveryInput) error {
	f.requestCalls++
	return nil
}
func (f *recoveryHTTPFake) Reset(_ context.Context, in identityapp.PasswordRecoveryResetInput) error {
	f.lastReset = in
	f.resetCalls++
	return nil
}

type recoveryKeyerFake struct{}

func (recoveryKeyerFake) RecoveryNetwork(netip.Addr) (attempt.Key, error) {
	var k attempt.Key
	k[0] = 1
	return k, nil
}
func recoveryHTTPHarness(t *testing.T) (*fiber.App, *recoveryHTTPFake) {
	t.Helper()
	protection, e := browser.NewProtection([]string{"https://app.example.test"})
	if e != nil {
		t.Fatal(e)
	}
	use := &recoveryHTTPFake{}
	handler, e := httpapi.NewPasswordRecoveryHandler(use, use, protection, &loginIPFake{address: netip.MustParseAddr("198.51.100.1")}, recoveryKeyerFake{})
	if e != nil {
		t.Fatal(e)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.PasswordRecoveryRequestPath, handler.Request)
	app.Post(httpapi.PasswordRecoveryResetPath, handler.Reset)
	return app, use
}
func TestPasswordRecoveryHTTPStrictBindingAndGenericResponses(t *testing.T) {
	app, use := recoveryHTTPHarness(t)
	invalid := []struct{ path, body string }{{httpapi.PasswordRecoveryRequestPath, `{"email":"a@example.test","unknown":true}`}, {httpapi.PasswordRecoveryRequestPath, `{"email":"a@example.test","email":"b@example.test"}`}, {httpapi.PasswordRecoveryResetPath, `{"token":"abc","new_password":"replacement password"}{}`}}
	for _, tc := range invalid {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://app.example.test")
		res, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatal("invalid recovery JSON accepted", res.StatusCode)
		}
	}
	if use.requestCalls != 0 || use.resetCalls != 0 {
		t.Fatal("invalid recovery request crossed boundary")
	}
	req := httptest.NewRequest(http.MethodPost, httpapi.PasswordRecoveryRequestPath, strings.NewReader(`{"email":"unknown@example.test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://app.example.test")
	res, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	if res.StatusCode != http.StatusAccepted {
		t.Fatal("generic recovery response drifted", res.StatusCode)
	}
}
