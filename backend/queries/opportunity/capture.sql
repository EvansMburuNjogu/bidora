-- name: ClaimCaptureIdempotency :one
INSERT INTO bidora_commands.idempotency_records (
    id, actor_id, operation, key_hash, request_hash, state, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(actor_id), 'opportunity.capture_external',
    sqlc.arg(key_hash), sqlc.arg(request_hash), 'PENDING', sqlc.arg(created_at)
)
ON CONFLICT (actor_id, operation, key_hash) DO NOTHING
RETURNING id;

-- name: GetCaptureIdempotencyForUpdate :one
SELECT r.id, r.request_hash, r.state, r.response_status, r.response_ref,
       r.response_version, r.created_at, r.completed_at,
       o.current_source_version_id, o.created_at AS opportunity_created_at
FROM bidora_commands.idempotency_records AS r
LEFT JOIN bidora_opportunity.opportunities AS o ON o.id = r.response_ref
WHERE r.actor_id = sqlc.arg(actor_id)
  AND r.operation = 'opportunity.capture_external'
  AND r.key_hash = sqlc.arg(key_hash)
FOR UPDATE OF r;

-- name: InsertCapturedOpportunity :exec
INSERT INTO bidora_opportunity.opportunities (
    id, version, current_source_version_id, created_at, created_by, updated_at
) VALUES (
    sqlc.arg(id), 1, NULL, sqlc.arg(created_at), sqlc.arg(created_by), sqlc.arg(created_at)
);

-- name: InsertUnverifiedSourceVersion :exec
INSERT INTO bidora_opportunity.source_versions (
    id, opportunity_id, revision, title, buyer_name, source_uri,
    source_document_version_id, closes_at, source_timezone, source_date_text,
    verification_state, request_hash, created_at, created_by
) VALUES (
    sqlc.arg(id), sqlc.arg(opportunity_id), 1, sqlc.arg(title), sqlc.narg(buyer_name),
    sqlc.narg(source_uri), sqlc.narg(source_document_version_id), sqlc.narg(closes_at),
    sqlc.narg(source_timezone), sqlc.narg(source_date_text), 'UNVERIFIED',
    sqlc.arg(request_hash), sqlc.arg(created_at), sqlc.arg(created_by)
);

-- name: SetOpportunityCurrentSource :execrows
UPDATE bidora_opportunity.opportunities
SET current_source_version_id = sqlc.arg(source_version_id), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(opportunity_id)
  AND version = 1
  AND current_source_version_id IS NULL;

-- name: InsertCaptureAudit :exec
INSERT INTO bidora_audit.audit_events (
    id, actor_id, action, target_type, target_id, occurred_at, outcome,
    correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(actor_id), 'ExternalTenderImported', 'opportunity',
    sqlc.arg(target_id), sqlc.arg(occurred_at), 'SUCCEEDED',
    sqlc.arg(correlation_id), sqlc.arg(change_summary), NULL, sqlc.arg(occurred_at)
);

-- name: InsertCaptureOutbox :exec
INSERT INTO bidora_eventing.outbox_events (
    id, actor_id, aggregate_type, aggregate_id, aggregate_version, event_type,
    schema_version, payload, occurred_at, published_at, attempts,
    next_attempt_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(actor_id), 'opportunity', sqlc.arg(aggregate_id), 1,
    'opportunity.external_captured', 1, sqlc.arg(payload),
    sqlc.arg(occurred_at), NULL, 0, sqlc.arg(occurred_at), sqlc.arg(occurred_at)
);

-- name: CompleteCaptureIdempotency :execrows
UPDATE bidora_commands.idempotency_records
SET state = 'COMPLETED', response_status = 201,
    response_ref = sqlc.arg(response_ref), response_version = 1,
    completed_at = sqlc.arg(completed_at)
WHERE id = sqlc.arg(id)
  AND state = 'PENDING'
  AND request_hash = sqlc.arg(request_hash);
