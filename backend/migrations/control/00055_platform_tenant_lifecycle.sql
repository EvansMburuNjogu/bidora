-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bidora_tenancy.platform_tenant_commands(
 actor_id uuid NOT NULL REFERENCES identity.users(id),
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id),
 operation text NOT NULL CHECK(operation IN('SUSPEND','REACTIVATE')),
 result_state text NOT NULL CHECK(result_state IN('SUSPENDED','ACTIVE')),
 result_version bigint NOT NULL CHECK(result_version>0),
 reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000 AND reason=btrim(reason)),
 created_at timestamptz NOT NULL,
 PRIMARY KEY(actor_id,key_hash)
);
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT e.id,e.actor_id,e.action,e.occurred_at,
 CASE WHEN e.target_type IN ('platform_integration_setting','platform_google_configuration','notification_template') THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN e.target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting','platform_google_configuration','notification_template','platform_regional_policy','platform_support_case','platform_plan','platform_subscription','tenant') THEN e.target_id ELSE NULL END::uuid AS affected_record_id,
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
  OR (e.target_type='tenant' AND e.action IN ('PlatformTenantSuspended','PlatformTenantReactivated'))
  OR (e.target_type='platform_subscription_inventory' AND e.action='PlatformSubscriptionInventoryRead')
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_details_v1 WITH (security_barrier=true) AS
SELECT a.*,
 coalesce(nullif(btrim(concat_ws(' ',u.first_name,u.last_name)),''),u.email,
   CASE WHEN a.actor_id IS NULL THEN 'System' ELSE 'Unknown administrator' END)::text AS administrator_name,
 CASE e.target_type
  WHEN 'platform_information' THEN 'Platform information'
  WHEN 'platform_session_policy' THEN 'Session policy'
  WHEN 'platform_google_configuration' THEN 'Google login configuration'
  WHEN 'platform_integration_setting' THEN 'Integration configuration'
  WHEN 'platform_regional_policy' THEN CASE WHEN e.action='CountryConfigurationUpdated' THEN 'Country configuration' WHEN e.action='RestrictedRequestMetadataRead' THEN 'Restricted request metadata' ELSE 'Operational policy' END
  WHEN 'notification_template' THEN coalesce(nt.name,'Notification template')
  WHEN 'platform_administrator' THEN coalesce(nullif(btrim(concat_ws(' ',target.first_name,target.last_name)),''),target.email,'Administrator')
  WHEN 'platform_administrator_change' THEN coalesce(nullif(btrim(concat_ws(' ',target.first_name,target.last_name)),''),target.email,'Administrator access')
  WHEN 'platform_administrator_invitation' THEN 'Administrator invitation'
  WHEN 'platform_plan' THEN 'Subscription plan'
  WHEN 'platform_subscription' THEN 'Subscription'
  WHEN 'tenant' THEN coalesce(tenant.name,'Organisation')
  WHEN 'platform_support_case' THEN 'Support case'
  WHEN 'platform_audit_export' THEN 'Audit export'
  WHEN 'session' THEN 'Sign-in'
  WHEN 'mfa_factor' THEN 'Sign-in verification'
  ELSE 'Platform activity' END::text AS affected_record_name,
 coalesce((SELECT jsonb_agg(jsonb_build_object('field',v->>'field','operation',v->>'operation','before',v->'before','after',v->'after') ORDER BY ord)
 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(e.change_summary->'changes')='array' THEN e.change_summary->'changes' ELSE '[]'::jsonb END) WITH ORDINALITY AS entry(v,ord)
 WHERE ord<=30 AND v->>'field' IN ('platform_name','contact_phone','support_email','website','enabled','allow_signup','client_id','allowed_domains','client_secret','idle_seconds','absolute_seconds','country_code','currency','language','timezone','locale','is_default','organization_registration_enabled','ai_features_enabled','document_uploads_enabled','max_document_bytes','maintenance_enabled','maintenance_message','maintenance_starts_at','maintenance_ends_at','restricted_metadata_retention_days','template_name','template_subject','template_body')
 AND v->>'operation' IN ('ADDED','DELETED','MODIFIED')
 AND (v->'before'='null'::jsonb OR jsonb_typeof(v->'before')='string' AND length(v->>'before')<=512)
 AND (v->'after'='null'::jsonb OR jsonb_typeof(v->'after')='string' AND length(v->>'after')<=512)
 -- Secrets are recorded only as a change marker, never as old/new values.
 AND (v->>'field'<>'client_secret' OR v->'before'='null'::jsonb AND v->>'after'='[redacted]')),'[]'::jsonb) || CASE
 WHEN e.target_type='platform_administrator_change' AND e.change_summary->>'State'='EXECUTED'
 AND e.change_summary->>'RoleKey' IN ('PLATFORM_OWNER','OPERATIONS_ADMINISTRATOR','BILLING_ADMINISTRATOR','INTEGRATION_ADMINISTRATOR','SUPPORT_ADMINISTRATOR','SECURITY_AUDIT_ADMINISTRATOR')
 AND e.action IN ('PlatformRoleAssignmentChanged','PlatformRoleRevocationChanged')
 THEN jsonb_build_array(jsonb_build_object('field','role','operation',CASE WHEN e.action='PlatformRoleAssignmentChanged' THEN 'ADDED' ELSE 'DELETED' END,
 'before',CASE WHEN e.action='PlatformRoleRevocationChanged' THEN e.change_summary->>'RoleKey' END,
 'after',CASE WHEN e.action='PlatformRoleAssignmentChanged' THEN e.change_summary->>'RoleKey' END))
 ELSE '[]'::jsonb END AS changes,
 m.ip_address,m.browser
FROM bidora_control_audit.platform_action_times_v1 a
JOIN bidora_control_audit.audit_events e ON e.id=a.id AND e.tenant_id IS NULL
LEFT JOIN identity.users u ON u.id=a.actor_id
LEFT JOIN platform_administration.administrator_changes ac ON e.target_type='platform_administrator_change' AND ac.id=e.target_id
LEFT JOIN identity.users target ON target.account_type='PLATFORM_ADMIN' AND
 (e.target_type='platform_administrator' AND target.id=e.target_id OR e.target_type='platform_administrator_change' AND target.id=ac.target_user_id)
LEFT JOIN bidora_tenancy.tenants tenant ON e.target_type='tenant' AND tenant.id=e.target_id
LEFT JOIN notification.templates nt ON e.target_type='notification_template' AND nt.template_key=e.change_summary->>'template_key'
LEFT JOIN platform_settings.operational_policy policy ON policy.singleton
LEFT JOIN bidora_control_audit.platform_request_metadata_v1 m ON m.audit_id=a.id
 AND m.observed_at>=CURRENT_TIMESTAMP-make_interval(days=>policy.restricted_metadata_retention_days);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=55,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=54;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility must be 54'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'forward-only migration'; END $$;
-- +goose StatementEnd
