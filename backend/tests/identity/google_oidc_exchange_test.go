package identity_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"bidora/backend/internal/identity/googleoidc"
)

type googleExchangeDoer struct {
	response               *http.Response
	err                    error
	calls                  int
	wantCode, wantVerifier string
}

func (d *googleExchangeDoer) RoundTrip(request *http.Request) (*http.Response, error) {
	d.calls++
	if request.Method != http.MethodPost || request.URL.String() != "https://oauth2.googleapis.com/token" || request.Header.Get("Accept") != "application/json" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		return nil, errors.New("unsafe token request")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 8193))
	if err != nil || len(body) > 8192 {
		return nil, errors.New("invalid token request")
	}
	form, err := url.ParseQuery(string(body))
	if err != nil || form.Get("client_id") != "client.apps.googleusercontent.com" || form.Get("client_secret") != "deployment-secret" || form.Get("redirect_uri") != "https://app.example.test/v1/auth/google/callback" || form.Get("grant_type") != "authorization_code" || form.Get("code") != d.wantCode || form.Get("code_verifier") != d.wantVerifier || len(form) != 6 {
		return nil, errors.New("token request drifted")
	}
	if d.response != nil {
		d.response.Request = request
	}
	return d.response, d.err
}
func exchangeResponse(status int, body, contentType string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
}
func exchangeConfig() googleoidc.ExchangeConfig {
	return googleoidc.ExchangeConfig{ClientID: "client.apps.googleusercontent.com", ClientSecret: "deployment-secret", RedirectURI: "https://app.example.test/v1/auth/google/callback"}
}
func TestGoogleCodeExchangeUsesExactPKCERequestAndReturnsOnlyIDToken(t *testing.T) {
	transaction := googleTransaction(t)
	client := &googleExchangeDoer{response: exchangeResponse(200, `{"id_token":"header.payload.signature","access_token":"must-not-return","refresh_token":"must-not-return"}`, "application/json; charset=utf-8"), wantCode: "authorization-code", wantVerifier: transaction.MaterialForStore().Verifier}
	exchanger, err := googleoidc.NewCodeExchanger(client, exchangeConfig())
	if err != nil {
		t.Fatal(err)
	}
	result, err := exchanger.Exchange(context.Background(), "authorization-code", transaction.MaterialForStore().Verifier)
	if err != nil {
		t.Fatal(err)
	}
	if result.IDToken != "header.payload.signature" || client.calls != 1 {
		t.Fatal("Google code exchange drifted", result)
	}
	formatted := fmt.Sprintf("%#v %#v", exchangeConfig(), result)
	for _, secret := range []string{"deployment-secret", "must-not-return", "header.payload.signature"} {
		if strings.Contains(formatted, secret) {
			t.Fatal("Google exchange formatting leaked secret")
		}
	}
}
func TestGoogleCodeExchangeFailsClosed(t *testing.T) {
	transaction := googleTransaction(t)
	verifier := transaction.MaterialForStore().Verifier
	for _, tc := range []struct {
		name     string
		response *http.Response
		cause    error
		want     string
	}{{"invalid code", exchangeResponse(400, `{"error":"invalid_grant"}`, "application/json"), nil, "google_oidc_exchange_invalid"}, {"server", exchangeResponse(500, `{}`, "application/json"), nil, "google_oidc_exchange_unavailable"}, {"content type", exchangeResponse(200, `{"id_token":"token"}`, "text/plain"), nil, "google_oidc_exchange_unavailable"}, {"duplicate", exchangeResponse(200, `{"id_token":"one","id_token":"two"}`, "application/json"), nil, "google_oidc_exchange_unavailable"}, {"network", nil, errors.New("secret-canary transport"), "google_oidc_exchange_unavailable"}} {
		t.Run(tc.name, func(t *testing.T) {
			client := &googleExchangeDoer{response: tc.response, err: tc.cause, wantCode: "code", wantVerifier: verifier}
			exchanger, _ := googleoidc.NewCodeExchanger(client, exchangeConfig())
			result, err := exchanger.Exchange(context.Background(), "code", verifier)
			if err == nil || err.Error() != tc.want || result != (googleoidc.ExchangeResult{}) || strings.Contains(fmt.Sprintf("%#v", err), "secret-canary") {
				t.Fatal("unsafe exchange result", result, err)
			}
		})
	}
	for _, config := range []googleoidc.ExchangeConfig{{}, {ClientID: "client", ClientSecret: "secret", RedirectURI: "http://app.example.test/callback"}, {ClientID: "client", ClientSecret: "secret", RedirectURI: "https://user@app.example.test/callback"}} {
		if exchanger, err := googleoidc.NewCodeExchanger(&googleExchangeDoer{}, config); err == nil || exchanger != nil {
			t.Fatal("unsafe exchange config accepted")
		}
	}
	var zero *googleoidc.CodeExchanger
	if _, err := zero.Exchange(context.Background(), "code", verifier); err == nil {
		t.Fatal("zero exchanger accepted")
	}
}
