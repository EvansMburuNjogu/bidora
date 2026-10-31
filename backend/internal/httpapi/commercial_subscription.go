package httpapi

import (
	"context"
	"errors"
	"strconv"
	"time"

	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	CommercialSubscriptionPath         = "/v1/tenderpreneur/commercial/subscription"
	CommercialCancellationPath         = "/v1/tenderpreneur/commercial/subscription/cancellation"
	CommercialCancellationWithdrawPath = "/v1/tenderpreneur/commercial/subscription/cancellation/withdraw"
	CommercialTrialStartPath           = "/v1/tenderpreneur/commercial/subscription/trial"
)

type CommercialSubscriptionUseCase interface {
	SubscriptionSummary(context.Context, commercialapp.SubscriptionAccess) (commercialapp.SubscriptionSummary, error)
	ScheduleSubscriptionCancellation(context.Context, commercialapp.SubscriptionLifecycleInput) (commercialapp.SubscriptionSummary, error)
	WithdrawSubscriptionCancellation(context.Context, commercialapp.SubscriptionLifecycleInput) (commercialapp.SubscriptionSummary, error)
}
type CommercialTrialUseCase interface {
	StartTrialCommand(context.Context, commercialapp.TrialStartInput) (commercialapp.TrialStartResult, error)
}
type CommercialSubscriptionHandler struct {
	usecase CommercialSubscriptionUseCase
	trial   CommercialTrialUseCase
}

func NewCommercialSubscriptionHandler(usecase CommercialSubscriptionUseCase) (*CommercialSubscriptionHandler, error) {
	if usecase == nil {
		return nil, errors.New("commercial_subscription_handler_invalid")
	}
	return &CommercialSubscriptionHandler{usecase: usecase}, nil
}
func NewCommercialSubscriptionHandlerWithTrial(usecase CommercialSubscriptionUseCase, trial CommercialTrialUseCase) (*CommercialSubscriptionHandler, error) {
	if usecase == nil || trial == nil {
		return nil, errors.New("commercial_subscription_handler_invalid")
	}
	return &CommercialSubscriptionHandler{usecase: usecase, trial: trial}, nil
}
func RegisterCommercialSubscriptionRoutes(app *fiber.App, browser *BrowserSessionMiddleware, handler *CommercialSubscriptionHandler) error {
	if app == nil || browser == nil || handler == nil {
		return errors.New("commercial_subscription_routes_invalid")
	}
	app.Get(CommercialSubscriptionPath, browser.Platform, handler.Summary)
	if handler.trial != nil {
		app.Post(CommercialTrialStartPath, browser.Platform, handler.StartTrial)
	}
	app.Post(CommercialCancellationPath, browser.Platform, handler.ScheduleCancellation)
	app.Post(CommercialCancellationWithdrawPath, browser.Platform, handler.WithdrawCancellation)
	return nil
}

type commercialLifecycleBody struct {
	ExpectedVersion int64 `json:"expected_version"`
}

func (h *CommercialSubscriptionHandler) Summary(c fiber.Ctx) error {
	c.Set("Cache-Control", "private, no-store")
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	access, ok := commercialAccess(c)
	if !ok {
		return nil
	}
	summary, err := h.usecase.SubscriptionSummary(c.Context(), access)
	if err != nil {
		return commercialSubscriptionFailure(c, err)
	}
	return c.Status(200).JSON(commercialSummaryView(summary))
}
func (h *CommercialSubscriptionHandler) StartTrial(c fiber.Ctx) error {
	prepareCommercialResponse(c)
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	access, ok := commercialAccess(c)
	if !ok {
		return nil
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		return invalidHeader(c, IdempotencyKeyHeader, "required")
	}
	if _, err := uuid.Parse(key); err != nil {
		return invalidHeader(c, IdempotencyKeyHeader, "uuid")
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok || h.trial == nil {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	result, err := h.trial.StartTrialCommand(c.Context(), commercialapp.TrialStartInput{
		TenantID: access.TenantID, InitiatingAdminID: access.ActorID,
		CorrelationID: domain.ID(correlation), IdempotencyKey: key, ContextGeneration: access.ContextGeneration,
	})
	if err != nil {
		return commercialTrialFailure(c, err)
	}
	return c.Status(201).JSON(fiber.Map{
		"subscription_id": uuid.UUID(result.SubscriptionID).String(), "state": result.State,
		"version": result.Version, "trial_ends_at": commercialTimestamp(result.TrialEndsAt), "replay": result.Replay,
	})
}
func commercialTrialFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, commercialapp.ErrTrialStartInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(err, commercialapp.ErrTrialStartIneligible):
		return fail(c, 403, "TRIAL_INELIGIBLE", "The trial is not available.", false)
	case errors.Is(err, commercialapp.ErrTrialStartConflict):
		return fail(c, 409, "SUBSCRIPTION_CONFLICT", "The subscription changed or the operation is unavailable.", false)
	default:
		return fail(c, 503, "SUBSCRIPTION_UNAVAILABLE", "Subscription service is temporarily unavailable.", true)
	}
}
func (h *CommercialSubscriptionHandler) ScheduleCancellation(c fiber.Ctx) error {
	return h.lifecycle(c, false)
}
func (h *CommercialSubscriptionHandler) WithdrawCancellation(c fiber.Ctx) error {
	return h.lifecycle(c, true)
}
func (h *CommercialSubscriptionHandler) lifecycle(c fiber.Ctx, withdraw bool) error {
	var body commercialLifecycleBody
	if len(c.Body()) > 4096 {
		return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
	}
	if err := bindOrganization(c, &body); err != nil {
		return err
	}
	if !requireOrganizationFields(c.Body(), "expected_version") {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	access, ok := commercialAccess(c)
	if !ok {
		return nil
	}
	key, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		return invalidHeader(c, IdempotencyKeyHeader, "required")
	}
	if _, err := uuid.Parse(key); err != nil {
		return invalidHeader(c, IdempotencyKeyHeader, "uuid")
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		return fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
	}
	input := commercialapp.SubscriptionLifecycleInput{SubscriptionAccess: access, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: key, CorrelationID: domain.ID(correlation)}
	var result commercialapp.SubscriptionSummary
	var err error
	if withdraw {
		result, err = h.usecase.WithdrawSubscriptionCancellation(c.Context(), input)
	} else {
		result, err = h.usecase.ScheduleSubscriptionCancellation(c.Context(), input)
	}
	if err != nil {
		return commercialSubscriptionFailure(c, err)
	}
	return c.Status(200).JSON(commercialSummaryView(result))
}
func commercialAccess(c fiber.Ctx) (commercialapp.SubscriptionAccess, bool) {
	principal, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return commercialapp.SubscriptionAccess{}, false
	}
	tenantRaw, ok := singleHeader(c, TenantIDHeader)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "required")
		return commercialapp.SubscriptionAccess{}, false
	}
	tenant, ok := parseIdentityID(tenantRaw)
	if !ok {
		_ = invalidHeader(c, TenantIDHeader, "uuid")
		return commercialapp.SubscriptionAccess{}, false
	}
	raw, ok := singleHeader(c, ContextGenerationHeader)
	if !ok {
		_ = invalidHeader(c, ContextGenerationHeader, "required")
		return commercialapp.SubscriptionAccess{}, false
	}
	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || generation < 1 {
		_ = invalidHeader(c, ContextGenerationHeader, "positive_integer")
		return commercialapp.SubscriptionAccess{}, false
	}
	return commercialapp.SubscriptionAccess{ActorID: domain.ID(principal.UserID), TenantID: domain.ID(tenant), ContextGeneration: generation}, true
}
func commercialSubscriptionFailure(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, commercialapp.ErrSubscriptionAccessDenied):
		return fail(c, 403, "FORBIDDEN", "The operation is not permitted.", false)
	case errors.Is(err, commercialapp.ErrSubscriptionNotFound):
		return fail(c, 404, "NOT_FOUND", "The subscription was not found.", false)
	case errors.Is(err, commercialapp.ErrSubscriptionLifecycleConflict):
		return fail(c, 409, "SUBSCRIPTION_CONFLICT", "The subscription changed or the operation is unavailable.", false)
	case errors.Is(err, commercialapp.ErrSubscriptionRequestInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	default:
		return fail(c, 503, "SUBSCRIPTION_UNAVAILABLE", "Subscription service is temporarily unavailable.", true)
	}
}
func commercialSummaryView(value commercialapp.SubscriptionSummary) fiber.Map {
	return fiber.Map{"subscription_id": uuid.UUID(value.SubscriptionID).String(), "state": value.Subscription.State, "access_mode": value.Subscription.Access(), "version": value.Subscription.Version, "plan_key": value.Subscription.Term.PlanKey, "catalogue_version": value.Subscription.Term.CatalogueVersion, "cadence": value.Subscription.Term.Cadence, "commercial_period_start": commercialTimestamp(value.Subscription.Term.CommercialPeriodStart), "commercial_period_end": commercialTimestamp(value.Subscription.Term.CommercialPeriodEnd), "usage_period_start": commercialTimestamp(value.Subscription.Term.UsagePeriodStart), "usage_period_end": commercialTimestamp(value.Subscription.Term.UsagePeriodEnd), "trial_ends_at": commercialOptionalTimestamp(value.Subscription.TrialEndsAt), "grace_ends_at": commercialOptionalTimestamp(value.Subscription.GraceEndsAt), "cancel_at": commercialOptionalTimestamp(value.Subscription.CancelAt), "limits": value.Subscription.Term.Limits, "usage": value.Usage}
}
func commercialTimestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func commercialOptionalTimestamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return commercialTimestamp(*value)
}
