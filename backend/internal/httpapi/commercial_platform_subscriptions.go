package httpapi

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformSubscriptionUseCase interface {
	InventoryPlatformSubscriptions(context.Context, app.PlatformSubscriptionInventoryInput) (app.PlatformSubscriptionInventoryPage, error)
	ReadPlatformSubscription(context.Context, app.PlatformSubscriptionSubject, domain.ID) (app.SubscriptionSummary, error)
	ChangePlatformSubscription(context.Context, app.PlatformSubscriptionChange) (app.PlatformSubscriptionReceipt, error)
}
type PlatformSubscriptionHandler struct{ usecase PlatformSubscriptionUseCase }

func NewPlatformSubscriptionHandler(u PlatformSubscriptionUseCase) (*PlatformSubscriptionHandler, error) {
	if u == nil {
		return nil, errors.New("platform_subscription_handler_invalid")
	}
	return &PlatformSubscriptionHandler{u}, nil
}
func RegisterPlatformSubscriptionRoutes(api *fiber.App, browser *BrowserSessionMiddleware, h *PlatformSubscriptionHandler) error {
	if api == nil || browser == nil || h == nil {
		return errors.New("platform_subscription_routes_invalid")
	}
	api.Get("/v1/platform/commercial/subscriptions", browser.Platform, h.inventory)
	g := api.Group("/v1/platform/commercial/subscriptions", browser.Platform)
	g.Get("/:tenant_id", h.read)
	g.Post("/:tenant_id/cancellation", h.schedule)
	g.Post("/:tenant_id/cancellation/withdraw", h.withdraw)
	return nil
}
func subscriptionSubject(c fiber.Ctx) (app.PlatformSubscriptionSubject, bool) {
	actor, correlation, ok := platformCommercialPrincipal(c)
	return app.PlatformSubscriptionSubject{UserID: actor.UserID, SessionID: actor.SessionID, CorrelationID: correlation, ContextGeneration: actor.ContextGeneration}, ok
}
func (h *PlatformSubscriptionHandler) inventory(c fiber.Ctx) error {
	prepareCommercialResponse(c)
	if len(c.Body()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	values, e := url.ParseQuery(string(c.Request().URI().QueryString()))
	if e != nil || len(values) > 5 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	for name, list := range values {
		if len(list) != 1 || name != "limit" && name != "cursor" && name != "state" && name != "plan" && name != "cadence" {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	limit := int64(20)
	if raw, ok := values["limit"]; ok {
		value := raw[0]
		if len(value) == 0 || len(value) > 2 || value[0] < '1' || value[0] > '9' {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		for _, digit := range value {
			if digit < '0' || digit > '9' {
				return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
			}
		}
		limit, e = strconv.ParseInt(value, 10, 32)
		if e != nil || limit < 1 || limit > 50 {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	var cursor domain.ID
	if raw, ok := values["cursor"]; ok {
		var valid bool
		cursor, valid = commercialID(raw[0])
		if !valid {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
	}
	state := values.Get("state")
	if _, ok := values["state"]; ok && state == "" {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	if values.Has("plan") && values.Get("plan") == "" || values.Has("cadence") && values.Get("cadence") == "" {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := subscriptionSubject(c)
	if !ok {
		return nil
	}
	result, e := h.usecase.InventoryPlatformSubscriptions(c.Context(), app.PlatformSubscriptionInventoryInput{Subject: subject, AfterID: cursor, Limit: int32(limit), State: state, Plan: values.Get("plan"), Cadence: values.Get("cadence")})
	if e != nil {
		return platformSubscriptionFailure(c, e)
	}
	rows := make([]fiber.Map, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, fiber.Map{"tenant_name": row.TenantName, "subscription_id": uuid.UUID(row.SubscriptionID).String(), "tenant_id": uuid.UUID(row.TenantID).String(), "state": row.State, "version": row.Version, "plan_key": row.PlanKey, "cadence": row.Cadence, "has_current_term": row.HasCurrentTerm, "commercial_period_end": commercialOptionalTimestamp(row.PeriodEnd), "cancel_at": commercialOptionalTimestamp(row.CancelAt), "updated_at": commercialTimestamp(row.UpdatedAt)})
	}
	var next any
	if result.Next != nil {
		next = uuid.UUID(*result.Next).String()
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"subscriptions": rows}, "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID"), "next_cursor": next}})
}
func (h *PlatformSubscriptionHandler) read(c fiber.Ctx) error {
	prepareCommercialResponse(c)
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	tenant, ok := commercialPathID(c, "tenant_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := subscriptionSubject(c)
	if !ok {
		return nil
	}
	v, e := h.usecase.ReadPlatformSubscription(c.Context(), subject, tenant)
	if e != nil {
		return platformSubscriptionFailure(c, e)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"tenant_id": uuid.UUID(tenant).String(), "subscription": commercialSummaryView(v)}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func (h *PlatformSubscriptionHandler) schedule(c fiber.Ctx) error {
	return h.change(c, "SCHEDULE_CANCELLATION")
}
func (h *PlatformSubscriptionHandler) withdraw(c fiber.Ctx) error {
	return h.change(c, "WITHDRAW_CANCELLATION")
}
func (h *PlatformSubscriptionHandler) change(c fiber.Ctx, op string) error {
	prepareCommercialResponse(c)
	tenant, ok := commercialPathID(c, "tenant_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := subscriptionSubject(c)
	if !ok {
		return nil
	}
	key, _, ok := commercialMutationIDs(c)
	if !ok {
		return nil
	}
	var body commercialLifecycleBody
	if !platformPlanDecode(c, &body) || body.ExpectedVersion < 1 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, e := h.usecase.ChangePlatformSubscription(c.Context(), app.PlatformSubscriptionChange{Subject: subject, TenantID: tenant, ExpectedVersion: body.ExpectedVersion, Operation: op, IdempotencyKey: key})
	if e != nil {
		return platformSubscriptionFailure(c, e)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"tenant_id": uuid.UUID(result.TenantID).String(), "subscription_id": uuid.UUID(result.SubscriptionID).String(), "state": result.State, "version": result.Version, "cancel_at": commercialOptionalTimestamp(result.CancelAt), "replay": result.Replay, "payment_executed": false}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func platformSubscriptionFailure(c fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, app.ErrPlatformSubscriptionInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(e, app.ErrPlatformSubscriptionForbidden):
		return fail(c, 403, "FORBIDDEN", "The operation is not permitted.", false)
	case errors.Is(e, app.ErrPlatformSubscriptionNotFound):
		return fail(c, 404, "NOT_FOUND", "The subscription was not found.", false)
	case errors.Is(e, app.ErrPlatformSubscriptionConflict):
		return fail(c, 409, "SUBSCRIPTION_CONFLICT", "The subscription changed or cannot be updated.", false)
	default:
		return fail(c, 503, "SUBSCRIPTION_UNAVAILABLE", "Subscription data is temporarily unavailable.", true)
	}
}
