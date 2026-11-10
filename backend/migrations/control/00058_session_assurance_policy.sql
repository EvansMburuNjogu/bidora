-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE platform_settings.session_policy ADD COLUMN step_up_seconds integer NOT NULL DEFAULT 900 CHECK(step_up_seconds BETWEEN 300 AND 3600 AND step_up_seconds%60=0);
-- No identity, verified-email timestamp, existing session or assurance timestamp
-- is extended or reset. This is a separate sensitive-action freshness policy.
-- Fresh checkouts advance from 56; existing sandbox databases may already be at 57.
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=58,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version IN (56,57);
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 56 or 57'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DO $$ BEGIN RAISE EXCEPTION 'Forward-only migration: retain schema 58 with a compatible application rollback'; END $$;
