-- +goose Up
-- +goose StatementBegin
-- Approved narrow pilot invitation: one fixed capture-contributor grant.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE TABLE tenancy.invitations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
    email text NOT NULL CHECK (length(email) BETWEEN 3 AND 254),
    email_lookup text NOT NULL CHECK (
        length(email_lookup) BETWEEN 3 AND 254
        AND email_lookup = lower(email_lookup)
        AND email_lookup !~ '[^\x20-\x7E]'
    ),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    role_key text NOT NULL CHECK (role_key = 'TENDERPRENEUR_CAPTURE_CONTRIBUTOR'),
    capability text NOT NULL CHECK (capability = 'tenderpreneur.opportunity.capture_external'),
    created_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at = created_at + interval '72 hours'),
    invalidated_at timestamptz,
    consumed_at timestamptz,
    accepted_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
    CHECK (invalidated_at IS NULL OR invalidated_at >= created_at),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at),
    CHECK ((consumed_at IS NULL) = (accepted_by IS NULL)),
    CHECK (NOT (invalidated_at IS NOT NULL AND consumed_at IS NOT NULL))
);
CREATE UNIQUE INDEX invitations_one_active_email_idx
ON tenancy.invitations (tenant_id, email_lookup)
WHERE invalidated_at IS NULL AND consumed_at IS NULL;
CREATE INDEX invitations_tenant_time_idx
ON tenancy.invitations (tenant_id, created_at DESC, id DESC);

CREATE FUNCTION tenancy.protect_invitation_facts() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'invitation history is retained' USING ERRCODE = '55000';
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.email IS DISTINCT FROM OLD.email
       OR NEW.email_lookup IS DISTINCT FROM OLD.email_lookup
       OR NEW.token_hash IS DISTINCT FROM OLD.token_hash
       OR NEW.role_key IS DISTINCT FROM OLD.role_key
       OR NEW.capability IS DISTINCT FROM OLD.capability
       OR NEW.created_by IS DISTINCT FROM OLD.created_by
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.expires_at IS DISTINCT FROM OLD.expires_at
       OR OLD.invalidated_at IS NOT NULL
       OR OLD.consumed_at IS NOT NULL
       OR (NEW.invalidated_at IS NULL AND NEW.consumed_at IS NULL)
       OR (NEW.invalidated_at IS NOT NULL AND (NEW.consumed_at IS NOT NULL OR NEW.accepted_by IS NOT NULL))
       OR (NEW.consumed_at IS NOT NULL AND (NEW.invalidated_at IS NOT NULL OR NEW.accepted_by IS NULL)) THEN
        RAISE EXCEPTION 'invitation facts are immutable; only first invalidate or consume is allowed' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER invitations_protect_facts
BEFORE UPDATE OR DELETE ON tenancy.invitations
FOR EACH ROW EXECUTE FUNCTION tenancy.protect_invitation_facts();

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 9, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 8;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 8';
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
SET schema_version = 8, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 9;
DROP TABLE tenancy.invitations;
DROP FUNCTION tenancy.protect_invitation_facts();
-- +goose StatementEnd
