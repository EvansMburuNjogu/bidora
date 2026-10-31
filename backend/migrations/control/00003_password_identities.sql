-- +goose Up
-- +goose StatementBegin
-- Selected identity path: Go-owned email/password with Argon2id. This migration
-- does not define registration, verification, recovery, MFA or session policy.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA identity;

CREATE TABLE identity.users (
    id uuid PRIMARY KEY,
    email text NOT NULL CHECK (length(email) BETWEEN 3 AND 320 AND email = btrim(email)),
    email_lookup text NOT NULL UNIQUE CHECK (length(email_lookup) BETWEEN 3 AND 320 AND email_lookup = btrim(email_lookup)),
    email_verified_at timestamptz,
    disabled_at timestamptz,
    auth_epoch bigint NOT NULL DEFAULT 1 CHECK (auth_epoch > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at),
    CHECK (email_verified_at IS NULL OR email_verified_at >= created_at),
    CHECK (disabled_at IS NULL OR disabled_at >= created_at)
);
COMMENT ON COLUMN identity.users.email_lookup IS
'Deterministic approved lookup form supplied by identity application policy; never display this value.';
COMMENT ON COLUMN identity.users.auth_epoch IS
'Increment to invalidate sessions after approved security transitions; sessions are not defined by this migration.';

CREATE TABLE identity.authentication_identities (
    user_id uuid PRIMARY KEY REFERENCES identity.users(id) ON DELETE RESTRICT,
    password_phc text NOT NULL CHECK (
        length(password_phc) BETWEEN 32 AND 1024
        AND password_phc LIKE '$argon2id$v=19$%'
    ),
    credential_version bigint NOT NULL DEFAULT 1 CHECK (credential_version > 0),
    password_changed_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at),
    CHECK (password_changed_at >= created_at AND password_changed_at <= updated_at)
);
COMMENT ON COLUMN identity.authentication_identities.password_phc IS
'Argon2id PHC string only; plaintext passwords, recovery material and tokens are forbidden.';

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 3, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 2;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 2';
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
SET schema_version = 2, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 3;
DROP SCHEMA identity CASCADE;
-- +goose StatementEnd
