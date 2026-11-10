-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE platform_administration.administrator_invitations
 ADD COLUMN first_name text NOT NULL DEFAULT '' CHECK(octet_length(first_name)<=100 AND first_name=btrim(first_name) AND first_name!~'[[:cntrl:]]'),
 ADD COLUMN last_name text NOT NULL DEFAULT '' CHECK(octet_length(last_name)<=100 AND last_name=btrim(last_name) AND last_name!~'[[:cntrl:]]');
ALTER TABLE platform_administration.administrator_profiles ADD COLUMN removed_at timestamptz CHECK(removed_at IS NULL OR removed_at>=created_at);
DO $$ DECLARE n text; BEGIN
 FOR n IN SELECT conname FROM pg_constraint WHERE conrelid='platform_administration.administrator_changes'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%action%' LOOP
 EXECUTE format('ALTER TABLE platform_administration.administrator_changes DROP CONSTRAINT %I',n);
 END LOOP;
END $$;
ALTER TABLE platform_administration.administrator_changes ADD CONSTRAINT administrator_change_action_v2 CHECK(action IN ('ASSIGN_ROLE','REVOKE_ROLE','DISABLE_IDENTITY','REACTIVATE_IDENTITY','REMOVE_IDENTITY','ACTIVATE_OWNER_INVITATION'));
ALTER TABLE platform_administration.administrator_changes ADD CONSTRAINT administrator_change_shape_v2 CHECK ((action = 'ASSIGN_ROLE' AND target_user_id IS NOT NULL AND invitation_id IS NULL AND role_key IS NOT NULL AND assignment_id IS NULL AND expected_user_version IS NOT NULL AND expected_target_context_generation IS NOT NULL AND expected_assignment_version IS NULL)
        OR (action = 'REVOKE_ROLE' AND target_user_id IS NOT NULL AND invitation_id IS NULL AND role_key IS NOT NULL AND assignment_id IS NOT NULL AND expected_user_version IS NOT NULL AND expected_target_context_generation IS NOT NULL AND expected_assignment_version IS NOT NULL)
        OR (action IN ('DISABLE_IDENTITY','REACTIVATE_IDENTITY','REMOVE_IDENTITY') AND target_user_id IS NOT NULL AND invitation_id IS NULL AND role_key IS NULL AND assignment_id IS NULL AND expected_user_version IS NOT NULL AND expected_target_context_generation IS NOT NULL AND expected_assignment_version IS NULL)
        OR (action = 'ACTIVATE_OWNER_INVITATION' AND target_user_id IS NULL AND invitation_id IS NOT NULL AND role_key = 'PLATFORM_OWNER' AND assignment_id IS NULL AND expected_user_version IS NULL AND expected_target_context_generation IS NULL AND expected_assignment_version IS NULL));
CREATE FUNCTION platform_administration.guard_removed_administrator() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF OLD.removed_at IS NOT NULL AND NEW.removed_at IS DISTINCT FROM OLD.removed_at THEN RAISE EXCEPTION 'Removal evidence is immutable' USING ERRCODE='55000';END IF;
 IF NEW.removed_at IS NOT NULL AND NOT EXISTS(SELECT 1 FROM identity.users u WHERE u.id=NEW.user_id AND u.disabled_at IS NOT NULL) THEN RAISE EXCEPTION 'Removed administrator must be disabled' USING ERRCODE='55000';END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_removed_administrator BEFORE UPDATE OF removed_at ON platform_administration.administrator_profiles FOR EACH ROW EXECUTE FUNCTION platform_administration.guard_removed_administrator();
CREATE FUNCTION identity.guard_removed_administrator_reactivation() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.disabled_at IS NULL AND EXISTS(SELECT 1 FROM platform_administration.administrator_profiles p WHERE p.user_id=NEW.id AND p.removed_at IS NOT NULL) THEN RAISE EXCEPTION 'Removed administrator cannot be reactivated' USING ERRCODE='55000';END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_removed_administrator_reactivation BEFORE UPDATE OF disabled_at ON identity.users FOR EACH ROW EXECUTE FUNCTION identity.guard_removed_administrator_reactivation();
CREATE FUNCTION platform_administration.guard_invitation_person_names() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.first_name IS DISTINCT FROM OLD.first_name OR NEW.last_name IS DISTINCT FROM OLD.last_name THEN RAISE EXCEPTION 'Invitation names are immutable' USING ERRCODE='55000';END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_invitation_person_names BEFORE UPDATE OF first_name,last_name ON platform_administration.administrator_invitations FOR EACH ROW EXECUTE FUNCTION platform_administration.guard_invitation_person_names();
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
  OR (e.target_type='platform_administrator_change' AND e.action IN ('PlatformRoleAssignmentChanged','PlatformRoleRevocationChanged','PlatformAdministratorDisabled','PlatformAdministratorReactivated','PlatformAdministratorRemoved'))
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

UPDATE bidora_control_meta.schema_compatibility SET schema_version=57,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=56;
-- +goose StatementEnd
-- +goose Down
DO $$ BEGIN RAISE EXCEPTION 'Forward-only migration: use a schema-57-compatible application rollback'; END $$;
