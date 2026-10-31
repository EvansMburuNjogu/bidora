-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Migration-owned, security-barrier projection. Runtime SELECT is on this view,
-- never the audit base table. No summaries, tenant rows or subject identifiers.
CREATE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
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
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=36,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=35;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 35'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=35,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=36;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 36'; END IF;
END $$;
DROP VIEW bidora_control_audit.platform_action_times_v1;
-- +goose StatementEnd
