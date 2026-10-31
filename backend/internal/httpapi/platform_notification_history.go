package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	notificationapp "bidora/backend/internal/notification/application"
	"bidora/backend/internal/notification/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformNotificationHistoryReader interface {
	List(context.Context, notificationapp.HistorySubject, int32, *notificationapp.HistoryCursor) (notificationapp.HistoryPage, error)
}
type PlatformNotificationHistoryHandler struct {
	reader PlatformNotificationHistoryReader
}

func NewPlatformNotificationHistoryHandler(reader PlatformNotificationHistoryReader) (*PlatformNotificationHistoryHandler, error) {
	if reader == nil {
		return nil, errors.New("platform_notification_history_handler_invalid")
	}
	return &PlatformNotificationHistoryHandler{reader}, nil
}
func RegisterPlatformNotificationHistoryRoutes(app *fiber.App, browser *BrowserSessionMiddleware, handler *PlatformNotificationHistoryHandler) error {
	if app == nil || browser == nil || handler == nil {
		return errors.New("platform_notification_history_routes_invalid")
	}
	app.Get("/v1/platform/notifications/attempts", browser.Platform, handler.list)
	return nil
}
func (h *PlatformNotificationHistoryHandler) list(c fiber.Ctx) error {
	if len(c.Body()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return nil
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 503, "NOTIFICATION_HISTORY_UNAVAILABLE", "Notification history is unavailable.", true)
	}
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil || len(values) > 2 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	for key, entries := range values {
		if (key != "limit" && key != "cursor") || len(entries) != 1 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	limit := int32(25)
	if raw := values.Get("limit"); raw != "" {
		parsed, e := strconv.Atoi(raw)
		if e != nil || parsed < 1 || parsed > 100 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		limit = int32(parsed)
	}
	cursor, ok := decodeNotificationHistoryCursor(values.Get("cursor"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	page, err := h.reader.List(c.Context(), notificationapp.HistorySubject{UserID: domain.ID(principal.UserID), SessionID: domain.ID(principal.SessionID), CorrelationID: domain.ID(correlation), ContextGeneration: generation}, limit, cursor)
	if err != nil {
		switch {
		case errors.Is(err, notificationapp.ErrHistoryInvalid):
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		case errors.Is(err, notificationapp.ErrHistoryForbidden):
			return fail(c, 403, "PLATFORM_FORBIDDEN", "This operation is not permitted.", false)
		default:
			return fail(c, 503, "NOTIFICATION_HISTORY_UNAVAILABLE", "Notification history is unavailable.", true)
		}
	}
	items := make([]any, len(page.Rows))
	for i, row := range page.Rows {
		var finished *string
		if row.FinishedAt != nil {
			value := row.FinishedAt.UTC().Format(time.RFC3339Nano)
			finished = &value
		}
		items[i] = fiber.Map{"category": row.Category, "attempt_number": row.AttemptNumber, "disposition": row.Disposition, "failure_code": row.FailureCode, "started_at": row.StartedAt.UTC().Format(time.RFC3339Nano), "finished_at": finished}
	}
	next, err := encodeNotificationHistoryCursor(page.Next)
	if err != nil {
		return fail(c, 503, "NOTIFICATION_HISTORY_UNAVAILABLE", "Notification history is unavailable.", true)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(200).JSON(fiber.Map{"data": items, "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID"), "next_cursor": next}})
}

type notificationCursorWire struct {
	At string `json:"at"`
	ID string `json:"id"`
}

func decodeNotificationHistoryCursor(raw string) (*notificationapp.HistoryCursor, bool) {
	if raw == "" {
		return nil, true
	}
	if len(raw) > 384 {
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) > 256 {
		return nil, false
	}
	var wire notificationCursorWire
	if json.Unmarshal(decoded, &wire) != nil {
		return nil, false
	}
	at, err := time.Parse(time.RFC3339Nano, wire.At)
	id, errID := uuid.Parse(wire.ID)
	if err != nil || errID != nil || id == uuid.Nil || at.IsZero() {
		return nil, false
	}
	return &notificationapp.HistoryCursor{StartedAt: at.UTC(), ID: domain.ID(id)}, true
}
func encodeNotificationHistoryCursor(cursor *notificationapp.HistoryCursor) (*string, error) {
	if cursor == nil {
		return nil, nil
	}
	if cursor.ID.IsZero() || cursor.StartedAt.IsZero() {
		return nil, errors.New("platform_notification_cursor_invalid")
	}
	encoded, err := json.Marshal(notificationCursorWire{At: cursor.StartedAt.UTC().Format(time.RFC3339Nano), ID: uuid.UUID(cursor.ID).String()})
	if err != nil {
		return nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(encoded)
	return &value, nil
}
