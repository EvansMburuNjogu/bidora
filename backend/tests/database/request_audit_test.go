package database_test

import (
	"strings"
	"testing"
)

func TestRequestMetadataTriggerIsAtomicScopedAndFailClosed(t *testing.T) {
	migration := read(t, "migrations/control/00051_atomic_request_audit_metadata.sql")
	for _, required := range []string{
		"AFTER INSERT ON bidora_control_audit.audit_events",
		"SECURITY DEFINER SET search_path=pg_catalog,pg_temp",
		"REVOKE ALL ON FUNCTION bidora_control_audit.capture_request_metadata() FROM PUBLIC",
		"NEW.tenant_id IS NOT NULL OR NEW.actor_id IS NULL",
		"account_type='PLATFORM_ADMIN'",
		"current_setting('bidora.audit_required',true)",
		"INSERT INTO bidora_control_audit.request_metadata",
		"VALUES(NEW.correlation_id,NEW.actor_id,client_ip::inet,client_browser,NEW.occurred_at)",
		"RAISE EXCEPTION 'audit request metadata unavailable'",
		"RAISE EXCEPTION 'audit request metadata conflict'",
	} {
		if !strings.Contains(migration, required) {
			t.Fatal("atomic audit boundary missing", required)
		}
	}
	if strings.Contains(migration, "EXCEPTION WHEN") || strings.Contains(migration, "WHEN OTHERS") {
		t.Fatal("metadata failure must not be swallowed")
	}
	composition := read(t, "internal/pilot/regional_policy_composition.go")
	if !strings.Contains(composition, "ConfigureRequestAudit(ips)") || strings.Contains(composition, "store.ObserveRequest(") {
		t.Fatal("runtime still relies on best-effort post-request observation")
	}
	for _, file := range []string{"internal/httpapi/pilot.go", "internal/httpapi/identity_account.go"} {
		source := read(t, file)
		audit, login := strings.Index(source, "app.Use(\"/v1\", routes.Browser.CaptureAuditContext)"), strings.Index(source, "app.Post(LoginPath")
		if audit < 0 || login < audit {
			t.Fatal("metadata middleware is not before authentication routes", file)
		}
	}
}
