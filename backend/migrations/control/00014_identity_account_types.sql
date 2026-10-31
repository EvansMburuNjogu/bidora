-- +goose Up
-- +goose StatementBegin
-- BID-AUTH-003A: one immutable portal-routing account type per verified identity.
-- Account type is a routing fact only; it never grants platform or tenant authority.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE identity.users
    ADD COLUMN account_type text
    CHECK (account_type IN ('PLATFORM_ADMIN','PROCUREMENT','TENDERPRENEUR'));

-- Existing Option A users with a historical Tenderpreneur membership retain their
-- established portal. Identities without membership remain explicitly unassigned.
UPDATE identity.users AS u
SET account_type = 'TENDERPRENEUR',
    updated_at = GREATEST(u.updated_at, CURRENT_TIMESTAMP)
WHERE u.account_type IS NULL
  AND EXISTS (
      SELECT 1
      FROM tenancy.memberships AS m
      JOIN bidora_tenancy.tenants AS t ON t.id = m.tenant_id
      WHERE m.user_id = u.id
        AND t.tenant_type = 'TENDERPRENEUR'
  );

CREATE FUNCTION identity.protect_user_account_type() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.account_type IS NOT NULL
       OR NEW.account_type IS NULL
       OR NEW.updated_at < OLD.updated_at THEN
        RAISE EXCEPTION 'account type transition invalid' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER users_protect_account_type
BEFORE UPDATE OF account_type ON identity.users
FOR EACH ROW EXECUTE FUNCTION identity.protect_user_account_type();

CREATE TABLE identity.account_type_selection_commands (
    id uuid PRIMARY KEY,
    actor_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    key_hash bytea NOT NULL CHECK (octet_length(key_hash) = 32),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    selected_account_type text NOT NULL CHECK (selected_account_type IN ('PROCUREMENT','TENDERPRENEUR')),
    response_status integer NOT NULL CHECK (response_status = 204),
    created_at timestamptz NOT NULL,
    completed_at timestamptz NOT NULL,
    UNIQUE (actor_user_id, key_hash),
    CHECK (completed_at >= created_at)
);

CREATE FUNCTION identity.reject_account_type_command_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'account type selection commands are immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER account_type_selection_commands_immutable
BEFORE UPDATE OR DELETE ON identity.account_type_selection_commands
FOR EACH ROW EXECUTE FUNCTION identity.reject_account_type_command_mutation();

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 14, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 13;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 13';
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
SET schema_version = 13, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 14;
DROP TABLE identity.account_type_selection_commands;
DROP FUNCTION identity.reject_account_type_command_mutation();
DROP TRIGGER users_protect_account_type ON identity.users;
DROP FUNCTION identity.protect_user_account_type();
ALTER TABLE identity.users DROP COLUMN account_type;
-- +goose StatementEnd
