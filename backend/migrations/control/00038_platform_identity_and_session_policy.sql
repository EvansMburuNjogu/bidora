-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE platform_settings.platform_information
 ADD COLUMN platform_name text NOT NULL DEFAULT 'Bidora' CHECK (char_length(platform_name) BETWEEN 1 AND 120 AND platform_name=btrim(platform_name) AND platform_name !~ '[[:cntrl:]]'),
 ADD COLUMN contact_phone text NOT NULL DEFAULT '' CHECK (contact_phone='' OR (contact_phone=btrim(contact_phone) AND contact_phone ~ '^[+]?[0-9 ()-]{7,32}$' AND length(regexp_replace(contact_phone,'[^0-9]','','g'))>=7));

-- Only stricter ordinary-session limits are configurable. Existing protected
-- seven-day administrator profiles, MFA, password and browser controls remain.
CREATE TABLE platform_settings.session_policy (
 id boolean PRIMARY KEY DEFAULT true CHECK(id),
 idle_seconds integer NOT NULL CHECK(idle_seconds IN (300,600,900,1800)),
 absolute_seconds integer NOT NULL CHECK(absolute_seconds IN (3600,14400,28800,43200) AND absolute_seconds>=idle_seconds),
 version bigint NOT NULL CHECK(version>0),
 updated_at timestamptz, updated_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT
);
INSERT INTO platform_settings.session_policy(id,idle_seconds,absolute_seconds,version) VALUES(true,1800,43200,1);
CREATE TABLE platform_settings.session_policy_commands (
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 result_version bigint NOT NULL CHECK(result_version>1),
 PRIMARY KEY(actor_id,key_hash)
);
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
  OR (e.target_type='platform_information' AND e.action='PlatformInformationUpdated')
  OR (e.target_type='platform_session_policy' AND e.action='PlatformSessionPolicyUpdated')
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=38,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=37;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 37'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Never auto-run Down: restore/forward repair preserves settings and receipts.
DO $$ BEGIN RAISE EXCEPTION 'settings history requires forward recovery'; END $$;
-- +goose StatementEnd
