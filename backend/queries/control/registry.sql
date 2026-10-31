-- name: GetControlRegistryCompatibility :one
SELECT migration_set, schema_version,
       min_reader_generation, max_reader_generation,
       min_writer_generation, max_writer_generation
FROM bidora_control_meta.schema_compatibility
WHERE migration_set = 'control_registry';

-- name: GetActiveTenantDatabaseAssignment :one
SELECT t.id AS tenant_id, t.assignment_generation,
       a.cluster_id, a.database_name, a.secret_ref,
       c.endpoint_ref, c.region
FROM bidora_tenancy.tenants AS t
JOIN bidora_tenancy.tenant_database_assignments AS a
  ON a.tenant_id = t.id
 AND a.active
 AND a.generation = t.assignment_generation
JOIN bidora_tenancy.database_clusters AS c
  ON c.id = a.cluster_id
 AND c.state = 'ACTIVE'
WHERE t.id = sqlc.arg(tenant_id)
  AND t.tenant_type = 'TENDERPRENEUR'
  AND t.state = 'ACTIVE'
  AND EXISTS (
      SELECT 1 FROM bidora_tenancy.tenant_schema_versions AS s
      WHERE s.tenant_id = t.id
        AND s.migration_set = 'tenant_common'
        AND s.current_version >= 5
        AND s.state = 'COMPATIBLE'
        AND s.generation = a.generation
  )
  AND EXISTS (
      SELECT 1 FROM bidora_tenancy.tenant_schema_versions AS s
      WHERE s.tenant_id = t.id
        AND s.migration_set = 'tenant_tenderpreneur'
        AND s.current_version >= 2
        AND s.state = 'COMPATIBLE'
        AND s.generation = a.generation
  );

-- name: GetDocumentCompletionAssignment :one
SELECT t.id AS tenant_id,t.assignment_generation,a.cluster_id,a.database_name,a.secret_ref,c.endpoint_ref,c.region
FROM bidora_tenancy.tenants t
JOIN bidora_tenancy.tenant_database_assignments a ON a.tenant_id=t.id AND a.active AND a.generation=t.assignment_generation
JOIN bidora_tenancy.database_clusters c ON c.id=a.cluster_id AND c.state='ACTIVE'
WHERE t.id=sqlc.arg(tenant_id) AND t.tenant_type='TENDERPRENEUR' AND t.state='ACTIVE'
 AND EXISTS(SELECT 1 FROM bidora_tenancy.tenant_schema_versions s WHERE s.tenant_id=t.id AND s.migration_set='tenant_common' AND s.current_version>=8 AND s.state='COMPATIBLE' AND s.generation=a.generation)
 AND EXISTS(SELECT 1 FROM bidora_tenancy.tenant_schema_versions s WHERE s.tenant_id=t.id AND s.migration_set='tenant_tenderpreneur' AND s.current_version>=34 AND s.state='COMPATIBLE' AND s.generation=a.generation);

-- name: ObserveAIEntitlementAcknowledgementAssignment :one
SELECT t.id AS tenant_id,t.assignment_generation,
 CASE WHEN t.tenant_type='TENDERPRENEUR' AND t.state='ACTIVE' AND a.id IS NOT NULL AND c.id IS NOT NULL
  AND EXISTS(SELECT 1 FROM bidora_tenancy.tenant_schema_versions s WHERE s.tenant_id=t.id AND s.migration_set='tenant_common' AND s.current_version>=6 AND s.state='COMPATIBLE' AND s.generation=t.assignment_generation)
  AND EXISTS(SELECT 1 FROM bidora_tenancy.tenant_schema_versions s WHERE s.tenant_id=t.id AND s.migration_set='tenant_tenderpreneur' AND s.current_version>=29 AND s.state='COMPATIBLE' AND s.generation=t.assignment_generation)
 THEN TRUE ELSE FALSE END AS dispatch_ready,
 a.cluster_id,a.database_name,a.secret_ref,c.endpoint_ref,c.region
FROM bidora_tenancy.tenants t
LEFT JOIN bidora_tenancy.tenant_database_assignments a ON a.tenant_id=t.id AND a.active AND a.generation=t.assignment_generation
LEFT JOIN bidora_tenancy.database_clusters c ON c.id=a.cluster_id AND c.state='ACTIVE'
WHERE t.id=sqlc.arg(tenant_id);

-- name: ListPlatformTenantDirectory :many
SELECT t.id, t.tenant_type, t.state, t.name, t.country, t.timezone, t.locale,
       t.assignment_generation, t.version, t.created_at, t.updated_at,
       EXISTS (
           SELECT 1
           FROM bidora_tenancy.tenant_database_assignments AS a
           JOIN bidora_tenancy.database_clusters AS c ON c.id = a.cluster_id AND c.state = 'ACTIVE'
           WHERE a.tenant_id = t.id AND a.active AND a.generation = t.assignment_generation
       ) AS assignment_present,
       common.current_version AS common_schema_version,
       common.state AS common_schema_state,
       common.generation AS common_schema_generation,
       common.updated_at AS common_schema_updated_at,
       tenant.current_version AS tenant_schema_version,
       tenant.state AS tenant_schema_state,
       tenant.generation AS tenant_schema_generation,
       tenant.updated_at AS tenant_schema_updated_at
FROM bidora_tenancy.tenants AS t
LEFT JOIN bidora_tenancy.tenant_schema_versions AS common
  ON common.tenant_id = t.id AND common.migration_set = 'tenant_common'
LEFT JOIN bidora_tenancy.tenant_schema_versions AS tenant
  ON tenant.tenant_id = t.id AND tenant.migration_set = 'tenant_tenderpreneur'
WHERE t.tenant_type = 'TENDERPRENEUR'
  AND (sqlc.narg(filter_state)::text IS NULL OR t.state = sqlc.narg(filter_state))
  AND (sqlc.narg(filter_country)::text IS NULL OR t.country = sqlc.narg(filter_country)::text)
  AND (sqlc.narg(filter_search)::text IS NULL OR strpos(lower(t.name), lower(sqlc.narg(filter_search)::text)) > 0)
  AND (sqlc.narg(before_created_at)::timestamptz IS NULL
       OR (t.created_at, t.id) < (sqlc.narg(before_created_at), sqlc.narg(before_id)::uuid))
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetPlatformTenantDirectoryEntry :one
SELECT t.id, t.tenant_type, t.state, t.name, t.country, t.timezone, t.locale,
       t.assignment_generation, t.version, t.created_at, t.updated_at,
       EXISTS (
           SELECT 1
           FROM bidora_tenancy.tenant_database_assignments AS a
           JOIN bidora_tenancy.database_clusters AS c ON c.id = a.cluster_id AND c.state = 'ACTIVE'
           WHERE a.tenant_id = t.id AND a.active AND a.generation = t.assignment_generation
       ) AS assignment_present,
       common.current_version AS common_schema_version,
       common.state AS common_schema_state,
       common.generation AS common_schema_generation,
       common.updated_at AS common_schema_updated_at,
       tenant.current_version AS tenant_schema_version,
       tenant.state AS tenant_schema_state,
       tenant.generation AS tenant_schema_generation,
       tenant.updated_at AS tenant_schema_updated_at
FROM bidora_tenancy.tenants AS t
LEFT JOIN bidora_tenancy.tenant_schema_versions AS common
  ON common.tenant_id = t.id AND common.migration_set = 'tenant_common'
LEFT JOIN bidora_tenancy.tenant_schema_versions AS tenant
  ON tenant.tenant_id = t.id AND tenant.migration_set = 'tenant_tenderpreneur'
WHERE t.id = sqlc.arg(tenant_id);
