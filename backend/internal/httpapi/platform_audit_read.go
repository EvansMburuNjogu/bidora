package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	controlapp "bidora/backend/internal/controlplane/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformAuditReader interface {
	List(context.Context, controlapp.PlatformAuditSubject, controlapp.PlatformAuditFilter, int32, *controlapp.PlatformAuditCursor) (controlapp.PlatformAuditPage, error)
	Detail(context.Context, controlapp.PlatformAuditSubject, controlapp.ID) (controlapp.PlatformAuditAction, error)
	Export(context.Context, controlapp.PlatformAuditSubject, controlapp.PlatformAuditFilter) (controlapp.PlatformAuditPage, error)
}
type PlatformAuditHandler struct{ reader PlatformAuditReader }

func NewPlatformAuditHandler(reader PlatformAuditReader) (*PlatformAuditHandler, error) {
	if reader == nil {
		return nil, errors.New("platform_audit_handler_invalid")
	}
	return &PlatformAuditHandler{reader}, nil
}
func RegisterPlatformAuditRoutes(app *fiber.App, browser *BrowserSessionMiddleware, handler *PlatformAuditHandler) error {
	if app == nil || browser == nil || handler == nil {
		return errors.New("platform_audit_routes_invalid")
	}
	app.Get("/v1/platform/audit/actions/export", browser.Platform, handler.export)
	app.Get("/v1/platform/audit/actions", browser.Platform, handler.list)
	app.Get("/v1/platform/audit/actions/:audit_id", browser.Platform, handler.detail)
	return nil
}
func platformAuditSubject(c fiber.Ctx) (controlapp.PlatformAuditSubject, bool) {
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return controlapp.PlatformAuditSubject{}, false
	}
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return controlapp.PlatformAuditSubject{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return controlapp.PlatformAuditSubject{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 503, "AUDIT_UNAVAILABLE", "The audit view is unavailable.", true)
		return controlapp.PlatformAuditSubject{}, false
	}
	return controlapp.PlatformAuditSubject{UserID: controlapp.ID(principal.UserID), SessionID: controlapp.ID(principal.SessionID), CorrelationID: controlapp.ID(correlation), ContextGeneration: generation}, true
}
func platformAuditFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, controlapp.ErrPlatformAuditInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(err, controlapp.ErrPlatformAuditForbidden):
		return fail(c, 403, "PLATFORM_FORBIDDEN", "This operation is not permitted.", false)
	case errors.Is(err, controlapp.ErrPlatformAuditNotFound):
		return fail(c, 404, "AUDIT_NOT_FOUND", "The activity was not found.", false)
	default:
		return fail(c, 503, "AUDIT_UNAVAILABLE", "The audit view is unavailable.", true)
	}
}
func (h *PlatformAuditHandler) list(c fiber.Ctx) error {
	subject, ok := platformAuditSubject(c)
	if !ok {
		return nil
	}
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil || len(values) > 10 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	allowed := map[string]bool{"limit": true, "cursor": true, "search": true, "administrator": true, "action": true, "module": true, "result": true, "from": true, "to": true, "sort": true}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	limit := int32(10)
	if raw := values.Get("limit"); raw != "" {
		parsed, e := strconv.Atoi(raw)
		if e != nil || parsed < 1 || parsed > 100 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		limit = int32(parsed)
	}
	cursor, ok := decodePlatformAuditCursor(values.Get("cursor"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	filter := controlapp.PlatformAuditFilter{Search: values.Get("search"), Action: values.Get("action"), Module: values.Get("module"), Result: values.Get("result"), Sort: values.Get("sort")}
	if filter.Sort == "" {
		filter.Sort = "DESC"
	}
	if raw := values.Get("administrator"); raw != "" {
		id, e := uuid.Parse(raw)
		if e != nil || id == uuid.Nil || id.String() != raw {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		filter.Administrator = controlapp.ID(id)
	}
	if raw := values.Get("from"); raw != "" {
		filter.From, err = time.Parse("2006-01-02", raw)
		if err != nil || filter.From.Format("2006-01-02") != raw {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	if raw := values.Get("to"); raw != "" {
		var date time.Time
		date, err = time.Parse("2006-01-02", raw)
		if err != nil || date.Format("2006-01-02") != raw {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		filter.Before = date.AddDate(0, 0, 1)
	}
	if !controlapp.ValidPlatformAuditFilter(filter) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	page, err := h.reader.List(c.Context(), subject, filter, limit, cursor)
	if err != nil {
		return platformAuditFailure(c, err)
	}
	items := make([]any, len(page.Actions))
	for i, row := range page.Actions {
		items[i] = platformAuditView(row)
	}
	next, err := encodePlatformAuditCursor(page.Next)
	if err != nil {
		return platformAuditFailure(c, controlapp.ErrPlatformAuditUnavailable)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(200).JSON(fiber.Map{"data": items, "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID"), "next_cursor": next}})
}
func (h *PlatformAuditHandler) export(c fiber.Ctx) error {
	subject, ok := platformAuditSubject(c)
	if !ok {
		return nil
	}
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil || len(values) != 2 || len(values["from"]) != 1 || len(values["to"]) != 1 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	from, fromErr := time.Parse("2006-01-02", values.Get("from"))
	to, toErr := time.Parse("2006-01-02", values.Get("to"))
	if fromErr != nil || toErr != nil || from.Format("2006-01-02") != values.Get("from") || to.Format("2006-01-02") != values.Get("to") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	page, err := h.reader.Export(c.Context(), subject, controlapp.PlatformAuditFilter{Sort: "DESC", From: from, Before: to.AddDate(0, 0, 1)})
	if err != nil {
		return platformAuditFailure(c, err)
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err = writer.Write([]string{"id", "occurred_at_utc", "action", "module", "result", "administrator_id", "affected_record_id"}); err != nil {
		return platformAuditFailure(c, err)
	}
	for _, row := range page.Actions {
		actor, affected := "", ""
		if !row.Administrator.IsZero() {
			actor = uuid.UUID(row.Administrator).String()
		}
		if !row.AffectedRecord.IsZero() {
			affected = uuid.UUID(row.AffectedRecord).String()
		}
		if err = writer.Write([]string{uuid.UUID(row.ID).String(), row.OccurredAt.UTC().Format(time.RFC3339Nano), row.Action, row.Module, row.Result, actor, affected}); err != nil {
			return platformAuditFailure(c, err)
		}
	}
	writer.Flush()
	if writer.Error() != nil {
		return platformAuditFailure(c, writer.Error())
	}
	c.Set("Cache-Control", "private, no-store")
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="platform-audit-`+values.Get("from")+`-`+values.Get("to")+`.csv"`)
	return c.Status(200).Send(buf.Bytes())
}
func (h *PlatformAuditHandler) detail(c fiber.Ctx) error {
	subject, ok := platformAuditSubject(c)
	if !ok {
		return nil
	}
	if len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	id, err := uuid.Parse(c.Params("audit_id"))
	if err != nil || id == uuid.Nil || id.String() != c.Params("audit_id") {
		return fail(c, 404, "AUDIT_NOT_FOUND", "The activity was not found.", false)
	}
	row, err := h.reader.Detail(c.Context(), subject, controlapp.ID(id))
	if err != nil {
		return platformAuditFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(200).JSON(fiber.Map{"data": platformAuditView(row), "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID")}})
}
func platformAuditView(row controlapp.PlatformAuditAction) fiber.Map {
	var administrator, affected *string
	if !row.Administrator.IsZero() {
		v := uuid.UUID(row.Administrator).String()
		administrator = &v
	}
	if !row.AffectedRecord.IsZero() {
		v := uuid.UUID(row.AffectedRecord).String()
		affected = &v
	}
	changes := row.Changes
	if changes == nil {
		changes = []controlapp.PlatformAuditChange{}
	}
	return fiber.Map{"administrator_name": optionalAuditText(row.AdministratorName), "affected_record_name": optionalAuditText(row.AffectedRecordName), "ip_address": optionalAuditText(row.IPAddress), "browser": optionalAuditText(row.Browser), "changes": changes, "id": uuid.UUID(row.ID).String(), "administrator_id": administrator, "action": row.Action, "module": row.Module, "affected_record_id": affected, "result": row.Result, "occurred_at": row.OccurredAt.UTC().Format(time.RFC3339Nano)}
}

func optionalAuditText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

type platformAuditCursorWire struct {
	At string `json:"at"`
	ID string `json:"id"`
}

func decodePlatformAuditCursor(raw string) (*controlapp.PlatformAuditCursor, bool) {
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
	var wire platformAuditCursorWire
	if json.Unmarshal(decoded, &wire) != nil {
		return nil, false
	}
	at, err := time.Parse(time.RFC3339Nano, wire.At)
	id, idErr := uuid.Parse(wire.ID)
	if err != nil || idErr != nil || id == uuid.Nil || at.IsZero() {
		return nil, false
	}
	return &controlapp.PlatformAuditCursor{OccurredAt: at.UTC(), ID: controlapp.ID(id)}, true
}
func encodePlatformAuditCursor(cursor *controlapp.PlatformAuditCursor) (*string, error) {
	if cursor == nil {
		return nil, nil
	}
	if cursor.ID.IsZero() || cursor.OccurredAt.IsZero() {
		return nil, errors.New("platform_audit_cursor_invalid")
	}
	encoded, err := json.Marshal(platformAuditCursorWire{At: cursor.OccurredAt.UTC().Format(time.RFC3339Nano), ID: uuid.UUID(cursor.ID).String()})
	if err != nil {
		return nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(encoded)
	return &value, nil
}
