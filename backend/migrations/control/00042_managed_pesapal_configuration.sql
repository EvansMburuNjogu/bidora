-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE platform_settings.integration_heads DROP CONSTRAINT integration_heads_setting_kind_check;
ALTER TABLE platform_settings.integration_heads ADD CONSTRAINT integration_heads_setting_kind_check CHECK(setting_kind IN('SMTP','OPENAI','PESAPAL'));
ALTER TABLE platform_settings.smtp_secrets DROP CONSTRAINT smtp_secrets_secret_kind_check;
ALTER TABLE platform_settings.smtp_secrets ADD CONSTRAINT smtp_secrets_secret_kind_check CHECK(secret_kind IN('CREDENTIALS','TRUST_ROOTS','OPENAI_API_KEY','PESAPAL_CREDENTIALS'));
ALTER TABLE platform_settings.integration_test_attempts DROP CONSTRAINT integration_test_attempts_result_code_check;
ALTER TABLE platform_settings.integration_test_attempts ADD CONSTRAINT integration_test_attempts_result_code_check CHECK(result_code IN('SMTP_STARTTLS_VERIFIED','OPENAI_STRUCTURED_RESPONSE_VERIFIED','OPENAI_DISABLED_CONFIG_VERIFIED','PESAPAL_AUTHENTICATION_VERIFIED','PESAPAL_DISABLED_CONFIG_VERIFIED','CONNECTIVITY_FAILED','AUTHENTICATION_FAILED','TLS_FAILED','PROVIDER_REJECTED','INVALID_RESPONSE'));
INSERT INTO platform_settings.integration_heads(setting_kind,updated_at)VALUES('PESAPAL',CURRENT_TIMESTAMP);
CREATE TABLE platform_settings.pesapal_versions(
 version_id uuid PRIMARY KEY REFERENCES platform_settings.integration_versions(id) ON DELETE RESTRICT,
 enabled boolean NOT NULL,environment text NOT NULL CHECK(environment IN('SANDBOX','LIVE')),
 merchant_name text NOT NULL CHECK(length(merchant_name) BETWEEN 1 AND 120),
 credentials_secret_ref text NOT NULL CHECK(length(credentials_secret_ref) BETWEEN 1 AND 500),
 notification_id uuid,
 CHECK(NOT enabled OR(notification_id IS NOT NULL AND notification_id<>'00000000-0000-0000-0000-000000000000'))
);
CREATE TRIGGER pesapal_versions_immutable BEFORE UPDATE OR DELETE ON platform_settings.pesapal_versions FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
-- Untrusted provider/browser callbacks can only retain bounded tracking hints
-- and wake the existing authoritative-query reconciliation state machine.
CREATE TABLE commercial.payment_callback_hints(payment_id uuid NOT NULL REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,tracking_id uuid NOT NULL,received_at timestamptz NOT NULL,PRIMARY KEY(payment_id,tracking_id));
CREATE TRIGGER payment_callback_hints_immutable BEFORE UPDATE OR DELETE ON commercial.payment_callback_hints FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=42,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=41; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 41'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'managed PesaPal recovery requires compatible code or a reviewed forward migration'; END $$;
-- +goose StatementEnd
