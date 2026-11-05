-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Separate Platform command evidence: tenant-scoped lifecycle grants cannot issue these commands.
CREATE TABLE commercial.platform_subscription_commands(
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 operation text NOT NULL CHECK(operation IN('SCHEDULE_CANCELLATION','WITHDRAW_CANCELLATION')),
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 result_state text NOT NULL CHECK(length(result_state)>0),result_version bigint NOT NULL CHECK(result_version>1),
 result_cancel_at timestamptz,created_at timestamptz NOT NULL,PRIMARY KEY(actor_id,operation,key_hash)
);
CREATE TRIGGER platform_subscription_commands_immutable BEFORE UPDATE OR DELETE ON commercial.platform_subscription_commands FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
ALTER TABLE commercial.subscription_transitions DROP CONSTRAINT subscription_transitions_action_check;
ALTER TABLE commercial.subscription_transitions ADD CONSTRAINT subscription_transitions_action_check CHECK(action IN('SCHEDULE_CANCELLATION','WITHDRAW_CANCELLATION','PLATFORM_SCHEDULE_CANCELLATION','PLATFORM_WITHDRAW_CANCELLATION'));
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT e.id,e.actor_id,e.action,e.occurred_at,
 CASE WHEN e.target_type IN ('platform_integration_setting','platform_google_configuration','notification_template') THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN e.target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting','platform_google_configuration','notification_template','platform_regional_policy','platform_support_case','platform_plan','platform_subscription') THEN e.target_id ELSE NULL END::uuid AS affected_record_id,
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
  OR (e.target_type='platform_subscription' AND e.action IN ('PlatformSubscriptionRead','PlatformSubscriptionCancellationScheduled','PlatformSubscriptionCancellationWithdrawn'))
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );
DO $$ BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=47,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=46;
 IF NOT FOUND THEN RAISE EXCEPTION 'control compatibility must be at 46'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'platform subscription transitions require forward recovery'; END $$;
-- +goose StatementEnd
