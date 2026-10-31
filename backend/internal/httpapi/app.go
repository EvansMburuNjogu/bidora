package httpapi

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"bidora/backend/internal/health"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

type meta struct {
	TraceID string `json:"trace_id"`
}
type statusData struct {
	Status string `json:"status"`
}
type success struct {
	Data statusData `json:"data"`
	Meta meta       `json:"meta"`
}
type fieldError struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type errorData struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	FieldErrors []fieldError `json:"field_errors"`
	TraceID     string       `json:"trace_id"`
	Retryable   bool         `json:"retryable"`
}
type failure struct {
	Error errorData `json:"error"`
}

func requestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("secure random unavailable")
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func fail(c fiber.Ctx, status int, code, message string, retryable bool) error {
	return c.Status(status).JSON(failure{Error: errorData{Code: code, Message: message,
		FieldErrors: []fieldError{}, TraceID: c.GetRespHeader("X-Request-ID"), Retryable: retryable}})
}

func newBase(bodyLimit int) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: bodyLimit, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 10 * time.Second,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			// Never serialize a framework error, panic, URL or underlying provider error.
			if c.GetRespHeader("X-Request-ID") == "" {
				c.Set("X-Request-ID", requestID())
			}
			c.Set("Cache-Control", "no-store")
			c.Set("X-Content-Type-Options", "nosniff")
			if errors.Is(err, fiber.ErrRequestEntityTooLarge) {
				return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
			}
			return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		},
	})
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Request-ID", requestID())
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		return c.Next()
	})
	app.Use(recover.New())
	return app
}

// InstallHealthFallback finalizes an app after every operation route has been
// registered. Fiber evaluates routes in registration order, so callers that
// compose routes across modules must install this fallback last.
func InstallHealthFallback(app *fiber.App, state *health.State) error {
	if app == nil || state == nil {
		return errors.New("health_fallback_invalid")
	}
	installHealthFallback(app, state)
	return nil
}

func installHealthFallback(app *fiber.App, state *health.State) {
	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		if path != "/health/live" && path != "/health/ready" {
			return fail(c, 404, "NOT_FOUND", "The resource was not found.", false)
		}
		if c.Method() != fiber.MethodGet {
			c.Set("Allow", "GET")
			return fail(c, 405, "METHOD_NOT_ALLOWED", "Only GET is supported.", false)
		}
		if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
			return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
		}
		status := "live"
		if path == "/health/ready" {
			if !state.Ready() {
				return fail(c, 503, "SERVICE_NOT_READY", "The service is not ready to accept requests.", true)
			}
			status = "ready"
		}
		return c.JSON(success{Data: statusData{Status: status}, Meta: meta{TraceID: c.GetRespHeader("X-Request-ID")}})
	})
}

func New(state *health.State) *fiber.App {
	app := newBase(4096)
	installHealthFallback(app, state)
	return app
}
