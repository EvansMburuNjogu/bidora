-- name: ListPublishedPlatformPlans :many
SELECT h.plan_key,h.active_version,h.head_version,v.monthly_minor,v.annual_minor,v.additional_licence_minor,v.bid_workspaces,v.included_licences,v.storage_bytes,v.document_versions,v.ai_credits,v.concurrent_ai_jobs,v.created_by,v.created_at
FROM commercial.platform_plan_heads h JOIN commercial.platform_plan_versions v ON v.plan_key=h.plan_key AND v.version=h.active_version
ORDER BY h.plan_key;

-- name: LockPlatformPlanHead :one
SELECT plan_key,active_version,next_version,head_version FROM commercial.platform_plan_heads WHERE plan_key=sqlc.arg(plan_key) FOR UPDATE;

-- name: GetPlatformPlanVersion :one
SELECT plan_key,version,monthly_minor,annual_minor,additional_licence_minor,bid_workspaces,included_licences,storage_bytes,document_versions,ai_credits,concurrent_ai_jobs,created_by,created_at
FROM commercial.platform_plan_versions WHERE plan_key=sqlc.arg(plan_key) AND version=sqlc.arg(version);

-- name: InsertPlatformPlanVersion :exec
INSERT INTO commercial.platform_plan_versions(plan_key,version,monthly_minor,annual_minor,additional_licence_minor,bid_workspaces,included_licences,storage_bytes,document_versions,ai_credits,concurrent_ai_jobs,created_by,created_at)
VALUES(sqlc.arg(plan_key),sqlc.arg(version),sqlc.arg(monthly_minor),sqlc.arg(annual_minor),sqlc.arg(additional_licence_minor),sqlc.arg(bid_workspaces),sqlc.arg(included_licences),sqlc.arg(storage_bytes),sqlc.arg(document_versions),sqlc.arg(ai_credits),sqlc.arg(concurrent_ai_jobs),sqlc.arg(created_by),sqlc.arg(at));

-- name: AdvancePlatformPlanDraft :execrows
UPDATE commercial.platform_plan_heads SET next_version=next_version+1,head_version=head_version+1,updated_at=sqlc.arg(at)
WHERE plan_key=sqlc.arg(plan_key) AND head_version=sqlc.arg(expected_head_version) AND next_version=sqlc.arg(next_version);

-- name: PublishPlatformPlanVersion :execrows
UPDATE commercial.platform_plan_heads SET active_version=sqlc.arg(version),head_version=head_version+1,updated_at=sqlc.arg(at)
WHERE plan_key=sqlc.arg(plan_key) AND head_version=sqlc.arg(expected_head_version) AND active_version<sqlc.arg(version) AND next_version>sqlc.arg(version);

-- name: GetPlatformPlanCommand :one
SELECT request_hash,plan_key,result_version,result_head_version FROM commercial.platform_plan_commands
WHERE actor_id=sqlc.arg(actor_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);

-- name: InsertPlatformPlanCommand :exec
INSERT INTO commercial.platform_plan_commands(actor_id,operation,key_hash,request_hash,plan_key,result_version,result_head_version,created_at)
VALUES(sqlc.arg(actor_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(plan_key),sqlc.arg(result_version),sqlc.arg(result_head_version),sqlc.arg(at));

-- name: InsertPlatformPlanAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'platform_plan',sqlc.arg(target_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));

-- name: InsertPlatformPlanManagementRead :exec
INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformPlansRead','platform_plan_list',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),'{}'::jsonb,1,sqlc.arg(at));

-- name: InsertPlatformPlanOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'platform_plan',sqlc.arg(aggregate_id),sqlc.arg(version),sqlc.arg(event_type),1,'INTERNAL',sqlc.arg(payload),sqlc.arg(at),NULL,0,sqlc.arg(at),sqlc.arg(at));
