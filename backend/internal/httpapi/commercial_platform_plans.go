package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	app "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	"github.com/gofiber/fiber/v3"
)

type PlatformPlanUseCase interface {
	ListPlatformPlans(context.Context, app.PlanSubject) ([]app.PlanManagementItem, error)
	ChangePlatformPlan(context.Context, app.PlanChange) (app.PlanReceipt, error)
}
type PlatformPlanHandler struct{ usecase PlatformPlanUseCase }

func NewPlatformPlanHandler(u PlatformPlanUseCase) (*PlatformPlanHandler, error) {
	if u == nil {
		return nil, errors.New("platform_plan_handler_invalid")
	}
	return &PlatformPlanHandler{u}, nil
}
func RegisterPlatformPlanRoutes(api *fiber.App, browser *BrowserSessionMiddleware, h *PlatformPlanHandler) error {
	if api == nil || browser == nil || h == nil {
		return errors.New("platform_plan_routes_invalid")
	}
	g := api.Group("/v1/platform/commercial/plans", browser.Platform)
	g.Get("/management", h.list)
	g.Post("/:plan_key/drafts", h.draft)
	g.Post("/:plan_key/publish", h.publish)
	return nil
}
func platformPlanSubject(c fiber.Ctx) (app.PlanSubject, bool) {
	actor, correlation, ok := platformCommercialPrincipal(c)
	return app.PlanSubject{UserID: actor.UserID, SessionID: actor.SessionID, ContextGeneration: actor.ContextGeneration, CorrelationID: correlation}, ok
}
func (h *PlatformPlanHandler) list(c fiber.Ctx) error {
	prepareCommercialResponse(c)
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := platformPlanSubject(c)
	if !ok {
		return nil
	}
	items, err := h.usecase.ListPlatformPlans(c.Context(), subject)
	if err != nil {
		return platformPlanFailure(c, err)
	}
	result := make([]fiber.Map, 0, len(items))
	for _, item := range items {
		var draft any
		if item.Draft != nil {
			draft = platformPlanConfigView(*item.Draft)
		}
		result = append(result, fiber.Map{"plan_key": item.Published.Key, "published": platformPlanConfigView(item.Published), "draft": draft, "active_version": item.Head.ActiveVersion, "head_version": item.Head.HeadVersion})
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"plans": result}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}

type platformPlanMoney struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}
type platformDraftBody struct {
	ExpectedHeadVersion           int64             `json:"expected_head_version"`
	MonthlyPrice                  platformPlanMoney `json:"monthly_price"`
	AnnualPrice                   platformPlanMoney `json:"annual_price"`
	AdditionalLicenceMonthlyPrice platformPlanMoney `json:"additional_licence_monthly_price"`
	BidWorkspaces                 int32             `json:"bid_workspaces"`
	IncludedLicences              int32             `json:"included_licences"`
	StorageBytes                  int64             `json:"storage_bytes"`
	DocumentVersions              int64             `json:"document_versions"`
	AICredits                     int64             `json:"ai_credits"`
	ConcurrentAIJobs              int32             `json:"concurrent_ai_jobs"`
}
type platformPublishBody struct {
	ExpectedHeadVersion int64 `json:"expected_head_version"`
	Version             int64 `json:"version"`
}

func platformPlanDecode(c fiber.Ctx, v any) bool {
	if len(c.Body()) == 0 || len(c.Body()) > 4096 {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(c.Body()))
	dec.DisallowUnknownFields()
	if dec.Decode(v) != nil {
		return false
	}
	return dec.Decode(new(any)) == io.EOF
}
func planMinor(m platformPlanMoney) (int64, bool) {
	if m.Currency != "KES" || len(m.Amount) < 4 || len(m.Amount) > 16 || strings.Count(m.Amount, ".") != 1 {
		return 0, false
	}
	p := strings.Split(m.Amount, ".")
	if len(p[1]) != 2 || len(p[0]) < 1 || len(p[0]) > 13 || len(p[0]) > 1 && p[0][0] == '0' {
		return 0, false
	}
	for _, digit := range p[0] + p[1] {
		if digit < '0' || digit > '9' {
			return 0, false
		}
	}
	whole, e := strconv.ParseInt(p[0], 10, 64)
	if e != nil || whole > 10_000_000_000 {
		return 0, false
	}
	cents, _ := strconv.ParseInt(p[1], 10, 64)
	result := whole*100 + cents
	return result, result > 0 && result <= 1_000_000_000_000
}
func platformPlanKey(c fiber.Ctx) (domain.PlanKey, bool) {
	key, e := domain.ParsePlan(c.Params("plan_key"))
	return key, e == nil && (key == domain.Basic || key == domain.Growth || key == domain.Professional)
}
func (h *PlatformPlanHandler) draft(c fiber.Ctx) error {
	return h.change(c, "DRAFT")
}
func (h *PlatformPlanHandler) publish(c fiber.Ctx) error { return h.change(c, "PUBLISH") }
func (h *PlatformPlanHandler) change(c fiber.Ctx, operation string) error {
	prepareCommercialResponse(c)
	key, ok := platformPlanKey(c)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	subject, ok := platformPlanSubject(c)
	if !ok {
		return nil
	}
	idempotency, _, ok := commercialMutationIDs(c)
	if !ok {
		return nil
	}
	in := app.PlanChange{Subject: subject, Operation: operation, Config: app.PlanConfig{Key: key}, IdempotencyKey: idempotency}
	if operation == "DRAFT" {
		var body platformDraftBody
		if !platformPlanDecode(c, &body) {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		monthly, m := planMinor(body.MonthlyPrice)
		annual, a := planMinor(body.AnnualPrice)
		licence, l := planMinor(body.AdditionalLicenceMonthlyPrice)
		if !m || !a || !l {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		in.ExpectedHeadVersion = body.ExpectedHeadVersion
		in.Config = app.PlanConfig{Key: key, MonthlyMinor: monthly, AnnualMinor: annual, AdditionalLicenceMinor: licence, BidWorkspaces: body.BidWorkspaces, IncludedLicences: body.IncludedLicences, StorageBytes: body.StorageBytes, DocumentVersions: body.DocumentVersions, AICredits: body.AICredits, ConcurrentAIJobs: body.ConcurrentAIJobs}
	} else {
		var body platformPublishBody
		if !platformPlanDecode(c, &body) {
			return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		}
		in.ExpectedHeadVersion = body.ExpectedHeadVersion
		in.PublishVersion = body.Version
	}
	result, err := h.usecase.ChangePlatformPlan(c.Context(), in)
	if err != nil {
		return platformPlanFailure(c, err)
	}
	if !result.Replay {
		c.Status(201)
	}
	return c.JSON(fiber.Map{"data": fiber.Map{"plan_key": result.Key, "version": result.Version, "head_version": result.HeadVersion, "published": result.Published, "replay": result.Replay}, "meta": meta{TraceID: c.GetRespHeader("X-Request-ID")}})
}
func platformPlanConfigView(v app.PlanConfig) fiber.Map {
	return fiber.Map{
		"version": v.Version, "head_version": v.HeadVersion, "monthly_price": moneyView(v.MonthlyMinor), "annual_price": moneyView(v.AnnualMinor), "additional_licence_monthly_price": moneyView(v.AdditionalLicenceMinor),
		"bid_workspaces": v.BidWorkspaces, "included_licences": v.IncludedLicences, "storage_bytes": v.StorageBytes, "document_versions": v.DocumentVersions, "ai_credits": v.AICredits, "concurrent_ai_jobs": v.ConcurrentAIJobs,
	}
}
func platformPlanFailure(c fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, app.ErrPlatformPlanInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(e, app.ErrPlatformPlanForbidden):
		return fail(c, 403, "FORBIDDEN", "The operation is not permitted.", false)
	case errors.Is(e, app.ErrPlatformPlanConflict):
		return fail(c, 409, "PLAN_CONFLICT", "The plan changed or cannot be published.", false)
	default:
		return fail(c, 503, "PLAN_UNAVAILABLE", "Plans are temporarily unavailable.", true)
	}
}
