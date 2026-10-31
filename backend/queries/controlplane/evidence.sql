-- name: InsertControlAudit :exec
INSERT INTO bidora_control_audit.audit_events
(id, tenant_id, actor_id, action, target_type, target_id, occurred_at, outcome,
 correlation_id, change_summary, policy_version, created_at)
VALUES (sqlc.arg(id), NULL, sqlc.narg(actor_id), sqlc.arg(action), sqlc.arg(target_type),
        sqlc.arg(target_id), sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
        sqlc.arg(change_summary), sqlc.arg(policy_version), sqlc.arg(occurred_at));

-- name: InsertControlOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events
(id, tenant_id, actor_id, aggregate_type, aggregate_id, aggregate_version,
 event_type, schema_version, classification, payload, occurred_at,
 published_at, attempts, next_attempt_at, created_at)
VALUES (sqlc.arg(id), NULL, sqlc.narg(actor_id), sqlc.arg(aggregate_type),
        sqlc.arg(aggregate_id), sqlc.arg(aggregate_version), sqlc.arg(event_type),
        sqlc.arg(schema_version), sqlc.arg(classification), sqlc.arg(payload),
        sqlc.arg(occurred_at), NULL, 0, sqlc.arg(occurred_at), sqlc.arg(occurred_at));

-- Invitation lifecycle uses the same minimal shared append primitives above;
-- action/event/payload values are constructed by the application-specific
-- evidence adapter and never include email, reason or raw token material.
