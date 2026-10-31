package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

type mfaHTTPFake struct {
	completeCalls int
	completeErr   error
}

func (*mfaHTTPFake) Enroll(context.Context, identityapp.SessionPrincipal, identityapp.ID) (identityapp.MFAEnrollment, error) {
	return identityapp.MFAEnrollment{}, nil
}
func (*mfaHTTPFake) Confirm(context.Context, identityapp.SessionPrincipal, identityapp.ID, string) (identityapp.MFAActivationResult, error) {
	return identityapp.MFAActivationResult{}, nil
}
func (*mfaHTTPFake) StepUp(context.Context, identityapp.SessionPrincipal, identityapp.ID, string, string) (identityapp.LoginResult, error) {
	return identityapp.LoginResult{}, nil
}
func (f *mfaHTTPFake) CompleteLogin(context.Context, string, string, string, identityapp.ID) (identityapp.LoginResult, error) {
	f.completeCalls++
	return identityapp.LoginResult{}, f.completeErr
}
func (*mfaHTTPFake) ResendLogin(context.Context, string, [32]byte) (identityapp.MFALoginGateResult, error) {
	return identityapp.MFALoginGateResult{}, nil
}
func (*mfaHTTPFake) CancelLogin(context.Context, string) error { return nil }
func (*mfaHTTPFake) RegenerateRecoveryCodes(context.Context, identityapp.SessionPrincipal, identityapp.ID) ([]string, error) {
	return nil, nil
}
func (*mfaHTTPFake) BeginReplacement(context.Context, identityapp.SessionPrincipal, identityapp.ID) (identityapp.MFAEnrollment, error) {
	return identityapp.MFAEnrollment{}, nil
}
func (*mfaHTTPFake) ConfirmReplacement(context.Context, identityapp.SessionPrincipal, identityapp.ID, string) (identityapp.MFAActivationResult, error) {
	return identityapp.MFAActivationResult{}, nil
}

func TestMFAChallengeMapsLimiterDenialTo429WithTenMinuteRetry(t *testing.T) {
	protection, _ := browser.NewProtection([]string{"https://app.example.test"})
	useCase := &mfaHTTPFake{completeErr: identityapp.ErrMFAAttemptDenied}
	handler, err := httpapi.NewMFAHandler(useCase, protection)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.MFAChallengePath, handler.Challenge)
	request := httptest.NewRequest(http.MethodPost, httpapi.MFAChallengePath, strings.NewReader(`{"code":"123456","method":"TOTP"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.test")
	request.Header.Set("Cookie", "__Host-bidora_login_code="+strings.Repeat("a", 43))
	response, requestErr := app.Test(request)
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	if response.StatusCode != http.StatusTooManyRequests || response.Header.Get("Retry-After") != "600" || useCase.completeCalls != 1 {
		t.Fatal("MFA limiter HTTP mapping drifted", response.StatusCode, response.Header.Get("Retry-After"))
	}
}

func TestMFAChallengeStrictJSONRejectsUnknownDuplicateAndTrailingInput(t *testing.T) {
	protection, err := browser.NewProtection([]string{"https://app.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	useCase := &mfaHTTPFake{}
	handler, err := httpapi.NewMFAHandler(useCase, protection)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post(httpapi.MFAChallengePath, handler.Challenge)
	bodies := map[string]string{
		"unknown":   `{"code":"123456","method":"TOTP","unknown":true}`,
		"duplicate": `{"code":"123456","code":"654321","method":"TOTP"}`,
		"trailing":  `{"code":"123456","method":"TOTP"}{}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, httpapi.MFAChallengePath, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://app.example.test")
			request.Header.Set("Cookie", "__Host-bidora_login_code="+strings.Repeat("a", 43))
			response, requestErr := app.Test(request)
			if requestErr != nil {
				t.Fatal(requestErr)
			}
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("strict MFA body accepted: %d", response.StatusCode)
			}
		})
	}
	if useCase.completeCalls != 0 {
		t.Fatal("invalid MFA body crossed HTTP boundary")
	}
}
