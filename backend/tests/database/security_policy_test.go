package database_test

import (
	"strings"
	"testing"
)

func TestDynamicSecurityMigrationPreservesBoundsAndPrivateTrigger(t *testing.T) {
	migration := read(t, "migrations/control/00052_dynamic_security_policy.sql")
	for _, required := range []string{"schema_version=51", "schema_version=52", "failed_login_attempts BETWEEN 1 AND 5", "minimum_password_bytes BETWEEN 12 AND 128", "password_reset_seconds BETWEEN 300 AND 1800", "FOR UPDATE", "FOR SHARE", "SECURITY DEFINER SET search_path=pg_catalog", "REVOKE ALL ON FUNCTION identity.enforce_single_session_policy() FROM PUBLIC", "epoch<>NEW.auth_epoch", "verified IS NULL", "disabled IS NOT NULL", "NEW.assurance_level='AAL2'", "NEW.idle_timeout_seconds:=LEAST", "NEW.absolute_timeout_seconds:=LEAST", "IF NOT p.allow_multiple_sessions", "dynamic security policy requires forward recovery"} {
		if !strings.Contains(migration, required) {
			t.Fatal("security migration invariant missing", required)
		}
	}
	for _, forbidden := range []string{"DELETE FROM", "DROP TABLE", "DISABLE TRIGGER", "require_mfa=false", "email_verified_at=NULL"} {
		if strings.Contains(migration, forbidden) {
			t.Fatal("security invariant weakened", forbidden)
		}
	}
}
