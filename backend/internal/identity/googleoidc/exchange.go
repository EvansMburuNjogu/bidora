package googleoidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const googleTokenEndpoint = "https://oauth2.googleapis.com/token"
const maxTokenResponseBytes = 32 * 1024

type ExchangeConfig struct {
	ClientID, ClientSecret, RedirectURI string
}

func (ExchangeConfig) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google OIDC exchange config]")
}

type ExchangeResult struct{ IDToken string }

func (ExchangeResult) Format(s fmt.State, verb rune) {
	fmt.Fprint(s, "[REDACTED Google OIDC exchange result]")
}

type CodeExchanger struct {
	client HTTPRoundTripper
	config ExchangeConfig
}

func NewCodeExchanger(client HTTPRoundTripper, config ExchangeConfig) (*CodeExchanger, error) {
	if client == nil || !boundedASCII(config.ClientID, 1, 255) || !boundedSecret(config.ClientSecret, 1, 1024) || !validRedirectURI(config.RedirectURI) {
		return nil, errors.New("google_oidc_exchange_config_invalid")
	}
	return &CodeExchanger{client: client, config: config}, nil
}
func (e *CodeExchanger) Exchange(ctx context.Context, code, verifier string) (ExchangeResult, error) {
	if e == nil || e.client == nil || ctx == nil || !boundedSecret(code, 1, 4096) || !validTransactionValue(verifier) {
		return ExchangeResult{}, errors.New("google_oidc_exchange_invalid")
	}
	form := url.Values{"client_id": {e.config.ClientID}, "client_secret": {e.config.ClientSecret}, "code": {code}, "code_verifier": {verifier}, "grant_type": {"authorization_code"}, "redirect_uri": {e.config.RedirectURI}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return ExchangeResult{}, errors.New("google_oidc_exchange_unavailable")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := e.client.RoundTrip(request)
	if err != nil {
		return ExchangeResult{}, &exchangeError{cause: err}
	}
	if response == nil || response.Body == nil {
		return ExchangeResult{}, errors.New("google_oidc_exchange_unavailable")
	}
	defer response.Body.Close()
	if response.Request != nil && (response.Request.URL == nil || response.Request.URL.String() != googleTokenEndpoint) {
		return ExchangeResult{}, errors.New("google_oidc_exchange_unavailable")
	}
	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized {
		return ExchangeResult{}, errors.New("google_oidc_exchange_invalid")
	}
	if response.StatusCode != http.StatusOK {
		return ExchangeResult{}, errors.New("google_oidc_exchange_unavailable")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if contentType != "application/json" {
		return ExchangeResult{}, errors.New("google_oidc_exchange_unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseBytes+1))
	if err != nil || len(body) > maxTokenResponseBytes || uniqueJSON(body) != nil {
		return ExchangeResult{}, errors.New("google_oidc_exchange_unavailable")
	}
	var payload struct {
		IDToken string `json:"id_token"`
	}
	if json.Unmarshal(body, &payload) != nil || len(payload.IDToken) < 1 || len(payload.IDToken) > maxIDTokenBytes || strings.TrimSpace(payload.IDToken) != payload.IDToken {
		return ExchangeResult{}, errors.New("google_oidc_exchange_invalid")
	}
	return ExchangeResult{IDToken: payload.IDToken}, nil
}
func validRedirectURI(value string) bool {
	if len(value) < 1 || len(value) > 2048 || strings.TrimSpace(value) != value {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return false
	}
	return parsed.String() == value
}
func boundedSecret(value string, min, max int) bool {
	if len(value) < min || len(value) > max || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

type exchangeError struct{ cause error }

func (e *exchangeError) Error() string                 { return "google_oidc_exchange_unavailable" }
func (e *exchangeError) Unwrap() error                 { return e.cause }
func (e *exchangeError) Format(s fmt.State, verb rune) { fmt.Fprint(s, e.Error()) }
