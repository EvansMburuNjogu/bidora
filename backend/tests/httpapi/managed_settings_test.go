package httpapi_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	n "bidora/backend/internal/notification/application"
	nd "bidora/backend/internal/notification/domain"
	g "bidora/backend/internal/platformsettings/application"
	gd "bidora/backend/internal/platformsettings/domain"
	"github.com/gofiber/fiber/v3"
)

type googleHTTPUnit struct {
	g.GoogleTx
	calls int
	at    time.Time
}

func (f *googleHTTPUnit) WithinGoogle(ctx context.Context, _ bool, fn func(context.Context, g.GoogleTx) error) error {
	f.calls++
	return fn(ctx, f)
}
func (f *googleHTTPUnit) AuthorizeInformation(context.Context, g.Subject, time.Time) (g.Authority, error) {
	return g.Authority{Capabilities: map[string]bool{"platform.integration.read": true}, AssuranceLevel: "AAL2", AssuranceMethod: "TOTP", AssuredAt: f.at}, nil
}
func (f *googleHTTPUnit) ReadGoogle(context.Context) (g.GoogleStored, error) {
	return g.GoogleStored{Configuration: g.GoogleConfiguration{ID: gd.ID{39}, Version: 1, CreatedAt: f.at}, Secret: g.GoogleCipher{ID: gd.ID{40}, Ciphertext: []byte("secret-ciphertext-canary")}}, nil
}

type googleHTTPIDs struct{}

func (googleHTTPIDs) NewID() (gd.ID, error) { return gd.ID{41}, nil }

type notificationHTTPUnit struct {
	n.TemplateTx
	calls int
	at    time.Time
}

func (f *notificationHTTPUnit) WithinTemplates(ctx context.Context, _ bool, fn func(context.Context, n.TemplateTx) error) error {
	f.calls++
	return fn(ctx, f)
}
func (f *notificationHTTPUnit) AuthorizeTemplates(context.Context, n.TemplateSubject, bool, time.Time) error {
	return nil
}
func (f *notificationHTTPUnit) ListTemplates(context.Context) ([]n.Template, error) {
	return []n.Template{{Key: "identity.password_recovery", Name: "Recovery", Subject: "Recovery", Body: "{{token}} {{expires_at}}", Enabled: true, Version: 1, UpdatedAt: f.at}}, nil
}

type notificationHTTPIDs struct{}

func (notificationHTTPIDs) NewID() (nd.ID, error) { return nd.ID{42}, nil }
func TestManagedSettingsHTTPStrictBindingRedactionAndPublicOrigin(t *testing.T) {
	at := time.Now().UTC()
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 29})
	if err != nil {
		t.Fatal(err)
	}
	validator := &sessionValidatorFake{}
	validator.principal.SessionID = httpIdentityID(1)
	validator.principal.UserID = httpIdentityID(2)
	validator.principal.AuthEpoch = 1
	validator.principal.CSRFHash = pair.CSRF.Hash()
	protection, err := browser.NewProtection([]string{"https://app.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{at})
	if err != nil {
		t.Fatal(err)
	}
	server := fiber.New()
	server.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		c.Set("Cache-Control", "no-store") // mirror the real base middleware
		return c.Next()
	})
	googleUnit := &googleHTTPUnit{at: at}
	googleService, err := g.NewGoogleService(googleUnit, nil, nil, googleHTTPIDs{}, browserClockFake{at}, false)
	if err != nil {
		t.Fatal(err)
	}
	googleHandler, err := httpapi.NewPlatformGoogleSettingsHandler(googleService, protection)
	if err != nil {
		t.Fatal(err)
	}
	if err = httpapi.RegisterPlatformGoogleSettingsRoutes(server, middleware, googleHandler); err != nil {
		t.Fatal(err)
	}
	notificationUnit := &notificationHTTPUnit{at: at}
	notificationService, err := n.NewTemplateService(notificationUnit, notificationHTTPIDs{}, browserClockFake{at}, nil)
	if err != nil {
		t.Fatal(err)
	}
	notificationHandler, err := httpapi.NewPlatformNotificationTemplatesHandler(notificationService)
	if err != nil {
		t.Fatal(err)
	}
	if err = httpapi.RegisterPlatformNotificationTemplateRoutes(server, middleware, notificationHandler); err != nil {
		t.Fatal(err)
	}
	send := func(method, path, body, origin string, authenticated bool) (int, string) {
		t.Helper()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Origin", origin)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Bidora-Platform-Context-Generation", "7")
		request.Header.Set("If-Match", "\"1\"")
		request.Header.Set("Idempotency-Key", "019c0000-0000-4000-8000-000000000002")
		validator.err = nil
		if !authenticated {
			validator.err = errors.New("session_invalid")
		}
		if authenticated {
			request.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
			request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			request.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
		}
		response, e := server.Test(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		raw, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(response.Header.Get("Cache-Control"), "no-store") {
			t.Fatal("private response cacheable")
		}
		return response.StatusCode, string(raw)
	}
	for _, path := range []string{"/v1/platform/settings/google", "/v1/platform/notification-templates"} {
		status, body := send("GET", path, "", "https://app.example.com", true)
		if status != 200 || strings.Contains(body, "secret-ciphertext-canary") || strings.Contains(body, "client_secret\"") {
			t.Fatal("unsafe settings response", status)
		}
		status, _ = send("GET", path, "", "https://app.example.com", false)
		if status != 401 {
			t.Fatal("anonymous settings permitted", status)
		}
	}
	status, body := send("GET", "/v1/auth/google/status", "", "https://app.example.com", false)
	if status != 200 || strings.Contains(body, "client_id") || strings.Contains(body, "secret_") || !strings.Contains(body, "\"enabled\":false") {
		t.Fatal("public status leaked configuration or false readiness", status)
	}
	status, _ = send("GET", "/v1/auth/google/status", "", "https://evil.example", false)
	if status != 403 {
		t.Fatal("public status accepted foreign origin", status)
	}
	gCalls, nCalls := googleUnit.calls, notificationUnit.calls
	for _, body := range []string{`{"enabled":false,"enabled":false,"allow_signup":false,"client_id":"","allowed_domains":[]}`, `{"enabled":false,"allow_signup":false,"client_id":"","allowed_domains":[],"client_secret":null}`, `{"enabled":false,"allow_signup":false,"client_id":"","allowed_domains":[],"unexpected":true}`, `{"allow_signup":false,"client_id":"","allowed_domains":[]}`, `null`, `{} {}`, `{"enabled":false,"allow_signup":false,"client_id":"","allowed_domains":null}`} {
		status, _ := send("PUT", "/v1/platform/settings/google", body, "https://app.example.com", true)
		if status != 400 {
			t.Fatal("ambiguous Google request accepted", status, body)
		}
	}
	for _, body := range []string{`{"name":"Recovery","subject":"Recovery","body":"{{token}} {{expires_at}}","enabled":true,"enabled":true}`, `{"name":"Recovery","subject":"Recovery","body":"{{token}} {{expires_at}}","enabled":true,"channel":"SMS"}`, `{"name":"Recovery","subject":"Recovery","body":null,"enabled":true}`, `{"name":"Recovery","body":"{{token}} {{expires_at}}","enabled":true}`} {
		status, _ := send("PUT", "/v1/platform/notification-templates/identity.password_recovery", body, "https://app.example.com", true)
		if status != 400 {
			t.Fatal("ambiguous template request accepted", status, body)
		}
	}
	if googleUnit.calls != gCalls || notificationUnit.calls != nCalls {
		t.Fatal("malformed request reached owner persistence")
	}
}
