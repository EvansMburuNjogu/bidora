-- name: LockPaymentInvoice :one
SELECT id,tenant_id,state,version,currency,outstanding_minor_units
FROM commercial.invoices WHERE id=sqlc.arg(invoice_id) AND tenant_id=sqlc.arg(tenant_id) FOR UPDATE;

-- name: GetActivePaymentForInvoice :one
SELECT id,invoice_id,tenant_id,provider,merchant_operation_id,provider_order_reference,state,version,currency,amount_minor_units,provider_configuration_version_id,create_attempt_id,created_by,created_at,updated_at,reconciliation_state,reconciliation_reason,reconciliation_attempts,reconciliation_due_at
FROM commercial.payment_operations
WHERE invoice_id=sqlc.arg(invoice_id) AND state IN('CREATED','PROVIDER_CREATE_PENDING','AWAITING_CUSTOMER','PROVIDER_PENDING','REVIEW_REQUIRED')
ORDER BY created_at DESC,id DESC LIMIT 1 FOR UPDATE;

-- name: InsertPaymentCreateIntent :exec
INSERT INTO commercial.payment_operations(id,invoice_id,tenant_id,provider,merchant_operation_id,state,version,currency,amount_minor_units,provider_configuration_version_id,created_by,created_at,updated_at,create_attempt_id,create_dispatched_at,reconciliation_state,reconciliation_reason,reconciliation_due_at)
VALUES(sqlc.arg(id),sqlc.arg(invoice_id),sqlc.arg(tenant_id),'PESAPAL',sqlc.arg(merchant_operation_id),'PROVIDER_CREATE_PENDING',1,'KES',sqlc.arg(amount_minor_units),sqlc.arg(provider_configuration_version_id),sqlc.arg(created_by),sqlc.arg(at),sqlc.arg(at),sqlc.arg(create_attempt_id),sqlc.arg(at),'DUE','CREATE_AMBIGUOUS',sqlc.arg(reconciliation_due_at));

-- name: MarkInvoicePaymentPending :execrows
UPDATE commercial.invoices SET state='PAYMENT_PENDING',version=version+1,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(invoice_id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND state IN('ISSUED','PAYMENT_PENDING') AND outstanding_minor_units>0;

-- name: InsertPaymentFinancialCommand :exec
INSERT INTO commercial.financial_commands(id,actor_user_id,tenant_id,operation,key_hash,request_hash,target_id,response_state,response_version,response_status,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_user_id),sqlc.arg(tenant_id),'INITIATE_PAYMENT',sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(payment_id),'PROVIDER_CREATE_PENDING',1,202,sqlc.arg(at),sqlc.arg(at));

-- name: GetCommercialPayment :one
SELECT id,invoice_id,tenant_id,provider,merchant_operation_id,provider_order_reference,state,version,currency,amount_minor_units,provider_configuration_version_id,create_attempt_id,created_by,created_at,updated_at,reconciled_at,reconciliation_state,reconciliation_reason,reconciliation_attempts,reconciliation_due_at,last_queried_at
FROM commercial.payment_operations WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id);

-- name: LockCommercialPayment :one
SELECT id,invoice_id,tenant_id,provider,merchant_operation_id,provider_order_reference,state,version,currency,amount_minor_units,provider_configuration_version_id,create_attempt_id,created_by,created_at,updated_at,reconciled_at,reconciliation_state,reconciliation_reason,reconciliation_attempts,reconciliation_due_at,last_queried_at
FROM commercial.payment_operations WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id) FOR UPDATE;

-- name: ConfirmPaymentOrderCreated :execrows
UPDATE commercial.payment_operations SET provider_order_reference=sqlc.arg(provider_order_reference),state='AWAITING_CUSTOMER',version=version+1,updated_at=sqlc.arg(at),reconciliation_state='COMPLETE',reconciliation_reason=NULL,reconciliation_due_at=NULL,reconciliation_lease_id=NULL,reconciliation_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND create_attempt_id=sqlc.arg(create_attempt_id) AND state='PROVIDER_CREATE_PENDING' AND (provider_order_reference IS NULL OR provider_order_reference=sqlc.arg(provider_order_reference));

-- name: ConfirmPaymentCreateNoEffect :execrows
UPDATE commercial.payment_operations SET state='FAILED',version=version+1,updated_at=sqlc.arg(at),reconciled_at=sqlc.arg(at),reconciliation_state='COMPLETE',reconciliation_reason=NULL,reconciliation_due_at=NULL,reconciliation_lease_id=NULL,reconciliation_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND create_attempt_id=sqlc.arg(create_attempt_id) AND state='PROVIDER_CREATE_PENDING' AND provider_order_reference IS NULL;

-- name: RecordPaymentCreateAmbiguous :execrows
UPDATE commercial.payment_operations SET version=version+1,updated_at=sqlc.arg(at),reconciliation_state='DUE',reconciliation_reason='CREATE_AMBIGUOUS'
WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id) AND version=sqlc.arg(expected_version) AND create_attempt_id=sqlc.arg(create_attempt_id) AND state='PROVIDER_CREATE_PENDING' AND reconciliation_state='DUE';

-- name: InsertPaymentObservation :exec
INSERT INTO commercial.payment_observations(id,payment_id,provider_event_key_hash,source,provider_state_code,normalized_state,amount_minor_units,currency,raw_evidence_hash,provider_observed_at,recorded_at)
VALUES(sqlc.arg(id),sqlc.arg(payment_id),sqlc.narg(provider_event_key_hash),sqlc.arg(source),sqlc.arg(provider_state_code),sqlc.arg(normalized_state),sqlc.narg(amount_minor_units),sqlc.narg(currency),sqlc.arg(raw_evidence_hash),sqlc.narg(provider_observed_at),sqlc.arg(recorded_at));

-- name: ClaimPaymentReconciliation :one
WITH candidate AS (
 SELECT id FROM commercial.payment_operations
 WHERE reconciliation_attempts<9 AND ((reconciliation_state IN('DUE','RETRY_WAIT') AND reconciliation_due_at<=CURRENT_TIMESTAMP) OR (reconciliation_state='LEASED' AND reconciliation_lease_expires_at<=CURRENT_TIMESTAMP))
 ORDER BY reconciliation_due_at,id FOR UPDATE SKIP LOCKED LIMIT 1
), updated AS (
 UPDATE commercial.payment_operations p SET reconciliation_state='LEASED',reconciliation_attempts=p.reconciliation_attempts+1,reconciliation_lease_id=sqlc.arg(lease_id),reconciliation_lease_expires_at=CURRENT_TIMESTAMP+interval '2 minutes',version=p.version+1,updated_at=CURRENT_TIMESTAMP
 FROM candidate c WHERE p.id=c.id
 RETURNING p.id,p.invoice_id,p.tenant_id,p.provider,p.merchant_operation_id,p.provider_order_reference,p.state,p.version,p.currency,p.amount_minor_units,p.provider_configuration_version_id,p.create_attempt_id,p.created_by,p.reconciliation_attempts,p.reconciliation_reason,p.created_at,p.updated_at,p.reconciliation_lease_expires_at
), claim AS (
 INSERT INTO commercial.payment_reconciliation_claims(id,payment_id,attempt_number,lease_id,started_at,lease_expires_at)
 SELECT sqlc.arg(claim_id),id,reconciliation_attempts,sqlc.arg(lease_id),updated_at,reconciliation_lease_expires_at FROM updated RETURNING payment_id
)
SELECT u.id,u.invoice_id,u.tenant_id,u.provider,u.merchant_operation_id,u.provider_order_reference,u.state,u.version,u.currency,u.amount_minor_units,u.provider_configuration_version_id,u.create_attempt_id,u.created_by,u.reconciliation_attempts,u.reconciliation_reason,u.created_at,u.updated_at,u.reconciliation_lease_expires_at
FROM updated u JOIN claim c ON c.payment_id=u.id;

-- name: CompletePaymentReconciliation :execrows
UPDATE commercial.payment_operations SET provider_order_reference=COALESCE(sqlc.narg(provider_order_reference),provider_order_reference),state=sqlc.arg(payment_state),version=version+1,updated_at=sqlc.arg(at),reconciled_at=sqlc.arg(at),last_queried_at=sqlc.arg(at),reconciliation_state=sqlc.arg(reconciliation_state),reconciliation_reason=sqlc.narg(reconciliation_reason),reconciliation_due_at=sqlc.narg(reconciliation_due_at),reconciliation_lease_id=NULL,reconciliation_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND reconciliation_state='LEASED' AND reconciliation_lease_id=sqlc.arg(lease_id) AND reconciliation_lease_expires_at>CURRENT_TIMESTAMP;

-- name: InsertPaymentReconciliationOutcome :exec
INSERT INTO commercial.payment_reconciliation_outcomes(claim_id,outcome,observation_id,normalized_state,provider_state_code,raw_evidence_hash,completed_at)
VALUES(sqlc.arg(claim_id),sqlc.arg(outcome),sqlc.narg(observation_id),sqlc.narg(normalized_state),sqlc.narg(provider_state_code),sqlc.narg(raw_evidence_hash),sqlc.arg(completed_at));
