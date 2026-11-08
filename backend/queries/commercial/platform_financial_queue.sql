-- name: ListPlatformAIUsageSettlements :many
SELECT id,tenant_id,kind,actual_credits,settled_at
FROM commercial.ai_usage_reservations
WHERE status='SETTLED' AND actual_credits IS NOT NULL AND settled_at IS NOT NULL
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (settled_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY settled_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: ListPlatformInvoiceQueue :many
SELECT id,tenant_id,subscription_id,invoice_number,state,version,purchase_kind,currency,total_minor_units,outstanding_minor_units,issued_at,due_at,
 coalesce((SELECT max(p.version)::bigint FROM commercial.payment_operations p JOIN commercial.payment_settlement_effects e ON e.payment_id=p.id WHERE p.invoice_id=commercial.invoices.id AND p.state='SETTLED' AND (SELECT count(*) FROM commercial.payment_operations exact_payment WHERE exact_payment.invoice_id=commercial.invoices.id AND exact_payment.state='SETTLED')=1 HAVING count(*)=1),0)::bigint AS settled_payment_version,
 coalesce((SELECT name FROM bidora_tenancy.tenants t WHERE t.id=commercial.invoices.tenant_id),'')::text AS tenant_name
FROM commercial.invoices
WHERE (sqlc.narg(state_filter)::text IS NULL OR state=sqlc.narg(state_filter)::text)
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (issued_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY issued_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: ListPlatformPaymentQueue :many
SELECT id,tenant_id,invoice_id,state,version,currency,amount_minor_units,reconciliation_state,created_at,updated_at,reconciled_at,
 coalesce((SELECT name FROM bidora_tenancy.tenants t WHERE t.id=commercial.payment_operations.tenant_id),'')::text AS tenant_name
FROM commercial.payment_operations
WHERE (sqlc.narg(state_filter)::text IS NULL OR state=sqlc.narg(state_filter)::text)
 AND (sqlc.narg(reconciliation_filter)::text IS NULL OR reconciliation_state=sqlc.narg(reconciliation_filter)::text)
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (created_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: ListPlatformAdjustmentQueue :many
SELECT id,tenant_id,invoice_id,payment_id,action,state,reason_code,entitlement_disposition,maker_user_id,checker_user_id,version,amount_minor_units,net_minor_units,tax_minor_units,created_at,expires_at,decided_at,
 coalesce((SELECT name FROM bidora_tenancy.tenants t WHERE t.id=commercial.financial_changes.tenant_id),'')::text AS tenant_name
FROM commercial.financial_changes
WHERE (sqlc.narg(state_filter)::text IS NULL OR state=sqlc.narg(state_filter)::text)
 AND (sqlc.narg(action_filter)::text IS NULL OR action=sqlc.narg(action_filter)::text)
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (created_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: ListPlatformReversalQueue :many
WITH queued AS (
 SELECT r.id,r.payment_id,p.tenant_id,p.invoice_id,r.actor_user_id,r.expected_payment_version,p.state AS payment_state,p.reconciliation_state,
  CASE WHEN e.payment_id IS NOT NULL THEN 'APPLIED' WHEN p.reconciliation_state='HELD' THEN 'HELD' WHEN p.reconciliation_state='COMPLETE' AND p.state='SETTLED' THEN 'CONFIRMED' ELSE 'PENDING' END::text AS queue_state,
  p.currency,p.amount_minor_units,r.requested_at,e.applied_at,h.state AS hold_state
 FROM commercial.payment_reconciliation_requests r
 JOIN commercial.payment_operations p ON p.id=r.payment_id
 LEFT JOIN commercial.payment_reversal_effects e ON e.payment_id=p.id
 LEFT JOIN commercial.subscription_access_holds h ON h.source_payment_id=p.id
)
SELECT id,payment_id,tenant_id,invoice_id,actor_user_id,expected_payment_version,payment_state,reconciliation_state,queue_state,currency,amount_minor_units,requested_at,applied_at,hold_state,
 coalesce((SELECT name FROM bidora_tenancy.tenants t WHERE t.id=queued.tenant_id),'')::text AS tenant_name
FROM queued
WHERE (sqlc.narg(state_filter)::text IS NULL OR queue_state=sqlc.narg(state_filter)::text)
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (requested_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY requested_at DESC,id DESC LIMIT sqlc.arg(page_limit);
