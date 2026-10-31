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

type TenderpreneurDashboardUseCase interface {
	Tender(context.Context, dashboardapp.TenderRequest) (dashboardapp.TenderDashboard, error)
}

type TenderpreneurDashboardHandler struct{ usecase TenderpreneurDashboardUseCase }

func NewTenderpreneurDashboardHandler(usecase TenderpreneurDashboardUseCase) (*TenderpreneurDashboardHandler, error) {
	if usecase == nil {
		return nil, errors.New("tenderpreneur_dashboard_handler_invalid")
	}
	return &TenderpreneurDashboardHandler{usecase: usecase}, nil
}

func RegisterTenderpreneurDashboardRoute(app *fiber.App, browser *BrowserSessionMiddleware, handler *TenderpreneurDashboardHandler) error {
	if app == nil || browser == nil || handler == nil {
		return errors.New("tenderpreneur_dashboard_route_invalid")
	}
	app.Get("/v1/tenderpreneur/dashboard", browser.Handle, handler.read)
	return nil
}

func (h *TenderpreneurDashboardHandler) read(c fiber.Ctx) error {
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
	tenantText, ok := singleHeader(c, TenantIDHeader)
	if !ok {
		return invalidHeader(c, TenantIDHeader, "required")
	}
	tenantID, ok := parseOpportunityID(tenantText)
	if !ok {
		return invalidHeader(c, TenantIDHeader, "uuid")
	}
	generationText, ok := singleHeader(c, ContextGenerationHeader)
	if !ok {
		return invalidHeader(c, ContextGenerationHeader, "required")
	}
	generation, ok := positiveInt64(generationText)
	if !ok {
		return invalidHeader(c, ContextGenerationHeader, "positive_integer")
	}
	result, err := h.usecase.Tender(c.Context(), dashboardapp.TenderRequest{Subject: dashboardapp.TenderSubject{ActorID: dashboardapp.ID(principal.UserID), TenantID: dashboardapp.ID(tenantID), ContextGeneration: generation}, Limit: int32(limit)})
	if err != nil {
		switch {
		case errors.Is(err, dashboardapp.ErrDashboardInvalid):
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		case errors.Is(err, dashboardapp.ErrDashboardNotAuthorized):
			return fail(c, 403, "TENDERPRENEUR_DASHBOARD_FORBIDDEN", "Dashboard access is forbidden.", false)
		default:
			return fail(c, 503, "TENDERPRENEUR_DASHBOARD_UNAVAILABLE", "The dashboard is temporarily unavailable.", true)
		}
	}
	return platformJSON(c, 200, tenderpreneurDashboardView(result), nil)
}

func tenderpreneurDashboardView(v dashboardapp.TenderDashboard) any {
	opportunities := make([]any, len(v.Opportunities.Items))
	for i, item := range v.Opportunities.Items {
		opportunities[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "title": item.Title, "verification_state": item.VerificationState, "buyer_name": item.BuyerName, "closes_at": item.ClosesAt, "version": item.Version, "updated_at": item.UpdatedAt}
	}
	bids := make([]any, len(v.Bids.Items))
	for i, item := range v.Bids.Items {
		bids[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "opportunity_id": uuid.UUID(item.OpportunityID).String(), "owner_id": uuid.UUID(item.OwnerID).String(), "title": item.Title, "state": item.State, "buyer_name": item.BuyerName, "closes_at": item.ClosesAt, "version": item.Version, "updated_at": item.UpdatedAt}
	}
	invoices := make([]any, len(v.Invoices.Items))
	for i, item := range v.Invoices.Items {
		invoices[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "invoice_number": item.Number, "state": item.State, "purchase_kind": item.PurchaseKind, "version": item.Version, "total": moneyView(item.TotalMinorUnits), "outstanding": moneyView(item.OutstandingMinorUnits), "issued_at": item.IssuedAt, "due_at": item.DueAt}
	}
	payments := make([]any, len(v.Payments.Items))
	for i, item := range v.Payments.Items {
		payments[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "invoice_id": uuid.UUID(item.InvoiceID).String(), "state": item.State, "version": item.Version, "amount": moneyView(item.AmountMinorUnits), "reconciliation_pending": item.ReconciliationPending, "updated_at": item.UpdatedAt}
	}
	activity := make([]any, len(v.FinancialActivity.Items))
	for i, item := range v.FinancialActivity.Items {
		activity[i] = fiber.Map{"id": uuid.UUID(item.ID).String(), "invoice_id": uuid.UUID(item.InvoiceID).String(), "payment_id": optionalDashboardID(item.PaymentID), "kind": item.Kind, "document_number": nullableText(item.DocumentNumber), "state": item.State, "amount": moneyView(item.AmountMinorUnits), "occurred_at": item.OccurredAt}
	}
	s := v.Subscription
	return fiber.Map{
		"tenant_id": uuid.UUID(v.TenantID).String(), "observed_at": v.ObservedAt,
		"subscription":       fiber.Map{"id": uuid.UUID(s.ID).String(), "state": s.State, "access_mode": s.AccessMode, "plan_key": s.PlanKey, "catalogue_version": s.CatalogueVersion, "cadence": s.Cadence, "version": s.Version, "commercial_period_start": s.CommercialPeriodStart, "commercial_period_end": s.CommercialPeriodEnd, "limits": fiber.Map{"bid_workspaces": s.BidWorkspaceLimit, "document_versions": s.DocumentVersionLimit, "ai_credits": s.AICreditLimit, "storage_bytes": s.StorageLimitBytes, "included_licences": s.IncludedLicenceLimit, "concurrent_ai_jobs": s.ConcurrentAIJobLimit}, "usage": fiber.Map{"bid_workspaces": s.BidWorkspacesUsed, "document_versions": s.DocumentVersionsUsed, "ai_credits_used": s.AICreditsUsed, "ai_credits_reserved": s.AICreditsReserved, "storage_bytes": s.StorageBytesUsed, "concurrent_ai_jobs": s.ConcurrentAIJobs}},
		"opportunities":      fiber.Map{"items": opportunities, "has_more": v.Opportunities.HasMore},
		"bids":               fiber.Map{"items": bids, "has_more": v.Bids.HasMore},
		"invoices":           fiber.Map{"items": invoices, "has_more": v.Invoices.HasMore},
		"payments":           fiber.Map{"items": payments, "has_more": v.Payments.HasMore},
		"financial_activity": fiber.Map{"items": activity, "has_more": v.FinancialActivity.HasMore},
	}
}

func optionalDashboardID(id dashboardapp.ID) any {
	if id.IsZero() {
		return nil
	}
	return uuid.UUID(id).String()
}
