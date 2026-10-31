package httpapi

import (
	"context"
	"errors"
	"mime"
	"net/netip"
	"strings"
	"time"

	identityapp "bidora/backend/internal/identity/application"
	"bidora/backend/internal/identity/browser"
	"github.com/gofiber/fiber/v3"
)

const GoogleLinkStartPath = "/v1/auth/google/link/start"
const GoogleLinkCallbackPath = "/v1/auth/google/link/callback"

type GoogleLinkStartUseCase interface {
	Start(context.Context, identityapp.GoogleLinkReauthenticationInput) (identityapp.GoogleAuthorizationStart, error)
}
type GoogleLinkCallbackUseCase interface {
	Callback(context.Context, identityapp.GoogleLinkCallbackInput) (identityapp.LoginResult, error)
}
type GoogleLinkStartHandler struct {
	start GoogleLinkStartUseCase
	ips   LoginClientIPResolver
	keys  LoginNetworkKeyer
	ttl   time.Duration
}
type GoogleLinkCallbackHandler struct {
	callback                         GoogleLinkCallbackUseCase
	successRedirect, failureRedirect string
}

func NewGoogleLinkHTTPHandlers(start GoogleLinkStartUseCase, callback GoogleLinkCallbackUseCase, ips LoginClientIPResolver, keys LoginNetworkKeyer, config GoogleHTTPConfig) (*GoogleLinkStartHandler, *GoogleLinkCallbackHandler, error) {
	if start == nil || callback == nil || ips == nil || keys == nil || !validFixedRedirect(config.SuccessRedirect) || !validFixedRedirect(config.FailureRedirect) || config.SuccessRedirect == config.FailureRedirect || config.TransactionTTL < time.Minute || config.TransactionTTL > 15*time.Minute {
		return nil, nil, errors.New("google_link_http_handler_invalid")
	}
	return &GoogleLinkStartHandler{start: start, ips: ips, keys: keys, ttl: config.TransactionTTL}, &GoogleLinkCallbackHandler{callback: callback, successRedirect: config.SuccessRedirect, failureRedirect: config.FailureRedirect}, nil
}
func (h *GoogleLinkStartHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("google_link_start_http_context_required")
	}
	googleHeaders(c)
	if h == nil || h.start == nil || h.ips == nil || h.keys == nil {
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	contentType, ok := singleHeader(c, "Content-Type")
	if !ok {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" || len(params) > 1 || (len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
		return fail(c, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json.", false)
	}
	emailValue, passwordValue, err := decodeLogin(c.Body())
	if err != nil {
		return fail(c, errorStatus(err.Error()), err.Error(), "The request is invalid.", false)
	}
	passwordBytes := []byte(passwordValue)
	defer func() {
		for index := range passwordBytes {
			passwordBytes[index] = 0
		}
	}()
	remote := c.RemoteAddr()
	if remote == nil {
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
	peer, err := netip.ParseAddrPort(remote.String())
	if err != nil {
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
	clientIP, err := h.ips.Resolve(peer, c.Get("X-Forwarded-For"))
	if err != nil {
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
	networkScope, err := h.keys.Network(clientIP)
	if err != nil || networkScope.IsZero() {
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
	result, err := h.start.Start(c.Context(), identityapp.GoogleLinkReauthenticationInput{Principal: principal, Credentials: identityapp.AuthenticateInput{Email: emailValue, Password: passwordBytes, NetworkScope: networkScope}})
	if err != nil {
		return googleLinkStartFailure(c, err)
	}
	target := result.URLForRedirect()
	if !validGoogleAuthorizationRedirect(target) {
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
	setGoogleCookie(c, googleCorrelationCookie(result.BrowserBindingForCookie(), time.Now().UTC().Add(h.ttl), int(h.ttl/time.Second)))
	c.Location(target)
	return c.Status(fiber.StatusSeeOther).Send(nil)
}
func (h *GoogleLinkCallbackHandler) Handle(c fiber.Ctx) error {
	if c == nil {
		return errors.New("google_link_callback_http_context_required")
	}
	googleHeaders(c)
	if h == nil || h.callback == nil || !validFixedRedirect(h.successRedirect) || !validFixedRedirect(h.failureRedirect) {
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
	defer setGoogleCookie(c, clearGoogleCorrelationCookie())
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return h.redirectFailure(c)
	}
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
	result, err := h.callback.Callback(c.Context(), identityapp.GoogleLinkCallbackInput{Principal: principal, State: state, BrowserBinding: binding, Code: code, CorrelationID: correlationID})
	if err != nil {
		return h.redirectFailure(c)
	}
	sessionCookie, err := browser.SessionCookie(result.Session.RevealForCookie(), result.AbsoluteExpiresAt)
	if err != nil {
		return h.redirectFailure(c)
	}
	csrfCookie, err := browser.CSRFCookie(result.CSRF.RevealForBootstrap(), result.AbsoluteExpiresAt)
	if err != nil {
		return h.redirectFailure(c)
	}
	setLoginCookie(c, sessionCookie)
	setLoginCookie(c, csrfCookie)
	c.Location(h.successRedirect)
	return c.Status(fiber.StatusSeeOther).Send(nil)
}
func (h *GoogleLinkCallbackHandler) redirectFailure(c fiber.Ctx) error {
	c.Location(h.failureRedirect)
	return c.Status(fiber.StatusSeeOther).Send(nil)
}
func googleLinkStartFailure(c fiber.Ctx, err error) error {
	switch err.Error() {
	case "attempt_denied":
		c.Set("Retry-After", "900")
		return fail(c, 429, "ATTEMPT_DENIED", "Authentication is temporarily unavailable.", true)
	case "google_link_reauthentication_invalid", "google_link_invalid":
		return fail(c, 401, "CREDENTIALS_INVALID", "The credentials are invalid.", false)
	default:
		return fail(c, 503, "GOOGLE_LINK_UNAVAILABLE", "Google linking is temporarily unavailable.", true)
	}
}
