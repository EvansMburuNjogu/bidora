-- name: GetReconciliationRequestCommand :one
SELECT request_hash,target_id,response_state,response_version FROM commercial.financial_commands
WHERE actor_user_id=sqlc.arg(actor_user_id) AND operation='REQUEST_RECONCILIATION' AND key_hash=sqlc.arg(key_hash);

-- name: LockPaymentForReconciliationRequest :one
SELECT id,invoice_id,tenant_id,merchant_operation_id,provider_order_reference,state,version,currency,amount_minor_units,provider_configuration_version_id,create_attempt_id,created_by,created_at,updated_at,reconciled_at,reconciliation_state,reconciliation_reason,reconciliation_attempts,reconciliation_due_at,last_queried_at
FROM commercial.payment_operations WHERE id=sqlc.arg(payment_id) FOR UPDATE;

-- name: ScheduleSettledPaymentReconciliation :execrows
UPDATE commercial.payment_operations SET version=version+1,updated_at=sqlc.arg(at),reconciliation_state='DUE',reconciliation_reason='POST_SETTLEMENT_RECHECK',reconciliation_attempts=0,reconciliation_due_at=sqlc.arg(at),reconciliation_lease_id=NULL,reconciliation_lease_expires_at=NULL
WHERE id=sqlc.arg(payment_id) AND version=sqlc.arg(expected_version) AND state='SETTLED' AND reconciliation_state='COMPLETE';

-- name: InsertPaymentReconciliationRequest :exec
INSERT INTO commercial.payment_reconciliation_requests(id,payment_id,actor_user_id,expected_payment_version,reason,authority_hash,request_hash,requested_at)
VALUES(sqlc.arg(id),sqlc.arg(payment_id),sqlc.arg(actor_user_id),sqlc.arg(expected_payment_version),sqlc.arg(reason),sqlc.arg(authority_hash),sqlc.arg(request_hash),sqlc.arg(at));

-- name: InsertReconciliationRequestCommand :exec
INSERT INTO commercial.financial_commands(id,actor_user_id,tenant_id,operation,key_hash,request_hash,target_id,response_state,response_version,response_status,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_user_id),sqlc.arg(tenant_id),'REQUEST_RECONCILIATION',sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(payment_id),'DUE',sqlc.arg(response_version),202,sqlc.arg(at),sqlc.arg(at));

-- name: GetPaymentReversalEffect :one
SELECT e.payment_id,p.tenant_id,e.invoice_id,e.subscription_id,e.settlement_observation_id,e.reversal_observation_id,e.access_hold_id,e.prior_payment_version,e.resulting_payment_version,e.prior_invoice_version,e.resulting_invoice_version,e.prior_subscription_version,e.resulting_subscription_version,e.amount_minor_units,e.currency,e.provider_observed_at,e.recorded_at,e.applied_at,e.settlement_effect_hash,e.effect_hash
FROM commercial.payment_reversal_effects e JOIN commercial.payment_operations p ON p.id=e.payment_id WHERE e.payment_id=sqlc.arg(payment_id);

-- name: LockPaymentReversalCandidate :one
SELECT p.id AS payment_id,p.invoice_id,p.tenant_id,p.version AS payment_version,p.state AS payment_state,p.currency,p.amount_minor_units,p.reconciliation_state,p.reconciliation_reason,
 i.version AS invoice_version,i.state AS invoice_state,i.subscription_id,
 s.version AS subscription_version,s.state AS subscription_state,
 se.observation_id AS settlement_observation_id,se.effect_hash AS settlement_effect_hash,se.effect_kind,
 ro.id AS reversal_observation_id,ro.normalized_state,ro.amount_minor_units AS observed_amount_minor_units,ro.currency AS observed_currency,ro.raw_evidence_hash,ro.provider_observed_at,ro.recorded_at
FROM commercial.payment_operations p
JOIN commercial.invoices i ON i.id=p.invoice_id AND i.tenant_id=p.tenant_id
JOIN commercial.subscriptions s ON s.id=i.subscription_id AND s.tenant_id=i.tenant_id
JOIN commercial.payment_settlement_effects se ON se.payment_id=p.id AND se.invoice_id=i.id AND se.subscription_id=s.id
JOIN commercial.payment_reconciliation_claims rc ON rc.payment_id=p.id
JOIN commercial.payment_reconciliation_outcomes rr ON rr.claim_id=rc.id AND rr.outcome='ACCOUNTING_REQUIRED'
JOIN commercial.payment_observations ro ON ro.id=rr.observation_id AND ro.payment_id=p.id AND ro.source='STATUS_QUERY' AND ro.normalized_state='REVERSED'
WHERE p.id=sqlc.arg(payment_id) AND p.state='REVIEW_REQUIRED' AND p.reconciliation_state='HELD' AND p.reconciliation_reason='ACCOUNTING_COMPLETION_REQUIRED'
ORDER BY rc.attempt_number DESC LIMIT 1 FOR UPDATE OF p,i,s;

-- name: MarkPaymentReversed :execrows
UPDATE commercial.payment_operations SET state='REVERSED',version=version+1,updated_at=sqlc.arg(at),reconciled_at=sqlc.arg(at),reconciliation_state='COMPLETE',reconciliation_reason=NULL,reconciliation_due_at=NULL,reconciliation_lease_id=NULL,reconciliation_lease_expires_at=NULL
WHERE id=sqlc.arg(payment_id) AND version=sqlc.arg(expected_version) AND state='REVIEW_REQUIRED' AND reconciliation_state='HELD' AND reconciliation_reason='ACCOUNTING_COMPLETION_REQUIRED';

-- name: MarkReversedInvoiceReviewRequired :execrows
UPDATE commercial.invoices SET state='REVIEW_REQUIRED',version=version+1,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(invoice_id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND state='PAID';

-- name: SuspendReversedSubscription :execrows
UPDATE commercial.subscriptions SET state='SUSPENDED',version=version+1,grace_ends_at=NULL,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(subscription_id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND state IN('ACTIVE','PAST_DUE','REVIEW_REQUIRED');

-- name: InsertReversalAccessHold :exec
INSERT INTO commercial.subscription_access_holds(id,tenant_id,subscription_id,source_payment_id,reason,access_mode,state,activated_at,effect_hash)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(subscription_id),sqlc.arg(payment_id),'PAYMENT_REVERSAL','RETAINED_READ_ONLY','ACTIVE',sqlc.arg(at),sqlc.arg(effect_hash));

-- name: InsertPaymentReversalEffect :exec
INSERT INTO commercial.payment_reversal_effects(payment_id,invoice_id,subscription_id,settlement_observation_id,reversal_observation_id,access_hold_id,prior_payment_version,resulting_payment_version,prior_invoice_version,resulting_invoice_version,prior_subscription_version,resulting_subscription_version,amount_minor_units,currency,provider_observed_at,recorded_at,applied_at,settlement_effect_hash,effect_hash)
VALUES(sqlc.arg(payment_id),sqlc.arg(invoice_id),sqlc.arg(subscription_id),sqlc.arg(settlement_observation_id),sqlc.arg(reversal_observation_id),sqlc.arg(access_hold_id),sqlc.arg(prior_payment_version),sqlc.arg(resulting_payment_version),sqlc.arg(prior_invoice_version),sqlc.arg(resulting_invoice_version),sqlc.arg(prior_subscription_version),sqlc.arg(resulting_subscription_version),sqlc.arg(amount_minor_units),'KES',sqlc.narg(provider_observed_at),sqlc.arg(recorded_at),sqlc.arg(applied_at),sqlc.arg(settlement_effect_hash),sqlc.arg(effect_hash));
