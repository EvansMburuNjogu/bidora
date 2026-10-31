-- name: InsertControlInbox :exec
INSERT INTO bidora_control_eventing.inbox_events(id,consumer,event_id,event_type,schema_version,aggregate_type,aggregate_id,aggregate_version,tenant_id,assignment_generation,request_hash,occurred_at,processed_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(consumer),sqlc.arg(event_id),sqlc.arg(event_type),sqlc.arg(schema_version),sqlc.arg(aggregate_type),sqlc.arg(aggregate_id),sqlc.arg(aggregate_version),sqlc.arg(tenant_id),sqlc.arg(assignment_generation),sqlc.arg(request_hash),sqlc.arg(occurred_at),sqlc.arg(processed_at),sqlc.arg(processed_at));

-- name: InsertRoutedControlAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.narg(actor_id),sqlc.arg(action),sqlc.arg(target_type),sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(change_summary),sqlc.arg(policy_version),sqlc.arg(occurred_at));

-- name: InsertRoutedControlOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.narg(actor_id),sqlc.arg(aggregate_type),sqlc.arg(aggregate_id),sqlc.arg(aggregate_version),sqlc.arg(event_type),sqlc.arg(schema_version),sqlc.arg(classification),sqlc.arg(payload),sqlc.arg(occurred_at),NULL,0,sqlc.arg(occurred_at),sqlc.arg(occurred_at));
