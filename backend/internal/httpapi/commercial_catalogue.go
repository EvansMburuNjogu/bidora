package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commercialdomain "bidora/backend/internal/commercial/domain"
	"github.com/gofiber/fiber/v3"
)

const CommercialPlanCataloguePath = "/v1/shared/commercial/plans"

type CommercialCatalogueUseCase interface {
	Plans(context.Context) ([]commercialdomain.Plan, error)
}

type CommercialCatalogueHandler struct {
	usecase CommercialCatalogueUseCase
}

func NewCommercialCatalogueHandler(usecase CommercialCatalogueUseCase) (*CommercialCatalogueHandler, error) {
	if usecase == nil {
		return nil, errors.New("commercial_catalogue_handler_invalid")
	}
	return &CommercialCatalogueHandler{usecase: usecase}, nil
}

func RegisterCommercialCatalogueRoutes(app *fiber.App, browser *BrowserSessionMiddleware, handler *CommercialCatalogueHandler) error {
	if app == nil || browser == nil || handler == nil {
		return errors.New("commercial_catalogue_routes_invalid")
	}
	app.Get(CommercialPlanCataloguePath, browser.OriginBound, handler.Get)
	return nil
}

func (h *CommercialCatalogueHandler) Get(c fiber.Ctx) error {
	prepareCommercialResponse(c)
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "This endpoint accepts no body or query parameters.", false)
	}
	if _, ok := SessionPrincipalFrom(c); !ok {
		return fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
	}
	plans, err := h.usecase.Plans(c.Context())
	if err != nil {
		return fail(c, 503, "COMMERCIAL_CATALOGUE_UNAVAILABLE", "The plan catalogue is temporarily unavailable.", true)
	}
	items := make([]fiber.Map, len(plans))
	for index, plan := range plans {
		items[index] = commercialPlanView(plan)
	}
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"catalogue_version":      commercialCatalogueVersion(plans),
			"currency":               commercialdomain.CurrencyKES,
			"prices_include_vat":     false,
			"vat_calculated":         false,
			"subscription_activated": false,
			"entitlements_activated": false,
			"ai_credit_policy": fiber.Map{
				"pdf_page_credits":                1,
				"docx_text_unit_characters":       commercialdomain.AITextUnitCharacters,
				"docx_text_unit_credits":          1,
				"generation_base_credits":         commercialdomain.AIGenerationBaseCredits,
				"generation_text_unit_characters": commercialdomain.AITextUnitCharacters,
				"generation_text_unit_credits":    1,
				"rollover":                        false,
				"duplicate_platform_retry_charge": false,
				"platform_rebuild_charge":         false,
			},
			"plans": items,
		},
		"meta": meta{TraceID: c.GetRespHeader("X-Request-ID")},
	})
}

func commercialCatalogueVersion(plans []commercialdomain.Plan) string {
	version := commercialdomain.CatalogueVersion
	for _, plan := range plans {
		if plan.CatalogueVersion != commercialdomain.CatalogueVersion {
			version += "-" + strings.TrimPrefix(plan.CatalogueVersion, commercialdomain.CatalogueVersion+"-")
		}
	}
	return version
}
func prepareCommercialResponse(c fiber.Ctx) {
	c.Set("Cache-Control", "private, no-store")
	c.Set("Vary", "Cookie, Origin")
	c.Set("X-Content-Type-Options", "nosniff")
}

func commercialPlanView(plan commercialdomain.Plan) fiber.Map {
	cadences := []string{"MONTHLY", "ANNUAL"}
	allowancePeriod := "BILLING_MONTH"
	monthlyKind := "FIXED"
	switch plan.Key {
	case commercialdomain.Trial:
		cadences = []string{"TRIAL"}
		allowancePeriod = "TRIAL_TERM"
	case commercialdomain.Enterprise:
		cadences = []string{"CONTRACT"}
		allowancePeriod = "CONTRACT"
		monthlyKind = "FROM"
	}
	return fiber.Map{
		"key":                     plan.Key,
		"name":                    plan.Name,
		"catalogue_version":       plan.CatalogueVersion,
		"available_cadences":      cadences,
		"trial_days":              optionalCommercialInt(plan.TrialDays),
		"bid_workspace_allowance": fiber.Map{"count": optionalCommercialInt(plan.BidWorkspacesPerPeriod), "period": allowancePeriod},
		"included_licences":       optionalCommercialInt(plan.IncludedLicences),
		"limits": fiber.Map{
			"storage_bytes":               optionalCommercialInt64(plan.StorageLimitBytes),
			"completed_document_versions": optionalCommercialInt64(plan.DocumentVersionLimit),
			"ai_credits":                  optionalCommercialInt64(plan.AICreditLimit),
			"concurrent_ai_jobs":          optionalCommercialInt(plan.ConcurrentAIJobLimit),
			"reset_period":                plan.LimitResetPeriod,
			"rollover":                    false,
		},
		"trial_base_price":                 commercialMoneyView(plan.TrialBasePrice, "FIXED"),
		"monthly_base_price":               commercialMoneyView(plan.MonthlyBasePrice, monthlyKind),
		"annual_base_price":                commercialMoneyView(plan.AnnualBasePrice, "FIXED"),
		"additional_licence_monthly_price": commercialMoneyView(plan.AdditionalLicenceMonthlyPrice, "FIXED"),
		"contract_required":                plan.ContractRequired,
		"core_tender_assistant":            plan.CoreTenderAssistantTerm,
	}
}

func optionalCommercialInt(value *int32) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalCommercialInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func commercialMoneyView(value *commercialdomain.Money, kind string) any {
	if value == nil {
		return nil
	}
	return fiber.Map{"currency": value.Currency, "amount": commercialAmount(value.MinorUnits), "kind": kind}
}

func commercialAmount(minorUnits int64) string {
	return fmt.Sprintf("%d.%02d", minorUnits/100, minorUnits%100)
}
