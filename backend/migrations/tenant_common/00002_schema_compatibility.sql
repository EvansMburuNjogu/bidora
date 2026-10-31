-- +goose Up
-- Application/database compatibility metadata. Goose remains the migration ledger;
-- this table is the bounded startup contract, not a replacement migration runner.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA bidora_meta;
CREATE TABLE bidora_meta.schema_compatibility (
    migration_set text PRIMARY KEY CHECK (migration_set IN (
        'tenant_common', 'tenant_tenderpreneur', 'tenant_procurement'
    )),
    schema_version bigint NOT NULL CHECK (schema_version > 0),
    min_reader_generation bigint NOT NULL CHECK (min_reader_generation > 0),
    max_reader_generation bigint NOT NULL CHECK (max_reader_generation >= min_reader_generation),
    min_writer_generation bigint NOT NULL CHECK (min_writer_generation > 0),
    max_writer_generation bigint NOT NULL CHECK (max_writer_generation >= min_writer_generation),
    updated_at timestamptz NOT NULL
);

INSERT INTO bidora_meta.schema_compatibility (
    migration_set, schema_version,
    min_reader_generation, max_reader_generation,
    min_writer_generation, max_writer_generation,
    updated_at
) VALUES ('tenant_common', 2, 1, 1, 1, 1, CURRENT_TIMESTAMP);

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
DROP SCHEMA bidora_meta CASCADE;
