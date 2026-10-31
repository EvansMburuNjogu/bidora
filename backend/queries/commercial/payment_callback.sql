-- name: LockPesaPalCallbackPayment :one
SELECT id,tenant_id,provider_order_reference,state,reconciliation_state,reconciliation_reason,reconciliation_attempts,last_queried_at,version,invoice_id,amount_minor_units FROM commercial.payment_operations WHERE merchant_operation_id=sqlc.arg(merchant_operation_id)AND provider='PESAPAL' FOR UPDATE;

-- name: CountPaymentCallbackHints :one
SELECT count(*) FROM commercial.payment_callback_hints WHERE payment_id=sqlc.arg(payment_id);

-- name: InsertPaymentCallbackHint :execrows
INSERT INTO commercial.payment_callback_hints(payment_id,tracking_id,received_at)VALUES(sqlc.arg(payment_id),sqlc.arg(tracking_id),sqlc.arg(at))ON CONFLICT DO NOTHING;

-- name: LatestPaymentCallbackHint :one
SELECT tracking_id FROM commercial.payment_callback_hints WHERE payment_id=sqlc.arg(payment_id)ORDER BY received_at DESC,tracking_id LIMIT 1;

-- name: WakePesaPalReconciliation :execrows
UPDATE commercial.payment_operations SET reconciliation_state='DUE',reconciliation_due_at=sqlc.arg(at),version=version+1,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id)AND version=sqlc.arg(expected_version)AND reconciliation_attempts<9 AND reconciliation_state<>'LEASED'
AND NOT(reconciliation_state='DUE' AND reconciliation_due_at<=sqlc.arg(at))
AND (last_queried_at IS NULL OR last_queried_at<=sqlc.arg(at)::timestamptz-interval '1 minute')
AND (state IN('PROVIDER_CREATE_PENDING','AWAITING_CUSTOMER','PROVIDER_PENDING','SETTLED'))
AND (reconciliation_state<>'HELD' OR reconciliation_reason='DELIVERY_EXHAUSTED');

-- name: AppendPaymentCallbackAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,created_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),NULL,'UntrustedPaymentCallbackRecorded','commercial_payment',sqlc.arg(payment_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('source','UNTRUSTED_CALLBACK','financial_state_unchanged',true),sqlc.arg(at));

-- name: AppendPaymentCallbackOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,event_type,event_version,aggregate_type,aggregate_id,aggregate_version,occurred_at,payload,created_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),'commercial.payment.callback_hint.v1',1,'commercial_payment',sqlc.arg(payment_id),sqlc.arg(version),sqlc.arg(at),jsonb_build_object('source','UNTRUSTED_CALLBACK','financial_state_unchanged',true),sqlc.arg(at));
