-- name: GetCurrentLicenceEntitlement :one
SELECT s.id AS subscription_id,s.version AS subscription_version,t.id AS term_id,t.included_licence_limit,
 (t.included_licence_limit+COALESCE(SUM(g.quantity) FILTER(WHERE g.effective_from<=sqlc.arg(at) AND g.effective_until>sqlc.arg(at)),0)::bigint)::text AS effective_licence_limit
FROM commercial.subscriptions s JOIN commercial.subscription_terms t ON t.id=s.current_term_id
LEFT JOIN commercial.additional_licence_grants g ON g.subscription_id=s.id AND g.term_id=t.id
WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.state IN('ACTIVE','PAST_DUE') AND NOT EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE')
GROUP BY s.id,s.version,t.id,t.included_licence_limit;

-- name: GetPaymentSettlementEffect :one
SELECT e.payment_id,e.invoice_id,e.observation_id,e.receipt_id,e.subscription_id,e.effect_kind,e.resulting_term_id,e.additional_licence_grant_id,e.prior_subscription_version,e.resulting_subscription_version,e.amount_minor_units,e.currency,e.settled_at,e.effect_hash,r.receipt_number,r.issued_at,r.content_hash,p.version AS payment_version,i.version AS invoice_version
FROM commercial.payment_settlement_effects e JOIN commercial.receipts r ON r.id=e.receipt_id JOIN commercial.payment_operations p ON p.id=e.payment_id JOIN commercial.invoices i ON i.id=e.invoice_id
WHERE e.payment_id=sqlc.arg(payment_id);

-- name: LockSettlementPayment :one
SELECT id,invoice_id,tenant_id,merchant_operation_id,provider_order_reference,state,version,currency,amount_minor_units,provider_configuration_version_id,create_attempt_id,created_by,created_at,updated_at,reconciled_at,reconciliation_state,reconciliation_reason,reconciliation_attempts,reconciliation_due_at,last_queried_at
FROM commercial.payment_operations WHERE id=sqlc.arg(payment_id) FOR UPDATE;

-- name: LockSettlementInvoice :one
SELECT id,tenant_id,subscription_id,purchase_operation_id,invoice_number,state,version,purchase_kind,quantity,plan_key,catalogue_version,cadence,term_start,term_end,currency,subtotal_minor_units,tax_minor_units,total_minor_units,outstanding_minor_units
FROM commercial.invoices WHERE id=sqlc.arg(invoice_id) AND tenant_id=sqlc.arg(tenant_id) FOR UPDATE;

-- name: GetAuthoritativeSettlementObservation :one
SELECT o.id,o.normalized_state,o.amount_minor_units,o.currency,o.raw_evidence_hash,o.provider_observed_at,o.recorded_at
FROM commercial.payment_reconciliation_outcomes r JOIN commercial.payment_reconciliation_claims c ON c.id=r.claim_id JOIN commercial.payment_observations o ON o.id=r.observation_id
WHERE c.payment_id=sqlc.arg(payment_id) AND r.outcome='ACCOUNTING_REQUIRED' AND o.source='STATUS_QUERY' AND o.normalized_state='SETTLED';

-- name: GetInvoiceEntitlementSnapshot :one
SELECT invoice_id,subscription_id,effect_kind,expected_subscription_version,target_term_id,plan_key,catalogue_version,cadence,price_minor_units,additional_licence_monthly_minor_units,bid_workspace_limit,included_licence_limit,storage_limit_bytes,document_version_limit,ai_credit_limit,concurrent_ai_job_limit,commercial_period_start,commercial_period_end,usage_period_start,usage_period_end,additional_licence_quantity,snapshot_hash
FROM commercial.invoice_entitlement_snapshots WHERE invoice_id=sqlc.arg(invoice_id);

-- name: LockSettlementSubscription :one
SELECT s.id,s.tenant_id,s.state,s.version,s.current_term_id,t.plan_key,t.cadence,t.commercial_period_start,t.commercial_period_end
FROM commercial.subscriptions s LEFT JOIN commercial.subscription_terms t ON t.id=s.current_term_id
WHERE s.id=sqlc.arg(subscription_id) AND s.tenant_id=sqlc.arg(tenant_id) AND NOT EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE') FOR UPDATE OF s;

-- name: NextSettlementTermVersion :one
SELECT (COALESCE(MAX(term_version),0::bigint)+1::bigint)::text AS next_version FROM commercial.subscription_terms WHERE subscription_id=sqlc.arg(subscription_id);

-- name: EnsurePaidLifetimeMarker :exec
INSERT INTO commercial.trial_lifetime_markers(tenant_id,subscription_id,reason,initiating_user_id,execution_id,marked_at)
VALUES(sqlc.arg(tenant_id),sqlc.arg(subscription_id),'PAID_ACTIVATED_WITHOUT_TRIAL',sqlc.arg(initiating_user_id),sqlc.arg(execution_id),sqlc.arg(marked_at)) ON CONFLICT(tenant_id) DO NOTHING;

-- name: PaidLifetimeMarkerMatches :one
SELECT EXISTS(SELECT 1 FROM commercial.trial_lifetime_markers WHERE tenant_id=sqlc.arg(tenant_id) AND subscription_id=sqlc.arg(subscription_id));

-- name: InsertSettledSubscriptionTerm :exec
INSERT INTO commercial.subscription_terms(id,subscription_id,term_version,plan_key,catalogue_version,cadence,currency,price_minor_units,price_kind,additional_licence_monthly_minor_units,bid_workspace_limit,included_licence_limit,storage_limit_bytes,document_version_limit,ai_credit_limit,concurrent_ai_job_limit,commercial_period_start,commercial_period_end,usage_period_start,usage_period_end,created_at,settlement_payment_id)
VALUES(sqlc.arg(id),sqlc.arg(subscription_id),sqlc.arg(term_version),sqlc.arg(plan_key),sqlc.arg(catalogue_version),sqlc.arg(cadence),'KES',sqlc.arg(price_minor_units),'FIXED',sqlc.narg(additional_licence_monthly_minor_units),sqlc.arg(bid_workspace_limit),sqlc.arg(included_licence_limit),sqlc.arg(storage_limit_bytes),sqlc.arg(document_version_limit),sqlc.arg(ai_credit_limit),sqlc.arg(concurrent_ai_job_limit),sqlc.arg(commercial_period_start),sqlc.arg(commercial_period_end),sqlc.arg(usage_period_start),sqlc.arg(usage_period_end),sqlc.arg(created_at),sqlc.arg(payment_id));

-- name: InsertSettledUsagePeriod :exec
INSERT INTO commercial.usage_periods(id,subscription_id,term_id,period_start,period_end,bid_workspaces_consumed,document_versions_consumed,ai_credits_consumed,ai_credits_reserved,storage_bytes_used,concurrent_ai_jobs,version,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(subscription_id),sqlc.arg(term_id),sqlc.arg(period_start),sqlc.arg(period_end),0,0,0,0,0,0,1,sqlc.arg(created_at),sqlc.arg(created_at));

-- name: ActivateSettledSubscription :execrows
UPDATE commercial.subscriptions SET state='ACTIVE',current_term_id=sqlc.arg(term_id),version=version+1,grace_ends_at=NULL,cancel_at=NULL,pending_plan_key=NULL,pending_cadence=NULL,last_reconciled_state='ACTIVE',last_reconciled_at=sqlc.arg(settled_at),updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version);

-- name: AdvanceSubscriptionForLicenceGrant :execrows
UPDATE commercial.subscriptions SET version=version+1,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND current_term_id=sqlc.arg(term_id) AND state IN('ACTIVE','PAST_DUE');

-- name: InsertAdditionalLicenceGrant :exec
INSERT INTO commercial.additional_licence_grants(id,subscription_id,term_id,invoice_id,payment_id,quantity,effective_from,effective_until,created_at)
VALUES(sqlc.arg(id),sqlc.arg(subscription_id),sqlc.arg(term_id),sqlc.arg(invoice_id),sqlc.arg(payment_id),sqlc.arg(quantity),sqlc.arg(effective_from),sqlc.arg(effective_until),sqlc.arg(created_at));

-- name: MarkPaymentSettled :execrows
UPDATE commercial.payment_operations SET state='SETTLED',version=version+1,reconciled_at=sqlc.arg(settled_at),updated_at=sqlc.arg(at),reconciliation_state='COMPLETE',reconciliation_reason=NULL,reconciliation_due_at=NULL,reconciliation_lease_id=NULL,reconciliation_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND state='REVIEW_REQUIRED' AND reconciliation_state='HELD' AND reconciliation_reason='ACCOUNTING_COMPLETION_REQUIRED';

-- name: MarkInvoicePaid :execrows
UPDATE commercial.invoices SET state='PAID',version=version+1,outstanding_minor_units=0,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND state='PAYMENT_PENDING' AND outstanding_minor_units=sqlc.arg(expected_outstanding);

-- name: InsertCommercialReceipt :exec
INSERT INTO commercial.receipts(id,payment_id,invoice_id,receipt_number,currency,amount_minor_units,issued_at,content_hash)
VALUES(sqlc.arg(id),sqlc.arg(payment_id),sqlc.arg(invoice_id),sqlc.arg(receipt_number),'KES',sqlc.arg(amount_minor_units),sqlc.arg(issued_at),sqlc.arg(content_hash));

-- name: InsertPaymentSettlementEffect :exec
INSERT INTO commercial.payment_settlement_effects(payment_id,invoice_id,observation_id,receipt_id,subscription_id,effect_kind,resulting_term_id,additional_licence_grant_id,prior_subscription_version,resulting_subscription_version,amount_minor_units,currency,settled_at,effect_hash)
VALUES(sqlc.arg(payment_id),sqlc.arg(invoice_id),sqlc.arg(observation_id),sqlc.arg(receipt_id),sqlc.arg(subscription_id),sqlc.arg(effect_kind),sqlc.narg(resulting_term_id),sqlc.narg(additional_licence_grant_id),sqlc.arg(prior_subscription_version),sqlc.arg(resulting_subscription_version),sqlc.arg(amount_minor_units),'KES',sqlc.arg(settled_at),sqlc.arg(effect_hash));
