-- +goose Up
-- +goose StatementBegin
-- Google OIDC addition: stable provider subject binding. Email is retained only
-- as the verified claim spelling observed when the binding is created; it is
-- never sufficient to link an existing account.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE TABLE identity.external_identities (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    provider text NOT NULL CHECK (provider = 'GOOGLE'),
    provider_subject text NOT NULL CHECK (
        length(provider_subject) BETWEEN 1 AND 255
        AND provider_subject = btrim(provider_subject)
    ),
    email_at_link text NOT NULL CHECK (
        length(email_at_link) BETWEEN 3 AND 254
        AND email_at_link = btrim(email_at_link)
    ),
    created_at timestamptz NOT NULL,
    last_authenticated_at timestamptz NOT NULL,
    UNIQUE (provider, provider_subject),
    UNIQUE (user_id, provider),
    CHECK (last_authenticated_at >= created_at)
);
COMMENT ON COLUMN identity.external_identities.provider_subject IS
'Google OIDC sub; stable binding authority. A matching email alone must never create this row.';
COMMENT ON COLUMN identity.external_identities.email_at_link IS
'Verified Google claim observed at link time; not an automatic canonical-email update authority.';

DO $$
DECLARE affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 8, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 7;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 7';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
UPDATE bidora_control_meta.schema_compatibility
SET schema_version = 7, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 8;
DROP TABLE identity.external_identities;
-- +goose StatementEnd
