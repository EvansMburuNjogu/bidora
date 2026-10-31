package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

type emailVerificationUseCaseFake struct {
	result identityapp.EmailVerificationResult
	err    error
	input  identityapp.EmailVerificationInput
	calls  int
}

func (u *emailVerificationUseCaseFake) Verify(_ context.Context, input identityapp.EmailVerificationInput) (identityapp.EmailVerificationResult, error) {
	u.calls++
	u.input = input
	return u.result, u.err
}

func emailVerificationApp(t *testing.T, usecase *emailVerificationUseCaseFake) *fiber.App {
	t.Helper()
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	handler, err := httpapi.NewEmailVerificationHandler(usecase, protection)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{BodyLimit: httpapi.LoginBodyLimit})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.EmailVerificationPath, handler.Handle)
	return app
}

func emailVerificationRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, httpapi.EmailVerificationPath, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.com")
	return request
}

func TestEmailVerificationHandlerReturnsNoContentWithoutSession(t *testing.T) {
	usecase := &emailVerificationUseCaseFake{result: identityapp.EmailVerificationResult{}}
	response, err := emailVerificationApp(t, usecase).Test(emailVerificationRequest(`{"token":"synthetic-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent || len(response.Cookies()) != 0 || usecase.calls != 1 || usecase.input.Token != "synthetic-token" || usecase.input.CorrelationID.IsZero() {
		t.Fatal("email verification HTTP composition drifted", response.StatusCode, response.Cookies())
	}
}

func TestEmailVerificationHandlerRejectsUnsafeShapesAndConcealsTokenState(t *testing.T) {
	cases := []struct {
		name, body, origin, contentType string
		usecaseErr                      error
		status                          int
	}{{"origin", `{"token":"synthetic-token"}`, "https://evil.example", "application/json", nil, 403}, {"missing token", `{}`, "https://app.example.com", "application/json", nil, 400}, {"unknown field", `{"token":"synthetic-token","user_id":"019c0000-0000-4000-8000-000000000001"}`, "https://app.example.com", "application/json", nil, 400}, {"duplicate field", `{"token":"one","token":"two"}`, "https://app.example.com", "application/json", nil, 400}, {"media", `{"token":"synthetic-token"}`, "https://app.example.com", "text/plain", nil, 415}, {"invalid expired replayed", `{"token":"synthetic-token"}`, "https://app.example.com", "application/json", errors.New("verification_invalid"), 400}, {"dependency", `{"token":"synthetic-token"}`, "https://app.example.com", "application/json", errors.New("secret-canary database"), 503}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &emailVerificationUseCaseFake{err: tc.usecaseErr}
			request := emailVerificationRequest(tc.body)
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("Content-Type", tc.contentType)
			response, err := emailVerificationApp(t, usecase).Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != tc.status || len(response.Cookies()) != 0 {
				t.Fatal("verification failure mapping drifted", response.StatusCode)
			}
		})
	}
	if handler, err := httpapi.NewEmailVerificationHandler(nil, nil); err == nil || handler != nil {
		t.Fatal("nil verification dependencies accepted")
	}
}
