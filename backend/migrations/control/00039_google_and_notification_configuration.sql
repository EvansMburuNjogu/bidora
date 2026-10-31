-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE platform_settings.google_versions (
 id uuid PRIMARY KEY, version bigint NOT NULL UNIQUE CHECK(version>0),
 enabled boolean NOT NULL, allow_signup boolean NOT NULL,
 client_id text NOT NULL CHECK(length(client_id)<=255), allowed_domains text[] NOT NULL CHECK(cardinality(allowed_domains)<=50),
 secret_id uuid, key_version integer, nonce bytea, ciphertext bytea,
 created_at timestamptz NOT NULL, created_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
 CHECK ((secret_id IS NULL AND key_version IS NULL AND nonce IS NULL AND ciphertext IS NULL) OR
 (secret_id IS NOT NULL AND key_version IS NOT NULL AND nonce IS NOT NULL AND ciphertext IS NOT NULL AND key_version>0 AND octet_length(nonce)=12 AND octet_length(ciphertext) BETWEEN 17 AND 1040)),
 CHECK(NOT enabled OR (client_id<>'' AND secret_id IS NOT NULL)), CHECK(NOT allow_signup OR enabled)
);
INSERT INTO platform_settings.google_versions(id,version,enabled,allow_signup,client_id,allowed_domains,created_at)
VALUES('00000000-0000-4000-8000-000000000039',1,false,false,'',ARRAY[]::text[],CURRENT_TIMESTAMP);
CREATE TABLE platform_settings.google_head(id boolean PRIMARY KEY CHECK(id),version_id uuid NOT NULL REFERENCES platform_settings.google_versions(id),version bigint NOT NULL CHECK(version>0));
INSERT INTO platform_settings.google_head VALUES(true,'00000000-0000-4000-8000-000000000039',1);
CREATE TABLE platform_settings.google_tests(id uuid PRIMARY KEY,version_id uuid NOT NULL REFERENCES platform_settings.google_versions(id),actor_id uuid NOT NULL REFERENCES identity.users(id),status text NOT NULL CHECK(status IN ('JWKS_REACHABLE_CREDENTIALS_UNVERIFIED','UNAVAILABLE')),created_at timestamptz NOT NULL);
CREATE INDEX google_tests_actor_time ON platform_settings.google_tests(actor_id,created_at DESC);
CREATE TABLE platform_settings.google_commands(actor_id uuid NOT NULL REFERENCES identity.users(id),operation text NOT NULL CHECK(operation IN ('SAVE','TEST')),key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),version_id uuid NOT NULL REFERENCES platform_settings.google_versions(id),result_version bigint NOT NULL CHECK(result_version>0),state text NOT NULL CHECK(state IN ('PENDING','COMPLETED')),result_code text NOT NULL,created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(actor_id,operation,key_hash));
CREATE INDEX google_commands_test_actor_time ON platform_settings.google_commands(actor_id,created_at DESC) WHERE operation='TEST';

CREATE TABLE notification.templates (
 template_key text PRIMARY KEY, name text NOT NULL CHECK(length(name) BETWEEN 1 AND 80),category text NOT NULL,
 channel text NOT NULL CHECK(channel='EMAIL'),mandatory boolean NOT NULL,enabled boolean NOT NULL,
 subject text NOT NULL CHECK(length(subject) BETWEEN 1 AND 200 AND subject !~ '[[:cntrl:]]'),
 body text NOT NULL CHECK(length(body) BETWEEN 1 AND 4000),version bigint NOT NULL CHECK(version>0),
 updated_at timestamptz NOT NULL,updated_by uuid REFERENCES identity.users(id),CHECK(NOT mandatory OR enabled)
);
INSERT INTO notification.templates(template_key,name,category,channel,mandatory,enabled,subject,body,version,updated_at) VALUES
('identity.email_verification','Email verification','EMAIL_VERIFICATION','EMAIL',true,true,'Verify your {{platform_name}} email','Use this verification token: {{token}}. Expires: {{expires_at}}.',1,CURRENT_TIMESTAMP),
('identity.password_recovery','Password recovery','PASSWORD_RECOVERY','EMAIL',true,true,'Reset your {{platform_name}} password','Use this password recovery token: {{token}}. Expires: {{expires_at}}.',1,CURRENT_TIMESTAMP),
('identity.login_code','Sign-in code','EMAIL_LOGIN','EMAIL',true,true,'Your {{platform_name}} sign-in code','Your sign-in code is {{code}}. Expires: {{expires_at}}.',1,CURRENT_TIMESTAMP),
('tenant.invitation','Organisation invitation','TENANT_INVITATION','EMAIL',true,true,'Your {{platform_name}} invitation','You were invited as {{role}}. Invitation code: {{token}}. Expires: {{expires_at}}.',1,CURRENT_TIMESTAMP),
('platform.invitation.issued','Administrator invitation','PLATFORM_INVITATION','EMAIL',true,true,'Your {{platform_name}} administrator invitation','You were invited as {{role}}. Invitation code: {{token}}. Expires: {{expires_at}}.',1,CURRENT_TIMESTAMP),
('platform.invitation.reminder','Administrator invitation reminder','PLATFORM_INVITATION_REMINDER','EMAIL',false,true,'Your {{platform_name}} invitation reminder','Your {{role}} administrator invitation is awaiting acceptance. Expires: {{expires_at}}.',1,CURRENT_TIMESTAMP),
('platform.session_revocation','Administrator sessions revoked','PLATFORM_SESSION_REVOCATION','EMAIL',true,true,'{{platform_name}} sessions revoked','Your administrator sessions were revoked at {{occurred_at}}. Sign in again and complete TOTP. Contact an authorised Platform Owner if unexpected.',1,CURRENT_TIMESTAMP),
('platform.identity_state','Administrator access changed','PLATFORM_IDENTITY_STATE_CHANGE','EMAIL',true,true,'{{platform_name}} access changed','Your administrator access was {{action}} at {{occurred_at}}. Contact an authorised Platform Owner if unexpected.',1,CURRENT_TIMESTAMP),
('platform.role_change','Administrator role changed','PLATFORM_ROLE_CHANGE','EMAIL',true,true,'{{platform_name}} role changed','Your {{role}} administrator role was {{action}} at {{occurred_at}}. Contact an authorised Platform Owner if unexpected.',1,CURRENT_TIMESTAMP);
CREATE TABLE notification.template_versions(template_key text NOT NULL REFERENCES notification.templates(template_key),version bigint NOT NULL CHECK(version>0),name text NOT NULL,subject text NOT NULL,body text NOT NULL,enabled boolean NOT NULL,created_at timestamptz NOT NULL,created_by uuid REFERENCES identity.users(id),PRIMARY KEY(template_key,version));
INSERT INTO notification.template_versions SELECT template_key,version,name,subject,body,enabled,updated_at,updated_by FROM notification.templates;
CREATE TABLE notification.template_commands(actor_id uuid NOT NULL REFERENCES identity.users(id),operation text NOT NULL CHECK(operation IN ('SAVE','TEST')),key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),template_key text NOT NULL REFERENCES notification.templates(template_key),result_version bigint NOT NULL,state text NOT NULL CHECK(state IN ('PENDING','COMPLETED')),result_code text NOT NULL,created_at timestamptz NOT NULL,PRIMARY KEY(actor_id,operation,key_hash));
CREATE INDEX template_tests_actor_time ON notification.template_commands(actor_id,created_at DESC) WHERE operation='TEST';
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT e.id,e.actor_id,e.action,e.occurred_at,
 CASE WHEN e.target_type IN ('platform_integration_setting','platform_google_configuration','notification_template') THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN e.target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting','platform_google_configuration','notification_template') THEN e.target_id ELSE NULL END::uuid AS affected_record_id,
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
  OR (e.target_type='platform_information' AND e.action='PlatformInformationUpdated')
  OR (e.target_type='platform_session_policy' AND e.action='PlatformSessionPolicyUpdated')
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=39,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=38;
 GET DIAGNOSTICS affected=ROW_COUNT;IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 38';END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'configuration history requires forward recovery';END $$;
-- +goose StatementEnd
