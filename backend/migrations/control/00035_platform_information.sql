-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE platform_settings.platform_information (
 id uuid PRIMARY KEY CHECK (id='00000000-0000-4000-8000-000000000035'::uuid),
 support_email text CHECK (support_email IS NULL OR (octet_length(support_email) BETWEEN 3 AND 254 AND support_email=btrim(support_email))),
 website text CHECK (website IS NULL OR (octet_length(website) BETWEEN 9 AND 512 AND website LIKE 'https://%')),
 version bigint NOT NULL CHECK (version>0),
 updated_at timestamptz,
 updated_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
 CHECK ((updated_at IS NULL AND updated_by IS NULL AND support_email IS NULL AND website IS NULL) OR (updated_at IS NOT NULL AND updated_by IS NOT NULL AND support_email IS NOT NULL AND website IS NOT NULL))
);
INSERT INTO platform_settings.platform_information(id,version) VALUES ('00000000-0000-4000-8000-000000000035',1);
CREATE TABLE platform_settings.platform_information_commands (
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 key_hash bytea NOT NULL CHECK (octet_length(key_hash)=32),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
 result_version bigint NOT NULL CHECK (result_version>1),
 PRIMARY KEY(actor_id,key_hash)
);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=35,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=34;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 34'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM platform_settings.platform_information_commands) OR EXISTS(SELECT 1 FROM platform_settings.platform_information WHERE version>1) THEN RAISE EXCEPTION 'platform information history prevents downgrade'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=34,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=35;
DROP TABLE platform_settings.platform_information_commands;
DROP TABLE platform_settings.platform_information;
-- +goose StatementEnd
