package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

const GoogleStartPath = "/v1/auth/google/start"
const GoogleCallbackPath = "/v1/auth/google/callback"
const GoogleCorrelationCookie = "__Host-bidora_google_oidc"

type GoogleStartUseCase interface {
	Start(context.Context) (identityapp.GoogleAuthorizationStart, error)
}
type GoogleLoginUseCase interface {
	Login(context.Context, identityapp.GoogleCallbackInput) (identityapp.GoogleLoginResult, error)
}
type GoogleHTTPConfig struct {
	SuccessRedirect, FailureRedirect string
	TransactionTTL                   time.Duration
}

func NewGoogleHTTPHandlers(start GoogleStartUseCase, login GoogleLoginUseCase, protection *browser.Protection, config GoogleHTTPConfig) (*GoogleStartHandler, *GoogleCallbackHandler, error) {
	if start == nil || login == nil || protection == nil || !validFixedRedirect(config.SuccessRedirect) || !validFixedRedirect(config.FailureRedirect) || config.SuccessRedirect == config.FailureRedirect || config.TransactionTTL < time.Minute || config.TransactionTTL > 15*time.Minute {
		return nil, nil, errors.New("google_http_handler_invalid")
	}
	return &GoogleStartHandler{start: start, protection: protection, ttl: config.TransactionTTL}, &GoogleCallbackHandler{login: login, successRedirect: config.SuccessRedirect, failureRedirect: config.FailureRedirect}, nil
}

type GoogleStartHandler struct {
	start      GoogleStartUseCase
	protection *browser.Protection
	ttl        time.Duration
}

func (h *GoogleStartHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("google_start_http_context_required")
	}
	googleHeaders(c)
	if h == nil || h.start == nil || h.protection == nil || h.ttl < time.Minute || h.ttl > 15*time.Minute {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	if err := h.protection.ValidateOrigin(c.Get("Origin")); err != nil {
		return fail(c, 403, "BROWSER_REQUEST_FORBIDDEN", "The browser request was rejected.", false)
	}
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	result, err := h.start.Start(c.Context())
	if err != nil {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	target := result.URLForRedirect()
	if !validGoogleAuthorizationRedirect(target) {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	setGoogleCookie(c, googleCorrelationCookie(result.BrowserBindingForCookie(), time.Now().UTC().Add(h.ttl), int(h.ttl/time.Second)))
	c.Location(target)
	return c.Status(fiber.StatusSeeOther).Send(nil)
}

type GoogleCallbackHandler struct {
	login                            GoogleLoginUseCase
	successRedirect, failureRedirect string
}

func (h *GoogleCallbackHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("google_callback_http_context_required")
	}
	googleHeaders(c)
	if h == nil || h.login == nil || !validFixedRedirect(h.successRedirect) || !validFixedRedirect(h.failureRedirect) {
		return fail(c, 503, "AUTHENTICATION_UNAVAILABLE", "Authentication is temporarily unavailable.", true)
	}
	defer setGoogleCookie(c, clearGoogleCorrelationCookie())
	if len(c.Body()) != 0 {
		return h.redirectFailure(c)
	}
	state, code, ok := strictGoogleCallbackQuery(string(c.Request().URI().QueryString()))
	if !ok {
		return h.redirectFailure(c)
	}
	binding, ok := singleGoogleCookie(c.Get("Cookie"), GoogleCorrelationCookie)
	if !ok {
		return h.redirectFailure(c)
	}
	correlationID, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.login.Login(c.Context(), identityapp.GoogleCallbackInput{State: state, BrowserBinding: binding, Code: code, CorrelationID: correlationID})
	if err != nil {
		return h.redirectFailure(c)
	}
	if result.Login.MFARequired {
		c.Cookie(&fiber.Cookie{Name: "__Host-bidora_login_code", Value: result.Login.MFAToken, Path: "/", Expires: result.Login.MFAExpiresAt.Add(identityapp.EmailLoginResendGrace), Secure: true, HTTPOnly: true, SameSite: "Lax"})
		c.Location(h.successRedirect)
		return c.Status(fiber.StatusSeeOther).Send(nil)
	}
	sessionCookie, err := browser.SessionCookie(result.Login.Session.RevealForCookie(), result.Login.AbsoluteExpiresAt)
	if err != nil {
		return h.redirectFailure(c)
	}
	csrfCookie, err := browser.CSRFCookie(result.Login.CSRF.RevealForBootstrap(), result.Login.AbsoluteExpiresAt)
	if err != nil {
		return h.redirectFailure(c)
	}
	setLoginCookie(c, sessionCookie)
	setLoginCookie(c, csrfCookie)
	c.Location(h.successRedirect)
	return c.Status(fiber.StatusSeeOther).Send(nil)
}
func (h *GoogleCallbackHandler) redirectFailure(c fiber.Ctx) error {
	c.Location(h.failureRedirect)
	return c.Status(fiber.StatusSeeOther).Send(nil)
}
func googleHeaders(c fiber.Ctx) {
	if c.GetRespHeader("X-Request-ID") == "" {
		c.Set("X-Request-ID", requestID())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("Pragma", "no-cache")
	c.Set("Referrer-Policy", "no-referrer")
	c.Set("X-Content-Type-Options", "nosniff")
}
func googleCorrelationCookie(value string, expires time.Time, maxAge int) http.Cookie {
	return http.Cookie{Name: GoogleCorrelationCookie, Value: value, Path: "/", Expires: expires, MaxAge: maxAge, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}
func clearGoogleCorrelationCookie() http.Cookie {
	return http.Cookie{Name: GoogleCorrelationCookie, Path: "/", Expires: time.Unix(1, 0).UTC(), MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}
func setGoogleCookie(c fiber.Ctx, cookie http.Cookie) {
	c.Cookie(&fiber.Cookie{Name: cookie.Name, Value: cookie.Value, Path: cookie.Path, Expires: cookie.Expires, MaxAge: cookie.MaxAge, Secure: cookie.Secure, HTTPOnly: cookie.HttpOnly, SameSite: "Lax"})
}
func strictGoogleCallbackQuery(raw string) (string, string, bool) {
	values, err := url.ParseQuery(raw)
	if err != nil || len(values) != 2 || len(values["state"]) != 1 || len(values["code"]) != 1 {
		return "", "", false
	}
	state, code := values.Get("state"), values.Get("code")
	if len(state) != 43 || len(code) < 1 || len(code) > 4096 || strings.TrimSpace(state) != state || strings.TrimSpace(code) != code || !printableASCII(state) || !printableASCII(code) {
		return "", "", false
	}
	return state, code, true
}
func singleGoogleCookie(raw, name string) (string, bool) {
	request := http.Request{Header: http.Header{"Cookie": []string{raw}}}
	found := ""
	count := 0
	for _, cookie := range request.Cookies() {
		if cookie.Name == name {
			count++
			found = cookie.Value
		}
	}
	return found, count == 1 && len(found) == 43 && printableASCII(found)
}
func printableASCII(value string) bool {
	for _, character := range []byte(value) {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
func validFixedRedirect(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.String() == value
}
func validGoogleAuthorizationRedirect(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host == "accounts.google.com" && parsed.Path == "/o/oauth2/v2/auth" && parsed.User == nil && parsed.Fragment == "" && parsed.RawQuery != ""
}
