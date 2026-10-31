// Package browser implements approved cookie, Origin and CSRF primitives. It
// does not trust proxy headers, authenticate sessions or register HTTP routes.
package browser

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bidora/backend/internal/identity/session"
)

type Protection struct {
	origins           map[string]struct{}
	allowLoopbackHTTP bool
}

func NewProtection(allowedOrigins []string) (*Protection, error) {
	return newProtection(allowedOrigins, false)
}

// NewLoopbackDevelopmentProtection permits explicit HTTP origins only on IP
// loopback. Deployment composition must continue to use NewProtection.
func NewLoopbackDevelopmentProtection(allowedOrigins []string) (*Protection, error) {
	return newProtection(allowedOrigins, true)
}

func newProtection(allowedOrigins []string, allowLoopbackHTTP bool) (*Protection, error) {
	if len(allowedOrigins) == 0 {
		return nil, errors.New("browser_origins_required")
	}
	owned := make(map[string]struct{}, len(allowedOrigins))
	for _, value := range allowedOrigins {
		origin, err := canonicalOrigin(value, allowLoopbackHTTP)
		if err != nil {
			return nil, errors.New("browser_origin_invalid")
		}
		if _, exists := owned[origin]; exists {
			return nil, errors.New("browser_origin_duplicate")
		}
		owned[origin] = struct{}{}
	}
	return &Protection{origins: owned, allowLoopbackHTTP: allowLoopbackHTTP}, nil
}
func (p *Protection) ValidateOrigin(origin string) error {
	if p == nil || len(p.origins) == 0 {
		return errors.New("browser_request_forbidden")
	}
	canonical, err := canonicalOrigin(origin, p.allowLoopbackHTTP)
	if err != nil {
		return errors.New("browser_request_forbidden")
	}
	if _, ok := p.origins[canonical]; !ok {
		return errors.New("browser_request_forbidden")
	}
	return nil
}
func (p *Protection) ValidateUnsafe(origin, presentedHeader, presentedCookie string, expectedCSRFHash [32]byte) error {
	if err := p.ValidateBootstrap(origin, presentedCookie, expectedCSRFHash); err != nil {
		return err
	}
	headerMatched, headerErr := session.MatchesCSRF(presentedHeader, expectedCSRFHash)
	if headerErr != nil || !headerMatched {
		return errors.New("browser_request_forbidden")
	}
	return nil
}
func (p *Protection) ValidateBootstrap(origin, presentedCookie string, expectedCSRFHash [32]byte) error {
	if err := p.ValidateOrigin(origin); err != nil {
		return err
	}
	cookieMatched, cookieErr := session.MatchesCSRF(presentedCookie, expectedCSRFHash)
	if cookieErr != nil || !cookieMatched {
		return errors.New("browser_request_forbidden")
	}
	return nil
}
func RequiresCSRF(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}
func SessionCookie(raw string, absoluteExpiry time.Time) (http.Cookie, error) {
	if _, err := session.HashPresentedSession(raw); err != nil || absoluteExpiry.IsZero() {
		return http.Cookie{}, errors.New("session_cookie_invalid")
	}
	policy := session.PilotCookiePolicy()
	return http.Cookie{Name: policy.Name, Value: raw, Path: policy.Path, Expires: absoluteExpiry.UTC(), HttpOnly: policy.HttpOnly, Secure: policy.Secure, SameSite: http.SameSiteLaxMode}, nil
}
func CSRFCookie(raw string, absoluteExpiry time.Time) (http.Cookie, error) {
	if _, err := session.HashPresentedCSRF(raw); err != nil || absoluteExpiry.IsZero() {
		return http.Cookie{}, errors.New("csrf_cookie_invalid")
	}
	policy := session.PilotCSRFCookiePolicy()
	return http.Cookie{Name: policy.Name, Value: raw, Path: policy.Path, Expires: absoluteExpiry.UTC(), HttpOnly: policy.HttpOnly, Secure: policy.Secure, SameSite: http.SameSiteLaxMode}, nil
}
func ClearSessionCookie() http.Cookie { return clearCookie(session.PilotCookiePolicy()) }
func ClearCSRFCookie() http.Cookie    { return clearCookie(session.PilotCSRFCookiePolicy()) }
func clearCookie(policy session.CookiePolicy) http.Cookie {
	return http.Cookie{Name: policy.Name, Path: policy.Path, Expires: time.Unix(1, 0).UTC(), MaxAge: -1, HttpOnly: policy.HttpOnly, Secure: policy.Secure, SameSite: http.SameSiteLaxMode}
}
func validHost(value string) bool {
	if address, err := netip.ParseAddr(value); err == nil {
		return address.IsValid()
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || !hostAlphaNumeric(label[0]) || !hostAlphaNumeric(label[len(label)-1]) {
			return false
		}
		for i := 0; i < len(label); i++ {
			if !hostAlphaNumeric(label[i]) && label[i] != '-' {
				return false
			}
		}
	}
	return len(value) <= 253
}
func hostAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func canonicalOrigin(value string, allowLoopbackHTTP bool) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("origin_invalid")
	}
	hostname := strings.ToLower(parsed.Hostname())
	loopbackHTTP := false
	if parsed.Scheme == "http" && allowLoopbackHTTP {
		address, parseErr := netip.ParseAddr(hostname)
		loopbackHTTP = parseErr == nil && address.Unmap().IsLoopback()
	}
	if parsed.Scheme != "https" && !loopbackHTTP {
		return "", errors.New("origin_invalid")
	}
	if !validHost(hostname) || strings.Contains(hostname, "%") {
		return "", errors.New("origin_invalid")
	}
	port := parsed.Port()
	if port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", errors.New("origin_invalid")
		}
		if parsed.Scheme == "https" && number == 443 || parsed.Scheme == "http" && number == 80 {
			port = ""
		}
	}
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	}
	return parsed.Scheme + "://" + host, nil
}
