-- name: ClaimAIEntitlementOutbox :one
WITH candidate AS (
 SELECT o.id FROM bidora_eventing.outbox_events o
 WHERE o.published_at IS NULL AND o.delivery_hold_code IS NULL AND o.attempts < 9 AND o.event_type='tenant.ai_entitlement.reserve_requested.v1'
   AND o.next_attempt_at <= sqlc.arg(claimed_at)
   AND (o.delivery_lease_expires_at IS NULL OR o.delivery_lease_expires_at <= sqlc.arg(claimed_at))
 ORDER BY o.next_attempt_at,o.id FOR UPDATE OF o SKIP LOCKED LIMIT 1
)
UPDATE bidora_eventing.outbox_events e
SET attempts=e.attempts+1,delivery_lease_id=sqlc.arg(lease_id),delivery_lease_expires_at=sqlc.arg(lease_expires_at)
FROM candidate c WHERE e.id=c.id
RETURNING e.id,e.actor_id,e.aggregate_type,e.aggregate_id,e.aggregate_version,e.event_type,e.schema_version,e.payload,e.occurred_at,e.attempts;

-- name: MarkAIEntitlementOutboxPublished :execrows
UPDATE bidora_eventing.outbox_events SET published_at=sqlc.arg(published_at),delivery_lease_id=NULL,delivery_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND published_at IS NULL AND delivery_lease_id=sqlc.arg(lease_id) AND attempts=sqlc.arg(expected_attempts);

-- name: HoldAIEntitlementOutbox :execrows
UPDATE bidora_eventing.outbox_events SET delivery_hold_code='JETSTREAM_DELIVERY_EXHAUSTED',delivery_lease_id=NULL,delivery_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND published_at IS NULL AND delivery_lease_id=sqlc.arg(lease_id) AND attempts=9;

-- name: RetryAIEntitlementOutbox :execrows
UPDATE bidora_eventing.outbox_events SET next_attempt_at=sqlc.arg(next_attempt_at),delivery_lease_id=NULL,delivery_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND published_at IS NULL AND delivery_lease_id=sqlc.arg(lease_id) AND attempts=sqlc.arg(expected_attempts);
