-- +goose Up
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE identity.users
    ADD COLUMN first_name text,
    ADD COLUMN last_name text,
    ADD CONSTRAINT users_personal_names_pair_check CHECK (
        (first_name IS NULL AND last_name IS NULL)
        OR (
            first_name IS NOT NULL AND last_name IS NOT NULL
            AND octet_length(first_name) BETWEEN 1 AND 100
            AND octet_length(last_name) BETWEEN 1 AND 100
            AND first_name = btrim(first_name)
            AND last_name = btrim(last_name)
            AND first_name !~ '[[:cntrl:]]'
            AND last_name !~ '[[:cntrl:]]'
        )
    );

COMMENT ON COLUMN identity.users.first_name IS
'User-supplied display spelling. Nullable only for identities created before the personal-name registration contract or through providers that do not supply an approved name.';
COMMENT ON COLUMN identity.users.last_name IS
'User-supplied display spelling. Nullable only for identities created before the personal-name registration contract or through providers that do not supply an approved name.';

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 29, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 28;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 28';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Existing names are not destructively discarded by rollback.
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM identity.users WHERE first_name IS NOT NULL OR last_name IS NOT NULL) THEN
        RAISE EXCEPTION 'personal-name data prevents downgrade';
    END IF;
END;
$$;
UPDATE bidora_control_meta.schema_compatibility
SET schema_version = 28, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 29;
ALTER TABLE identity.users DROP CONSTRAINT users_personal_names_pair_check;
ALTER TABLE identity.users DROP COLUMN last_name;
ALTER TABLE identity.users DROP COLUMN first_name;
-- +goose StatementEnd
