-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE platform_settings.countries (
 country_code char(2) PRIMARY KEY CHECK(country_code ~ '^[A-Z]{2}$'),currency char(3) NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),language text NOT NULL,timezone text NOT NULL,locale text NOT NULL,enabled boolean NOT NULL,is_default boolean NOT NULL DEFAULT false,version bigint NOT NULL CHECK(version>0),updated_by uuid REFERENCES identity.users(id),updated_at timestamptz NOT NULL,
 CHECK(NOT is_default OR enabled),CHECK(country_code<>'KE' OR currency='KES')
);
CREATE UNIQUE INDEX one_default_country ON platform_settings.countries(is_default) WHERE is_default;
INSERT INTO platform_settings.countries VALUES('KE','KES','en','Africa/Nairobi','en-KE',true,true,1,NULL,CURRENT_TIMESTAMP);
CREATE TABLE platform_settings.country_versions (country_code char(2) NOT NULL,version bigint NOT NULL,currency char(3) NOT NULL,language text NOT NULL,timezone text NOT NULL,locale text NOT NULL,enabled boolean NOT NULL,is_default boolean NOT NULL,updated_by uuid REFERENCES identity.users(id),updated_at timestamptz NOT NULL,PRIMARY KEY(country_code,version));
INSERT INTO platform_settings.country_versions SELECT country_code,version,currency,language,timezone,locale,enabled,is_default,updated_by,updated_at FROM platform_settings.countries;
CREATE FUNCTION platform_settings.record_country_version() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$ BEGIN INSERT INTO platform_settings.country_versions VALUES(NEW.country_code,NEW.version,NEW.currency,NEW.language,NEW.timezone,NEW.locale,NEW.enabled,NEW.is_default,NEW.updated_by,NEW.updated_at);RETURN NEW;END $$;
REVOKE ALL ON FUNCTION platform_settings.record_country_version() FROM PUBLIC;
CREATE TRIGGER country_version AFTER INSERT OR UPDATE ON platform_settings.countries FOR EACH ROW EXECUTE FUNCTION platform_settings.record_country_version();
CREATE TABLE platform_settings.operational_policy (
 singleton boolean PRIMARY KEY CHECK(singleton),version bigint NOT NULL CHECK(version>0),organization_registration_enabled boolean NOT NULL,ai_features_enabled boolean NOT NULL,document_uploads_enabled boolean NOT NULL,max_document_bytes bigint NOT NULL CHECK(max_document_bytes BETWEEN 1048576 AND 26214400),maintenance_enabled boolean NOT NULL,maintenance_message text NOT NULL CHECK(length(maintenance_message)<=240),maintenance_starts_at timestamptz,maintenance_ends_at timestamptz,restricted_metadata_retention_days integer NOT NULL CHECK(restricted_metadata_retention_days BETWEEN 7 AND 90),updated_by uuid REFERENCES identity.users(id),updated_at timestamptz NOT NULL,
 CHECK(maintenance_ends_at IS NULL OR maintenance_starts_at IS NULL OR maintenance_ends_at>maintenance_starts_at)
);
INSERT INTO platform_settings.operational_policy VALUES(true,1,true,true,true,26214400,false,'',NULL,NULL,90,NULL,CURRENT_TIMESTAMP);
CREATE TABLE platform_settings.operational_policy_versions (version bigint PRIMARY KEY,value jsonb NOT NULL,updated_by uuid REFERENCES identity.users(id),updated_at timestamptz NOT NULL);
INSERT INTO platform_settings.operational_policy_versions SELECT version,to_jsonb(p)-'updated_by'-'singleton',updated_by,updated_at FROM platform_settings.operational_policy p;
CREATE TABLE platform_settings.regional_policy_commands(actor_id uuid NOT NULL REFERENCES identity.users(id),kind text NOT NULL CHECK(kind IN('COUNTRY','OPERATIONS')),key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),result_version bigint NOT NULL CHECK(result_version>0),target text NOT NULL,created_at timestamptz NOT NULL,PRIMARY KEY(actor_id,kind,key_hash));
-- Restricted request metadata is separate from immutable action events. No
-- deletion worker, legal hold or production collection is activated here.
CREATE TABLE bidora_control_audit.request_metadata(correlation_id uuid PRIMARY KEY,actor_id uuid NOT NULL REFERENCES identity.users(id),ip_address inet NOT NULL,browser text NOT NULL CHECK(length(browser)<=512),observed_at timestamptz NOT NULL);
CREATE INDEX restricted_request_metadata_time ON bidora_control_audit.request_metadata(observed_at);
CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH (security_barrier=true) AS
SELECT e.id,e.actor_id,e.action,e.occurred_at,
 CASE WHEN e.target_type IN ('platform_integration_setting','platform_google_configuration','notification_template') THEN 'INTEGRATIONS' ELSE 'ADMINISTRATION' END::text AS module,
 CASE WHEN e.target_type IN ('platform_administrator_invitation','platform_administrator_change','platform_integration_setting','platform_google_configuration','notification_template','platform_regional_policy') THEN e.target_id ELSE NULL END::uuid AS affected_record_id,
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
  OR (e.outcome='SUCCEEDED' AND
      ((e.target_type='session' AND e.action IN ('UserLoggedIn','EmailLoginCodeVerified'))
        OR (e.target_type='mfa_factor' AND e.action='MFALoginCompleted'))
      AND EXISTS (SELECT 1 FROM identity.users AS u WHERE u.id=e.actor_id AND u.account_type='PLATFORM_ADMIN'))
 );

CREATE VIEW bidora_control_audit.platform_request_metadata_v1 WITH(security_barrier=true) AS SELECT a.id AS audit_id,m.ip_address,m.browser,m.observed_at FROM bidora_control_audit.platform_action_times_v1 a JOIN bidora_control_audit.audit_events e ON e.id=a.id AND e.tenant_id IS NULL JOIN bidora_control_audit.request_metadata m ON m.correlation_id=e.correlation_id AND m.actor_id=e.actor_id;

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=41,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=40;
 GET DIAGNOSTICS affected=ROW_COUNT;IF affected<>1 THEN RAISE EXCEPTION 'control compatibility must be 40';END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'settings histories require forward recovery';END $$;
-- +goose StatementEnd
