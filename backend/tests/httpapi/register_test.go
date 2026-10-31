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

type registrationUseCaseFake struct {
	result identityapp.OpenRegistrationResult
	err    error
	input  identityapp.OpenRegistrationInput
	calls  int
}

func (r *registrationUseCaseFake) Register(_ context.Context, input identityapp.OpenRegistrationInput) (identityapp.OpenRegistrationResult, error) {
	r.calls++
	r.input = input
	r.input.Registration.Password = append([]byte(nil), input.Registration.Password...)
	return r.result, r.err
}

type registrationNetworkKeyerFake struct {
	key     attempt.Key
	err     error
	address netip.Addr
	calls   int
}

func (k *registrationNetworkKeyerFake) RegistrationNetwork(address netip.Addr) (attempt.Key, error) {
	k.calls++
	k.address = address
	return k.key, k.err
}
func registrationHandlerApp(t *testing.T, usecase *registrationUseCaseFake, ips *loginIPFake, keys *registrationNetworkKeyerFake) *fiber.App {
	t.Helper()
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	handler, err := httpapi.NewRegistrationHandler(usecase, protection, ips, keys)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{BodyLimit: httpapi.LoginBodyLimit})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	app.Post("/v1/auth/register", handler.Handle)
	return app
}
func registrationRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://app.example.com")
	return request
}
func TestRegistrationHandlerReturnsGenericAcceptedAndDerivesNetworkScope(t *testing.T) {
	var key attempt.Key
	key[31] = 8
	usecase := &registrationUseCaseFake{result: identityapp.OpenRegistrationResult{Accepted: true, Created: true, VerificationIssued: true}}
	ips := &loginIPFake{address: netip.MustParseAddr("203.0.113.9")}
	keys := &registrationNetworkKeyerFake{key: key}
	response, err := registrationHandlerApp(t, usecase, ips, keys).Test(registrationRequest(`{"first_name":"Amina","last_name":"Njoroge","email":"User@example.test","password":"synthetic password"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 202 || usecase.calls != 1 || usecase.input.Registration.FirstName != "Amina" || usecase.input.Registration.LastName != "Njoroge" || usecase.input.Registration.Email != "User@example.test" || string(usecase.input.Registration.Password) != "synthetic password" || usecase.input.NetworkScope != key || ips.calls != 1 || keys.calls != 1 {
		t.Fatal("registration HTTP composition drifted", response.StatusCode)
	}
}
func TestRegistrationHandlerMapsStrictAndGenericFailures(t *testing.T) {
	var key attempt.Key
	key[31] = 1
	for _, tc := range []struct {
		name, body, origin string
		usecaseErr         error
		status             int
		retry              string
	}{{"origin", `{"first_name":"Amina","last_name":"Njoroge","email":"user@example.test","password":"synthetic password"}`, "https://evil.example", nil, 403, ""}, {"unknown field", `{"first_name":"Amina","last_name":"Njoroge","email":"user@example.test","password":"synthetic password","role":"admin"}`, "https://app.example.com", nil, 400, ""}, {"missing first name", `{"last_name":"Njoroge","email":"user@example.test","password":"synthetic password"}`, "https://app.example.com", nil, 400, ""}, {"duplicate first name", `{"first_name":"Amina","first_name":"Changed","last_name":"Njoroge","email":"user@example.test","password":"synthetic password"}`, "https://app.example.com", nil, 400, ""}, {"wrong name type", `{"first_name":1,"last_name":"Njoroge","email":"user@example.test","password":"synthetic password"}`, "https://app.example.com", nil, 400, ""}, {"trailing value", `{"first_name":"Amina","last_name":"Njoroge","email":"user@example.test","password":"synthetic password"}{}`, "https://app.example.com", nil, 400, ""}, {"limited", `{"first_name":"Amina","last_name":"Njoroge","email":"user@example.test","password":"synthetic password"}`, "https://app.example.com", errors.New("registration_attempt_denied"), 429, "900"}, {"dependency", `{"first_name":"Amina","last_name":"Njoroge","email":"user@example.test","password":"synthetic password"}`, "https://app.example.com", errors.New("secret-canary database"), 503, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &registrationUseCaseFake{result: identityapp.OpenRegistrationResult{Accepted: true}, err: tc.usecaseErr}
			request := registrationRequest(tc.body)
			request.Header.Set("Origin", tc.origin)
			response, err := registrationHandlerApp(t, usecase, &loginIPFake{address: netip.MustParseAddr("203.0.113.9")}, &registrationNetworkKeyerFake{key: key}).Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != tc.status || response.Header.Get("Retry-After") != tc.retry {
				t.Fatal("registration failure mapping drifted", response.StatusCode)
			}
		})
	}
	if handler, err := httpapi.NewRegistrationHandler(nil, nil, nil, nil); err == nil || handler != nil {
		t.Fatal("nil registration dependencies accepted")
	}
}
