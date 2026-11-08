-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- New verified-Owner invitations are direct for every role. Historical approval
-- evidence and invitation states remain unchanged; no request is auto-approved.
DO $$ DECLARE constraint_name text; BEGIN
 SELECT conname INTO STRICT constraint_name FROM pg_constraint
 WHERE conrelid='platform_administration.administrator_invitations'::regclass AND contype='c'
 AND pg_get_constraintdef(oid) LIKE '%role_key%'
 AND pg_get_constraintdef(oid) LIKE '%authorization_path%';
 EXECUTE format('ALTER TABLE platform_administration.administrator_invitations DROP CONSTRAINT %I',constraint_name);
END $$;
ALTER TABLE platform_administration.administrator_invitations ADD CONSTRAINT invitations_role_authorization_v2 CHECK (
 authorization_path='OWNER_DIRECT' OR (role_key='PLATFORM_OWNER' AND authorization_path IN ('FIRST_OWNER_EXCEPTION','MAKER_CHECKER'))
);
UPDATE bidora_control_meta.schema_compatibility SET schema_version=56,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=55;
-- +goose StatementEnd

-- +goose Down
DO $$ BEGIN RAISE EXCEPTION 'Forward-only migration: use a schema-56-compatible application rollback'; END $$;
