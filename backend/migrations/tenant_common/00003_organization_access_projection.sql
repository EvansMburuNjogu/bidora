-- +goose Up
-- +goose StatementBegin
-- Tenant-local identity projection for ownership/display only. Control membership
-- remains authoritative; no projection row grants access.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA bidora_tenant;

CREATE TABLE bidora_tenant.organization_profile (
    singleton_key boolean PRIMARY KEY DEFAULT true CHECK (singleton_key),
    id uuid NOT NULL UNIQUE,
    tenant_registry_id uuid NOT NULL UNIQUE,
    legal_name text NOT NULL CHECK (length(legal_name) BETWEEN 1 AND 300 AND legal_name ~ '\S'),
    country char(2) NOT NULL CHECK (country ~ '^[A-Z]{2}$'),
    timezone text NOT NULL CHECK (length(timezone) BETWEEN 1 AND 128 AND timezone ~ '\S'),
    currency char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    policy_version bigint NOT NULL CHECK (policy_version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);

CREATE TABLE bidora_tenant.user_profiles (
    id uuid PRIMARY KEY,
    identity_id uuid NOT NULL UNIQUE,
    source_membership_id uuid NOT NULL UNIQUE,
    display_name text NOT NULL CHECK (length(display_name) BETWEEN 1 AND 300 AND display_name ~ '\S'),
    membership_state text NOT NULL CHECK (length(membership_state) BETWEEN 1 AND 50 AND membership_state ~ '\S'),
    projection_version bigint NOT NULL CHECK (projection_version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);
CREATE INDEX user_profiles_membership_state_idx
    ON bidora_tenant.user_profiles (membership_state, identity_id);

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_meta.schema_compatibility
    SET schema_version = 3, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'tenant_common' AND schema_version = 2;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'tenant_common compatibility row is not at version 2' USING ERRCODE = '55000';
    END IF;
END;
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
UPDATE bidora_meta.schema_compatibility
SET schema_version = 2, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'tenant_common' AND schema_version = 3;
DROP SCHEMA bidora_tenant CASCADE;
-- +goose StatementEnd
