package httpapi

import (
	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"net/url"
	"strconv"
	"time"
)

type CommercialPlatformFinancialUseCase interface {
	ProposeFinancialAdjustment(context.Context, commercialapp.ProposeAdjustmentInput) (commercialapp.AdjustmentStored, error)
	DecideFinancialAdjustment(context.Context, commercialapp.DecideAdjustmentInput) (commercialapp.AdjustmentStored, error)
	RequestPaymentReconciliation(context.Context, commercialapp.RequestReconciliationInput) (domain.Payment, error)
	ReadPlatformFinancialAdjustment(context.Context, commercialapp.PlatformFinancialReadInput) (commercialapp.FinancialAdjustmentRead, error)
	ReadPlatformPaymentReversal(context.Context, commercialapp.PlatformFinancialReadInput) (commercialapp.PaymentReversalRead, error)
	ListPlatformAIUsage(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformAIUsagePage, error)
	ListPlatformInvoiceQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformInvoiceQueuePage, error)
	ListPlatformPaymentQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformPaymentQueuePage, error)
	ListPlatformAdjustmentQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformAdjustmentQueuePage, error)
	ListPlatformReversalQueue(context.Context, commercialapp.PlatformQueueRequest) (commercialapp.PlatformReversalQueuePage, error)
}
type CommercialPlatformFinancialHandler struct {
	usecase CommercialPlatformFinancialUseCase
}

func NewCommercialPlatformFinancialHandler(u CommercialPlatformFinancialUseCase) (*CommercialPlatformFinancialHandler, error) {
	if u == nil {
		return nil, errors.New("commercial_platform_financial_handler_invalid")
	}
	return &CommercialPlatformFinancialHandler{u}, nil
}
func RegisterCommercialPlatformFinancialRoutes(app *fiber.App, browser *BrowserSessionMiddleware, h *CommercialPlatformFinancialHandler) error {
	if err := RegisterCommercialPlatformFinancialReadRoutes(app, browser, h); err != nil {
		return err
	}
	g := app.Group("/v1/platform/commercial", browser.Platform)
	g.Post("/payments/:payment_id/reconciliation", h.requestReconciliation)
	g.Post("/financial-adjustments", h.proposeAdjustment)
	g.Post("/financial-adjustments/:change_id/decision", h.decideAdjustment)
	return nil
}

// RegisterCommercialPlatformFinancialReadRoutes mounts only audited oversight
// reads. It deliberately excludes adjustment, reconciliation and provider-effect
// commands so read composition cannot accidentally activate financial writes.
func RegisterCommercialPlatformFinancialReadRoutes(app *fiber.App, browser *BrowserSessionMiddleware, h *CommercialPlatformFinancialHandler) error {
	if app == nil || browser == nil || h == nil {
		return errors.New("commercial_platform_financial_read_routes_invalid")
	}
	g := app.Group("/v1/platform/commercial", browser.Platform)
	g.Get("/ai-usage", h.aiUsage)
	g.Get("/invoices", h.invoices)
	g.Get("/payments", h.payments)
	g.Get("/financial-adjustments", h.adjustments)
	g.Get("/financial-adjustments/:change_id", h.adjustment)
	g.Get("/payment-reversals", h.reversals)
	g.Get("/payment-reversals/:payment_id", h.reversal)
	return nil
}

type adjustmentProposalBody struct {
	InvoiceID              string `json:"invoice_id"`
	Action                 string `json:"action"`
	ReasonCode             string `json:"reason_code"`
	AmountMinorUnits       int64  `json:"amount_minor_units"`
	EntitlementDisposition string `json:"entitlement_disposition"`
	ProposalReason         string `json:"proposal_reason"`
	ExpectedInvoiceVersion int64  `json:"expected_invoice_version"`
	ExpectedPaymentVersion int64  `json:"expected_payment_version"`
}
type adjustmentDecisionBody struct {
	Decision        string `json:"decision"`
	Reason          string `json:"reason"`
	ExpectedVersion int64  `json:"expected_version"`
}
type reconciliationBody struct {
	ExpectedVersion int64  `json:"expected_version"`
	Reason          string `json:"reason"`
}

func (h *CommercialPlatformFinancialHandler) proposeAdjustment(c fiber.Ctx) error {
	var b adjustmentProposalBody
	if e := bindCommercialJSON(c, &b, 4096, "invoice_id", "action", "reason_code", "amount_minor_units", "entitlement_disposition", "proposal_reason", "expected_invoice_version", "expected_payment_version"); e != nil {
		return e
	}
	actor, key, correlation, ok := platformCommercialMutation(c)
	if !ok {
		return nil
	}
	invoice, ok := commercialID(b.InvoiceID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	v, e := h.usecase.ProposeFinancialAdjustment(c.Context(), commercialapp.ProposeAdjustmentInput{Actor: actor, InvoiceID: invoice, CorrelationID: correlation, Action: b.Action, ReasonCode: b.ReasonCode, AmountMinorUnits: b.AmountMinorUnits, EntitlementDisposition: b.EntitlementDisposition, ProposalReason: b.ProposalReason, ExpectedInvoiceVersion: b.ExpectedInvoiceVersion, ExpectedPaymentVersion: b.ExpectedPaymentVersion, IdempotencyKey: key})
	if e != nil {
		return commercialPlatformFailure(c, e)
	}
	return commercialJSON(c, 201, adjustmentStoredView(v), nil)
}
func (h *CommercialPlatformFinancialHandler) decideAdjustment(c fiber.Ctx) error {
	var b adjustmentDecisionBody
	if e := bindCommercialJSON(c, &b, 4096, "decision", "reason", "expected_version"); e != nil {
		return e
	}
	actor, key, correlation, ok := platformCommercialMutation(c)
	if !ok {
		return nil
	}
	change, ok := commercialPathID(c, "change_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	v, e := h.usecase.DecideFinancialAdjustment(c.Context(), commercialapp.DecideAdjustmentInput{Actor: actor, ChangeID: change, CorrelationID: correlation, Decision: b.Decision, Reason: b.Reason, ExpectedVersion: b.ExpectedVersion, IdempotencyKey: key})
	if e != nil {
		return commercialPlatformFailure(c, e)
	}
	return commercialJSON(c, 200, adjustmentStoredView(v), nil)
}
func (h *CommercialPlatformFinancialHandler) requestReconciliation(c fiber.Ctx) error {
	var b reconciliationBody
	if e := bindCommercialJSON(c, &b, 4096, "expected_version", "reason"); e != nil {
		return e
	}
	actor, key, correlation, ok := platformCommercialMutation(c)
	if !ok {
		return nil
	}
	payment, ok := commercialPathID(c, "payment_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	v, e := h.usecase.RequestPaymentReconciliation(c.Context(), commercialapp.RequestReconciliationInput{Actor: actor, PaymentID: payment, CorrelationID: correlation, ExpectedVersion: b.ExpectedVersion, Reason: b.Reason, IdempotencyKey: key})
	if e != nil {
		return commercialPlatformFailure(c, e)
	}
	return commercialJSON(c, 202, reconciliationScheduledView(v), nil)
}
func (h *CommercialPlatformFinancialHandler) adjustment(c fiber.Ctx) error {
	actor, correlation, ok := platformCommercialRead(c)
	if !ok {
		return nil
	}
	id, ok := commercialPathID(c, "change_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	v, e := h.usecase.ReadPlatformFinancialAdjustment(c.Context(), commercialapp.PlatformFinancialReadInput{Actor: actor, TargetID: id, CorrelationID: correlation})
	if e != nil {
		return commercialPlatformFailure(c, e)
	}
	return commercialJSON(c, 200, adjustmentReadView(v), nil)
}
func (h *CommercialPlatformFinancialHandler) reversal(c fiber.Ctx) error {
	actor, correlation, ok := platformCommercialRead(c)
	if !ok {
		return nil
	}
	id, ok := commercialPathID(c, "payment_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	v, e := h.usecase.ReadPlatformPaymentReversal(c.Context(), commercialapp.PlatformFinancialReadInput{Actor: actor, TargetID: id, CorrelationID: correlation})
	if e != nil {
		return commercialPlatformFailure(c, e)
	}
	return commercialJSON(c, 200, reversalReadView(v), nil)
}
func (h *CommercialPlatformFinancialHandler) aiUsage(c fiber.Ctx) error {
	actor, correlation, ok := platformCommercialReadBody(c)
	if !ok {
		return nil
	}
	page, _, ok := parsePlatformCommercialPage(c, "", "")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	result, err := h.usecase.ListPlatformAIUsage(c.Context(), commercialapp.PlatformQueueRequest{Actor: actor, CorrelationID: correlation, Limit: page.limit, Cursor: page.cursor})
	if err != nil {
		return commercialPlatformFailure(c, err)
	}
	items := make([]any, len(result.Items))
	for i, v := range result.Items {
		items[i] = fiber.Map{"id": uuid.UUID(v.ID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "kind": v.Kind, "actual_credits": strconv.FormatInt(v.ActualCredits, 10), "settled_at": commercialTimestamp(v.SettledAt)}
	}
	return commercialJSON(c, 200, items, result.NextCursor)
}
func (h *CommercialPlatformFinancialHandler) invoices(c fiber.Ctx) error {
	return h.queue(c, "state", "", func(r commercialapp.PlatformQueueRequest) error {
		v, e := h.usecase.ListPlatformInvoiceQueue(c.Context(), r)
		if e != nil {
			return e
		}
		items := make([]any, len(v.Items))
		for i, x := range v.Items {
			items[i] = platformInvoiceView(x)
		}
		return commercialJSON(c, 200, items, v.NextCursor)
	})
}
func (h *CommercialPlatformFinancialHandler) payments(c fiber.Ctx) error {
	return h.queue(c, "state", "reconciliation_state", func(r commercialapp.PlatformQueueRequest) error {
		v, e := h.usecase.ListPlatformPaymentQueue(c.Context(), r)
		if e != nil {
			return e
		}
		items := make([]any, len(v.Items))
		for i, x := range v.Items {
			items[i] = platformPaymentView(x)
		}
		return commercialJSON(c, 200, items, v.NextCursor)
	})
}
func (h *CommercialPlatformFinancialHandler) adjustments(c fiber.Ctx) error {
	return h.queue(c, "state", "action", func(r commercialapp.PlatformQueueRequest) error {
		v, e := h.usecase.ListPlatformAdjustmentQueue(c.Context(), r)
		if e != nil {
			return e
		}
		items := make([]any, len(v.Items))
		for i, x := range v.Items {
			items[i] = platformAdjustmentView(x)
		}
		return commercialJSON(c, 200, items, v.NextCursor)
	})
}
func (h *CommercialPlatformFinancialHandler) reversals(c fiber.Ctx) error {
	return h.queue(c, "state", "", func(r commercialapp.PlatformQueueRequest) error {
		v, e := h.usecase.ListPlatformReversalQueue(c.Context(), r)
		if e != nil {
			return e
		}
		items := make([]any, len(v.Items))
		for i, x := range v.Items {
			items[i] = platformReversalView(x)
		}
		return commercialJSON(c, 200, items, v.NextCursor)
	})
}
func (h *CommercialPlatformFinancialHandler) queue(c fiber.Ctx, primary, secondary string, load func(commercialapp.PlatformQueueRequest) error) error {
	actor, correlation, ok := platformCommercialReadBody(c)
	if !ok {
		return nil
	}
	page, values, ok := parsePlatformCommercialPage(c, primary, secondary)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	e := load(commercialapp.PlatformQueueRequest{Actor: actor, CorrelationID: correlation, Limit: page.limit, Cursor: page.cursor, StateFilter: values.Get(primary), SecondaryFilter: values.Get(secondary)})
	if e != nil {
		return commercialPlatformFailure(c, e)
	}
	return nil
}
func platformCommercialMutation(c fiber.Ctx) (commercialapp.AdjustmentActor, string, domain.ID, bool) {
	actor, correlation, ok := platformCommercialPrincipal(c)
	if !ok {
		return actor, "", correlation, false
	}
	key, _, ok := commercialMutationIDs(c)
	return actor, key, correlation, ok
}
func platformCommercialRead(c fiber.Ctx) (commercialapp.AdjustmentActor, domain.ID, bool) {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return commercialapp.AdjustmentActor{}, domain.ID{}, false
	}
	return platformCommercialPrincipal(c)
}
func platformCommercialReadBody(c fiber.Ctx) (commercialapp.AdjustmentActor, domain.ID, bool) {
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return commercialapp.AdjustmentActor{}, domain.ID{}, false
	}
	return platformCommercialPrincipal(c)
}
func platformCommercialPrincipal(c fiber.Ctx) (commercialapp.AdjustmentActor, domain.ID, bool) {
	p, ok := SessionPrincipalFrom(c)
	if !ok {
		_ = fail(c, 401, "SESSION_INVALID", "Authentication is required.", false)
		return commercialapp.AdjustmentActor{}, domain.ID{}, false
	}
	generation, ok := platformGeneration(c)
	if !ok {
		return commercialapp.AdjustmentActor{}, domain.ID{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		return commercialapp.AdjustmentActor{}, domain.ID{}, false
	}
	return commercialapp.AdjustmentActor{UserID: domain.ID(p.UserID), SessionID: domain.ID(p.SessionID), ContextGeneration: generation}, domain.ID(correlation), true
}
func parsePlatformCommercialPage(c fiber.Ctx, primary, secondary string) (commercialPage, url.Values, bool) {
	values, e := url.ParseQuery(string(c.Request().URI().QueryString()))
	if e != nil {
		return commercialPage{}, nil, false
	}
	allowed := map[string]bool{"limit": true, "cursor": true}
	if primary != "" {
		allowed[primary] = true
	}
	if secondary != "" {
		allowed[secondary] = true
	}
	for k, v := range values {
		if !allowed[k] || len(v) != 1 {
			return commercialPage{}, nil, false
		}
	}
	limit := int32(25)
	if raw := values.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			return commercialPage{}, nil, false
		}
		limit = int32(n)
	}
	page := commercialPage{limit: limit}
	if raw := values.Get("cursor"); raw != "" {
		decoded, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil || len(decoded) > 384 {
			return page, nil, false
		}
		var w struct{ At, ID, Kind string }
		if json.Unmarshal(decoded, &w) != nil || w.Kind != "" {
			return page, nil, false
		}
		at, e := time.Parse(time.RFC3339Nano, w.At)
		id, ok := commercialID(w.ID)
		if e != nil || !ok {
			return page, nil, false
		}
		page.cursor = &commercialapp.CommercialListCursor{OccurredAt: at.UTC(), ID: id}
	}
	return page, values, true
}
func reconciliationScheduledView(v domain.Payment) any {
	return fiber.Map{"payment_id": uuid.UUID(v.ID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "state": v.State, "version": v.Version, "reconciliation_state": v.ReconciliationState, "reconciliation_requested": true, "updated_at": commercialTimestamp(v.UpdatedAt)}
}
func adjustmentStoredView(v commercialapp.AdjustmentStored) any {
	return fiber.Map{"id": uuid.UUID(v.Proposal.ID).String(), "invoice_id": uuid.UUID(v.Proposal.InvoiceID).String(), "payment_id": uuid.UUID(v.Proposal.PaymentID).String(), "action": v.Proposal.Action, "state": v.State, "version": v.Proposal.Version, "reason_code": v.Proposal.ReasonCode, "amount": moneyView(v.Proposal.AmountMinorUnits), "net": moneyView(v.Proposal.NetMinorUnits), "tax": moneyView(v.Proposal.TaxMinorUnits), "entitlement_disposition": v.Proposal.EntitlementDisposition, "expires_at": commercialTimestamp(v.Proposal.ExpiresAt), "credit_note_id": optionalCommercialID(v.CreditNoteID), "credit_number": commercialOptionalString(v.CreditNumber), "refund_operation_id": optionalCommercialID(v.RefundID), "provider_refund_executed": false, "entitlement_disposition_executed": false}
}
func adjustmentReadView(v commercialapp.FinancialAdjustmentRead) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "payment_id": uuid.UUID(v.PaymentID).String(), "action": v.Action, "state": v.State, "version": v.Version, "reason_code": v.ReasonCode, "entitlement_disposition": v.EntitlementDisposition, "proposal_reason": v.ProposalReason, "decision_reason": commercialOptionalString(v.DecisionReason), "maker_user_id": uuid.UUID(v.MakerUserID).String(), "checker_user_id": optionalCommercialID(v.CheckerUserID), "amount": moneyView(v.AmountMinorUnits), "net": moneyView(v.NetMinorUnits), "tax": moneyView(v.TaxMinorUnits), "created_at": commercialTimestamp(v.CreatedAt), "expires_at": commercialTimestamp(v.ExpiresAt), "decided_at": commercialOptionalTimestamp(v.DecidedAt), "credit_note_id": optionalCommercialID(v.CreditNoteID), "credit_number": commercialOptionalString(v.CreditNumber), "credit_note_state": commercialOptionalString(v.CreditNoteState), "refund_operation_id": optionalCommercialID(v.RefundID), "refund_state": commercialOptionalString(v.RefundState)}
}
func reversalReadView(v commercialapp.PaymentReversalRead) any {
	return fiber.Map{"payment_id": uuid.UUID(v.PaymentID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "subscription_id": uuid.UUID(v.SubscriptionID).String(), "settlement_observation_id": uuid.UUID(v.SettlementObservationID).String(), "reversal_observation_id": uuid.UUID(v.ReversalObservationID).String(), "access_hold_id": uuid.UUID(v.AccessHoldID).String(), "payment_state": v.PaymentState, "invoice_state": v.InvoiceState, "subscription_state": v.SubscriptionState, "access_mode": v.AccessMode, "hold_state": v.HoldState, "amount": moneyView(v.AmountMinorUnits), "provider_observed_at": commercialOptionalTimestamp(v.ProviderObservedAt), "recorded_at": commercialTimestamp(v.RecordedAt), "applied_at": commercialTimestamp(v.AppliedAt)}
}
func platformInvoiceView(v commercialapp.PlatformInvoiceQueueItem) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "subscription_id": uuid.UUID(v.SubscriptionID).String(), "invoice_number": v.Number, "state": v.State, "version": v.Version, "purchase_kind": v.PurchaseKind, "total": moneyView(v.TotalMinorUnits), "outstanding": moneyView(v.OutstandingMinorUnits), "issued_at": commercialTimestamp(v.IssuedAt), "due_at": commercialTimestamp(v.DueAt)}
}
func platformPaymentView(v commercialapp.PlatformPaymentQueueItem) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "state": v.State, "version": v.Version, "amount": moneyView(v.AmountMinorUnits), "reconciliation_state": v.ReconciliationState, "reconciliation_pending": v.ReconciliationPending, "created_at": commercialTimestamp(v.CreatedAt), "updated_at": commercialTimestamp(v.UpdatedAt), "reconciled_at": commercialOptionalTimestamp(v.ReconciledAt)}
}
func platformAdjustmentView(v commercialapp.PlatformAdjustmentQueueItem) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "payment_id": uuid.UUID(v.PaymentID).String(), "maker_user_id": uuid.UUID(v.MakerUserID).String(), "checker_user_id": optionalCommercialID(v.CheckerUserID), "action": v.Action, "state": v.State, "reason_code": v.ReasonCode, "entitlement_disposition": v.EntitlementDisposition, "version": v.Version, "amount": moneyView(v.AmountMinorUnits), "net": moneyView(v.NetMinorUnits), "tax": moneyView(v.TaxMinorUnits), "created_at": commercialTimestamp(v.CreatedAt), "expires_at": commercialTimestamp(v.ExpiresAt), "decided_at": commercialOptionalTimestamp(v.DecidedAt)}
}
func platformReversalView(v commercialapp.PlatformReversalQueueItem) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "payment_id": uuid.UUID(v.PaymentID).String(), "tenant_id": uuid.UUID(v.TenantID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "actor_user_id": uuid.UUID(v.ActorUserID).String(), "expected_payment_version": v.ExpectedPaymentVersion, "payment_state": v.PaymentState, "reconciliation_state": v.ReconciliationState, "queue_state": v.QueueState, "amount": moneyView(v.AmountMinorUnits), "requested_at": commercialTimestamp(v.RequestedAt), "applied_at": commercialOptionalTimestamp(v.AppliedAt), "hold_state": commercialOptionalString(v.HoldState)}
}
func commercialPlatformFailure(c fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, commercialapp.ErrAdjustmentRequestInvalid), errors.Is(e, commercialapp.ErrReversalRequestInvalid), errors.Is(e, commercialapp.ErrFinancialReadInvalid), errors.Is(e, commercialapp.ErrPlatformFinancialQueueInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(e, commercialapp.ErrAdjustmentNotPermitted), errors.Is(e, commercialapp.ErrReversalNotPermitted), errors.Is(e, commercialapp.ErrFinancialReadNotAuthorized), errors.Is(e, commercialapp.ErrPlatformFinancialQueueNotAuthorized):
		return fail(c, 403, "FORBIDDEN", "The operation is not permitted.", false)
	case errors.Is(e, commercialapp.ErrAdjustmentNotFound), errors.Is(e, commercialapp.ErrReversalNotFound), errors.Is(e, commercialapp.ErrFinancialReadNotFound):
		return fail(c, 404, "NOT_FOUND", "The financial record was not found.", false)
	case errors.Is(e, commercialapp.ErrAdjustmentApprovalRequired), errors.Is(e, commercialapp.ErrAdjustmentConflict), errors.Is(e, commercialapp.ErrReversalConflict), errors.Is(e, commercialapp.ErrReversalNotReady):
		return fail(c, 409, "COMMERCIAL_CONFLICT", "The commercial record changed or is not ready.", false)
	default:
		return fail(c, 503, "COMMERCIAL_UNAVAILABLE", "Commercial service is temporarily unavailable.", true)
	}
}
