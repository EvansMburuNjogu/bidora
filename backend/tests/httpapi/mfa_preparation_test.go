package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"github.com/gofiber/fiber/v3"
)

type readinessHTTPFake struct{ blockers []string }

func (f readinessHTTPFake) Get(context.Context, identityapp.SessionPrincipal) (identityapp.MFAReadiness, error) {
	return identityapp.MFAReadiness{Blockers: f.blockers, MFAEnrollmentAvailable: true, SessionAuthenticatedAt: time.Now().UTC(), ObservedAt: time.Now().UTC()}, nil
}
func TestMFAReadinessAlwaysSerializesBlockersAsArray(t *testing.T) {
	for _, tc := range []struct {
		name     string
		blockers []string
		want     []string
	}{{"nil", nil, []string{}}, {"empty", []string{}, []string{}}, {"nonempty", []string{"EXAMPLE_BLOCKER"}, []string{"EXAMPLE_BLOCKER"}}} {
		t.Run(tc.name, func(t *testing.T) {
			pair, err := session.GeneratePair()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			protection, err := browser.NewProtection([]string{"https://app.example.com"})
			if err != nil {
				t.Fatal(err)
			}
			validator := &sessionValidatorFake{principal: identityapp.SessionPrincipal{SessionID: httpIdentityID(1), UserID: httpIdentityID(2), CSRFHash: pair.CSRF.Hash(), AuthEpoch: 1, IdleExpiresAt: now.Add(time.Minute), AbsoluteExpiresAt: now.Add(time.Hour)}}
			middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at: now})
			if err != nil {
				t.Fatal(err)
			}
			handler, err := httpapi.NewMFAReadinessHandler(readinessHTTPFake{tc.blockers}, protection)
			if err != nil {
				t.Fatal(err)
			}
			app := fiber.New()
			app.Get(httpapi.MFAReadinessPath, middleware.Bootstrap, handler.Get)
			request := bootstrapRequest(pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap(), "https://app.example.com", httpapi.MFAReadinessPath)
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("readiness returned %d", response.StatusCode)
			}
			var body struct {
				Data struct {
					Blockers []string `json:"blockers"`
				} `json:"data"`
			}
			if err = json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Data.Blockers == nil || !reflect.DeepEqual(body.Data.Blockers, tc.want) {
				t.Fatalf("invalid blockers array: %#v", body.Data.Blockers)
			}
			if response.Header.Get("Cache-Control") != "private, no-store" {
				t.Fatal("private readiness became cacheable")
			}
		})
	}
}
