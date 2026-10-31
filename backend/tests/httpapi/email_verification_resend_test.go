package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/attempt"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

type verificationResendFake struct {
	result identityapp.EmailVerificationResendResult
	err    error
	input  identityapp.EmailVerificationResendInput
	calls  int
}

func (f *verificationResendFake) Resend(_ context.Context, input identityapp.EmailVerificationResendInput) (identityapp.EmailVerificationResendResult, error) {
	f.calls++
	f.input = input
	return f.result, f.err
}

func verificationResendApp(t *testing.T, usecase *verificationResendFake) *fiber.App {
	t.Helper()
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	var key attempt.Key
	key[31] = 9
	handler, err := httpapi.NewEmailVerificationResendHandler(usecase, protection, &loginIPFake{address: netip.MustParseAddr("203.0.113.9")}, &registrationNetworkKeyerFake{key: key})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{BodyLimit: httpapi.LoginBodyLimit})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.EmailVerificationResendPath, handler.Handle)
	return app
}

func verificationResendRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, httpapi.EmailVerificationResendPath, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.com")
	return request
}

func TestEmailVerificationResendHandlerReturnsGenericAccepted(t *testing.T) {
	usecase := &verificationResendFake{result: identityapp.EmailVerificationResendResult{Accepted: true}}
	response, err := verificationResendApp(t, usecase).Test(verificationResendRequest(`{"email":"User@example.test"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 202 || usecase.calls != 1 || usecase.input.Email != "User@example.test" || usecase.input.NetworkScope.IsZero() || usecase.input.CorrelationID.IsZero() || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("verification resend HTTP composition drifted", response.StatusCode)
	}
}

func TestEmailVerificationResendHandlerRejectsStrictInputAndMapsLimits(t *testing.T) {
	for _, tc := range []struct {
		name, body, origin string
		err                error
		status             int
		retry              string
	}{
		{name: "missing", body: `{}`, origin: "https://app.example.com", status: 400},
		{name: "unknown", body: `{"email":"user@example.test","role":"admin"}`, origin: "https://app.example.com", status: 400},
		{name: "duplicate", body: `{"email":"a@example.test","email":"b@example.test"}`, origin: "https://app.example.com", status: 400},
		{name: "trailing", body: `{"email":"user@example.test"}{}`, origin: "https://app.example.com", status: 400},
		{name: "origin", body: `{"email":"user@example.test"}`, origin: "https://evil.example", status: 403},
		{name: "limited", body: `{"email":"user@example.test"}`, origin: "https://app.example.com", err: errors.New("verification_resend_attempt_denied"), status: 429, retry: "900"},
		{name: "dependency", body: `{"email":"user@example.test"}`, origin: "https://app.example.com", err: errors.New("secret-canary database"), status: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &verificationResendFake{result: identityapp.EmailVerificationResendResult{Accepted: true}, err: tc.err}
			request := verificationResendRequest(tc.body)
			request.Header.Set("Origin", tc.origin)
			response, err := verificationResendApp(t, usecase).Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != tc.status || response.Header.Get("Retry-After") != tc.retry {
				t.Fatal("verification resend failure mapping drifted", response.StatusCode)
			}
		})
	}
	if handler, err := httpapi.NewEmailVerificationResendHandler(nil, nil, nil, nil); err == nil || handler != nil {
		t.Fatal("nil verification resend dependencies accepted")
	}
}
