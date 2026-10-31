-- +goose Up
-- +goose StatementBegin
-- Preconditions: control_registry is at schema version 3. Existing hypothetical
-- rows require reconciliation before constraint validation or authentication.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE identity.users
    ADD CONSTRAINT users_email_pilot_form CHECK (
        octet_length(email) BETWEEN 3 AND 254
        AND email !~ '[^!-~]'
    ) NOT VALID;
ALTER TABLE identity.users
    ADD CONSTRAINT users_email_lookup_pilot_form CHECK (
        octet_length(email_lookup) BETWEEN 3 AND 254
        AND email_lookup !~ '[^!-~]'
        AND email_lookup = lower(email_lookup)
    ) NOT VALID;
COMMENT ON CONSTRAINT users_email_lookup_pilot_form ON identity.users IS
'Application validates ASCII mailbox syntax; lookup lowercases the full address. No provider alias rewriting.';

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 4, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 3;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 3';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
UPDATE bidora_control_meta.schema_compatibility
SET schema_version = 3, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 4;
ALTER TABLE identity.users DROP CONSTRAINT IF EXISTS users_email_lookup_pilot_form;
ALTER TABLE identity.users DROP CONSTRAINT IF EXISTS users_email_pilot_form;
-- +goose StatementEnd
