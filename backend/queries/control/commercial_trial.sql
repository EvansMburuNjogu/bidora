-- name: LockCommercialTrialTenant :one
SELECT t.id,t.state,t.tenant_type,
 EXISTS(SELECT 1 FROM bidora_tenancy.tenant_database_assignments a WHERE a.tenant_id=t.id AND a.active) AS has_active_assignment,
 (SELECT count(*) FROM bidora_tenancy.tenant_schema_versions v WHERE v.tenant_id=t.id AND v.state='COMPATIBLE' AND v.migration_set IN('tenant_common','tenant_tenderpreneur')) AS compatible_schema_count
FROM bidora_tenancy.tenants t WHERE t.id=sqlc.arg(tenant_id) FOR UPDATE;

-- name: LockAIEntitlementTenantAssignment :one
SELECT t.id,t.state,t.tenant_type,t.assignment_generation,
 EXISTS(SELECT 1 FROM bidora_tenancy.tenant_database_assignments a JOIN bidora_tenancy.database_clusters c ON c.id=a.cluster_id AND c.state='ACTIVE' WHERE a.tenant_id=t.id AND a.active AND a.generation=t.assignment_generation) AS has_active_assignment,
 EXISTS(SELECT 1 FROM bidora_tenancy.tenant_schema_versions v WHERE v.tenant_id=t.id AND v.migration_set='tenant_common' AND v.current_version>=6 AND v.state='COMPATIBLE' AND v.generation=t.assignment_generation) AS has_compatible_checkpoint_schema
FROM bidora_tenancy.tenants t WHERE t.id=sqlc.arg(tenant_id) FOR UPDATE;
