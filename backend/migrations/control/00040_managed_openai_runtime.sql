-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE platform_settings.smtp_secrets DROP CONSTRAINT smtp_secrets_secret_kind_check;
ALTER TABLE platform_settings.smtp_secrets ADD CONSTRAINT smtp_secrets_secret_kind_check CHECK(secret_kind IN('CREDENTIALS','TRUST_ROOTS','OPENAI_API_KEY'));
COMMENT ON TABLE platform_settings.smtp_secrets IS 'Immutable encrypted integration vault material; typed SMTP/OpenAI references and AAD. Deployment keys never stored here.';
ALTER TABLE platform_settings.openai_versions ADD COLUMN enabled boolean NOT NULL DEFAULT true;
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=40,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=39;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility must be 39';END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'managed settings require forward recovery'; END $$;
-- +goose StatementEnd
