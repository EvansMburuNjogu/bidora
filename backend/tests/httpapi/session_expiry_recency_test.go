package httpapi_test

import (
	"bidora/backend/internal/httpapi"
	"net/http"
	"testing"
	"time"
)

func TestSessionRevocationUsesTrustedSignInTimeRatherThanConfiguredExpiry(t *testing.T) {
	for _, expiry := range []time.Duration{time.Hour, 12 * time.Hour, 7 * 24 * time.Hour} {
		t.Run(expiry.String(), func(t *testing.T) {
			service := &sessionAdministrationHTTPFake{}
			app, session, csrf, principal := sessionAdministrationHTTPHarness(t, &sessionInventoryHTTPFake{}, service, expiry)
			request := sessionAdministrationRequest(http.MethodDelete, "/v1/auth/sessions/00000000-0000-0000-0000-000000000003", session, csrf, "https://app.example.com")
			request.Header.Set(httpapi.IdempotencyKeyHeader, "019c0000-0000-4000-8000-000000000010")
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 204 || !service.otherInput.SessionAuthenticatedAt.Equal(principal.AuthenticatedAt) {
				t.Fatal("revoke-other inferred recency from expiry")
			}
			response, err = app.Test(sessionAdministrationRequest(http.MethodPost, httpapi.SessionRevokeAllPath, session, csrf, "https://app.example.com"))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 204 || !service.allAuthenticatedAt.Equal(principal.AuthenticatedAt) {
				t.Fatal("revoke-all inferred recency from expiry")
			}
		})
	}
}
