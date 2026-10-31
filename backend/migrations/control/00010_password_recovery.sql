-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE identity.password_recovery_challenges (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 purpose text NOT NULL CHECK(purpose='PASSWORD_RECOVERY'),
 token_hash bytea NOT NULL UNIQUE CHECK(octet_length(token_hash)=32),
 expires_at timestamptz NOT NULL,
 consumed_at timestamptz,
 created_at timestamptz NOT NULL,
 CHECK(expires_at=created_at+interval '30 minutes'),
 CHECK(consumed_at IS NULL OR consumed_at>=created_at)
);
CREATE UNIQUE INDEX password_recovery_one_active_user_idx ON identity.password_recovery_challenges(user_id) WHERE consumed_at IS NULL;
CREATE INDEX password_recovery_expiry_idx ON identity.password_recovery_challenges(expires_at,id) WHERE consumed_at IS NULL;
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=10,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=9;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 9'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_control_meta.schema_compatibility SET schema_version=9,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=10;
DROP TABLE identity.password_recovery_challenges;
-- +goose StatementEnd
