package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	app "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	"bidora/backend/internal/postgres"
	"github.com/gofiber/fiber/v3"
)

type auditIPResolver struct{ err error }

func (r auditIPResolver) Resolve(netip.AddrPort, string) (netip.Addr, error) {
	return netip.MustParseAddr("192.0.2.20"), r.err
}

type auditMFAUseCase struct {
	mfaHTTPFake
	metadata postgres.RequestAudit
	captured bool
}

func (f *auditMFAUseCase) CompleteLogin(ctx context.Context, _, _, _ string, _ app.ID) (app.LoginResult, error) {
	f.completeCalls++
	f.metadata, f.captured = postgres.RequestAuditFrom(ctx)
	pair, err := session.GeneratePair()
	return app.LoginResult{Session: pair.Session, CSRF: pair.CSRF, AbsoluteExpiresAt: time.Now().Add(time.Hour)}, err
}
func TestLoginCodeCompletionReceivesAuditContextBeforeSessionExists(t *testing.T) {
	protection, _ := browser.NewProtection([]string{"https://app.example.test"})
	useCase := &auditMFAUseCase{}
	handler, _ := httpapi.NewMFAHandler(useCase, protection)
	middleware := &httpapi.BrowserSessionMiddleware{}
	if err := middleware.ConfigureRequestAudit(auditIPResolver{}); err != nil {
		t.Fatal(err)
	}
	server := fiber.New()
	server.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	server.Use("/v1", middleware.CaptureAuditContext)
	server.Post(httpapi.MFAChallengePath, handler.Challenge)
	for index, agent := range []string{"Mozilla/5.0 Firefox/123", "", "Mozilla/5.0 Firefox/124"} {
		path := httpapi.MFAChallengePath
		if index == 2 {
			path = strings.ToUpper(path)
		}
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"code":"123456","method":"EMAIL_OTP"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://app.example.test")
		request.Header.Set("Cookie", "__Host-bidora_login_code="+strings.Repeat("a", 43))
		request.Header.Set("User-Agent", agent)
		response, err := server.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		want := "Firefox"
		if agent == "" {
			want = "Unknown browser"
		}
		if response.StatusCode != 204 || !useCase.captured || useCase.metadata.IP.String() != "192.0.2.20" || useCase.metadata.Browser != want {
			t.Fatal("sign-in completion omitted request context", response.StatusCode)
		}
	}
	if useCase.completeCalls != 3 {
		t.Fatal("repeat sign-in did not capture again")
	}
	if err := middleware.ConfigureRequestAudit(auditIPResolver{err: errors.New("untrusted metadata")}); err != nil {
		t.Fatal(err)
	}
	response, err := server.Test(httptest.NewRequest(http.MethodPost, httpapi.MFAChallengePath, strings.NewReader(`{}`)))
	if err != nil || response.StatusCode != 503 || useCase.completeCalls != 3 {
		t.Fatal("missing trusted network information did not fail closed", err)
	}
}
