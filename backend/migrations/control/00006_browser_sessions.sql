-- +goose Up
-- +goose StatementBegin
-- Approved browser session foundation: 32-byte opaque session/CSRF tokens,
-- 30-minute idle and 12-hour absolute lifetimes. Raw tokens are forbidden.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE TABLE identity.sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_hash bytea NOT NULL CHECK (octet_length(csrf_hash) = 32),
    auth_epoch bigint NOT NULL CHECK (auth_epoch > 0),
    rotated_from_session_id uuid UNIQUE REFERENCES identity.sessions(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    idle_expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (last_seen_at >= created_at),
    CHECK (absolute_expires_at = created_at + interval '12 hours'),
    CHECK (idle_expires_at <= LEAST(last_seen_at + interval '30 minutes', absolute_expires_at)),
    CHECK (idle_expires_at > last_seen_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE INDEX sessions_user_active_idx ON identity.sessions (user_id, absolute_expires_at, id)
    WHERE revoked_at IS NULL;
COMMENT ON TABLE identity.sessions IS
'Only domain-separated token hashes are stored. User/auth epoch remain authoritative; no raw cookie or CSRF value.';

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 6, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 5;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 5';
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
SET schema_version = 5, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 6;
DROP TABLE identity.sessions;
-- +goose StatementEnd
