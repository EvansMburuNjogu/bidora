-- +goose Up
-- Preconditions: tenant_common is at version 2 and owns bidora_meta.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

INSERT INTO bidora_meta.schema_compatibility (
    migration_set, schema_version,
    min_reader_generation, max_reader_generation,
    min_writer_generation, max_writer_generation,
    updated_at
) VALUES ('tenant_tenderpreneur', 2, 1, 1, 1, 1, CURRENT_TIMESTAMP);

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
DELETE FROM bidora_meta.schema_compatibility
WHERE migration_set = 'tenant_tenderpreneur' AND schema_version = 2;
