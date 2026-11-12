-- name: LockOrganizationCreator :one
SELECT (u.email_verified_at IS NOT NULL)::boolean AS verified,(u.disabled_at IS NOT NULL)::boolean AS disabled,COALESCE(u.account_type='TENDERPRENEUR',false)::boolean AS tenderpreneur,
EXISTS(SELECT 1 FROM identity.mfa_factors f WHERE f.user_id=u.id AND f.state='ACTIVE') AS mfa_enrolled
FROM identity.users u WHERE u.id=sqlc.arg(user_id) FOR UPDATE;

-- name: GetLatestOrganizationCreationStatus :one
SELECT o.id,o.tenant_id,o.state,o.commercial_path,o.created_at,o.updated_at,t.name,j.state AS job_state
FROM tenancy.organization_creation_operations o
JOIN bidora_tenancy.tenants t ON t.id=o.tenant_id
JOIN tenancy.provisioning_job_status_v1 j ON j.operation_id=o.id
WHERE o.creator_user_id=sqlc.arg(creator_user_id)
ORDER BY o.created_at DESC,o.id DESC LIMIT 1;

-- name: GetOrganizationCreationCommand :one
SELECT request_hash,operation_id,tenant_id,response_state FROM tenancy.organization_creation_commands
WHERE creator_user_id=sqlc.arg(creator_user_id) AND key_hash=sqlc.arg(key_hash);

-- name: OrganizationCreatorLifetimeMarkerExists :one
SELECT EXISTS(SELECT 1 FROM tenancy.organization_creation_lifetime_markers WHERE creator_user_id=sqlc.arg(creator_user_id));

-- name: OrganizationCreatorOpenOperationExists :one
SELECT EXISTS(SELECT 1 FROM tenancy.organization_creation_operations WHERE creator_user_id=sqlc.arg(creator_user_id) AND state IN('REQUESTED','PLAN_SELECTED','PROVISIONING','SUBSCRIPTION_ACTIVATION','REVIEW_REQUIRED'));

-- name: InsertRequestedTenderpreneurTenant :exec
INSERT INTO bidora_tenancy.tenants(id,tenant_type,state,name,country,timezone,locale,assignment_generation,version,created_at,updated_at)
VALUES(sqlc.arg(id),'TENDERPRENEUR','REQUESTED',sqlc.arg(name),sqlc.arg(country),sqlc.arg(timezone),sqlc.arg(locale),0,1,sqlc.arg(at),sqlc.arg(at));

-- name: InsertOrganizationCreatorMembership :exec
INSERT INTO tenancy.memberships(id,user_id,tenant_id,state,context_generation,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(user_id),sqlc.arg(tenant_id),'ACTIVE',1,sqlc.arg(at),sqlc.arg(at));

-- name: InsertOrganizationCreatorGrant :exec
INSERT INTO tenancy.role_grants(id,membership_id,role_key,capability,effect,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(membership_id),'ORGANIZATION_ADMIN',sqlc.arg(capability),'ALLOW',sqlc.arg(created_by),sqlc.arg(at));

-- name: InsertOrganizationCreationOperation :exec
INSERT INTO tenancy.organization_creation_operations(id,tenant_id,creator_user_id,creator_membership_id,commercial_path,business_type,primary_industry,preferred_language,currency,state,version,correlation_id,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(creator_user_id),sqlc.arg(creator_membership_id),sqlc.arg(commercial_path),sqlc.arg(business_type),sqlc.arg(primary_industry),sqlc.arg(preferred_language),sqlc.arg(currency),'REQUESTED',1,sqlc.arg(correlation_id),sqlc.arg(at),sqlc.arg(at));

-- name: InsertOrganizationProvisioningJob :exec
INSERT INTO tenancy.provisioning_jobs(id,operation_id,tenant_id,state,checkpoint,attempts,next_attempt_at,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(operation_id),sqlc.arg(tenant_id),sqlc.arg(state),'PLAN_SELECTED',0,sqlc.arg(next_attempt_at),sqlc.arg(at),sqlc.arg(at));

-- name: InsertOrganizationCreationCommand :exec
INSERT INTO tenancy.organization_creation_commands(id,creator_user_id,key_hash,request_hash,operation_id,tenant_id,response_state,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(creator_user_id),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(operation_id),sqlc.arg(tenant_id),'REQUESTED',sqlc.arg(at),sqlc.arg(at));

-- name: InsertOrganizationCreationAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(actor_id),'TenderpreneurOrganizationRequested','OrganizationCreation',sqlc.arg(operation_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('commercial_path',sqlc.arg(commercial_path)::text),1,sqlc.arg(at));

-- name: InsertOrganizationCreationOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(actor_id),'ORGANIZATION_CREATION',sqlc.arg(operation_id),1,'tenderpreneur.organization.creation.requested.v1',1,'INTERNAL',jsonb_build_object('tenant_id',sqlc.arg(tenant_id)::text,'operation_id',sqlc.arg(operation_id)::text,'commercial_path',sqlc.arg(commercial_path)::text),sqlc.arg(at),sqlc.arg(at),sqlc.arg(at));
