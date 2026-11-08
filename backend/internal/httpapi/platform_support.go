package httpapi

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"

	app "bidora/backend/internal/controlplane/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformSupportUseCase interface {
	List(context.Context, app.PlatformAuditSubject, int32, *app.PlatformAuditCursor) (app.SupportPage, error)
	Detail(context.Context, app.PlatformAuditSubject, app.ID) (app.SupportDetail, error)
	Command(context.Context, app.SupportCommand) (app.SupportReceipt, error)
}
type PlatformSupportHandler struct{ service PlatformSupportUseCase }

func NewPlatformSupportHandler(service PlatformSupportUseCase) (*PlatformSupportHandler, error) {
	if service == nil {
		return nil, errors.New("platform_support_handler_invalid")
	}
	return &PlatformSupportHandler{service}, nil
}
func RegisterPlatformSupportRoutes(app *fiber.App, browser *BrowserSessionMiddleware, h *PlatformSupportHandler) error {
	if app == nil || browser == nil || h == nil {
		return errors.New("platform_support_routes_invalid")
	}
	app.Get("/v1/platform/support/cases", browser.Platform, h.list)
	app.Post("/v1/platform/support/cases", browser.Platform, h.command("CREATE"))
	app.Get("/v1/platform/support/cases/:case_id", browser.Platform, h.detail)
	for _, operation := range []struct{ path, action string }{{"assign", "ASSIGN"}, {"reply", "REPLY"}, {"resolve", "RESOLVE"}, {"reopen", "REOPEN"}} {
		app.Post("/v1/platform/support/cases/:case_id/"+operation.path, browser.Platform, h.command(operation.action))
	}
	return nil
}
func supportSubject(c fiber.Ctx) (app.PlatformAuditSubject, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return app.PlatformAuditSubject{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return app.PlatformAuditSubject{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 503, "SUPPORT_UNAVAILABLE", "Support is unavailable.", true)
		return app.PlatformAuditSubject{}, false
	}
	return app.PlatformAuditSubject{UserID: app.ID(principal.UserID), SessionID: app.ID(principal.SessionID), CorrelationID: app.ID(correlation), ContextGeneration: generation}, true
}
func supportFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, app.ErrSupportInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(err, app.ErrSupportForbidden):
		return fail(c, 403, "PLATFORM_FORBIDDEN", "This operation is not permitted.", false)
	case errors.Is(err, app.ErrSupportNotFound):
		return fail(c, 404, "SUPPORT_NOT_FOUND", "The support case was not found.", false)
	case errors.Is(err, app.ErrSupportConflict):
		return fail(c, 409, "SUPPORT_CONFLICT", "The support case has changed.", false)
	default:
		return fail(c, 503, "SUPPORT_UNAVAILABLE", "Support is unavailable.", true)
	}
}
func supportID(raw string) (app.ID, bool) {
	id, e := uuid.Parse(raw)
	return app.ID(id), e == nil && id != uuid.Nil && id.String() == raw
}
func supportCaseView(row app.SupportCase) fiber.Map {
	var assigned *string
	if !row.AssignedTo.IsZero() {
		v := uuid.UUID(row.AssignedTo).String()
		assigned = &v
	}
	return fiber.Map{"id": uuid.UUID(row.ID).String(), "tenant_id": uuid.UUID(row.TenantID).String(), "title": row.Title, "description": row.Description, "state": row.State, "assigned_to": assigned, "created_by": uuid.UUID(row.CreatedBy).String(), "version": row.Version, "created_at": row.CreatedAt.UTC().Format(time.RFC3339Nano), "updated_at": row.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
func (h *PlatformSupportHandler) list(c fiber.Ctx) error {
	if len(c.Body()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := supportSubject(c)
	if !ok {
		return nil
	}
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil || len(values) > 2 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	for k, v := range values {
		if (k != "limit" && k != "cursor") || len(v) != 1 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	limit := int32(20)
	if raw := values.Get("limit"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v < 1 || v > 50 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		limit = int32(v)
	}
	cursor, ok := decodePlatformAuditCursor(values.Get("cursor"))
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	page, err := h.service.List(c.Context(), subject, limit, cursor)
	if err != nil {
		return supportFailure(c, err)
	}
	rows := make([]any, len(page.Cases))
	for i, row := range page.Cases {
		rows[i] = supportCaseView(row)
	}
	next, err := encodePlatformAuditCursor(page.Next)
	if err != nil {
		return supportFailure(c, err)
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(200).JSON(fiber.Map{"data": rows, "meta": fiber.Map{"next_cursor": next, "trace_id": c.GetRespHeader("X-Request-ID")}})
}
func (h *PlatformSupportHandler) detail(c fiber.Ctx) error {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := supportSubject(c)
	if !ok {
		return nil
	}
	id, ok := supportID(c.Params("case_id"))
	if !ok {
		return fail(c, 404, "SUPPORT_NOT_FOUND", "The support case was not found.", false)
	}
	detail, err := h.service.Detail(c.Context(), subject, id)
	if err != nil {
		return supportFailure(c, err)
	}
	messages := make([]any, len(detail.Messages))
	for i, row := range detail.Messages {
		messages[i] = fiber.Map{"id": uuid.UUID(row.ID).String(), "actor_id": uuid.UUID(row.ActorID).String(), "body": row.Body, "created_at": row.CreatedAt.UTC().Format(time.RFC3339Nano)}
	}
	c.Set("Cache-Control", "private, no-store")
	return c.Status(200).JSON(fiber.Map{"data": fiber.Map{"case": supportCaseView(detail.Case), "messages": messages}, "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID")}})
}

type supportCommandBody struct {
	TenantID        string `json:"tenant_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	AssigneeID      string `json:"assignee_id"`
	Message         string `json:"message"`
	ExpectedVersion int64  `json:"expected_version"`
}

func (h *PlatformSupportHandler) command(operation string) func(fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		if len(c.Request().URI().QueryString()) != 0 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		if len(c.Body()) > 4096 {
			return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
		}
		var body supportCommandBody
		if err := bindOrganization(c, &body); err != nil {
			return err
		}
		var fields []string
		switch operation {
		case "CREATE":
			fields = []string{"tenant_id", "title", "description"}
		case "ASSIGN":
			fields = []string{"assignee_id", "expected_version"}
		case "REPLY":
			fields = []string{"message", "expected_version"}
		default:
			fields = []string{"expected_version"}
		}
		if !requireOrganizationFields(c.Body(), fields...) {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		subject, ok := supportSubject(c)
		if !ok {
			return nil
		}
		key, ok := singleHeader(c, IdempotencyKeyHeader)
		if !ok {
			return invalidHeader(c, IdempotencyKeyHeader, "required")
		}
		if id, e := uuid.Parse(key); e != nil || id == uuid.Nil || id.String() != key {
			return invalidHeader(c, IdempotencyKeyHeader, "uuid")
		}
		in := app.SupportCommand{Subject: subject, Operation: operation, Title: body.Title, Description: body.Description, Message: body.Message, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: key}
		if operation == "CREATE" {
			var parsed bool
			in.TenantID, parsed = supportID(body.TenantID)
			if !parsed {
				return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			}
		} else {
			var parsed bool
			in.CaseID, parsed = supportID(c.Params("case_id"))
			if !parsed {
				return fail(c, 404, "SUPPORT_NOT_FOUND", "The support case was not found.", false)
			}
		}
		if operation == "ASSIGN" {
			var parsed bool
			in.Assignee, parsed = supportID(body.AssigneeID)
			if !parsed {
				return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			}
		}
		result, err := h.service.Command(c.Context(), in)
		if err != nil {
			return supportFailure(c, err)
		}
		status := 200
		if operation == "CREATE" && !result.Replay {
			status = 201
		}
		c.Set("Cache-Control", "private, no-store")
		return c.Status(status).JSON(fiber.Map{"data": fiber.Map{"case_id": uuid.UUID(result.CaseID).String(), "version": result.Version, "replay": result.Replay}, "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID")}})
	}
}
