package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
	app "bidora/backend/internal/notification/application"
	"github.com/gofiber/fiber/v3"
)

type recipientHistoryHTTP struct {
	calls   int
	filters app.HistoryFilters
}

func (r *recipientHistoryHTTP) List(_ context.Context, _ app.HistorySubject, _ int32, _ *app.HistoryCursor, selected ...app.HistoryFilters) (app.HistoryPage, error) {
	r.calls++
	r.filters = selected[0]
	return app.HistoryPage{}, nil
}
func TestRecipientHistoryRequiresPrivateBodyOriginAndCSRF(t *testing.T) {
	pair, err := session.GeneratePairWithRandom(&repeatingReader{value: 19})
	if err != nil {
		t.Fatal(err)
	}
	validator := &sessionValidatorFake{}
	validator.principal.SessionID = httpIdentityID(1)
	validator.principal.UserID = httpIdentityID(2)
	validator.principal.AuthEpoch = 1
	validator.principal.CSRFHash = pair.CSRF.Hash()
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	middleware, _ := httpapi.NewBrowserSessionMiddleware(validator, &sessionToucherFake{}, protection, browserClockFake{time.Now().UTC()})
	server := fiber.New()
	server.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", "019c0000-0000-4000-8000-000000000001")
		return c.Next()
	})
	reader := &recipientHistoryHTTP{}
	handler, _ := httpapi.NewPlatformNotificationHistoryHandler(reader)
	if err = httpapi.RegisterPlatformNotificationHistoryRoutes(server, middleware, handler); err != nil {
		t.Fatal(err)
	}
	send := func(method, path, body, origin string, csrf bool) int {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Bidora-Platform-Context-Generation", "7")
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: pair.Session.RevealForCookie()})
		if csrf {
			req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: pair.CSRF.RevealForBootstrap()})
			req.Header.Set(session.CSRFHeader, pair.CSRF.RevealForBootstrap())
		}
		response, e := server.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	path := "/v1/platform/notifications/attempts/search?category=PASSWORD_RECOVERY&status=SMTP_ACCEPTED&from=2030-01-01&to=2030-01-31"
	valid := `{"recipient":"Person+Tag@Example.test"}`
	if got := send("POST", path, valid, "https://app.example.com", false); got != 403 || reader.calls != 0 {
		t.Fatal("search bypassed CSRF", got)
	}
	if got := send("POST", path, valid, "https://evil.example", true); got != 403 || reader.calls != 0 {
		t.Fatal("search bypassed Origin", got)
	}
	for _, body := range []string{`{"recipient":"bad"}`, `{"recipient":"a@example.test","unknown":1}`, `{"recipient":"a@example.test","recipient":"b@example.test"}`} {
		if got := send("POST", path, body, "https://app.example.com", true); got != 400 || reader.calls != 0 {
			t.Fatal("ambiguous/invalid search crossed boundary", got)
		}
	}
	if got := send("POST", path, valid, "https://app.example.com", true); got != 200 || reader.calls != 1 || reader.filters.Recipient != "Person+Tag@Example.test" || reader.filters.Category != "PASSWORD_RECOVERY" || reader.filters.Status != "SMTP_ACCEPTED" || reader.filters.To.Day() != 1 || reader.filters.To.Month() != time.February {
		t.Fatal("search filters not bound", got)
	}
	if got := send("GET", "/v1/platform/notifications/attempts?recipient=person@example.test", "", "https://app.example.com", false); got != 400 || reader.calls != 1 {
		t.Fatal("mailbox accepted in public query", got)
	}
}
