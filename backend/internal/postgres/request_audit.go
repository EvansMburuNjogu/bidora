package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

type requestAuditKey struct{}

// RequestAudit contains validated, reduced HTTP metadata, never request bodies,
// cookies, credentials or the raw User-Agent. It is not an authorization signal.
type RequestAudit struct {
	IP      netip.Addr
	Browser string
}

func (RequestAudit) Format(s fmt.State, _ rune) { fmt.Fprint(s, "[REDACTED audit request metadata]") }
func WithRequestAudit(ctx context.Context, ip netip.Addr, browser string) (context.Context, error) {
	if ctx == nil || !ip.IsValid() || ip.IsUnspecified() || ip.IsMulticast() || ip.Zone() != "" || browser == "" || len(browser) > 40 || strings.IndexFunc(browser, func(r rune) bool { return r < 32 || r > 126 }) >= 0 {
		return nil, errors.New("audit_request_context_invalid")
	}
	return context.WithValue(ctx, requestAuditKey{}, RequestAudit{IP: ip.Unmap(), Browser: browser}), nil
}
func RequestAuditFrom(ctx context.Context) (RequestAudit, bool) {
	if ctx == nil {
		return RequestAudit{}, false
	}
	value, ok := ctx.Value(requestAuditKey{}).(RequestAudit)
	return value, ok
}
func applyRequestAudit(ctx context.Context, db DBTX) error {
	metadata, ok := RequestAuditFrom(ctx)
	if !ok {
		return nil
	}
	// All values are transaction-local: commit/rollback clears them before the
	// connection returns to the pool. The audit INSERT trigger captures atomically.
	_, err := db.Exec(ctx, `SELECT pg_catalog.set_config('bidora.audit_client_ip',$1,true),pg_catalog.set_config('bidora.audit_browser',$2,true),pg_catalog.set_config('bidora.audit_required','1',true)`, metadata.IP.String(), metadata.Browser)
	if err != nil {
		return safeError("postgres_audit_context_failed", err)
	}
	return nil
}
