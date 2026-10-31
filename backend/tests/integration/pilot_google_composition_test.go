package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/googleoidc"
	"bidora/backend/internal/pilot"
)

func TestLivePilotCompositionRegistersGoogleSignupSigninStart(t *testing.T) {
	redisFixture := redisIntegration(t)
	config := recoveryDeploymentConfig(t, redisFixture)
	config.Runtime.Google = &pilot.GoogleConfig{ClientID: "synthetic-client.apps.googleusercontent.com", ClientSecret: "synthetic-client-secret", CallbackURL: "https://api.example.test/v1/auth/google/callback", SuccessRedirect: "https://app.example.test/auth/complete", FailureRedirect: "https://app.example.test/login", LinkCallbackURL: "https://api.example.test/v1/auth/google/link/callback", LinkSuccessRedirect: "https://app.example.test/settings/security/linked", LinkFailureRedirect: "https://app.example.test/settings/security", TransactionTTL: 5 * time.Minute, Transport: googleoidc.TransportConfig{RootCAs: config.Runtime.Redis.TLS.RootCAs, DialTimeout: time.Second, TLSHandshakeTimeout: time.Second, ResponseHeaderTimeout: time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 4, MaxIdleConnsPerHost: 2, MaxConnsPerHost: 4}}
	deployment, err := pilot.Open(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Shutdown(context.Background())
	request := httptest.NewRequest(http.MethodPost, httpapi.GoogleStartPath, nil)
	request.Header.Set("Origin", "https://app.example.test")
	response, err := deployment.Runtime.App.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") == "" || len(response.Cookies()) != 1 || response.Cookies()[0].Name != httpapi.GoogleCorrelationCookie {
		t.Fatal("pilot Google signup/signin start not composed", response.StatusCode, response.Header, response.Cookies())
	}
	linkRequest := httptest.NewRequest(http.MethodPost, httpapi.GoogleLinkStartPath, nil)
	linkRequest.Header.Set("Origin", "https://app.example.test")
	linkResponse, err := deployment.Runtime.App.Test(linkRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer linkResponse.Body.Close()
	if linkResponse.StatusCode != http.StatusUnauthorized {
		t.Fatal("pilot Google link route/middleware not composed", linkResponse.StatusCode)
	}
}
