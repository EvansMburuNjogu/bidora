package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"github.com/google/uuid"
)

func TestRecoveryLinkPassesValidatedUserSelectorWithoutReplacingToken(t *testing.T) {
	app, use := recoveryHTTPHarness(t)
	id := "019c0000-0000-4000-8000-000000000041"
	token := strings.Repeat("A", 43)
	for _, tc := range []struct {
		user   string
		status int
	}{{"bad", 400}, {"00000000-0000-0000-0000-000000000000", 400}, {id, 204}} {
		body := `{"user_id":"` + tc.user + `","token":"` + token + `","new_password":"a new secure passphrase"}`
		req := httptest.NewRequest(http.MethodPost, httpapi.PasswordRecoveryResetPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://app.example.test")
		res, err := app.Test(req)
		if err != nil || res.StatusCode != tc.status {
			t.Fatalf("selector validation: %v", err)
		}
	}
	if use.resetCalls != 1 || use.lastReset.UserID != identityapp.ID(uuid.MustParse(id)) || use.lastReset.Token != token {
		t.Fatal("recovery binding lost")
	}
}
