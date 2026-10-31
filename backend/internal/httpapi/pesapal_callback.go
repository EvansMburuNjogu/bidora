package httpapi

import (
	"bidora/backend/internal/commercial/domain"
	"context"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"sync"
	"time"
)

const PesaPalIPNPath = "/v1/shared/payments/pesapal/ipn"

type PesaPalHintReceiver interface {
	Receive(context.Context, domain.ID, domain.ID, domain.ID) error
}
type PesaPalCallbackHandler struct {
	receiver PesaPalHintReceiver
	clock    BrowserClock
	mu       sync.Mutex
	window   time.Time
	count    int
}

func NewPesaPalCallbackHandler(r PesaPalHintReceiver, clock BrowserClock) *PesaPalCallbackHandler {
	return &PesaPalCallbackHandler{receiver: r, clock: clock}
}
func RegisterPesaPalCallbackRoute(f *fiber.App, h *PesaPalCallbackHandler) {
	f.Get(PesaPalIPNPath, h.receive)
}
func (h *PesaPalCallbackHandler) receive(c fiber.Ctx) error {
	c.Set("Cache-Control", "private, no-store")
	if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) > 512 {
		return fail(c, 400, "INVALID_REQUEST", "The callback hint is invalid.", false)
	}
	values, e := parseStrictQuery(c, "OrderTrackingId", "OrderMerchantReference", "OrderNotificationType")
	if e != nil || values.Get("OrderNotificationType") != "" && values.Get("OrderNotificationType") != "IPNCHANGE" {
		return fail(c, 400, "INVALID_REQUEST", "The callback hint is invalid.", false)
	}
	tracking, e := uuid.Parse(values.Get("OrderTrackingId"))
	merchant, e2 := uuid.Parse(values.Get("OrderMerchantReference"))
	correlation, e3 := uuid.Parse(c.GetRespHeader("X-Request-ID"))
	if e != nil || e2 != nil || e3 != nil || tracking == uuid.Nil || merchant == uuid.Nil || correlation == uuid.Nil {
		return fail(c, 400, "INVALID_REQUEST", "The callback hint is invalid.", false)
	}
	at := h.clock.Now().UTC()
	h.mu.Lock()
	if h.window.IsZero() || !at.Before(h.window.Add(time.Minute)) {
		h.window = at
		h.count = 0
	}
	allowed := h.count < 120
	if allowed {
		h.count++
	}
	h.mu.Unlock()
	if !allowed {
		c.Set("Retry-After", "60")
		return fail(c, 429, "CALLBACK_RATE_LIMITED", "Try again later.", true)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	if e = h.receiver.Receive(ctx, domain.ID(merchant), domain.ID(tracking), domain.ID(correlation)); e != nil {
		return fail(c, 503, "PAYMENT_RECONCILIATION_UNAVAILABLE", "The callback hint could not be recorded.", true)
	} // Public receipt is deliberately not financial authority.
	return commercialJSON(c, 202, fiber.Map{"received": true, "authoritative_payment_status": false}, nil)
}
