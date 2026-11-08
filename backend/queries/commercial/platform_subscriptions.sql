-- name: ListPlatformSubscriptionInventory :many
SELECT s.id,s.tenant_id,coalesce((SELECT name FROM bidora_tenancy.tenants registry WHERE registry.id=s.tenant_id),'')::text AS tenant_name,
 CASE WHEN EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE') THEN 'SUSPENDED' ELSE s.state END::text AS effective_state,
 s.version,COALESCE(t.plan_key,s.pending_plan_key,'')::text AS plan_key,
 COALESCE(t.cadence,s.pending_cadence,'')::text AS cadence,
 (t.id IS NOT NULL)::boolean AS has_current_term,
 t.commercial_period_end,s.cancel_at,s.updated_at
FROM commercial.subscriptions s
LEFT JOIN commercial.subscription_terms t ON t.id=s.current_term_id AND t.subscription_id=s.id
WHERE s.id>sqlc.arg(after_id)::uuid
 AND (sqlc.arg(state_filter)::text='' OR CASE WHEN EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE') THEN 'SUSPENDED' ELSE s.state END=sqlc.arg(state_filter)::text)
ORDER BY s.id ASC LIMIT sqlc.arg(row_limit);

-- name: InsertPlatformSubscriptionInventoryRead :exec
INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformSubscriptionInventoryRead','platform_subscription_inventory',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));

-- name: GetPlatformSubscriptionCommand :one
SELECT request_hash,tenant_id,subscription_id,result_state,result_version,result_cancel_at
FROM commercial.platform_subscription_commands
WHERE actor_id=sqlc.arg(actor_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);

-- name: InsertPlatformSubscriptionCommand :exec
INSERT INTO commercial.platform_subscription_commands(actor_id,operation,key_hash,request_hash,tenant_id,subscription_id,result_state,result_version,result_cancel_at,created_at)
VALUES(sqlc.arg(actor_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(tenant_id),sqlc.arg(subscription_id),sqlc.arg(result_state),sqlc.arg(result_version),sqlc.arg(result_cancel_at),sqlc.arg(at));

-- name: InsertPlatformSubscriptionAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'platform_subscription',sqlc.arg(subscription_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));

-- name: InsertPlatformSubscriptionOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'commercial_subscription',sqlc.arg(subscription_id),sqlc.arg(version),sqlc.arg(event_type),1,'INTERNAL',sqlc.arg(payload),sqlc.arg(at),NULL,0,sqlc.arg(at),sqlc.arg(at));
