package database_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bidora/backend/internal/migrationrunner"
)

func backendRoot() string { return filepath.Join("..", "..") }

func TestMigrationAndGenerationInputsMatchReviewedChecksums(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(backendRoot(), "migrations", "checksums.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Status, Algorithm string
		Files             map[string]string
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Status != "pre-release-reviewed-slice" || manifest.Algorithm != "sha256" || len(manifest.Files) != 183 {
		t.Fatal("unexpected manifest scope")
	}
	for name, want := range manifest.Files {
		body, err := os.ReadFile(filepath.Join(backendRoot(), filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Fatalf("review required: %s checksum %s", name, got)
		}
	}
}

func TestMigrationsKeepCaptureBoundedAndRecoverable(t *testing.T) {
	common := read(t, "migrations/tenant_common/00001_command_evidence.sql")
	tenant := read(t, "migrations/tenant_tenderpreneur/00001_unverified_opportunity_capture.sql")
	for name, sql := range map[string]string{"common": common, "tenant": tenant} {
		up, down, ok := strings.Cut(sql, "-- +goose Down")
		if !ok {
			t.Fatal("missing down boundary", name)
		}
		for _, required := range []string{"-- +goose Up", "SET lock_timeout = '1s'", "SET statement_timeout = '5s'"} {
			if !strings.Contains(up, required) {
				t.Fatal("missing migration safeguard", name, required)
			}
		}
		if strings.Contains(strings.ToUpper(up), " DROP ") || strings.Contains(strings.ToUpper(up), "DROP TABLE") || strings.Contains(strings.ToUpper(up), "DROP SCHEMA") {
			t.Fatal("destructive up migration", name)
		}
		if !strings.Contains(down, "Never auto-run Down") || (!strings.Contains(down, "DROP SCHEMA") && !strings.Contains(down, "DELETE FROM")) {
			t.Fatal("recovery warning/down missing", name)
		}
	}
	for _, required := range []string{"audit_events_append_only", "outbox_event_facts_immutable", "octet_length(key_hash) = 32", "octet_length(request_hash) = 32", "UNIQUE (actor_id, operation, key_hash)", "No automatic expiry"} {
		if !strings.Contains(common, required) {
			t.Fatal("common invariant missing", required)
		}
	}
	if strings.Contains(common, "expires_at") {
		t.Fatal("approved no-TTL idempotency policy drifted")
	}
	for _, required := range []string{"verification_state = 'UNVERIFIED'", "source_versions_immutable", "UNIQUE (opportunity_id, revision)", "FOREIGN KEY (id, current_source_version_id)", "octet_length(request_hash) = 32", "No document FK is added until BID-DB-005"} {
		if !strings.Contains(tenant, required) {
			t.Fatal("capture invariant missing", required)
		}
	}
	for _, forbidden := range []string{"bid_workspaces", "bid_requirements", "approval_decisions", "submission_packages"} {
		if strings.Contains(tenant, forbidden) {
			t.Fatal("unapproved schema scope", forbidden)
		}
	}
}

func TestControlRegistrySchemaAndQueriesStayServerOwned(t *testing.T) {
	migration := read(t, "migrations/control/00001_tenant_registry.sql")
	up, down, ok := strings.Cut(migration, "-- +goose Down")
	if !ok || !strings.Contains(up, "SET lock_timeout = '1s'") || !strings.Contains(up, "SET statement_timeout = '5s'") || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("control migration safeguards missing")
	}
	for _, required := range []string{
		"CREATE TABLE bidora_tenancy.tenants", "tenant_type = 'TENDERPRENEUR'",
		"CREATE TABLE bidora_tenancy.database_clusters", "endpoint_ref text NOT NULL",
		"CREATE TABLE bidora_tenancy.tenant_database_assignments", "secret_ref text NOT NULL",
		"tenant_database_assignments_one_active_idx", "CREATE TABLE bidora_tenancy.tenant_schema_versions",
		"CREATE TABLE bidora_control_meta.schema_compatibility",
	} {
		if !strings.Contains(up, required) {
			t.Fatal("control invariant missing", required)
		}
	}
	for _, forbidden := range []string{"CREATE TABLE identity.password_credentials", "CREATE TABLE identity.external_identities", "CREATE TABLE bidora_tenancy.memberships", "CREATE TABLE bidora_tenancy.role_grants", "CREATE ROLE", "CREATE DATABASE"} {
		if strings.Contains(up, forbidden) {
			t.Fatal("control scope crossed unresolved policy", forbidden)
		}
	}
	queries := read(t, "queries/control/registry.sql")
	for _, required := range []string{
		"GetControlRegistryCompatibility", "GetActiveTenantDatabaseAssignment",
		"a.generation = t.assignment_generation", "c.state = 'ACTIVE'", "t.state = 'ACTIVE'",
		"s.migration_set = 'tenant_common'", "s.current_version >= 5", "s.migration_set = 'tenant_tenderpreneur'",
		"s.state = 'COMPATIBLE'", "s.generation = a.generation",
	} {
		if !strings.Contains(queries, required) {
			t.Fatal("safe routing guard missing", required)
		}
	}
	if strings.Contains(queries, "SELECT *") {
		t.Fatal("registry query has unstable private fields")
	}
}

func TestControlAuditAndOutboxFactsAreImmutable(t *testing.T) {
	sql := read(t, "migrations/control/00002_audit_outbox.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(up, "SET lock_timeout = '1s'") || !strings.Contains(up, "SET statement_timeout = '5s'") || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("control evidence migration safeguards missing")
	}
	for _, required := range []string{
		"CREATE TABLE bidora_control_audit.audit_events", "control_audit_events_append_only",
		"CREATE TABLE bidora_control_eventing.outbox_events", "control_outbox_event_facts_immutable",
		"tenant_id uuid REFERENCES bidora_tenancy.tenants", "NEW.payload IS DISTINCT FROM OLD.payload",
		"SET schema_version = 2", "affected <> 1",
	} {
		if !strings.Contains(up, required) {
			t.Fatal("control evidence invariant missing", required)
		}
	}
	for _, forbidden := range []string{"token", "password", "document_bytes", "pricing", "CREATE TABLE bidora_control_commands.idempotency_records"} {
		if strings.Contains(up, forbidden) {
			t.Fatal("private or unapproved control evidence field", forbidden)
		}
	}
}

func TestOptionATableCatalogIsUniqueAndGated(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(backendRoot(), "schema", "option-a-tables.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		ReleaseScope string                                                    `json:"release_scope"`
		Status       string                                                    `json:"status"`
		Databases    map[string][]struct{ Schema, Table, Status, Gate string } `json:"databases"`
		Excluded     []string                                                  `json:"excluded_future_release_databases"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.ReleaseScope == "" || catalog.Status != "inventory-not-blanket-schema-approval" || len(catalog.Databases) != 2 || len(catalog.Excluded) != 3 {
		t.Fatal("catalog scope drifted")
	}
	seen := map[string]bool{}
	count := 0
	implemented := 0
	blocked := 0
	derived := 0
	for database, tables := range catalog.Databases {
		if len(tables) == 0 {
			t.Fatal("empty database inventory", database)
		}
		for _, table := range tables {
			key := database + "/" + table.Schema + "/" + table.Table
			if table.Schema == "" || table.Table == "" || table.Gate == "" || seen[key] {
				t.Fatal("invalid/duplicate catalog entry", key)
			}
			seen[key] = true
			count++
			switch table.Status {
			case "implemented_not_db_verified", "implemented_not_verified":
				implemented++
			case "blocked_policy", "planned_dependency":
				blocked++
			case "superseded_no_table", "derived_no_table":
				derived++
			default:
				t.Fatal("unknown catalog status", table.Status)
			}
		}
	}
	if count != 95 || implemented != 78 || blocked != 10 || derived != 7 {
		t.Fatal("Option A inventory unexpectedly incomplete", count, implemented, blocked, derived)
	}
}

func TestTenantIdentityProjectionIsNonAuthoritativeAndSingleton(t *testing.T) {
	sql := read(t, "migrations/tenant_common/00003_organization_access_projection.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(up, "SET lock_timeout = '1s'") || !strings.Contains(up, "SET statement_timeout = '5s'") || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("projection migration safeguards missing")
	}
	for _, required := range []string{
		"CREATE TABLE bidora_tenant.organization_profile", "singleton_key boolean PRIMARY KEY", "tenant_registry_id uuid NOT NULL UNIQUE",
		"CREATE TABLE bidora_tenant.user_profiles", "identity_id uuid NOT NULL UNIQUE", "source_membership_id uuid NOT NULL UNIQUE",
		"projection_version bigint NOT NULL", "SET schema_version = 3", "affected <> 1",
	} {
		if !strings.Contains(up, required) {
			t.Fatal("projection invariant missing", required)
		}
	}
	for _, forbidden := range []string{"role_grants", "capability", "password", "session", "REFERENCES bidora_tenant.user_profiles"} {
		if strings.Contains(up, forbidden) {
			t.Fatal("projection silently became authority", forbidden)
		}
	}
}

func TestTenantInboxIsAppendOnlyAndPayloadFree(t *testing.T) {
	sql := read(t, "migrations/tenant_common/00004_event_inbox.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(up, "SET lock_timeout = '1s'") || !strings.Contains(up, "SET statement_timeout = '5s'") || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("inbox migration safeguards missing")
	}
	for _, required := range []string{
		"CREATE TABLE bidora_eventing.inbox_events", "UNIQUE (consumer, event_id)",
		"inbox_events_append_only", "No automatic expiry", "SET schema_version = 4", "affected <> 1",
	} {
		if !strings.Contains(up, required) {
			t.Fatal("inbox invariant missing", required)
		}
	}
	for _, forbidden := range []string{" payload ", "document", "pricing", "token", "DELETE FROM bidora_eventing.inbox_events"} {
		if strings.Contains(strings.ToLower(up), strings.ToLower(forbidden)) {
			t.Fatal("inbox stores private/unapproved data", forbidden)
		}
	}
	query := read(t, "queries/eventing/inbox.sql")
	if !strings.Contains(query, "-- name: ClaimInboxEvent :execrows") || !strings.Contains(query, "ON CONFLICT (consumer, event_id) DO NOTHING") || strings.Contains(query, "payload") {
		t.Fatal("inbox deduplication query drifted")
	}
}

func TestOutboxActorProvenanceIsAdditiveImmutableAndGated(t *testing.T) {
	sql := read(t, "migrations/tenant_common/00005_outbox_actor_provenance.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("actor provenance migration recovery guard missing")
	}
	for _, required := range []string{"SET lock_timeout = '1s'", "ADD COLUMN actor_id uuid", "CHECK (actor_id IS NOT NULL) NOT VALID", "NEW.actor_id IS DISTINCT FROM OLD.actor_id", "SET schema_version = 5", "schema_version = 4", "Pre-v5 rows require reconciliation before dispatch"} {
		if !strings.Contains(up, required) {
			t.Fatal("actor provenance invariant missing", required)
		}
	}
	if strings.Contains(up, "DROP ") {
		t.Fatal("actor provenance up migration is destructive")
	}
	query := read(t, "queries/opportunity/capture.sql")
	if !strings.Contains(query, "id, actor_id, aggregate_type") || !strings.Contains(query, "sqlc.arg(id), sqlc.arg(actor_id)") {
		t.Fatal("capture outbox omitted actor provenance")
	}
}

func TestCompatibilityMigrationsAreBoundedAndOrdered(t *testing.T) {
	common := read(t, "migrations/tenant_common/00002_schema_compatibility.sql")
	tenant := read(t, "migrations/tenant_tenderpreneur/00002_schema_compatibility.sql")
	for name, sql := range map[string]string{"common": common, "tenant": tenant} {
		up, _, ok := strings.Cut(sql, "-- +goose Down")
		if !ok || !strings.Contains(up, "SET lock_timeout = '1s'") || !strings.Contains(up, "SET statement_timeout = '5s'") {
			t.Fatal("compatibility migration safeguards missing", name)
		}
	}
	for _, required := range []string{"CREATE TABLE bidora_meta.schema_compatibility", "min_reader_generation", "max_writer_generation", "VALUES ('tenant_common', 2, 1, 1, 1, 1"} {
		if !strings.Contains(common, required) {
			t.Fatal("common compatibility contract missing", required)
		}
	}
	if !strings.Contains(tenant, "Preconditions: tenant_common is at version 2") || !strings.Contains(tenant, "VALUES ('tenant_tenderpreneur', 2, 1, 1, 1, 1") {
		t.Fatal("tenant compatibility ordering/contract missing")
	}
}

func TestAuthenticationMigrationTargetStopsBeforeNonAuthSchemas(t *testing.T) {
	auth, err := migrationrunner.Resolve("control-auth", 29)
	if err != nil || auth.Directory != "control" || auth.TableName != "bidora_migrations.goose_control_version" {
		t.Fatal("authentication migration plan drifted", auth, err)
	}
	if _, err = migrationrunner.Resolve("control-auth", 30); err == nil {
		t.Fatal("authentication migration plan accepted notification schema")
	}
	full, err := migrationrunner.Resolve("control", 53)
	if err != nil || full.Directory != auth.Directory || full.TableName != auth.TableName || full.LockKey != auth.LockKey {
		t.Fatal("control migration plans do not share one fenced ledger", full, err)
	}
}

func TestTenderpreneurMigrationTargetMatchesCurrentRuntime(t *testing.T) {
	plan, err := migrationrunner.Resolve("tenant-tenderpreneur", 34)
	if err != nil || plan.Target != 34 {
		t.Fatal("tenant migration target does not match runtime", plan, err)
	}
	for _, target := range []int64{32, 33, 35} {
		if _, err := migrationrunner.Resolve("tenant-tenderpreneur", target); err == nil {
			t.Fatal("unexpected tenant target accepted", target)
		}
	}
}

func TestPasswordRegistrationNamesPrecedeOrganizationCreation(t *testing.T) {
	names := read(t, "migrations/control/00029_password_registration_personal_names.sql")
	notifications := read(t, "migrations/control/00030_notification_delivery_evidence.sql")
	organization := read(t, "migrations/control/00031_tenderpreneur_organization_creation.sql")
	for _, required := range []string{"first_name text", "last_name text", "SET schema_version = 29", "schema_version = 28"} {
		if !strings.Contains(names, required) {
			t.Fatal("authentication migration 29 contract missing", required)
		}
	}
	for _, forbidden := range []string{"organization_creation_operations", "provisioning_jobs", "trial_lifetime_markers"} {
		if strings.Contains(names, forbidden) {
			t.Fatal("authentication migration depends on organization creation", forbidden)
		}
	}
	for _, required := range []string{"CREATE TABLE notification.notifications", "schema_version=30", "schema_version=29"} {
		if !strings.Contains(notifications, required) {
			t.Fatal("notification migration 30 ordering missing", required)
		}
	}
	for _, required := range []string{"organization_creation_operations", "provisioning_jobs", "schema_version=31", "schema_version=30"} {
		if !strings.Contains(organization, required) {
			t.Fatal("organization migration 31 ordering missing", required)
		}
	}
}

func TestPasswordIdentitySchemaIsBoundedAndFailClosed(t *testing.T) {
	sql := read(t, "migrations/control/00003_password_identities.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("identity migration recovery guard missing")
	}
	for _, required := range []string{"SET lock_timeout = '1s'", "CREATE TABLE identity.users", "email_lookup text NOT NULL UNIQUE", "auth_epoch bigint NOT NULL DEFAULT 1", "CREATE TABLE identity.authentication_identities", "password_phc LIKE '$argon2id$v=19$%'", "ON DELETE RESTRICT", "SET schema_version = 3", "schema_version = 2"} {
		if !strings.Contains(up, required) {
			t.Fatal("password identity invariant missing", required)
		}
	}
	for _, forbidden := range []string{"CREATE TABLE identity.sessions", "CREATE TABLE identity.challenges", "mfa_methods", "memberships", "role_grants", "recovery_token", "plaintext_password"} {
		if strings.Contains(strings.ToLower(up), strings.ToLower(forbidden)) {
			t.Fatal("unapproved identity scope", forbidden)
		}
	}
	queries := read(t, "queries/identity/password.sql")
	if strings.Contains(queries, "SELECT *") || strings.Contains(queries, "${") || strings.Contains(queries, "?") {
		t.Fatal("unsafe identity query")
	}
	for _, name := range []string{"InsertPasswordUser", "InsertPasswordIdentity", "RehashPasswordCredential", "GetPasswordLoginRecord"} {
		if strings.Count(queries, "-- name: "+name+" ") != 1 {
			t.Fatal("identity query missing or duplicate", name)
		}
	}
	if strings.Contains(strings.ToUpper(queries), "DELETE ") {
		t.Fatal("unapproved identity deletion query")
	}
	if !strings.Contains(queries, "credential_version = credential_version + 1") || !strings.Contains(queries, "credential_version = sqlc.arg(expected_credential_version)") {
		t.Fatal("rehash optimistic guard missing")
	}
	if !strings.Contains(queries, "email_verified_at, disabled_at") || !strings.Contains(queries, "JOIN identity.authentication_identities") {
		t.Fatal("login record omits fail-closed state")
	}
}

func TestEmailIdentityPolicyIsAdditiveAndExplicit(t *testing.T) {
	sql := read(t, "migrations/control/00004_email_identity_policy.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("email policy migration recovery guard missing")
	}
	for _, required := range []string{"SET lock_timeout = '1s'", "octet_length(email) BETWEEN 3 AND 254", "email !~ '[^!-~]'", "email_lookup = lower(email_lookup)", "NOT VALID", "No provider alias rewriting", "SET schema_version = 4", "schema_version = 3"} {
		if !strings.Contains(up, required) {
			t.Fatal("email policy invariant missing", required)
		}
	}
	for _, forbidden := range []string{"gmail", "citext", "CREATE EXTENSION", "verified_at =", "DELETE FROM"} {
		if strings.Contains(strings.ToLower(up), strings.ToLower(forbidden)) {
			t.Fatal("email migration invented lifecycle/provider behavior", forbidden)
		}
	}
}

func TestEmailVerificationChallengesAreHashedSingleUseAndBounded(t *testing.T) {
	sql := read(t, "migrations/control/00005_email_verification_challenges.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("verification migration recovery guard missing")
	}
	for _, required := range []string{"SET lock_timeout = '1s'", "CREATE TABLE identity.challenges", "purpose = 'EMAIL_VERIFY'", "octet_length(token_hash) = 32", "UNIQUE INDEX challenges_one_active_user_purpose_idx", "consumed_at IS NULL AND invalidated_at IS NULL", "expires_at > created_at", "no automatic deletion", "SET schema_version = 5", "schema_version = 4"} {
		if !strings.Contains(up, required) {
			t.Fatal("verification invariant missing", required)
		}
	}
	for _, forbidden := range []string{"raw_token", "token text", "token varchar", "password_phc", "delivery_provider", "DELETE FROM identity.challenges"} {
		if strings.Contains(strings.ToLower(up), strings.ToLower(forbidden)) {
			t.Fatal("verification schema stores secret/invents provider or cleanup", forbidden)
		}
	}
	queries := read(t, "queries/identity/verification.sql")
	for _, name := range []string{"InvalidateEmailVerificationChallenges", "InsertEmailVerificationChallenge", "ConsumeEmailVerificationChallenge", "InsertEmailVerifiedAudit"} {
		if strings.Count(queries, "-- name: "+name+" ") != 1 {
			t.Fatal("verification query missing", name)
		}
	}
	for _, required := range []string{"WITH consumed AS", "expires_at > sqlc.arg(consumed_at)", "consumed_at IS NULL", "invalidated_at IS NULL", "email_verified_at = COALESCE", "RETURNING u.id, u.email_verified_at, u.auth_epoch"} {
		if !strings.Contains(queries, required) {
			t.Fatal("verification consumption guard missing", required)
		}
	}
	if !strings.Contains(queries, "'EmailVerified'") || !strings.Contains(queries, "'SUCCEEDED'") {
		t.Fatal("email verification audit fact missing")
	}
	if strings.Contains(queries, "DELETE FROM") || strings.Contains(queries, "raw_token") {
		t.Fatal("verification query deletes history or stores raw token")
	}
}

func TestBrowserSessionsAreHashedBoundedAndRevocable(t *testing.T) {
	sql := read(t, "migrations/control/00006_browser_sessions.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("session migration recovery guard missing")
	}
	for _, required := range []string{"CREATE TABLE identity.sessions", "octet_length(token_hash) = 32", "octet_length(csrf_hash) = 32", "auth_epoch bigint NOT NULL", "rotated_from_session_id uuid UNIQUE", "interval '12 hours'", "interval '30 minutes'", "revoked_at", "no raw cookie or CSRF value", "SET schema_version = 6", "schema_version = 5"} {
		if !strings.Contains(up, required) {
			t.Fatal("session invariant missing", required)
		}
	}
	for _, forbidden := range []string{"token text", "csrf_token", "user_agent", "ip_address", "localstorage", "CREATE TABLE identity.mfa"} {
		if strings.Contains(strings.ToLower(up), strings.ToLower(forbidden)) {
			t.Fatal("session schema stores secret/invents policy", forbidden)
		}
	}
	queries := read(t, "queries/identity/session.sql")
	for _, name := range []string{"InsertSession", "InsertLoginAudit", "InsertLogoutAudit", "GetActiveSession", "TouchActiveSession", "RevokeSessionByToken", "RevokeSessionByID", "RevokeAllUserSessions"} {
		if strings.Count(queries, "-- name: "+name+" ") != 1 {
			t.Fatal("session query missing", name)
		}
	}
	for _, required := range []string{"u.email_verified_at IS NOT NULL", "u.disabled_at IS NULL", "u.auth_epoch=s.auth_epoch", "s.revoked_at IS NULL", "LEAST(s.idle_expires_at,s.last_seen_at+make_interval", "LEAST(s.absolute_expires_at,s.created_at+make_interval", "s.last_seen_at<=sqlc.arg(observed_at)", "policy_idle_seconds", "policy_absolute_seconds", "admin_idle_seconds", "admin_absolute_seconds", "u.account_type='PLATFORM_ADMIN'", "s.assurance_level='AAL2'"} {
		if !strings.Contains(queries, required) {
			t.Fatal("session refresh guard missing", required)
		}
	}
	if !strings.Contains(queries, "'UserLoggedIn'") || !strings.Contains(queries, "'UserLoggedOut'") || !strings.Contains(queries, "'SUCCEEDED'") {
		t.Fatal("session audit fact missing")
	}
	if strings.Contains(queries, "DELETE FROM") {
		t.Fatal("session history deletion added")
	}
}

func TestMembershipCapabilityAuthorizationIsCurrentExplicitAndDenyFirst(t *testing.T) {
	sql := read(t, "migrations/control/00007_tenant_membership_capabilities.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("membership migration recovery guard missing")
	}
	for _, required := range []string{"CREATE TABLE tenancy.memberships", "'INVITED','ACTIVE','SUSPENDED','REVOKED'", "context_generation bigint NOT NULL", "UNIQUE (user_id, tenant_id)", "CREATE TABLE tenancy.role_grants", "effect IN ('ALLOW','DENY')", "role_grants_active_fact_idx", "role grant facts are immutable", "role_grants_bump_membership_generation", "context_generation = context_generation + 1", "SET schema_version = 7", "schema_version = 6"} {
		if !strings.Contains(up, required) {
			t.Fatal("membership invariant missing", required)
		}
	}
	for _, forbidden := range []string{"platform_admin_bypass", "default_tenant", "header_tenant", "CREATE TABLE tenancy.invitations", "last_admin"} {
		if strings.Contains(strings.ToLower(up), strings.ToLower(forbidden)) {
			t.Fatal("membership schema invented blocked policy", forbidden)
		}
	}
	queries := read(t, "queries/authorization/membership.sql")
	for _, required := range []string{"ListActiveMembershipCapabilities", "array_agg(DISTINCT allow_grant.capability", "deny_grant.capability = allow_grant.capability", "ORDER BY t.name, m.tenant_id, m.id", "AuthorizeExternalOpportunityCapture", "m.context_generation = sqlc.arg(context_generation)", "m.state = 'ACTIVE'", "u.email_verified_at IS NOT NULL", "u.disabled_at IS NULL", "t.tenant_type = 'TENDERPRENEUR'", "t.state = 'ACTIVE'", "t.assignment_generation > 0", "tenderpreneur.opportunity.capture_external", "allow_grant.effect = 'ALLOW'", "NOT EXISTS", "deny_grant.effect = 'DENY'"} {
		if !strings.Contains(queries, required) {
			t.Fatal("capture authorization guard missing", required)
		}
	}
	if strings.Contains(queries, "SELECT *") || strings.Contains(queries, "role_key =") {
		t.Fatal("capture authorization trusts unstable columns/role names")
	}
}

func TestPasswordChangeRotatesCredentialEpochAndSessions(t *testing.T) {
	passwordQueries := read(t, "queries/identity/password.sql")
	for _, required := range []string{"GetPasswordChangePrincipalForUpdate", "FOR UPDATE OF u, a, s", "ChangePasswordCredential", "credential_version = credential_version + 1", "password_changed_at", "AdvancePasswordChangeAuthEpoch", "auth_epoch = auth_epoch + 1", "PasswordChanged"} {
		if !strings.Contains(passwordQueries, required) {
			t.Fatal("password change safeguard missing", required)
		}
	}
	sessionQueries := read(t, "queries/identity/session.sql")
	for _, required := range []string{"RevokeAllUserSessions", "InsertSession", "rotated_from_session_id"} {
		if !strings.Contains(sessionQueries, required) {
			t.Fatal("password change session rotation primitive missing", required)
		}
	}
}

func TestInvitationCancellationIsAuthorizedAuditedAndEvented(t *testing.T) {
	queries := read(t, "queries/authorization/invitation.sql")
	for _, required := range []string{"LockCancelableInvitation", "tenant.memberships.manage", "deny_grant.effect = 'DENY'", "i.expires_at >", "FOR UPDATE OF m, i", "CancelInvitation", "InvitationCancelled", "invitation.cancelled.v1", "'invitation', sqlc.arg(aggregate_id), 2"} {
		if !strings.Contains(queries, required) {
			t.Fatal("invitation cancellation safeguard missing", required)
		}
	}
}

func TestPendingInvitationDirectoryIsBoundedAndSecretFree(t *testing.T) {
	queries := read(t, "queries/authorization/invitation.sql")
	for _, required := range []string{"ListPendingInvitations", "i.invalidated_at IS NULL", "i.consumed_at IS NULL", "i.expires_at >", "(i.created_at, i.id) <", "ORDER BY i.created_at DESC, i.id DESC", "LIMIT sqlc.arg(max_rows)"} {
		if !strings.Contains(queries, required) {
			t.Fatal("pending invitation directory safeguard missing", required)
		}
	}
	list := queries[:strings.Index(queries, "-- name: LockAuthorizedInvitationIssuer")]
	for _, forbidden := range []string{"token_hash", "email_lookup", "created_by"} {
		if strings.Contains(list, forbidden) {
			t.Fatal("pending invitation directory selects secret/internal field", forbidden)
		}
	}
}

func TestCaptureContributorRevocationExcludesAdministratorMutation(t *testing.T) {
	queries := read(t, "queries/authorization/membership_management.sql")
	for _, required := range []string{"AuthorizeMembershipManagement", "ListCaptureContributorMemberships", "target_user.email", "ORDER BY target_user.email_lookup, target.id", "LockRevocableCaptureContributor", "tenant.memberships.manage", "manager_deny.effect = 'DENY'", "TENDERPRENEUR_CAPTURE_CONTRIBUTOR", "NOT EXISTS", "RevokeCaptureContributorMembership", "state = 'REVOKED'", "RevokeCaptureContributorGrant", "MembershipRevoked", "membership.revoked.v1"} {
		if !strings.Contains(queries, required) {
			t.Fatal("capture contributor revocation safeguard missing", required)
		}
	}
	for _, forbidden := range []string{"DELETE FROM", "ADMINISTRATOR", "platform_admin", "UPDATE identity.sessions"} {
		if strings.Contains(queries, forbidden) {
			t.Fatal("capture contributor revocation crossed scope", forbidden)
		}
	}
}

func TestGoogleExternalIdentityMigrationBindsStableSubjectWithoutEmailLinking(t *testing.T) {
	sql := read(t, "migrations/control/00008_google_external_identities.sql")
	for _, required := range []string{"CREATE TABLE identity.external_identities", "provider = 'GOOGLE'", "UNIQUE (provider, provider_subject)", "UNIQUE (user_id, provider)", "matching email alone must never create this row", "SET schema_version = 8", "schema_version = 7"} {
		if !strings.Contains(sql, required) {
			t.Fatal("Google external identity safeguard missing", required)
		}
	}
}

func TestTenantInvitationMigrationAndAcceptanceAreFixedScopeAndSingleUse(t *testing.T) {
	sql := read(t, "migrations/control/00009_tenant_invitations.sql")
	up, down, ok := strings.Cut(sql, "-- +goose Down")
	if !ok || !strings.Contains(down, "Never auto-run Down") {
		t.Fatal("invitation migration recovery guard missing")
	}
	for _, required := range []string{"CREATE TABLE tenancy.invitations", "token_hash bytea NOT NULL UNIQUE", "octet_length(token_hash) = 32", "TENDERPRENEUR_CAPTURE_CONTRIBUTOR", "tenderpreneur.opportunity.capture_external", "interval '72 hours'", "invitations_one_active_email_idx", "protect_invitation_facts", "only first invalidate or consume", "SET schema_version = 9", "schema_version = 8"} {
		if !strings.Contains(up, required) {
			t.Fatal("invitation invariant missing", required)
		}
	}
	queries := read(t, "queries/authorization/invitation.sql")
	for _, required := range []string{"LockAuthorizedInvitationIssuer", "tenant.memberships.manage", "deny_grant.effect = 'DENY'", "InvalidateActiveTenantInvitation", "InsertTenantInvitation", "InvitationIssued", "invitation.issued.v1", "LockInvitationForAcceptance", "u.email_lookup = i.email_lookup", "u.email_verified_at IS NOT NULL", "u.disabled_at IS NULL", "t.state = 'ACTIVE'", "FOR UPDATE OF i", "InsertInvitedMembership", "InsertInvitationCapabilityGrant", "ConsumeTenantInvitation", "InvitationAccepted", "membership.granted.v1"} {
		if !strings.Contains(queries, required) {
			t.Fatal("invitation acceptance safeguard missing", required)
		}
	}
	for _, forbidden := range []string{"DELETE FROM", "platform_admin", "last_admin", "SELECT *"} {
		if strings.Contains(strings.ToLower(queries), strings.ToLower(forbidden)) {
			t.Fatal("invitation acceptance contains forbidden authority", forbidden)
		}
	}
}

func TestGoogleLinkQueriesRequireCurrentPasswordSessionAndEpoch(t *testing.T) {
	queries := read(t, "queries/identity/google.sql")
	for _, required := range []string{"GetGoogleLinkPrincipalForUpdate", "JOIN identity.authentication_identities", "JOIN identity.sessions", "a.credential_version = sqlc.arg(expected_credential_version)", "s.auth_epoch = u.auth_epoch", "s.revoked_at IS NULL", "s.idle_expires_at > sqlc.arg(observed_at)", "FOR UPDATE OF u, a, s", "GetGoogleIdentityForUserForUpdate", "AdvanceGoogleLinkAuthEpoch", "auth_epoch = auth_epoch + 1", "GoogleIdentityLinked"} {
		if !strings.Contains(queries, required) {
			t.Fatal("Google link query safeguard missing", required)
		}
	}
	if strings.Contains(queries, "provider_subject = email") {
		t.Fatal("Google link query used email as provider authority")
	}
}

func TestCaptureQueriesAreQualifiedAndParameterized(t *testing.T) {
	queries := read(t, "queries/opportunity/capture.sql")
	if strings.Contains(queries, "SELECT *") || strings.Contains(queries, "${") || strings.Contains(queries, "?") {
		t.Fatal("unsafe/unstable query form")
	}
	for _, schema := range []string{"bidora_commands.", "bidora_opportunity.", "bidora_audit.", "bidora_eventing."} {
		if !strings.Contains(queries, schema) {
			t.Fatal("unqualified/missing schema", schema)
		}
	}
	for _, name := range []string{"ClaimCaptureIdempotency", "GetCaptureIdempotencyForUpdate", "InsertCapturedOpportunity", "InsertUnverifiedSourceVersion", "SetOpportunityCurrentSource", "InsertCaptureAudit", "InsertCaptureOutbox", "CompleteCaptureIdempotency"} {
		if strings.Count(queries, "-- name: "+name+" ") != 1 {
			t.Fatal("query missing or duplicate", name)
		}
	}
	if !strings.Contains(queries, "ON CONFLICT (actor_id, operation, key_hash) DO NOTHING") || !strings.Contains(queries, "FOR UPDATE OF r") || !strings.Contains(queries, "AND request_hash = sqlc.arg(request_hash)") || !strings.Contains(queries, "id, actor_id, aggregate_type") {
		t.Fatal("idempotency concurrency guards missing")
	}
	compatibility := read(t, "queries/opportunity/compatibility.sql")
	if strings.Contains(compatibility, "SELECT *") || !strings.Contains(compatibility, "FROM bidora_meta.schema_compatibility") || !strings.Contains(compatibility, "'tenant_common', 'tenant_tenderpreneur'") || !strings.Contains(compatibility, "ORDER BY migration_set") {
		t.Fatal("compatibility query is unbounded or unstable")
	}
}

func TestGeneratedSQLCFilesDeclarePinnedGenerator(t *testing.T) {
	groups := map[string][]string{
		"internal/opportunity/persistence/sqlc/":   {"capture.sql.go", "compatibility.sql.go", "db.go", "models.go", "querier.go"},
		"internal/tenancy/persistence/sqlc/":       {"registry.sql.go", "db.go", "models.go", "querier.go"},
		"internal/eventing/persistence/sqlc/":      {"inbox.sql.go", "db.go", "models.go", "querier.go"},
		"internal/identity/persistence/sqlc/":      {"password.sql.go", "verification.sql.go", "session.sql.go", "google.sql.go", "db.go", "models.go", "querier.go"},
		"internal/authorization/persistence/sqlc/": {"membership.sql.go", "db.go", "models.go", "querier.go"},
	}
	for dir, names := range groups {
		for _, name := range names {
			body := read(t, dir+name)
			if !strings.Contains(body, "Code generated by sqlc. DO NOT EDIT.") || !strings.Contains(body, "sqlc v1.31.1") {
				t.Fatal("generated artifact provenance missing", dir+name)
			}
		}
	}
}

func read(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(backendRoot(), filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
