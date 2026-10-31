-- name: GetTrialStartCommand :one
SELECT c.request_hash,c.subscription_id,c.response_version,s.state,s.trial_ends_at FROM commercial.subscription_commands c JOIN commercial.subscriptions s ON s.id=c.subscription_id
WHERE c.tenant_id=sqlc.arg(tenant_id) AND c.operation='START_TRIAL' AND key_hash=sqlc.arg(key_hash);

-- name: CommercialSubscriptionExists :one
SELECT EXISTS(SELECT 1 FROM commercial.subscriptions WHERE tenant_id=sqlc.arg(tenant_id));

-- name: CommercialTrialMarkerExists :one
SELECT EXISTS(SELECT 1 FROM commercial.trial_lifetime_markers WHERE tenant_id=sqlc.arg(tenant_id));

-- name: InsertTrialSubscription :exec
INSERT INTO commercial.subscriptions(id,tenant_id,state,current_term_id,version,trial_started_at,trial_ends_at,last_reconciled_state,last_reconciled_at,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),'TRIAL_ACTIVE',sqlc.arg(term_id),1,sqlc.arg(started_at),sqlc.arg(ends_at),'TRIAL_ACTIVE',sqlc.arg(started_at),sqlc.arg(started_at),sqlc.arg(started_at));

-- name: InsertTrialTerm :exec
INSERT INTO commercial.subscription_terms(id,subscription_id,term_version,plan_key,catalogue_version,cadence,currency,price_minor_units,price_kind,additional_licence_monthly_minor_units,bid_workspace_limit,included_licence_limit,storage_limit_bytes,document_version_limit,ai_credit_limit,concurrent_ai_job_limit,commercial_period_start,commercial_period_end,usage_period_start,usage_period_end,created_at)
VALUES(sqlc.arg(id),sqlc.arg(subscription_id),1,'TRIAL',sqlc.arg(catalogue_version),'TRIAL','KES',0,'FIXED',NULL,sqlc.arg(bid_workspace_limit),sqlc.arg(included_licence_limit),sqlc.arg(storage_limit_bytes),sqlc.arg(document_version_limit),sqlc.arg(ai_credit_limit),sqlc.arg(concurrent_ai_job_limit),sqlc.arg(period_start),sqlc.arg(period_end),sqlc.arg(period_start),sqlc.arg(period_end),sqlc.arg(period_start));

-- name: InsertTrialUsagePeriod :exec
INSERT INTO commercial.usage_periods(id,subscription_id,term_id,period_start,period_end,version,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(subscription_id),sqlc.arg(term_id),sqlc.arg(period_start),sqlc.arg(period_end),1,sqlc.arg(period_start),sqlc.arg(period_start));

-- name: InsertTrialLifetimeMarker :exec
INSERT INTO commercial.trial_lifetime_markers(tenant_id,subscription_id,reason,initiating_user_id,execution_id,marked_at)
VALUES(sqlc.arg(tenant_id),sqlc.arg(subscription_id),'TRIAL_STARTED',sqlc.arg(initiating_user_id),sqlc.arg(execution_id),sqlc.arg(marked_at));

-- name: InsertTrialStartCommand :exec
INSERT INTO commercial.subscription_commands(id,tenant_id,operation,key_hash,request_hash,subscription_id,response_version,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),'START_TRIAL',sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(subscription_id),1,sqlc.arg(at),sqlc.arg(at));
