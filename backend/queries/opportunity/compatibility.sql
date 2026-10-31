-- name: GetCaptureSchemaCompatibility :many
SELECT migration_set, schema_version,
       min_reader_generation, max_reader_generation,
       min_writer_generation, max_writer_generation
FROM bidora_meta.schema_compatibility
WHERE migration_set IN ('tenant_common', 'tenant_tenderpreneur')
ORDER BY migration_set;
