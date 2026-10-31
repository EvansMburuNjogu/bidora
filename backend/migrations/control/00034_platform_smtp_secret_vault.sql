-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE platform_settings.smtp_secrets(
 id uuid PRIMARY KEY,
 secret_kind text NOT NULL CHECK(secret_kind IN('CREDENTIALS','TRUST_ROOTS')),
 key_version integer NOT NULL CHECK(key_version>0),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 1048592),
 created_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL,
 retired_at timestamptz,
 CHECK(retired_at IS NULL OR retired_at>=created_at)
);
COMMENT ON TABLE platform_settings.smtp_secrets IS 'AES-256-GCM encrypted SMTP secret material. Keys are deployment supplied and never stored in PostgreSQL.';
CREATE FUNCTION platform_settings.protect_smtp_secret() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='DELETE' OR NEW.id IS DISTINCT FROM OLD.id OR NEW.secret_kind IS DISTINCT FROM OLD.secret_kind OR NEW.key_version IS DISTINCT FROM OLD.key_version OR NEW.nonce IS DISTINCT FROM OLD.nonce OR NEW.ciphertext IS DISTINCT FROM OLD.ciphertext OR NEW.created_by IS DISTINCT FROM OLD.created_by OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
  RAISE EXCEPTION 'platform SMTP secret material is immutable' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER smtp_secrets_protected BEFORE UPDATE OR DELETE ON platform_settings.smtp_secrets FOR EACH ROW EXECUTE FUNCTION platform_settings.protect_smtp_secret();
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=34,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=33;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 33'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM platform_settings.smtp_secrets) THEN RAISE EXCEPTION 'retained SMTP secrets prevent downgrade'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=33,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=34;
DROP TRIGGER smtp_secrets_protected ON platform_settings.smtp_secrets;
DROP FUNCTION platform_settings.protect_smtp_secret();
DROP TABLE platform_settings.smtp_secrets;
-- +goose StatementEnd
