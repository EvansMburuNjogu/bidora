-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Distinct command identity; a replay of an issued invitation must never send SMTP.
ALTER TABLE platform_administration.commands DROP CONSTRAINT commands_operation_check;
ALTER TABLE platform_administration.commands ADD CONSTRAINT commands_operation_check CHECK(operation IN(
 'INVITATION_ISSUE','INVITATION_CANCEL','INVITATION_ACCEPT','INVITATION_REISSUE',
 'ADMINISTRATOR_CHANGE_CREATE','ADMINISTRATOR_CHANGE_APPROVE','ADMINISTRATOR_CHANGE_REJECT',
 'ADMINISTRATOR_SESSIONS_REVOKE'));
DO $$ BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=48,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=47;
 IF NOT FOUND THEN RAISE EXCEPTION 'control compatibility must be at 47'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'invitation reissue command evidence requires forward recovery'; END $$;
-- +goose StatementEnd
