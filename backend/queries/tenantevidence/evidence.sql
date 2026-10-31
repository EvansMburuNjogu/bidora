-- name: GetTenantCommand :one
SELECT request_hash,response_ref,response_version,response_status FROM bidora_commands.idempotency_records WHERE actor_id=sqlc.arg(actor_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash) AND state='COMPLETED';
-- name: InsertCompletedTenantCommand :exec
INSERT INTO bidora_commands.idempotency_records(id,actor_id,operation,key_hash,request_hash,state,response_status,response_ref,response_version,created_at,completed_at) VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),'COMPLETED',sqlc.arg(response_status),sqlc.arg(response_ref),sqlc.arg(response_version),sqlc.arg(at),sqlc.arg(at));
-- name: InsertTenantAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),sqlc.arg(target_type),sqlc.arg(target_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(change_summary),sqlc.arg(policy_version),sqlc.arg(at));
-- name: InsertTenantOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at) VALUES(sqlc.arg(id),sqlc.arg(aggregate_type),sqlc.arg(aggregate_id),sqlc.arg(aggregate_version),sqlc.arg(event_type),1,sqlc.arg(payload),sqlc.arg(at),sqlc.arg(at),sqlc.arg(at));
