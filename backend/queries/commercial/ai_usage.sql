-- name: LockCurrentAIUsage :one
SELECT s.id AS subscription_id,s.state,s.last_reconciled_state,s.grace_ends_at,t.id AS term_id,t.ai_credit_limit,t.concurrent_ai_job_limit,t.usage_period_start,t.usage_period_end,
 u.id AS usage_period_id,u.ai_credits_consumed,u.ai_credits_reserved,u.concurrent_ai_jobs,u.version
FROM commercial.subscriptions s JOIN commercial.subscription_terms t ON t.id=s.current_term_id
JOIN commercial.usage_periods u ON u.subscription_id=s.id AND u.term_id=t.id AND u.period_start=t.usage_period_start
WHERE s.tenant_id=sqlc.arg(tenant_id) AND NOT EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE') FOR UPDATE OF s,u;

-- name: GetAIUsageReservationByOperation :one
SELECT id,checkpoint_id,assignment_generation,tenant_id,subscription_id,usage_period_id,source_operation_id,operation_hash,request_hash,requester_user_id,kind,estimated_credits,actual_credits,status,version,reserved_at,settled_at,released_at,updated_at
FROM commercial.ai_usage_reservations WHERE tenant_id=sqlc.arg(tenant_id) AND source_operation_id=sqlc.arg(source_operation_id);

-- name: LockAIUsageReservation :one
SELECT id,checkpoint_id,assignment_generation,tenant_id,subscription_id,usage_period_id,source_operation_id,operation_hash,request_hash,requester_user_id,kind,estimated_credits,actual_credits,status,version,reserved_at,settled_at,released_at,updated_at
FROM commercial.ai_usage_reservations WHERE id=sqlc.arg(id) FOR UPDATE;

-- name: GetAIUsageReservationCompletion :one
SELECT to_status,actual_credits,execution_id,occurred_at
FROM commercial.ai_usage_reservation_transitions
WHERE reservation_id=sqlc.arg(reservation_id) AND reservation_version=2;

-- name: GetAIUsagePeriod :one
SELECT t.ai_credit_limit,t.concurrent_ai_job_limit,t.usage_period_start,t.usage_period_end,u.ai_credits_consumed,u.ai_credits_reserved,u.concurrent_ai_jobs,u.version
FROM commercial.usage_periods u JOIN commercial.subscription_terms t ON t.id=u.term_id WHERE u.id=sqlc.arg(id);

-- name: LockAIUsagePeriod :one
SELECT t.ai_credit_limit,t.concurrent_ai_job_limit,t.usage_period_start,t.usage_period_end,u.ai_credits_consumed,u.ai_credits_reserved,u.concurrent_ai_jobs,u.version
FROM commercial.usage_periods u JOIN commercial.subscription_terms t ON t.id=u.term_id WHERE u.id=sqlc.arg(id) FOR UPDATE OF u;

-- name: ReserveAIUsageCounters :execrows
UPDATE commercial.usage_periods u SET ai_credits_reserved=ai_credits_reserved+sqlc.arg(credits),concurrent_ai_jobs=concurrent_ai_jobs+1,version=version+1,updated_at=sqlc.arg(at)
FROM commercial.subscription_terms t WHERE u.id=sqlc.arg(usage_period_id) AND u.term_id=t.id AND u.version=sqlc.arg(expected_version)
 AND sqlc.arg(credits)::numeric<=t.ai_credit_limit::numeric-u.ai_credits_consumed::numeric-u.ai_credits_reserved::numeric AND u.concurrent_ai_jobs<t.concurrent_ai_job_limit;

-- name: SettleAIUsageCounters :execrows
UPDATE commercial.usage_periods u SET ai_credits_reserved=ai_credits_reserved-sqlc.arg(estimated_credits),ai_credits_consumed=ai_credits_consumed+sqlc.arg(actual_credits),concurrent_ai_jobs=concurrent_ai_jobs-1,version=version+1,updated_at=sqlc.arg(at)
FROM commercial.subscription_terms t WHERE u.id=sqlc.arg(usage_period_id) AND u.term_id=t.id AND u.version=sqlc.arg(expected_version)
 AND u.ai_credits_reserved>=sqlc.arg(estimated_credits) AND u.concurrent_ai_jobs>0
 AND sqlc.arg(actual_credits)::numeric<=t.ai_credit_limit::numeric-u.ai_credits_consumed::numeric-(u.ai_credits_reserved::numeric-sqlc.arg(estimated_credits)::numeric);

-- name: ReleaseAIUsageCounters :execrows
UPDATE commercial.usage_periods SET ai_credits_reserved=ai_credits_reserved-sqlc.arg(estimated_credits),concurrent_ai_jobs=concurrent_ai_jobs-1,version=version+1,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(usage_period_id) AND version=sqlc.arg(expected_version) AND ai_credits_reserved>=sqlc.arg(estimated_credits) AND concurrent_ai_jobs>0;

-- name: InsertAIUsageReservation :exec
INSERT INTO commercial.ai_usage_reservations(id,checkpoint_id,assignment_generation,tenant_id,subscription_id,usage_period_id,source_operation_id,operation_hash,request_hash,requester_user_id,kind,estimated_credits,status,version,reserved_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(checkpoint_id),sqlc.arg(assignment_generation),sqlc.arg(tenant_id),sqlc.arg(subscription_id),sqlc.arg(usage_period_id),sqlc.arg(source_operation_id),sqlc.arg(operation_hash),sqlc.arg(request_hash),sqlc.arg(requester_user_id),sqlc.arg(kind),sqlc.arg(estimated_credits),'RESERVED',1,sqlc.arg(at),sqlc.arg(at));

-- name: SettleAIUsageReservation :execrows
UPDATE commercial.ai_usage_reservations SET status='SETTLED',actual_credits=sqlc.arg(actual_credits),version=2,settled_at=sqlc.arg(at),updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND status='RESERVED' AND version=1;

-- name: ReleaseAIUsageReservation :execrows
UPDATE commercial.ai_usage_reservations SET status='RELEASED',version=2,released_at=sqlc.arg(at),updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND status='RESERVED' AND version=1;

-- name: InsertAIUsageReservationTransition :exec
INSERT INTO commercial.ai_usage_reservation_transitions(id,reservation_id,from_status,to_status,reservation_version,actual_credits,execution_id,occurred_at)
VALUES(sqlc.arg(id),sqlc.arg(reservation_id),'RESERVED',sqlc.arg(to_status),2,sqlc.narg(actual_credits),sqlc.arg(execution_id),sqlc.arg(occurred_at));
