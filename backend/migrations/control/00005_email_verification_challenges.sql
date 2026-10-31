-- +goose Up
-- +goose StatementBegin
-- Approved bounded lifecycle: 32-byte opaque token, SHA-256 at rest, 30-minute
-- expiry, one active EMAIL_VERIFY challenge per user, single-use consumption.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE TABLE identity.challenges (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    purpose text NOT NULL CHECK (purpose = 'EMAIL_VERIFY'),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    invalidated_at timestamptz,
    created_at timestamptz NOT NULL,
    CHECK (expires_at > created_at),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at),
    CHECK (invalidated_at IS NULL OR invalidated_at >= created_at),
    CHECK (consumed_at IS NULL OR invalidated_at IS NULL)
);
CREATE UNIQUE INDEX challenges_one_active_user_purpose_idx
    ON identity.challenges (user_id, purpose)
    WHERE consumed_at IS NULL AND invalidated_at IS NULL;
CREATE INDEX challenges_user_time_idx
    ON identity.challenges (user_id, created_at DESC, id DESC);
COMMENT ON TABLE identity.challenges IS
'Raw tokens are forbidden. Metadata has no automatic deletion pending retention approval.';

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 5, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 4;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 4';
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
SET schema_version = 4, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 5;
DROP TABLE identity.challenges;
-- +goose StatementEnd
