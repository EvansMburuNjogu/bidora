-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Only successful control-plane sign-in evidence for immutable PLATFORM_ADMIN
-- identities joins the existing action projection. Never expose a session/factor
-- target, audit summary, authentication secret, or another account type's events.
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT e.id,e.actor_id,e.action,e.occurred_at,
 CASE WHEN e.target_type='platform_integration_setting' THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN e.target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting') THEN e.target_id ELSE NULL END::uuid AS affected_record_id,
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
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=37,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=36;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 36'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=36,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=37;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 37'; END IF;
END $$;
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT id,actor_id,action,occurred_at,
 CASE WHEN target_type='platform_integration_setting' THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting') THEN target_id ELSE NULL END::uuid AS affected_record_id,
 CASE WHEN outcome='FAILED' THEN 'FAILED'
      WHEN target_type='platform_administrator_change' AND change_summary->>'State'='PENDING_APPROVAL' THEN 'PENDING'
      ELSE 'SUCCESSFUL' END::text AS result
FROM bidora_control_audit.audit_events
WHERE tenant_id IS NULL
 AND (
  (target_type='platform_administrator' AND action IN ('PlatformOwnerBootstrapped','PlatformAdministratorSessionsRevoked'))
  OR (target_type='platform_administrator_invitation' AND action IN ('PlatformAdministratorInvitationIssued','PlatformAdministratorInvitationCancelled','PlatformAdministratorInvitationAccepted','PlatformOwnerInvitationActivated','PlatformOwnerInvitationRejected'))
  OR (target_type='platform_administrator_change' AND action IN ('PlatformRoleAssignmentChanged','PlatformRoleRevocationChanged','PlatformAdministratorDisabled','PlatformAdministratorReactivated'))
  OR (target_type='platform_integration_setting' AND action IN ('PlatformIntegrationVersionCreated','PlatformIntegrationVersionTested','PlatformIntegrationActivationProposed'))
 );
-- +goose StatementEnd
