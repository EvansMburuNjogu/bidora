-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Enrich only events already admitted by the security-barrier Platform view.
-- Never project the raw summary, credentials, session targets or tenant data.
CREATE VIEW bidora_control_audit.platform_action_details_v1 WITH (security_barrier=true) AS
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
LEFT JOIN notification.templates nt ON e.target_type='notification_template' AND nt.template_key=e.change_summary->>'template_key'
LEFT JOIN platform_settings.operational_policy policy ON policy.singleton
LEFT JOIN bidora_control_audit.platform_request_metadata_v1 m ON m.audit_id=a.id
 AND m.observed_at>=CURRENT_TIMESTAMP-make_interval(days=>policy.restricted_metadata_retention_days);
DO $$ BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=50,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=49;
 IF NOT FOUND THEN RAISE EXCEPTION 'control compatibility must be at 49'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'audit detail projection requires forward recovery'; END $$;
-- +goose StatementEnd
