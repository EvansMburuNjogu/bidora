package httpapi

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	dashboardapp "bidora/backend/internal/dashboard/application"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PlatformDashboardUseCase interface {
	Platform(context.Context, dashboardapp.PlatformRequest) (dashboardapp.PlatformDashboard, error)
}

type PlatformDashboardHandler struct{ usecase PlatformDashboardUseCase }

func NewPlatformDashboardHandler(usecase PlatformDashboardUseCase) (*PlatformDashboardHandler, error) {
	if usecase == nil {
		return nil, errors.New("platform_dashboard_handler_invalid")
	}
	return &PlatformDashboardHandler{usecase: usecase}, nil
}

func RegisterPlatformDashboardRoute(app *fiber.App, browser *BrowserSessionMiddleware, handler *PlatformDashboardHandler) error {
	if app == nil || browser == nil || handler == nil {
		return errors.New("platform_dashboard_route_invalid")
	}
	app.Get("/v1/platform/dashboard", browser.Platform, handler.read)
	return nil
}

func (h *PlatformDashboardHandler) read(c fiber.Ctx) error {
	if len(c.Body()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	values, err := url.ParseQuery(string(c.Request().URI().QueryString()))
	if err != nil || len(values) != 1 || len(values["limit"]) != 1 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	limit, err := strconv.Atoi(values.Get("limit"))
	if err != nil || limit < 1 || limit > 25 {
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
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.usecase.Platform(c.Context(), dashboardapp.PlatformRequest{Actor: dashboardapp.PlatformActor{UserID: dashboardapp.ID(principal.UserID), SessionID: dashboardapp.ID(principal.SessionID), CorrelationID: dashboardapp.ID(correlation), ContextGeneration: generation}, Limit: int32(limit)})
	if err != nil {
		switch {
		case errors.Is(err, dashboardapp.ErrDashboardInvalid):
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		case errors.Is(err, dashboardapp.ErrDashboardNotAuthorized):
			return fail(c, 403, "PLATFORM_DASHBOARD_FORBIDDEN", "Platform dashboard access is forbidden.", false)
		default:
			return fail(c, 503, "PLATFORM_DASHBOARD_UNAVAILABLE", "The platform dashboard is temporarily unavailable.", true)
		}
	}
	return platformJSON(c, 200, platformDashboardView(result), nil)
}

func platformDashboardView(v dashboardapp.PlatformDashboard) any {
	invoices := make([]any, len(v.Invoices.Items))
	for i, item := range v.Invoices.Items {
		invoices[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "tenant_id": uuid.UUID(item.TenantID).String(), "invoice_number": item.Number, "state": item.State, "version": item.Version, "total": moneyView(item.TotalMinorUnits), "outstanding": moneyView(item.OutstandingMinorUnits), "issued_at": commercialTimestamp(item.IssuedAt)}
	}
	payments := make([]any, len(v.Payments.Items))
	for i, item := range v.Payments.Items {
		payments[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "tenant_id": uuid.UUID(item.TenantID).String(), "invoice_id": uuid.UUID(item.InvoiceID).String(), "state": item.State, "reconciliation_state": item.ReconciliationState, "version": item.Version, "amount": moneyView(item.AmountMinorUnits), "updated_at": commercialTimestamp(item.UpdatedAt)}
	}
	adjustments := make([]any, len(v.Adjustments.Items))
	for i, item := range v.Adjustments.Items {
		adjustments[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "tenant_id": uuid.UUID(item.TenantID).String(), "invoice_id": uuid.UUID(item.InvoiceID).String(), "action": item.Action, "state": item.State, "reason_code": item.ReasonCode, "version": item.Version, "amount": moneyView(item.AmountMinorUnits), "created_at": commercialTimestamp(item.CreatedAt), "expires_at": commercialTimestamp(item.ExpiresAt)}
	}
	reversals := make([]any, len(v.Reversals.Items))
	for i, item := range v.Reversals.Items {
		reversals[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "tenant_id": uuid.UUID(item.TenantID).String(), "payment_id": uuid.UUID(item.PaymentID).String(), "payment_state": item.PaymentState, "queue_state": item.QueueState, "hold_state": commercialOptionalString(item.HoldState), "amount": moneyView(item.AmountMinorUnits), "requested_at": commercialTimestamp(item.RequestedAt), "applied_at": commercialOptionalTimestamp(item.AppliedAt)}
	}
	return fiber.Map{
		"observed_at": v.ObservedAt.UTC(),
		"invoices":    fiber.Map{"items": invoices, "has_more": v.Invoices.HasMore},
		"payments":    fiber.Map{"items": payments, "has_more": v.Payments.HasMore},
		"adjustments": fiber.Map{"items": adjustments, "has_more": v.Adjustments.HasMore},
		"reversals":   fiber.Map{"items": reversals, "has_more": v.Reversals.HasMore},
	}
}
