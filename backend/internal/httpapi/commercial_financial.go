package httpapi

import (
	commercialapp "bidora/backend/internal/commercial/application"
	"bidora/backend/internal/commercial/domain"
	"bidora/backend/internal/identity/assurance"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"net/url"
	"strconv"
	"time"
)

type CommercialFinancialUseCase interface {
	IssueInvoice(context.Context, commercialapp.IssueInvoiceInput) (domain.Invoice, error)
	ReadInvoice(context.Context, commercialapp.SubscriptionAccess, domain.ID) (domain.Invoice, error)
	ListInvoices(context.Context, commercialapp.FinancialListRequest) (commercialapp.InvoiceSummaryPage, error)
	InitiatePayment(context.Context, commercialapp.InitiatePaymentInput) (commercialapp.PaymentInitiationResult, error)
	ReadPaymentFinancialState(context.Context, commercialapp.SubscriptionAccess, domain.ID) (commercialapp.TenantPaymentRead, error)
	ListPayments(context.Context, commercialapp.FinancialListRequest) (commercialapp.PaymentSummaryPage, error)
	ReadReceipt(context.Context, commercialapp.SubscriptionAccess, domain.ID) (domain.Receipt, error)
	ReadCreditNote(context.Context, commercialapp.SubscriptionAccess, domain.ID) (domain.CreditNote, error)
	ListFinancialHistory(context.Context, commercialapp.FinancialListRequest) (commercialapp.FinancialHistoryPage, error)
}
type CommercialFinancialHandler struct{ usecase CommercialFinancialUseCase }

func NewCommercialFinancialHandler(u CommercialFinancialUseCase) (*CommercialFinancialHandler, error) {
	if u == nil {
		return nil, errors.New("commercial_financial_handler_invalid")
	}
	return &CommercialFinancialHandler{u}, nil
}
func RegisterCommercialFinancialRoutes(app *fiber.App, browser *BrowserSessionMiddleware, h *CommercialFinancialHandler) error {
	if err := RegisterCommercialFinancialReadRoutes(app, browser, h); err != nil {
		return err
	}
	g := app.Group("/v1/tenderpreneur/commercial", browser.Platform)
	g.Post("/invoices", h.issueInvoice)
	g.Post("/invoices/:invoice_id/payments", h.initiatePayment)
	return nil
}

// RegisterCommercialFinancialReadRoutes mounts only tenant-scoped redacted reads.
// Invoice issuance and payment-provider operations remain independently gated.
func RegisterCommercialFinancialReadRoutes(app *fiber.App, browser *BrowserSessionMiddleware, h *CommercialFinancialHandler) error {
	if app == nil || browser == nil || h == nil {
		return errors.New("commercial_financial_routes_invalid")
	}
	g := app.Group("/v1/tenderpreneur/commercial", browser.Platform)
	g.Get("/invoices", h.listInvoices)
	g.Get("/invoices/:invoice_id", h.invoice)
	g.Get("/payments", h.listPayments)
	g.Get("/payments/:payment_id", h.payment)
	g.Get("/receipts/:receipt_id", h.receipt)
	g.Get("/credit-notes/:credit_note_id", h.creditNote)
	g.Get("/financial-history", h.history)
	return nil
}

type issueInvoiceBody struct {
	SubscriptionID              string `json:"subscription_id"`
	PurchaseOperationID         string `json:"purchase_operation_id"`
	PurchaseKind                string `json:"purchase_kind"`
	PlanKey                     string `json:"plan_key"`
	Cadence                     string `json:"cadence"`
	Quantity                    int64  `json:"quantity"`
	ExpectedSubscriptionVersion int64  `json:"expected_subscription_version"`
}
type initiatePaymentBody struct {
	PayerEmail             string `json:"payer_email"`
	PayerCountry           string `json:"payer_country"`
	ExpectedInvoiceVersion int64  `json:"expected_invoice_version"`
}

func (h *CommercialFinancialHandler) issueInvoice(c fiber.Ctx) error {
	var b issueInvoiceBody
	if e := bindCommercialJSON(c, &b, 8192, "subscription_id", "purchase_operation_id", "purchase_kind", "plan_key", "cadence", "quantity", "expected_subscription_version"); e != nil {
		return e
	}
	access, ok := commercialAccess(c)
	if !ok {
		return nil
	}
	subscription, ok := commercialID(b.SubscriptionID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	operation, ok := commercialID(b.PurchaseOperationID)
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	key, correlation, ok := commercialMutationIDs(c)
	if !ok {
		return nil
	}
	v, e := h.usecase.IssueInvoice(c.Context(), commercialapp.IssueInvoiceInput{Access: access, Purchase: commercialapp.PurchaseRequest{TenantID: access.TenantID, SubscriptionID: subscription, OperationID: operation, Kind: b.PurchaseKind, PlanKey: domain.PlanKey(b.PlanKey), Cadence: domain.BillingCadence(b.Cadence), Quantity: b.Quantity}, ExpectedSubscriptionVersion: b.ExpectedSubscriptionVersion, IdempotencyKey: key, CorrelationID: correlation})
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	return commercialJSON(c, 201, invoiceView(v), nil)
}
func (h *CommercialFinancialHandler) invoice(c fiber.Ctx) error {
	access, id, ok := tenantCommercialRead(c, "invoice_id")
	if !ok {
		return nil
	}
	v, e := h.usecase.ReadInvoice(c.Context(), access, id)
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	return commercialJSON(c, 200, invoiceView(v), nil)
}
func (h *CommercialFinancialHandler) listInvoices(c fiber.Ctx) error {
	access, page, ok := tenantCommercialPage(c, false)
	if !ok {
		return nil
	}
	v, e := h.usecase.ListInvoices(c.Context(), commercialapp.FinancialListRequest{Access: access, Limit: page.limit, Cursor: page.cursor})
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	items := make([]any, len(v.Items))
	for i, x := range v.Items {
		items[i] = invoiceSummaryView(x)
	}
	return commercialJSON(c, 200, items, v.NextCursor)
}

// RegisterCommercialPaymentInitiationRoute reuses the existing payment intent
// use case without activating unrelated invoice/financial write surfaces.
func RegisterCommercialPaymentInitiationRoute(app *fiber.App, b *BrowserSessionMiddleware, h *CommercialFinancialHandler) {
	app.Post("/v1/tenderpreneur/commercial/invoices/:invoice_id/payments", b.OriginBound, h.initiatePayment)
}

func (h *CommercialFinancialHandler) initiatePayment(c fiber.Ctx) error {
	principal, ok := SessionPrincipalFrom(c)
	if !ok || !assurance.Platform(principal.AssuranceLevel+":"+principal.AssuranceMethod) || principal.AssuredAt.IsZero() || principal.AssuredAt.After(time.Now().UTC()) || time.Since(principal.AssuredAt) > 15*time.Minute {
		return fail(c, 403, "ASSURANCE_REQUIRED", "Recent two-step verification is required.", false)
	}
	var b initiatePaymentBody
	if e := bindCommercialJSON(c, &b, 4096, "expected_invoice_version", "payer_email", "payer_country"); e != nil {
		return e
	}
	access, ok := commercialAccess(c)
	if !ok {
		return nil
	}
	invoice, ok := commercialPathID(c, "invoice_id")
	if !ok {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	key, correlation, ok := commercialMutationIDs(c)
	if !ok {
		return nil
	}
	v, e := h.usecase.InitiatePayment(c.Context(), commercialapp.InitiatePaymentInput{PayerEmail: b.PayerEmail, PayerCountry: b.PayerCountry, Access: access, InvoiceID: invoice, ExpectedInvoiceVersion: b.ExpectedInvoiceVersion, IdempotencyKey: key, CorrelationID: correlation})
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	if v.RedirectURL != "" {
		prepareCommercialResponse(c)
		c.Location(v.RedirectURL)
		return c.SendStatus(303)
	}
	return commercialJSON(c, 202, paymentIntentView(v.Payment), nil)
}
func (h *CommercialFinancialHandler) payment(c fiber.Ctx) error {
	access, id, ok := tenantCommercialRead(c, "payment_id")
	if !ok {
		return nil
	}
	v, e := h.usecase.ReadPaymentFinancialState(c.Context(), access, id)
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	return commercialJSON(c, 200, tenantPaymentView(v), nil)
}
func (h *CommercialFinancialHandler) listPayments(c fiber.Ctx) error {
	access, page, ok := tenantCommercialPage(c, false)
	if !ok {
		return nil
	}
	v, e := h.usecase.ListPayments(c.Context(), commercialapp.FinancialListRequest{Access: access, Limit: page.limit, Cursor: page.cursor})
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	items := make([]any, len(v.Items))
	for i, x := range v.Items {
		items[i] = paymentSummaryView(x)
	}
	return commercialJSON(c, 200, items, v.NextCursor)
}
func (h *CommercialFinancialHandler) receipt(c fiber.Ctx) error {
	access, id, ok := tenantCommercialRead(c, "receipt_id")
	if !ok {
		return nil
	}
	v, e := h.usecase.ReadReceipt(c.Context(), access, id)
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	return commercialJSON(c, 200, receiptView(v), nil)
}
func (h *CommercialFinancialHandler) creditNote(c fiber.Ctx) error {
	access, id, ok := tenantCommercialRead(c, "credit_note_id")
	if !ok {
		return nil
	}
	v, e := h.usecase.ReadCreditNote(c.Context(), access, id)
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	return commercialJSON(c, 200, creditNoteView(v), nil)
}
func (h *CommercialFinancialHandler) history(c fiber.Ctx) error {
	access, page, ok := tenantCommercialPage(c, true)
	if !ok {
		return nil
	}
	v, e := h.usecase.ListFinancialHistory(c.Context(), commercialapp.FinancialListRequest{Access: access, Limit: page.limit, Cursor: page.cursor})
	if e != nil {
		return commercialFinancialFailure(c, e)
	}
	items := make([]any, len(v.Items))
	for i, x := range v.Items {
		items[i] = historyView(x)
	}
	return commercialJSON(c, 200, items, v.NextCursor)
}

type commercialPage struct {
	limit  int32
	cursor *commercialapp.CommercialListCursor
}

func tenantCommercialPage(c fiber.Ctx, history bool) (commercialapp.SubscriptionAccess, commercialPage, bool) {
	if len(c.Body()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return commercialapp.SubscriptionAccess{}, commercialPage{}, false
	}
	access, ok := commercialAccess(c)
	if !ok {
		return access, commercialPage{}, false
	}
	page, ok := parseCommercialPage(c, history)
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return access, page, false
	}
	return access, page, true
}
func tenantCommercialRead(c fiber.Ctx, name string) (commercialapp.SubscriptionAccess, domain.ID, bool) {
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
		return commercialapp.SubscriptionAccess{}, domain.ID{}, false
	}
	access, ok := commercialAccess(c)
	if !ok {
		return access, domain.ID{}, false
	}
	id, ok := commercialPathID(c, name)
	if !ok {
		_ = fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	return access, id, ok
}
func commercialMutationIDs(c fiber.Ctx) (string, domain.ID, bool) {
	raw, ok := singleHeader(c, IdempotencyKeyHeader)
	if !ok {
		_ = invalidHeader(c, IdempotencyKeyHeader, "required")
		return "", domain.ID{}, false
	}
	if _, e := uuid.Parse(raw); e != nil {
		_ = invalidHeader(c, IdempotencyKeyHeader, "uuid")
		return "", domain.ID{}, false
	}
	correlation, ok := parseIdentityID(c.GetRespHeader("X-Request-ID"))
	if !ok {
		_ = fail(c, 500, "INTERNAL_ERROR", "An unexpected error occurred.", false)
		return "", domain.ID{}, false
	}
	return raw, domain.ID(correlation), true
}
func commercialID(raw string) (domain.ID, bool) {
	v, e := uuid.Parse(raw)
	return domain.ID(v), e == nil && v != uuid.Nil
}
func commercialPathID(c fiber.Ctx, name string) (domain.ID, bool) {
	return commercialID(c.Params(name))
}
func bindCommercialJSON(c fiber.Ctx, target any, max int, names ...string) error {
	if len(c.Body()) > max {
		return fail(c, 413, "REQUEST_TOO_LARGE", "The request is too large.", false)
	}
	if e := bindOrganization(c, target); e != nil {
		return e
	}
	if !requireOrganizationFields(c.Body(), names...) {
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	}
	return nil
}
func parseCommercialPage(c fiber.Ctx, history bool) (commercialPage, bool) {
	values, e := url.ParseQuery(string(c.Request().URI().QueryString()))
	if e != nil {
		return commercialPage{}, false
	}
	for k, v := range values {
		if (k != "limit" && k != "cursor") || len(v) != 1 {
			return commercialPage{}, false
		}
	}
	limit := int32(25)
	if raw := values.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			return commercialPage{}, false
		}
		limit = int32(n)
	}
	page := commercialPage{limit: limit}
	if raw := values.Get("cursor"); raw != "" {
		decoded, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil || len(decoded) > 384 {
			return page, false
		}
		var w struct{ At, ID, Kind string }
		if json.Unmarshal(decoded, &w) != nil {
			return page, false
		}
		at, e := time.Parse(time.RFC3339Nano, w.At)
		id, ok := commercialID(w.ID)
		if e != nil || !ok || history == (w.Kind == "") {
			return page, false
		}
		page.cursor = &commercialapp.CommercialListCursor{OccurredAt: at.UTC(), Kind: w.Kind, ID: id}
	}
	return page, true
}
func encodeCommercialCursor(v *commercialapp.CommercialListCursor) (*string, error) {
	if v == nil {
		return nil, nil
	}
	raw, e := json.Marshal(struct{ At, ID, Kind string }{v.OccurredAt.UTC().Format(time.RFC3339Nano), uuid.UUID(v.ID).String(), v.Kind})
	if e != nil {
		return nil, e
	}
	x := base64.RawURLEncoding.EncodeToString(raw)
	return &x, nil
}
func commercialJSON(c fiber.Ctx, status int, data any, next *commercialapp.CommercialListCursor) error {
	cursor, e := encodeCommercialCursor(next)
	if e != nil {
		return fail(c, 503, "COMMERCIAL_UNAVAILABLE", "Commercial service is temporarily unavailable.", true)
	}
	prepareCommercialResponse(c)
	return c.Status(status).JSON(fiber.Map{"data": data, "meta": fiber.Map{"trace_id": c.GetRespHeader("X-Request-ID"), "next_cursor": cursor}})
}
func invoiceView(v domain.Invoice) any {
	lines := make([]any, len(v.Lines))
	for i, x := range v.Lines {
		lines[i] = fiber.Map{"line_number": x.Number, "line_kind": x.Kind, "description_code": x.DescriptionCode, "quantity": x.Quantity, "unit_amount": moneyView(x.UnitMinorUnits), "net_amount": moneyView(x.NetMinorUnits), "tax_code": x.TaxCode, "tax_amount": moneyView(x.TaxMinorUnits), "gross_amount": moneyView(x.GrossMinorUnits)}
	}
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "invoice_number": v.Number, "state": v.State, "version": v.Version, "purchase_kind": v.PurchaseKind, "plan_key": v.PlanKey, "catalogue_version": v.CatalogueVersion, "cadence": v.Cadence, "quantity": v.Quantity, "term_start": commercialTimestamp(v.TermStart), "term_end": commercialTimestamp(v.TermEnd), "arithmetic_profile": v.ArithmeticProfile, "subtotal": moneyView(v.SubtotalMinorUnits), "credit": moneyView(v.CreditMinorUnits), "taxable": moneyView(v.TaxableMinorUnits), "tax": moneyView(v.TaxMinorUnits), "total": moneyView(v.TotalMinorUnits), "outstanding": moneyView(v.OutstandingMinorUnits), "issued_at": commercialTimestamp(v.IssuedAt), "due_at": commercialTimestamp(v.DueAt), "lines": lines, "payment_initiated": v.State != "ISSUED" && v.State != "VOID", "entitlements_activated": v.State == "PAID" || v.State == "PARTIALLY_CREDITED" || v.State == "CREDITED"}
}
func invoiceSummaryView(v commercialapp.InvoiceSummary) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "subscription_id": uuid.UUID(v.SubscriptionID).String(), "invoice_number": v.Number, "state": v.State, "version": v.Version, "purchase_kind": v.PurchaseKind, "plan_key": v.PlanKey, "catalogue_version": v.CatalogueVersion, "cadence": v.Cadence, "quantity": v.Quantity, "subtotal": moneyView(v.SubtotalMinorUnits), "tax": moneyView(v.TaxMinorUnits), "total": moneyView(v.TotalMinorUnits), "outstanding": moneyView(v.OutstandingMinorUnits), "issued_at": commercialTimestamp(v.IssuedAt), "due_at": commercialTimestamp(v.DueAt)}
}
func paymentIntentView(v domain.Payment) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "state": v.State, "version": v.Version, "amount": moneyView(v.AmountMinorUnits), "reconciliation_state": v.ReconciliationState, "paid": v.State == "SETTLED", "entitlements_activated": false, "receipt_id": nil, "entitlement_effect": nil, "updated_at": commercialTimestamp(v.UpdatedAt)}
}
func tenantPaymentView(v commercialapp.TenantPaymentRead) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "state": v.State, "version": v.Version, "amount": moneyView(v.AmountMinorUnits), "reconciliation_state": v.ReconciliationState, "paid": v.Paid, "entitlements_activated": v.EntitlementsActivated, "receipt_id": optionalCommercialID(v.ReceiptID), "entitlement_effect": commercialOptionalString(v.EntitlementEffect), "updated_at": commercialTimestamp(v.UpdatedAt)}
}
func paymentSummaryView(v commercialapp.PaymentSummary) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "state": v.State, "version": v.Version, "amount": moneyView(v.AmountMinorUnits), "reconciliation_pending": v.ReconciliationPending, "created_at": commercialTimestamp(v.CreatedAt), "updated_at": commercialTimestamp(v.UpdatedAt), "reconciled_at": commercialOptionalTimestamp(v.ReconciledAt)}
}
func receiptView(v domain.Receipt) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "payment_id": uuid.UUID(v.PaymentID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "receipt_number": v.Number, "amount": moneyView(v.AmountMinorUnits), "issued_at": commercialTimestamp(v.IssuedAt), "content_hash": hex.EncodeToString(v.ContentHash[:])}
}
func creditNoteView(v domain.CreditNote) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "invoice_id": uuid.UUID(v.InvoiceID).String(), "credit_number": v.Number, "state": v.State, "version": v.Version, "reason_code": v.ReasonCode, "net": moneyView(v.NetMinorUnits), "tax": moneyView(v.TaxMinorUnits), "total": moneyView(v.TotalMinorUnits), "issued_at": commercialTimestamp(v.IssuedAt), "content_hash": hex.EncodeToString(v.ContentHash[:])}
}
func historyView(v commercialapp.FinancialHistoryItem) any {
	return fiber.Map{"id": uuid.UUID(v.ID).String(), "kind": v.Kind, "invoice_id": uuid.UUID(v.InvoiceID).String(), "payment_id": optionalCommercialID(v.PaymentID), "document_number": commercialOptionalString(v.DocumentNumber), "state": v.State, "amount": moneyView(v.AmountMinorUnits), "occurred_at": commercialTimestamp(v.OccurredAt)}
}
func moneyView(v int64) any {
	return fiber.Map{"currency": "KES", "amount_minor_units": v, "amount": commercialAmount(v)}
}
func optionalCommercialID(v domain.ID) any {
	if v.IsZero() {
		return nil
	}
	return uuid.UUID(v).String()
}
func commercialOptionalString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func commercialFinancialFailure(c fiber.Ctx, e error) error {
	switch {
	case errors.Is(e, commercialapp.ErrInvoiceRequestInvalid), errors.Is(e, commercialapp.ErrPaymentRequestInvalid), errors.Is(e, commercialapp.ErrFinancialReadInvalid), errors.Is(e, commercialapp.ErrFinancialListInvalid):
		return fail(c, 400, "INVALID_REQUEST", "The request is invalid.", false)
	case errors.Is(e, commercialapp.ErrInvoiceNotAuthorized), errors.Is(e, commercialapp.ErrPaymentNotAuthorized), errors.Is(e, commercialapp.ErrFinancialReadNotAuthorized), errors.Is(e, commercialapp.ErrFinancialListNotAuthorized):
		return fail(c, 403, "FORBIDDEN", "The operation is not permitted.", false)
	case errors.Is(e, commercialapp.ErrInvoiceNotFound), errors.Is(e, commercialapp.ErrPaymentNotFound), errors.Is(e, commercialapp.ErrFinancialReadNotFound):
		return fail(c, 404, "NOT_FOUND", "The financial record was not found.", false)
	case errors.Is(e, commercialapp.ErrInvoiceConflict), errors.Is(e, commercialapp.ErrInvoiceQuoteStale), errors.Is(e, commercialapp.ErrPaymentConflict), errors.Is(e, commercialapp.ErrPaymentInProgress), errors.Is(e, commercialapp.ErrPaymentReconciliationRequired), errors.Is(e, commercialapp.ErrPaymentAmountConflict):
		return fail(c, 409, "COMMERCIAL_CONFLICT", "The commercial record changed or requires reconciliation.", false)
	case errors.Is(e, commercialapp.ErrInvoiceTaxUnavailable):
		return fail(c, 503, "COMMERCIAL_TAX_PROFILE_UNAVAILABLE", "A reviewed tax profile is unavailable.", false)
	default:
		return fail(c, 503, "COMMERCIAL_UNAVAILABLE", "Commercial service is temporarily unavailable.", true)
	}
}
