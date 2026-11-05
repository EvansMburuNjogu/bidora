-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Only the three fixed-price self-service plans are managed here. Trial and
-- bespoke Enterprise contract terms retain their source-defined policy.
CREATE TABLE commercial.platform_plan_versions(
 plan_key text NOT NULL CHECK(plan_key IN('BASIC','GROWTH','PROFESSIONAL')),
 version bigint NOT NULL CHECK(version>0),
 monthly_minor bigint NOT NULL CHECK(monthly_minor>0),annual_minor bigint NOT NULL CHECK(annual_minor>0),
 additional_licence_minor bigint NOT NULL CHECK(additional_licence_minor>0),
 bid_workspaces integer NOT NULL CHECK(bid_workspaces>0),included_licences integer NOT NULL CHECK(included_licences>0),
 storage_bytes bigint NOT NULL CHECK(storage_bytes>0),document_versions bigint NOT NULL CHECK(document_versions>0),
 ai_credits bigint NOT NULL CHECK(ai_credits>0),concurrent_ai_jobs integer NOT NULL CHECK(concurrent_ai_jobs>0),
 created_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,created_at timestamptz NOT NULL,
 PRIMARY KEY(plan_key,version)
);
CREATE TRIGGER platform_plan_versions_immutable BEFORE UPDATE OR DELETE ON commercial.platform_plan_versions FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
CREATE TABLE commercial.platform_plan_heads(
 plan_key text PRIMARY KEY CHECK(plan_key IN('BASIC','GROWTH','PROFESSIONAL')),
 active_version bigint NOT NULL CHECK(active_version>0),next_version bigint NOT NULL CHECK(next_version>active_version),
 head_version bigint NOT NULL CHECK(head_version>0),updated_at timestamptz NOT NULL,
 FOREIGN KEY(plan_key,active_version) REFERENCES commercial.platform_plan_versions(plan_key,version) ON DELETE RESTRICT
);
-- Seed exactly the source-defined Phase 1 KES minor-unit values and limits.
INSERT INTO commercial.platform_plan_versions(plan_key,version,monthly_minor,annual_minor,additional_licence_minor,bid_workspaces,included_licences,storage_bytes,document_versions,ai_credits,concurrent_ai_jobs,created_by,created_at) VALUES
('BASIC',1,450000,4500000,75000,3,3,10737418240,250,5000,2,NULL,CURRENT_TIMESTAMP),
('GROWTH',1,1250000,12500000,65000,10,8,53687091200,1500,25000,5,NULL,CURRENT_TIMESTAMP),
('PROFESSIONAL',1,2950000,29500000,50000,30,20,214748364800,6000,100000,10,NULL,CURRENT_TIMESTAMP);
INSERT INTO commercial.platform_plan_heads(plan_key,active_version,next_version,head_version,updated_at)
SELECT plan_key,1,2,1,CURRENT_TIMESTAMP FROM commercial.platform_plan_versions;
CREATE TABLE commercial.platform_plan_commands(
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 operation text NOT NULL CHECK(operation IN('DRAFT','PUBLISH')),
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 plan_key text NOT NULL CHECK(plan_key IN('BASIC','GROWTH','PROFESSIONAL')),
 result_version bigint NOT NULL CHECK(result_version>0),result_head_version bigint NOT NULL CHECK(result_head_version>0),
 created_at timestamptz NOT NULL,PRIMARY KEY(actor_id,operation,key_hash)
);
CREATE TRIGGER platform_plan_commands_immutable BEFORE UPDATE OR DELETE ON commercial.platform_plan_commands FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT e.id,e.actor_id,e.action,e.occurred_at,
 CASE WHEN e.target_type IN ('platform_integration_setting','platform_google_configuration','notification_template') THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN e.target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting','platform_google_configuration','notification_template','platform_regional_policy','platform_support_case','platform_plan') THEN e.target_id ELSE NULL END::uuid AS affected_record_id,
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
  OR (e.target_type='platform_plan_list' AND e.action='PlatformPlansRead')
  OR (e.target_type='platform_plan' AND e.action IN ('PlatformPlanDraftCreated','PlatformPlanPublished'))
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );
DO $$ BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=46,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=45;
 IF NOT FOUND THEN RAISE EXCEPTION 'control compatibility must be at 45'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'published plan versions require forward recovery'; END $$;
-- +goose StatementEnd
