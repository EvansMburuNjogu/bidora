-- name: GetCommercialSubscriptionSummary :one
SELECT s.id,s.tenant_id,CASE WHEN EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE') THEN 'SUSPENDED' ELSE s.state END::text AS state,s.version,s.trial_started_at,s.trial_ends_at,s.grace_ends_at,s.cancel_at,
 t.plan_key,t.catalogue_version,t.cadence,t.currency,t.price_minor_units,t.bid_workspace_limit,t.included_licence_limit,t.storage_limit_bytes,t.document_version_limit,t.ai_credit_limit,t.concurrent_ai_job_limit,t.commercial_period_start,t.commercial_period_end,t.usage_period_start,t.usage_period_end,
 u.bid_workspaces_consumed,u.document_versions_consumed,u.ai_credits_consumed,u.ai_credits_reserved,u.storage_bytes_used,u.concurrent_ai_jobs,u.version AS usage_version
FROM commercial.subscriptions s JOIN commercial.subscription_terms t ON t.id=s.current_term_id
JOIN commercial.usage_periods u ON u.subscription_id=s.id AND u.term_id=t.id AND u.period_start=t.usage_period_start
WHERE s.tenant_id=sqlc.arg(tenant_id);

-- name: LockCommercialSubscription :one
SELECT s.id,s.tenant_id,CASE WHEN EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE') THEN 'SUSPENDED' ELSE s.state END::text AS state,s.version,s.trial_started_at,s.trial_ends_at,s.grace_ends_at,s.cancel_at,
 t.plan_key,t.catalogue_version,t.cadence,t.currency,t.price_minor_units,t.bid_workspace_limit,t.included_licence_limit,t.storage_limit_bytes,t.document_version_limit,t.ai_credit_limit,t.concurrent_ai_job_limit,t.commercial_period_start,t.commercial_period_end,t.usage_period_start,t.usage_period_end
FROM commercial.subscriptions s JOIN commercial.subscription_terms t ON t.id=s.current_term_id
WHERE s.tenant_id=sqlc.arg(tenant_id) FOR UPDATE OF s;

-- name: GetSubscriptionLifecycleCommand :one
SELECT request_hash,subscription_id,response_version FROM commercial.subscription_commands
WHERE tenant_id=sqlc.arg(tenant_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);

-- name: ScheduleSubscriptionCancellation :execrows
UPDATE commercial.subscriptions SET cancel_at=sqlc.arg(cancel_at),version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND cancel_at IS NULL;

-- name: WithdrawSubscriptionCancellation :execrows
UPDATE commercial.subscriptions SET cancel_at=NULL,version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND cancel_at IS NOT NULL;

-- name: InsertSubscriptionTransition :exec
INSERT INTO commercial.subscription_transitions(id,subscription_id,from_state,to_state,action,subscription_version,actor_user_id,occurred_at)
VALUES(sqlc.arg(id),sqlc.arg(subscription_id),sqlc.arg(from_state),sqlc.arg(to_state),sqlc.arg(action),sqlc.arg(subscription_version),sqlc.arg(actor_user_id),sqlc.arg(occurred_at));

-- name: InsertSubscriptionLifecycleCommand :exec
INSERT INTO commercial.subscription_commands(id,tenant_id,operation,key_hash,request_hash,subscription_id,response_version,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(subscription_id),sqlc.arg(response_version),sqlc.arg(at),sqlc.arg(at));
