-- +goose Up
-- +goose StatementBegin
-- Narrow Option A authorization foundation. Role labels are provenance/grouping;
-- authorization evaluates explicit capability effects with deny precedence.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA tenancy;
CREATE TABLE tenancy.memberships (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
    state text NOT NULL CHECK (state IN ('INVITED','ACTIVE','SUSPENDED','REVOKED')),
    context_generation bigint NOT NULL DEFAULT 1 CHECK (context_generation > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (user_id, tenant_id),
    CHECK (updated_at >= created_at)
);
CREATE INDEX memberships_tenant_state_idx ON tenancy.memberships (tenant_id, state, id);
CREATE FUNCTION tenancy.protect_membership_facts() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'membership history is retained' USING ERRCODE = '55000';
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.user_id IS DISTINCT FROM OLD.user_id
       OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.updated_at < OLD.updated_at
       OR (NEW.state IS DISTINCT FROM OLD.state AND NEW.context_generation <> OLD.context_generation + 1)
       OR (NEW.state IS NOT DISTINCT FROM OLD.state AND NEW.context_generation NOT IN (OLD.context_generation, OLD.context_generation + 1)) THEN
        RAISE EXCEPTION 'membership identity/state generation transition invalid' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER memberships_protect_facts
BEFORE UPDATE OR DELETE ON tenancy.memberships
FOR EACH ROW EXECUTE FUNCTION tenancy.protect_membership_facts();

CREATE TABLE tenancy.role_grants (
    id uuid PRIMARY KEY,
    membership_id uuid NOT NULL REFERENCES tenancy.memberships(id) ON DELETE RESTRICT,
    role_key text NOT NULL CHECK (length(role_key) BETWEEN 1 AND 100 AND role_key = btrim(role_key)),
    capability text NOT NULL CHECK (length(capability) BETWEEN 1 AND 150 AND capability = btrim(capability)),
    effect text NOT NULL CHECK (effect IN ('ALLOW','DENY')),
    created_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE UNIQUE INDEX role_grants_active_fact_idx
    ON tenancy.role_grants (membership_id, role_key, capability, effect)
    WHERE revoked_at IS NULL;
CREATE INDEX role_grants_authorization_idx
    ON tenancy.role_grants (membership_id, capability, effect)
    WHERE revoked_at IS NULL;
COMMENT ON COLUMN tenancy.role_grants.role_key IS
'Grouping/provenance only. A role label never authorizes without an active capability effect.';

CREATE FUNCTION tenancy.protect_role_grant_facts() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'role grant history is append-only' USING ERRCODE = '55000';
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.membership_id IS DISTINCT FROM OLD.membership_id
       OR NEW.role_key IS DISTINCT FROM OLD.role_key
       OR NEW.capability IS DISTINCT FROM OLD.capability
       OR NEW.effect IS DISTINCT FROM OLD.effect
       OR NEW.created_by IS DISTINCT FROM OLD.created_by
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR OLD.revoked_at IS NOT NULL
       OR NEW.revoked_at IS NULL THEN
        RAISE EXCEPTION 'role grant facts are immutable; only first revocation is allowed' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER role_grants_protect_facts
BEFORE UPDATE OR DELETE ON tenancy.role_grants
FOR EACH ROW EXECUTE FUNCTION tenancy.protect_role_grant_facts();
CREATE FUNCTION tenancy.bump_membership_generation_for_grant() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE tenancy.memberships
    SET context_generation = context_generation + 1,
        updated_at = GREATEST(updated_at, COALESCE(NEW.revoked_at, NEW.created_at))
    WHERE id = NEW.membership_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'membership missing for grant generation bump';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER role_grants_bump_membership_generation
AFTER INSERT OR UPDATE OF revoked_at ON tenancy.role_grants
FOR EACH ROW EXECUTE FUNCTION tenancy.bump_membership_generation_for_grant();

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 7, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 6;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row missing or not at version 6';
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
SET schema_version = 6, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 7;
DROP SCHEMA tenancy CASCADE;
-- +goose StatementEnd
