package identity_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"bidora/backend/internal/identity/browser"
)

func TestBrowserProtectionRequiresExactHTTPSOriginAndCSRF(t *testing.T) {
	protection, err := browser.NewProtection([]string{"https://App.Example.com:443/", "https://[2001:db8::1]:8443"})
	if err != nil {
		t.Fatal(err)
	}
	pair := sessionPair(t)
	for _, origin := range []string{"https://app.example.com", "https://APP.EXAMPLE.COM:443/", "https://[2001:db8::1]:8443"} {
		if err := protection.ValidateUnsafe(origin, pair.CSRF.RevealForBootstrap(), pair.CSRF.RevealForBootstrap(), pair.CSRF.Hash()); err != nil {
			t.Fatal("approved origin/CSRF rejected", origin, err)
		}
	}
	for _, tc := range []struct{ origin, header, cookie string }{{"https://evil.example", pair.CSRF.RevealForBootstrap(), pair.CSRF.RevealForBootstrap()}, {"http://app.example.com", pair.CSRF.RevealForBootstrap(), pair.CSRF.RevealForBootstrap()}, {"https://app.example.com/path", pair.CSRF.RevealForBootstrap(), pair.CSRF.RevealForBootstrap()}, {"https://app.example.com", pair.Session.RevealForCookie(), pair.CSRF.RevealForBootstrap()}, {"https://app.example.com", pair.CSRF.RevealForBootstrap(), pair.Session.RevealForCookie()}, {"https://app.example.com", "malformed", pair.CSRF.RevealForBootstrap()}, {"", pair.CSRF.RevealForBootstrap(), pair.CSRF.RevealForBootstrap()}} {
		if err := protection.ValidateUnsafe(tc.origin, tc.header, tc.cookie, pair.CSRF.Hash()); err == nil || err.Error() != "browser_request_forbidden" {
			t.Fatal("unsafe browser request accepted", tc)
		}
	}
	var zero *browser.Protection
	if err := zero.ValidateUnsafe("https://app.example.com", pair.CSRF.RevealForBootstrap(), pair.CSRF.RevealForBootstrap(), pair.CSRF.Hash()); err == nil {
		t.Fatal("nil protection accepted request")
	}
}

func TestBrowserOriginConfigurationRejectsAmbiguityAndWildcards(t *testing.T) {
	invalid := []string{"", "http://example.com", "https://*.example.com", "https://user@example.com", "https://example.com/path", "https://example.com?query=1", "https://example.com#fragment", "https://example.com:0", "https://example.com:65536", "https://-example.com", "https://example.com.", "https://éxample.com"}
	for _, origin := range invalid {
		if protection, err := browser.NewProtection([]string{origin}); err == nil || protection != nil {
			t.Fatal("invalid origin accepted", origin)
		}
	}
	if protection, err := browser.NewProtection(nil); err == nil || err.Error() != "browser_origins_required" || protection != nil {
		t.Fatal("empty origin policy accepted")
	}
	if protection, err := browser.NewProtection([]string{"https://example.com", "https://EXAMPLE.COM:443/"}); err == nil || err.Error() != "browser_origin_duplicate" || protection != nil {
		t.Fatal("canonical duplicate accepted")
	}
}

func TestApprovedSessionCookieAttributesAndClearing(t *testing.T) {
	pair := sessionPair(t)
	// The selected absolute Expires governs cookies; a fixed Max-Age would
	// override ordinary policy limits or truncate the administrator profile.
	expiry := time.Now().Add(12 * time.Hour).In(time.FixedZone("source", 3600))
	cookie, err := browser.SessionCookie(pair.Session.RevealForCookie(), expiry)
	if err != nil {
		t.Fatal(err)
	}
	if cookie.Name != "__Host-bidora_session" || cookie.Value != pair.Session.RevealForCookie() || cookie.Path != "/" || cookie.Domain != "" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 0 || !cookie.Expires.Equal(expiry.UTC()) {
		t.Fatal("session cookie policy drifted", cookie)
	}
	if _, err := browser.SessionCookie("malformed", expiry); err == nil || err.Error() != "session_cookie_invalid" {
		t.Fatal("malformed session cookie accepted")
	}
	if _, err := browser.SessionCookie(pair.Session.RevealForCookie(), time.Time{}); err == nil {
		t.Fatal("zero cookie expiry accepted")
	}
	csrf, err := browser.CSRFCookie(pair.CSRF.RevealForBootstrap(), expiry)
	if err != nil {
		t.Fatal(err)
	}
	if csrf.Name != "__Host-bidora_csrf" || csrf.Value != pair.CSRF.RevealForBootstrap() || csrf.Path != "/" || csrf.Domain != "" || !csrf.Secure || csrf.HttpOnly || csrf.SameSite != http.SameSiteLaxMode || csrf.MaxAge != 0 || !csrf.Expires.Equal(expiry.UTC()) {
		t.Fatal("csrf cookie policy drifted", csrf)
	}
	if _, err := browser.CSRFCookie("malformed", expiry); err == nil || err.Error() != "csrf_cookie_invalid" {
		t.Fatal("malformed csrf cookie accepted")
	}
	if _, err := browser.CSRFCookie(pair.CSRF.RevealForBootstrap(), time.Time{}); err == nil {
		t.Fatal("zero csrf cookie expiry accepted")
	}
	clear := browser.ClearSessionCookie()
	if clear.Name != cookie.Name || clear.Path != "/" || clear.Domain != "" || !clear.Secure || !clear.HttpOnly || clear.SameSite != http.SameSiteLaxMode || clear.MaxAge != -1 || !clear.Expires.Before(time.Now()) {
		t.Fatal("session clearing cookie drifted", clear)
	}
	clearCSRF := browser.ClearCSRFCookie()
	if clearCSRF.Name != csrf.Name || clearCSRF.Path != "/" || clearCSRF.Domain != "" || !clearCSRF.Secure || clearCSRF.HttpOnly || clearCSRF.SameSite != http.SameSiteLaxMode || clearCSRF.MaxAge != -1 || !clearCSRF.Expires.Before(time.Now()) {
		t.Fatal("csrf clearing cookie drifted", clearCSRF)
	}
}

func TestCSRFSafeMethodClassificationIsFailClosed(t *testing.T) {
	for _, method := range []string{"GET", "get", "HEAD", "OPTIONS"} {
		if browser.RequiresCSRF(method) {
			t.Fatal("safe method required CSRF", method)
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "TRACE", "CONNECT", "", "CUSTOM"} {
		if !browser.RequiresCSRF(method) {
			t.Fatal("unsafe/unknown method bypassed CSRF", method)
		}
	}
	clear := browser.ClearSessionCookie()
	if strings.Contains(clear.String(), "Domain=") {
		t.Fatal("host-only clearing cookie gained Domain")
	}
}
