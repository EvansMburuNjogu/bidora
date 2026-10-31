-- +goose Up
-- Provider-neutral control registry required for server-owned tenant DB routing.
-- No identity credentials, memberships, quotas or provisioning executor included.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA bidora_control_meta;
CREATE SCHEMA bidora_tenancy;

CREATE TABLE bidora_control_meta.schema_compatibility (
    migration_set text PRIMARY KEY CHECK (migration_set = 'control_registry'),
    schema_version bigint NOT NULL CHECK (schema_version > 0),
    min_reader_generation bigint NOT NULL CHECK (min_reader_generation > 0),
    max_reader_generation bigint NOT NULL CHECK (max_reader_generation >= min_reader_generation),
    min_writer_generation bigint NOT NULL CHECK (min_writer_generation > 0),
    max_writer_generation bigint NOT NULL CHECK (max_writer_generation >= min_writer_generation),
    updated_at timestamptz NOT NULL
);
INSERT INTO bidora_control_meta.schema_compatibility VALUES
    ('control_registry', 1, 1, 1, 1, 1, CURRENT_TIMESTAMP);

CREATE TABLE bidora_tenancy.tenants (
    id uuid PRIMARY KEY,
    tenant_type text NOT NULL CHECK (tenant_type = 'TENDERPRENEUR'),
    state text NOT NULL CHECK (state IN (
        'REQUESTED', 'PROVISIONING', 'MIGRATING', 'SEEDING',
        'ACTIVE', 'FAILED', 'SUSPENDED', 'DECOMMISSIONED'
    )),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 300 AND name ~ '\S'),
    country char(2) NOT NULL CHECK (country ~ '^[A-Z]{2}$'),
    timezone text NOT NULL CHECK (length(timezone) BETWEEN 1 AND 128 AND timezone ~ '\S'),
    locale text NOT NULL CHECK (length(locale) BETWEEN 1 AND 35 AND locale ~ '^[A-Za-z0-9-]+$'),
    assignment_generation bigint NOT NULL DEFAULT 0 CHECK (assignment_generation >= 0),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);
CREATE INDEX tenants_state_created_idx
    ON bidora_tenancy.tenants (state, created_at, id);

CREATE TABLE bidora_tenancy.database_clusters (
    id uuid PRIMARY KEY,
    region text NOT NULL CHECK (length(region) BETWEEN 1 AND 100 AND region ~ '\S'),
    endpoint_ref text NOT NULL CHECK (length(endpoint_ref) BETWEEN 1 AND 500 AND endpoint_ref ~ '\S'),
    state text NOT NULL CHECK (length(state) BETWEEN 1 AND 50 AND state ~ '\S'),
    capacity_policy jsonb NOT NULL CHECK (jsonb_typeof(capacity_policy) = 'object'),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (endpoint_ref),
    CHECK (updated_at >= created_at)
);

CREATE TABLE bidora_tenancy.tenant_database_assignments (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
    cluster_id uuid NOT NULL REFERENCES bidora_tenancy.database_clusters(id) ON DELETE RESTRICT,
    database_name text NOT NULL CHECK (
        length(database_name) BETWEEN 1 AND 63
        AND database_name ~ '^[a-z_][a-z0-9_]*$'
    ),
    secret_ref text NOT NULL CHECK (length(secret_ref) BETWEEN 1 AND 500 AND secret_ref ~ '\S'),
    generation bigint NOT NULL CHECK (generation > 0),
    active boolean NOT NULL,
    created_at timestamptz NOT NULL,
    deactivated_at timestamptz,
    UNIQUE (cluster_id, database_name),
    CHECK ((active AND deactivated_at IS NULL) OR (NOT active AND deactivated_at IS NOT NULL)),
    CHECK (deactivated_at IS NULL OR deactivated_at >= created_at)
);
CREATE UNIQUE INDEX tenant_database_assignments_one_active_idx
    ON bidora_tenancy.tenant_database_assignments (tenant_id)
    WHERE active;
CREATE INDEX tenant_database_assignments_cluster_idx
    ON bidora_tenancy.tenant_database_assignments (cluster_id, active, tenant_id);

CREATE TABLE bidora_tenancy.tenant_schema_versions (
    tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
    migration_set text NOT NULL CHECK (migration_set IN ('tenant_common', 'tenant_tenderpreneur')),
    current_version bigint NOT NULL CHECK (current_version > 0),
    target_version bigint CHECK (target_version >= current_version),
    state text NOT NULL CHECK (state IN ('COMPATIBLE', 'MIGRATING', 'FAILED', 'INCOMPATIBLE')),
    checksum text NOT NULL CHECK (length(checksum) = 64 AND checksum ~ '^[0-9a-f]{64}$'),
    generation bigint NOT NULL CHECK (generation > 0),
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, migration_set)
);
CREATE INDEX tenant_schema_versions_state_idx
    ON bidora_tenancy.tenant_schema_versions (state, updated_at, tenant_id);

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
DROP SCHEMA bidora_tenancy CASCADE;
DROP SCHEMA bidora_control_meta CASCADE;
