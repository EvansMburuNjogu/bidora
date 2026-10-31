-- name: GetAIEntitlementDispatchCompatibility :one
SELECT migration_set,schema_version,min_reader_generation,max_reader_generation,min_writer_generation,max_writer_generation
FROM bidora_control_meta.schema_compatibility
WHERE migration_set='control_registry';

-- name: ClaimAIEntitlementAcknowledgement :one
WITH candidate AS (
    SELECT pending.id
    FROM bidora_control_eventing.outbox_events AS pending
    WHERE pending.published_at IS NULL
      AND pending.next_attempt_at <= sqlc.arg(claimed_at)
      AND pending.event_type IN ('commercial.ai_usage.reserved.v1','commercial.ai_usage.denied.v1')
      AND pending.tenant_id IS NOT NULL
      AND pending.attempts < 9
      AND NOT EXISTS (
          SELECT 1 FROM bidora_control_eventing.ai_acknowledgement_dispatch_holds AS held
          WHERE held.event_id=pending.id
      )
    ORDER BY CASE WHEN sqlc.narg(previous_tenant_id)::uuid IS NOT NULL AND pending.tenant_id=sqlc.narg(previous_tenant_id)::uuid THEN 1 ELSE 0 END,
             pending.next_attempt_at,pending.id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE bidora_control_eventing.outbox_events AS event
SET attempts=event.attempts+1,next_attempt_at=sqlc.arg(lease_until)
FROM candidate
WHERE event.id=candidate.id
RETURNING event.id,event.tenant_id,event.actor_id,event.aggregate_type,event.aggregate_id,event.aggregate_version,event.event_type,event.schema_version,event.classification,event.payload,event.occurred_at,event.attempts;

-- name: MarkAIEntitlementAcknowledgementPublished :execrows
UPDATE bidora_control_eventing.outbox_events AS event
SET published_at=sqlc.arg(published_at),next_attempt_at=sqlc.arg(published_at)
WHERE event.id=sqlc.arg(id) AND event.published_at IS NULL AND event.attempts=sqlc.arg(expected_attempts)
  AND event.next_attempt_at>CURRENT_TIMESTAMP
  AND NOT EXISTS(SELECT 1 FROM bidora_control_eventing.ai_acknowledgement_dispatch_holds AS held WHERE held.event_id=event.id);

-- name: DeferAIEntitlementAcknowledgement :execrows
UPDATE bidora_control_eventing.outbox_events AS event
SET next_attempt_at=sqlc.arg(retry_at)
WHERE event.id=sqlc.arg(id) AND event.published_at IS NULL AND event.attempts=sqlc.arg(expected_attempts)
  AND event.next_attempt_at>CURRENT_TIMESTAMP
  AND event.attempts<9
  AND NOT EXISTS(SELECT 1 FROM bidora_control_eventing.ai_acknowledgement_dispatch_holds AS held WHERE held.event_id=event.id);

-- name: HoldAIEntitlementAcknowledgement :execrows
INSERT INTO bidora_control_eventing.ai_acknowledgement_dispatch_holds(
 event_id,tenant_id,envelope_hash,category,reason_code,event_assignment_generation,observed_assignment_generation,dispatch_attempt,held_at
)
SELECT event.id,event.tenant_id,sqlc.arg(envelope_hash),sqlc.arg(category),sqlc.arg(reason_code),
 sqlc.narg(event_assignment_generation),sqlc.narg(observed_assignment_generation),event.attempts,sqlc.arg(held_at)
FROM bidora_control_eventing.outbox_events AS event
WHERE event.id=sqlc.arg(event_id) AND event.tenant_id=sqlc.arg(tenant_id) AND event.published_at IS NULL
 AND event.attempts=sqlc.arg(expected_attempts) AND event.next_attempt_at>CURRENT_TIMESTAMP
 AND NOT EXISTS(SELECT 1 FROM bidora_control_eventing.ai_acknowledgement_dispatch_holds WHERE event_id=event.id);
