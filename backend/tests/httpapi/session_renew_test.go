package httpapi_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/httpapi"
	identity "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"bidora/backend/internal/identity/session"
)

func TestSessionRenewalUsesValidatedServerLeaseWithoutRotatingCookiesOrAssurance(t *testing.T) {
	v, touch := &sessionValidatorFake{}, &sessionToucherFake{}
	app, raw, csrf, at := browserHarness(t, "GET", v, touch)
	v.principal.AuthenticatedAt = at.Add(-2 * time.Hour)
	v.principal.AssuredAt = at.Add(-time.Hour)
	v.principal.AssuranceLevel = "AAL2"
	v.principal.AssuranceMethod = "EMAIL_OTP"
	protection, _ := browser.NewProtection([]string{"https://app.example.com"})
	m, _ := httpapi.NewBrowserSessionMiddleware(v, touch, protection, browserClockFake{at})
	app.Post(httpapi.SessionRenewPath, m.Renew, m.RenewResult)
	req := httptest.NewRequest("POST", httpapi.SessionRenewPath, nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set(session.CSRFHeader, csrf)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: raw})
	req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
	res, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || v.calls != 2 || touch.calls != 1 || len(res.Cookies()) != 0 {
		t.Fatal("renewal did not use server state without cookie rotation", res.StatusCode, v.calls, touch.calls)
	}
	var payload map[string]map[string]string
	if e = json.NewDecoder(res.Body).Decode(&payload); e != nil {
		t.Fatal(e)
	}
	if payload["data"]["absolute_expires_at"] != v.principal.AbsoluteExpiresAt.UTC().Format(time.RFC3339Nano) || !v.principal.AssuredAt.Equal(at.Add(-time.Hour)) {
		t.Fatal("renewal extended absolute or step-up evidence")
	}
}
func TestSessionRenewalRejectsInvalidRequestsAndNeverClearsValidCookiesOnTransientFailure(t *testing.T) {
	for _, tc := range []struct {
		name, origin, body, query string
		csrf                      bool
		validation, touch         error
		want                      int
	}{
		{name: "foreign origin", origin: "https://evil.example", csrf: true, want: 403},
		{name: "missing csrf", origin: "https://app.example.com", want: 403},
		{name: "body", origin: "https://app.example.com", csrf: true, body: "{}", want: 400},
		{name: "query", origin: "https://app.example.com", csrf: true, query: "?extend=forever", want: 400},
		{name: "invalid or revoked opaque token", origin: "https://app.example.com", csrf: true, validation: errors.New("session_invalid"), want: 401},
		{name: "revoked between validation and renewal", origin: "https://app.example.com", csrf: true, touch: identity.ErrSessionNotFound, want: 401},
		{name: "database unavailable", origin: "https://app.example.com", csrf: true, validation: errors.New("session_unavailable"), want: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, touch := &sessionValidatorFake{}, &sessionToucherFake{err: tc.touch}
			app, raw, csrf, at := browserHarness(t, "GET", v, touch)
			v.err = tc.validation
			p, _ := browser.NewProtection([]string{"https://app.example.com"})
			m, _ := httpapi.NewBrowserSessionMiddleware(v, touch, p, browserClockFake{at})
			app.Post(httpapi.SessionRenewPath, m.Renew, m.RenewResult)
			req := httptest.NewRequest("POST", httpapi.SessionRenewPath+tc.query, strings.NewReader(tc.body))
			req.Header.Set("Origin", tc.origin)
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: raw})
			if tc.csrf {
				req.Header.Set(session.CSRFHeader, csrf)
				req.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrf})
			}
			res, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.want || len(res.Cookies()) != 0 {
				t.Fatal("renewal boundary failed or cleared cookies", res.StatusCode)
			}
			if tc.touch == nil && touch.calls != 0 {
				t.Fatal("invalid request renewed the lease")
			}
		})
	}
}
