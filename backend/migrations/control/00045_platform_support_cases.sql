-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Control metadata only. Never store tenant-private bids, documents or support-access credentials.
CREATE SCHEMA bidora_support;
CREATE TABLE bidora_support.cases (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 title text NOT NULL CHECK(length(title) BETWEEN 3 AND 120 AND title=btrim(title)),
 description text NOT NULL CHECK(length(description) BETWEEN 3 AND 2000 AND description=btrim(description)),
 state text NOT NULL CHECK(state IN('OPEN','RESOLVED')),
 assigned_to uuid REFERENCES platform_administration.administrator_profiles(user_id) ON DELETE RESTRICT,
 created_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 version bigint NOT NULL CHECK(version>0),
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL CHECK(updated_at>=created_at)
);
CREATE INDEX support_cases_listing ON bidora_support.cases(updated_at DESC,id DESC);
CREATE INDEX support_cases_tenant ON bidora_support.cases(tenant_id,updated_at DESC,id DESC);
CREATE TABLE bidora_support.case_messages (
 id uuid PRIMARY KEY,case_id uuid NOT NULL REFERENCES bidora_support.cases(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 body text NOT NULL CHECK(length(body) BETWEEN 1 AND 2000 AND body=btrim(body)),
 created_at timestamptz NOT NULL
);
CREATE INDEX support_messages_case ON bidora_support.case_messages(case_id,created_at,id);
CREATE TABLE bidora_support.case_events (
 id uuid PRIMARY KEY,case_id uuid NOT NULL REFERENCES bidora_support.cases(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 action text NOT NULL CHECK(action IN('CREATE','ASSIGN','REPLY','RESOLVE','REOPEN')),
 version bigint NOT NULL CHECK(version>0),created_at timestamptz NOT NULL,
 UNIQUE(case_id,version)
);
CREATE TABLE bidora_support.case_commands (
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 operation text NOT NULL CHECK(operation IN('CREATE','ASSIGN','REPLY','RESOLVE','REOPEN')),
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 case_id uuid NOT NULL REFERENCES bidora_support.cases(id) ON DELETE RESTRICT,
 result_version bigint NOT NULL CHECK(result_version>0),
 created_at timestamptz NOT NULL,
 PRIMARY KEY(actor_id,operation,key_hash)
);
CREATE FUNCTION bidora_support.reject_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'support evidence is append-only' USING ERRCODE='55000'; END $$;
REVOKE ALL ON FUNCTION bidora_support.reject_mutation() FROM PUBLIC;
CREATE TRIGGER support_messages_immutable BEFORE UPDATE OR DELETE ON bidora_support.case_messages FOR EACH ROW EXECUTE FUNCTION bidora_support.reject_mutation();
CREATE TRIGGER support_events_immutable BEFORE UPDATE OR DELETE ON bidora_support.case_events FOR EACH ROW EXECUTE FUNCTION bidora_support.reject_mutation();
CREATE TRIGGER support_commands_immutable BEFORE UPDATE OR DELETE ON bidora_support.case_commands FOR EACH ROW EXECUTE FUNCTION bidora_support.reject_mutation();
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT e.id,e.actor_id,e.action,e.occurred_at,
 CASE WHEN e.target_type IN ('platform_integration_setting','platform_google_configuration','notification_template') THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN e.target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting','platform_google_configuration','notification_template','platform_regional_policy','platform_support_case') THEN e.target_id ELSE NULL END::uuid AS affected_record_id,
 CASE WHEN e.outcome='FAILED' THEN 'FAILED'
      WHEN e.target_type='platform_administrator_change' AND e.change_summary->>'State'='PENDING_APPROVAL' THEN 'PENDING'
      ELSE 'SUCCESSFUL' END::text AS result
FROM bidora_control_audit.audit_events AS e
WHERE e.tenant_id IS NULL
 AND (
  (e.target_type='platform_administrator' AND e.action IN ('PlatformOwnerBootstrapped','PlatformAdministratorSessionsRevoked'))
  OR (e.target_type='platform_administrator_invitation' AND e.action IN ('PlatformAdministratorInvitationIssued','PlatformAdministratorInvitationCancelled','PlatformAdministratorInvitationAccepted','PlatformOwnerInvitationActivated','PlatformOwnerInvitationRejected'))
  OR (e.target_type='platform_administrator_change' AND e.action IN ('PlatformRoleAssignmentChanged','PlatformRoleRevocationChanged','PlatformAdministratorDisabled','PlatformAdministratorReactivated'))
  OR (e.target_type='platform_integration_setting' AND e.action IN ('PlatformIntegrationVersionCreated','PlatformIntegrationVersionTested','PlatformIntegrationActivationProposed'))
  OR (e.target_type='platform_google_configuration' AND e.action IN ('PlatformGoogleConfigurationUpdated','PlatformGoogleConnectionTested'))
  OR (e.target_type='notification_template' AND e.action IN ('NotificationTemplateUpdated','NotificationTemplateTested'))
  OR (e.target_type='platform_regional_policy' AND e.action IN ('CountryConfigurationUpdated','OperationalPolicyUpdated','RestrictedRequestMetadataRead'))
  OR (e.target_type='platform_information' AND e.action='PlatformInformationUpdated')
  OR (e.target_type='platform_session_policy' AND e.action='PlatformSessionPolicyUpdated')
  OR (e.target_type='platform_support_page' AND e.action='PlatformSupportCasesRead')
  OR (e.target_type='platform_support_case' AND e.action IN ('PlatformSupportCaseRead','PlatformSupportCaseCreate','PlatformSupportCaseAssign','PlatformSupportCaseReply','PlatformSupportCaseResolve','PlatformSupportCaseReopen'))
  OR (e.target_type='platform_audit_export' AND e.action='PlatformControlAuditExport' AND e.outcome='SUCCEEDED')
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );
DO $$ BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=45,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=44;
 IF NOT FOUND THEN RAISE EXCEPTION 'control compatibility must be at 44'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'support cases and evidence require forward recovery'; END $$;
-- +goose StatementEnd
