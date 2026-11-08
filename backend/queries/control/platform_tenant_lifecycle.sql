-- name: LockPlatformTenantCommand :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text,0));

-- name: GetPlatformTenantCommand :one
SELECT request_hash,tenant_id,result_state,result_version FROM bidora_tenancy.platform_tenant_commands
WHERE actor_id=sqlc.arg(actor_id) AND key_hash=sqlc.arg(key_hash);

-- name: GetPlatformTenantForLifecycle :one
SELECT id,state,version FROM bidora_tenancy.tenants WHERE id=sqlc.arg(id) AND tenant_type='TENDERPRENEUR' FOR UPDATE;

-- name: PlatformTenantReadyForReactivation :one
SELECT EXISTS(
 SELECT 1 FROM bidora_tenancy.tenants t
 JOIN bidora_tenancy.tenant_database_assignments a ON a.tenant_id=t.id AND a.active AND a.generation=t.assignment_generation
 JOIN bidora_tenancy.database_clusters c ON c.id=a.cluster_id AND c.state='ACTIVE'
 WHERE t.id=sqlc.arg(tenant_id)
 AND EXISTS(SELECT 1 FROM bidora_tenancy.tenant_schema_versions s WHERE s.tenant_id=t.id AND s.migration_set='tenant_common' AND s.current_version>=8 AND s.state='COMPATIBLE' AND s.generation=a.generation)
 AND EXISTS(SELECT 1 FROM bidora_tenancy.tenant_schema_versions s WHERE s.tenant_id=t.id AND s.migration_set='tenant_tenderpreneur' AND s.current_version>=34 AND s.state='COMPATIBLE' AND s.generation=a.generation)
)::boolean;

-- name: UpdatePlatformTenantLifecycle :execrows
UPDATE bidora_tenancy.tenants SET state=sqlc.arg(state),version=version+1,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version);

-- name: InsertPlatformTenantCommand :exec
INSERT INTO bidora_tenancy.platform_tenant_commands(actor_id,key_hash,request_hash,tenant_id,operation,result_state,result_version,reason,created_at)
VALUES(sqlc.arg(actor_id),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(tenant_id),sqlc.arg(operation),sqlc.arg(result_state),sqlc.arg(result_version),sqlc.arg(reason),sqlc.arg(at));

-- name: InsertPlatformTenantLifecycleAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'tenant',sqlc.arg(tenant_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));
