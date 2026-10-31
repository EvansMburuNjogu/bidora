-- name: GetAIEntitlementCheckpointBySource :one
SELECT id,tenant_id,target_type,target_id,source_operation_id,request_hash,requester_user_id,assignment_generation,usage_kind,estimated_credits,state,reservation_id,denial_code,version,created_at,updated_at
FROM bidora_entitlement.ai_usage_checkpoints WHERE source_operation_id=sqlc.arg(source_operation_id);

-- name: LockAIEntitlementCheckpoint :one
SELECT id,tenant_id,target_type,target_id,source_operation_id,request_hash,requester_user_id,assignment_generation,usage_kind,estimated_credits,state,reservation_id,denial_code,version,created_at,updated_at
FROM bidora_entitlement.ai_usage_checkpoints WHERE id=sqlc.arg(id) FOR UPDATE;

-- name: InsertAIEntitlementCheckpoint :exec
INSERT INTO bidora_entitlement.ai_usage_checkpoints(id,tenant_id,target_type,target_id,source_operation_id,request_hash,requester_user_id,assignment_generation,usage_kind,estimated_credits,state,version,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(target_type),sqlc.arg(target_id),sqlc.arg(source_operation_id),sqlc.arg(request_hash),sqlc.arg(requester_user_id),sqlc.arg(assignment_generation),sqlc.arg(usage_kind),sqlc.arg(estimated_credits),'PENDING_RESERVATION',1,sqlc.arg(at),sqlc.arg(at));

-- name: AcknowledgeAIEntitlementReserved :execrows
UPDATE bidora_entitlement.ai_usage_checkpoints SET state='RESERVED',reservation_id=sqlc.arg(reservation_id),version=2,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='PENDING_RESERVATION' AND version=1;

-- name: AcknowledgeAIEntitlementDenied :execrows
UPDATE bidora_entitlement.ai_usage_checkpoints SET state='DENIED',denial_code=sqlc.arg(denial_code),version=2,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='PENDING_RESERVATION' AND version=1;

-- name: InsertAIEntitlementTransition :exec
INSERT INTO bidora_entitlement.ai_usage_checkpoint_transitions(id,checkpoint_id,from_state,to_state,checkpoint_version,source_event_id,source_event_type,source_aggregate_version,reservation_id,denial_code,occurred_at)
VALUES(sqlc.arg(id),sqlc.arg(checkpoint_id),'PENDING_RESERVATION',sqlc.arg(to_state),2,sqlc.arg(source_event_id),sqlc.arg(source_event_type),sqlc.arg(source_aggregate_version),sqlc.narg(reservation_id),sqlc.narg(denial_code),sqlc.arg(occurred_at));

-- name: AIEntitlementInboxSeen :one
SELECT EXISTS(SELECT 1 FROM bidora_eventing.inbox_events WHERE consumer='tenant-ai-entitlement-v1' AND event_id=sqlc.arg(event_id));

-- name: InsertAIEntitlementInbox :exec
INSERT INTO bidora_eventing.inbox_events(id,consumer,event_id,event_type,schema_version,aggregate_type,aggregate_id,aggregate_version,occurred_at,processed_at,created_at)
VALUES(sqlc.arg(id),'tenant-ai-entitlement-v1',sqlc.arg(event_id),sqlc.arg(event_type),1,'commercial_ai_usage_reservation',sqlc.arg(aggregate_id),sqlc.arg(aggregate_version),sqlc.arg(event_occurred_at),sqlc.arg(processed_at),sqlc.arg(processed_at));

-- name: GetAIEntitlementCompletionRequest :one
SELECT id,checkpoint_id,reservation_id,execution_id,operation,actual_credits,correlation_id,requested_at
FROM bidora_entitlement.ai_usage_completion_requests WHERE checkpoint_id=sqlc.arg(checkpoint_id);

-- name: FindPendingAIEntitlementCompletion :one
SELECT r.id,r.checkpoint_id,r.reservation_id,r.execution_id,r.operation,r.actual_credits,r.correlation_id,r.requested_at,
       c.tenant_id,c.requester_user_id,c.assignment_generation,c.estimated_credits,c.usage_kind,c.state,c.version
FROM bidora_entitlement.ai_usage_completion_requests r
JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=r.checkpoint_id
LEFT JOIN bidora_entitlement.ai_usage_completion_acknowledgements a ON a.request_id=r.id
WHERE a.id IS NULL AND c.state IN('SETTLE_PENDING','RELEASE_PENDING')
ORDER BY r.requested_at,r.id LIMIT 1;

-- name: RecoverExpiredAIEntitlementCompletion :one
WITH candidate AS (
 SELECT x.request_id,x.lease_id,x.attempts FROM bidora_entitlement.ai_usage_completion_reconciliations x
 WHERE x.state='LEASED' AND x.lease_expires_at<=CURRENT_TIMESTAMP
 ORDER BY x.lease_expires_at,x.request_id FOR UPDATE SKIP LOCKED LIMIT 1
), evidence AS (
 INSERT INTO bidora_entitlement.ai_usage_completion_reconciliation_outcomes(id,lease_id,outcome,result_code,occurred_at,next_attempt_at)
 SELECT sqlc.arg(outcome_id),c.lease_id,CASE WHEN c.attempts>=9 THEN 'HELD' ELSE 'LEASE_EXPIRED' END,CASE WHEN c.attempts>=9 THEN 'RECONCILIATION_EXHAUSTED' ELSE 'LEASE_EXPIRED' END,CURRENT_TIMESTAMP,
  CASE c.attempts WHEN 1 THEN CURRENT_TIMESTAMP+INTERVAL '30 seconds' WHEN 2 THEN CURRENT_TIMESTAMP+INTERVAL '2 minutes' WHEN 3 THEN CURRENT_TIMESTAMP+INTERVAL '10 minutes' WHEN 4 THEN CURRENT_TIMESTAMP+INTERVAL '30 minutes' WHEN 5 THEN CURRENT_TIMESTAMP+INTERVAL '2 hours' WHEN 6 THEN CURRENT_TIMESTAMP+INTERVAL '6 hours' WHEN 7 THEN CURRENT_TIMESTAMP+INTERVAL '12 hours' WHEN 8 THEN CURRENT_TIMESTAMP+INTERVAL '24 hours' ELSE NULL END
 FROM candidate c RETURNING lease_id
)
UPDATE bidora_entitlement.ai_usage_completion_reconciliations x SET
 state=CASE WHEN c.attempts>=9 THEN 'HELD' ELSE 'RETRY_WAIT' END,
 lease_id=NULL,lease_expires_at=NULL,
 next_attempt_at=CASE c.attempts WHEN 1 THEN CURRENT_TIMESTAMP+INTERVAL '30 seconds' WHEN 2 THEN CURRENT_TIMESTAMP+INTERVAL '2 minutes' WHEN 3 THEN CURRENT_TIMESTAMP+INTERVAL '10 minutes' WHEN 4 THEN CURRENT_TIMESTAMP+INTERVAL '30 minutes' WHEN 5 THEN CURRENT_TIMESTAMP+INTERVAL '2 hours' WHEN 6 THEN CURRENT_TIMESTAMP+INTERVAL '6 hours' WHEN 7 THEN CURRENT_TIMESTAMP+INTERVAL '12 hours' WHEN 8 THEN CURRENT_TIMESTAMP+INTERVAL '24 hours' ELSE NULL END,
 hold_code=CASE WHEN c.attempts>=9 THEN 'RECONCILIATION_EXHAUSTED' ELSE NULL END,updated_at=CURRENT_TIMESTAMP
FROM candidate c,evidence e WHERE x.request_id=c.request_id AND e.lease_id=c.lease_id RETURNING x.request_id;

-- name: ClaimPendingAIEntitlementCompletion :one
WITH candidate AS (
 SELECT x.request_id FROM bidora_entitlement.ai_usage_completion_reconciliations x
 JOIN bidora_entitlement.ai_usage_completion_requests r ON r.id=x.request_id
 JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=r.checkpoint_id
 WHERE x.state IN('PENDING','RETRY_WAIT') AND x.attempts<9 AND (x.next_attempt_at IS NULL OR x.next_attempt_at<=CURRENT_TIMESTAMP)
   AND c.state IN('SETTLE_PENDING','RELEASE_PENDING')
 ORDER BY COALESCE(x.next_attempt_at,r.requested_at),x.request_id FOR UPDATE OF x SKIP LOCKED LIMIT 1
), claimed AS (
 UPDATE bidora_entitlement.ai_usage_completion_reconciliations x SET state='LEASED',attempts=attempts+1,lease_id=sqlc.arg(lease_id),lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '2 minutes',next_attempt_at=NULL,hold_code=NULL,updated_at=CURRENT_TIMESTAMP
 FROM candidate c WHERE x.request_id=c.request_id
 RETURNING x.request_id,x.attempts,x.lease_id,x.lease_expires_at
), attempt AS (
 INSERT INTO bidora_entitlement.ai_usage_completion_reconciliation_attempts(lease_id,request_id,attempt_number,operation,assignment_generation,claimed_at,lease_expires_at)
 SELECT c.lease_id,c.request_id,c.attempts,r.operation,p.assignment_generation,CURRENT_TIMESTAMP,c.lease_expires_at
 FROM claimed c JOIN bidora_entitlement.ai_usage_completion_requests r ON r.id=c.request_id JOIN bidora_entitlement.ai_usage_checkpoints p ON p.id=r.checkpoint_id RETURNING lease_id
)
SELECT r.id,r.checkpoint_id,r.reservation_id,r.execution_id,r.operation,r.actual_credits,r.correlation_id,r.requested_at,
 p.tenant_id,p.requester_user_id,p.assignment_generation,p.estimated_credits,p.usage_kind,p.state,p.version,
 c.attempts,c.lease_id,c.lease_expires_at
FROM claimed c JOIN attempt a ON a.lease_id=c.lease_id JOIN bidora_entitlement.ai_usage_completion_requests r ON r.id=c.request_id JOIN bidora_entitlement.ai_usage_checkpoints p ON p.id=r.checkpoint_id;

-- name: InsertAIEntitlementCompletionReconciliationOutcome :exec
INSERT INTO bidora_entitlement.ai_usage_completion_reconciliation_outcomes(id,lease_id,outcome,result_code,occurred_at,next_attempt_at)
VALUES(sqlc.arg(id),sqlc.arg(lease_id),sqlc.arg(outcome),sqlc.arg(result_code),sqlc.arg(occurred_at),sqlc.narg(next_attempt_at));

-- name: RetryAIEntitlementCompletionReconciliation :execrows
UPDATE bidora_entitlement.ai_usage_completion_reconciliations SET state='RETRY_WAIT',lease_id=NULL,lease_expires_at=NULL,next_attempt_at=sqlc.arg(next_attempt_at),updated_at=sqlc.arg(occurred_at)
WHERE request_id=sqlc.arg(request_id) AND state='LEASED' AND lease_id=sqlc.arg(lease_id) AND lease_expires_at>CURRENT_TIMESTAMP AND attempts<9;

-- name: HoldAIEntitlementCompletionReconciliation :execrows
UPDATE bidora_entitlement.ai_usage_completion_reconciliations SET state='HELD',lease_id=NULL,lease_expires_at=NULL,next_attempt_at=NULL,hold_code=sqlc.arg(hold_code),updated_at=sqlc.arg(occurred_at)
WHERE request_id=sqlc.arg(request_id) AND state='LEASED' AND lease_id=sqlc.arg(lease_id) AND lease_expires_at>CURRENT_TIMESTAMP;

-- name: LockAIEntitlementCompletionReconciliation :one
SELECT request_id,attempts FROM bidora_entitlement.ai_usage_completion_reconciliations
WHERE request_id=sqlc.arg(request_id) AND state='LEASED' AND lease_id=sqlc.arg(lease_id) AND lease_expires_at>CURRENT_TIMESTAMP FOR UPDATE;

-- name: MarkAIEntitlementCompletionReconciliationAcknowledged :execrows
UPDATE bidora_entitlement.ai_usage_completion_reconciliations SET state='ACKNOWLEDGED',lease_id=NULL,lease_expires_at=NULL,next_attempt_at=NULL,updated_at=sqlc.arg(occurred_at)
WHERE request_id=sqlc.arg(request_id) AND state='LEASED' AND lease_id=sqlc.arg(lease_id) AND lease_expires_at>CURRENT_TIMESTAMP;

-- name: InsertAIEntitlementCompletionRequest :exec
INSERT INTO bidora_entitlement.ai_usage_completion_requests(id,checkpoint_id,reservation_id,execution_id,operation,actual_credits,correlation_id,requested_at)
VALUES(sqlc.arg(id),sqlc.arg(checkpoint_id),sqlc.arg(reservation_id),sqlc.arg(execution_id),sqlc.arg(operation),sqlc.narg(actual_credits),sqlc.arg(correlation_id),sqlc.arg(requested_at));

-- name: InsertAIEntitlementCompletionReconciliation :exec
INSERT INTO bidora_entitlement.ai_usage_completion_reconciliations(request_id,state,attempts,updated_at)
VALUES(sqlc.arg(request_id),'PENDING',0,sqlc.arg(updated_at));

-- name: MarkAIEntitlementCompletionPending :execrows
UPDATE bidora_entitlement.ai_usage_checkpoints SET state=sqlc.arg(state),version=3,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='RESERVED' AND version=2 AND reservation_id=sqlc.arg(reservation_id);

-- name: InsertAIEntitlementCompletionAcknowledgement :exec
INSERT INTO bidora_entitlement.ai_usage_completion_acknowledgements(id,request_id,checkpoint_id,reservation_id,execution_id,operation,final_state,actual_credits,acknowledged_at)
VALUES(sqlc.arg(id),sqlc.arg(request_id),sqlc.arg(checkpoint_id),sqlc.arg(reservation_id),sqlc.arg(execution_id),sqlc.arg(operation),sqlc.arg(final_state),sqlc.narg(actual_credits),sqlc.arg(acknowledged_at));

-- name: MarkAIEntitlementCompletionFinal :execrows
UPDATE bidora_entitlement.ai_usage_checkpoints SET state=sqlc.arg(state),version=4,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state=sqlc.arg(expected_state) AND version=3 AND reservation_id=sqlc.arg(reservation_id);

-- name: InsertAIEntitlementAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'ai_usage_checkpoint',sqlc.arg(checkpoint_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('state',sqlc.arg(state)::text,'version',sqlc.arg(version)::bigint),1,sqlc.arg(occurred_at));

-- name: InsertAIEntitlementOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ai_usage_checkpoint',sqlc.arg(checkpoint_id),sqlc.arg(checkpoint_version),sqlc.arg(event_type),1,sqlc.arg(payload),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));
