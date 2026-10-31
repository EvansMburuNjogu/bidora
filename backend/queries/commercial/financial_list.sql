-- name: ListTenantInvoiceSummaries :many
SELECT id,subscription_id,invoice_number,state,version,purchase_kind,plan_key,catalogue_version,cadence,quantity,currency,subtotal_minor_units,tax_minor_units,total_minor_units,outstanding_minor_units,issued_at,due_at
FROM commercial.invoices
WHERE tenant_id=sqlc.arg(tenant_id) AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (issued_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY issued_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: ListTenantPaymentSummaries :many
SELECT id,invoice_id,state,version,currency,amount_minor_units,reconciliation_state,created_at,updated_at,reconciled_at
FROM commercial.payment_operations
WHERE tenant_id=sqlc.arg(tenant_id) AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (created_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: ListTenantFinancialHistory :many
WITH history AS (
 SELECT i.id,'INVOICE'::text AS kind,i.id AS invoice_id,NULL::uuid AS payment_id,i.invoice_number AS document_number,i.state,i.currency,i.total_minor_units AS amount_minor_units,i.issued_at AS occurred_at
 FROM commercial.invoices i WHERE i.tenant_id=sqlc.arg(tenant_id)
 UNION ALL
 SELECT p.id,'PAYMENT'::text,p.invoice_id,p.id,NULL::text,p.state,p.currency,p.amount_minor_units,p.created_at
 FROM commercial.payment_operations p WHERE p.tenant_id=sqlc.arg(tenant_id)
 UNION ALL
 SELECT r.id,'RECEIPT'::text,r.invoice_id,r.payment_id,r.receipt_number,'ISSUED'::text,r.currency,r.amount_minor_units,r.issued_at
 FROM commercial.receipts r JOIN commercial.invoices i ON i.id=r.invoice_id WHERE i.tenant_id=sqlc.arg(tenant_id)
 UNION ALL
 SELECT n.id,'CREDIT_NOTE'::text,n.invoice_id,c.payment_id,n.credit_number,n.state,n.currency,n.total_minor_units,n.issued_at
 FROM commercial.credit_notes n JOIN commercial.invoices i ON i.id=n.invoice_id JOIN commercial.financial_changes c ON c.id=n.change_id WHERE i.tenant_id=sqlc.arg(tenant_id)
 UNION ALL
 SELECT r.id,'REFUND_INTENT'::text,p.invoice_id,p.id,NULL::text,r.state,'KES'::char(3),r.amount_minor_units,r.created_at
 FROM commercial.refund_operations r JOIN commercial.payment_operations p ON p.id=r.payment_id WHERE p.tenant_id=sqlc.arg(tenant_id)
 UNION ALL
 SELECT e.payment_id,'PAYMENT_REVERSAL'::text,e.invoice_id,e.payment_id,NULL::text,'REVERSED'::text,e.currency,e.amount_minor_units,e.applied_at
 FROM commercial.payment_reversal_effects e JOIN commercial.payment_operations p ON p.id=e.payment_id WHERE p.tenant_id=sqlc.arg(tenant_id)
)
SELECT id,kind,invoice_id,payment_id,COALESCE(document_number,''::text)::text AS document_number,state,currency,amount_minor_units,occurred_at
FROM history
WHERE sqlc.narg(cursor_at)::timestamptz IS NULL OR (occurred_at,kind,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_kind)::text,sqlc.narg(cursor_id)::uuid)
ORDER BY occurred_at DESC,kind DESC,id DESC LIMIT sqlc.arg(page_limit);
