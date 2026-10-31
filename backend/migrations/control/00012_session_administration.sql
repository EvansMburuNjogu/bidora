-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE identity.session_revocation_commands(
 id uuid PRIMARY KEY,actor_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 response_status integer NOT NULL CHECK(response_status=204),created_at timestamptz NOT NULL,completed_at timestamptz NOT NULL CHECK(completed_at>=created_at),
 UNIQUE(actor_user_id,key_hash)
);
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=12,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=11; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 11'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_control_meta.schema_compatibility SET schema_version=11,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=12;
DROP TABLE identity.session_revocation_commands;
-- +goose StatementEnd
