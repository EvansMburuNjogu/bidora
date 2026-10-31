-- name: ClaimInboxEvent :execrows
INSERT INTO bidora_eventing.inbox_events (
    id, consumer, event_id, event_type, schema_version,
    aggregate_type, aggregate_id, aggregate_version,
    occurred_at, processed_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(consumer), sqlc.arg(event_id), sqlc.arg(event_type),
    sqlc.arg(schema_version), sqlc.arg(aggregate_type), sqlc.arg(aggregate_id),
    sqlc.arg(aggregate_version), sqlc.arg(occurred_at),
    sqlc.arg(processed_at), sqlc.arg(processed_at)
)
ON CONFLICT (consumer, event_id) DO NOTHING;
