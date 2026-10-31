-- name: GetFinancialCommand :one
SELECT request_hash,target_id,response_version FROM commercial.financial_commands
WHERE actor_user_id=sqlc.arg(actor_user_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);

-- name: LockInvoiceSubscription :one
SELECT id,tenant_id,version FROM commercial.subscriptions
WHERE id=sqlc.arg(subscription_id) AND tenant_id=sqlc.arg(tenant_id) FOR UPDATE;

-- name: LockActiveTaxProfile :one
SELECT id,version,country,currency,tax_code,display_label,rate_numerator,rate_denominator,rounding_profile,effective_from,effective_until,configuration_hash,legal_review_reference_hash
FROM commercial.tax_profiles
WHERE country='KE' AND currency='KES' AND state='ACTIVE' AND effective_from<=sqlc.arg(at) AND (effective_until IS NULL OR effective_until>sqlc.arg(at))
FOR UPDATE;

-- name: EnsureFinancialNumberHead :exec
INSERT INTO commercial.financial_number_heads(document_kind,calendar_year,updated_at)
VALUES(sqlc.arg(document_kind),sqlc.arg(calendar_year),sqlc.arg(at)) ON CONFLICT(document_kind,calendar_year) DO NOTHING;

-- name: LockFinancialNumberHead :one
SELECT next_sequence,version FROM commercial.financial_number_heads
WHERE document_kind=sqlc.arg(document_kind) AND calendar_year=sqlc.arg(calendar_year) FOR UPDATE;

-- name: AdvanceFinancialNumberHead :execrows
UPDATE commercial.financial_number_heads SET next_sequence=next_sequence+1,version=version+1,updated_at=sqlc.arg(at)
WHERE document_kind=sqlc.arg(document_kind) AND calendar_year=sqlc.arg(calendar_year) AND next_sequence=sqlc.arg(expected_sequence) AND version=sqlc.arg(expected_version);

-- name: InsertCommercialInvoice :exec
INSERT INTO commercial.invoices(
 id,tenant_id,subscription_id,purchase_operation_id,invoice_number,state,version,purchase_kind,quantity,plan_key,catalogue_version,cadence,term_start,term_end,
 tax_profile_id,tax_profile_hash,arithmetic_profile,currency,subtotal_minor_units,credit_minor_units,taxable_minor_units,tax_minor_units,total_minor_units,outstanding_minor_units,
 seller_legal_name,seller_tax_identifier,seller_billing_address,customer_legal_name,customer_tax_identifier,customer_billing_address,billing_snapshot_version,billing_snapshot_hash,quote_hash,
 issued_by,issued_at,due_at,created_at,updated_at)
VALUES(
 sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(subscription_id),sqlc.arg(purchase_operation_id),sqlc.arg(invoice_number),'ISSUED',1,sqlc.arg(purchase_kind),sqlc.arg(quantity),sqlc.arg(plan_key),sqlc.arg(catalogue_version),sqlc.arg(cadence),sqlc.arg(term_start),sqlc.arg(term_end),
 sqlc.arg(tax_profile_id),sqlc.arg(tax_profile_hash),'KES_COMMERCIAL_V1','KES',sqlc.arg(subtotal_minor_units),sqlc.arg(credit_minor_units),sqlc.arg(taxable_minor_units),sqlc.arg(tax_minor_units),sqlc.arg(total_minor_units),sqlc.arg(outstanding_minor_units),
 sqlc.arg(seller_legal_name),sqlc.arg(seller_tax_identifier),sqlc.arg(seller_billing_address),sqlc.arg(customer_legal_name),sqlc.narg(customer_tax_identifier),sqlc.arg(customer_billing_address),sqlc.arg(billing_snapshot_version),sqlc.arg(billing_snapshot_hash),sqlc.arg(quote_hash),
 sqlc.arg(issued_by),sqlc.arg(issued_at),sqlc.arg(due_at),sqlc.arg(issued_at),sqlc.arg(issued_at));

-- name: InsertCommercialInvoiceLine :exec
INSERT INTO commercial.invoice_lines(id,invoice_id,line_number,line_kind,description_code,quantity,unit_minor_units,proration_numerator,proration_denominator,net_minor_units,tax_code,tax_minor_units,gross_minor_units,calculation_hash)
VALUES(sqlc.arg(id),sqlc.arg(invoice_id),sqlc.arg(line_number),sqlc.arg(line_kind),sqlc.arg(description_code),sqlc.arg(quantity),sqlc.arg(unit_minor_units),sqlc.arg(proration_numerator),sqlc.arg(proration_denominator),sqlc.arg(net_minor_units),sqlc.arg(tax_code),sqlc.arg(tax_minor_units),sqlc.arg(gross_minor_units),sqlc.arg(calculation_hash));

-- name: InsertInvoiceEntitlementSnapshot :exec
INSERT INTO commercial.invoice_entitlement_snapshots(invoice_id,subscription_id,effect_kind,expected_subscription_version,target_term_id,plan_key,catalogue_version,cadence,price_minor_units,additional_licence_monthly_minor_units,bid_workspace_limit,included_licence_limit,storage_limit_bytes,document_version_limit,ai_credit_limit,concurrent_ai_job_limit,commercial_period_start,commercial_period_end,usage_period_start,usage_period_end,additional_licence_quantity,snapshot_hash,created_at)
VALUES(sqlc.arg(invoice_id),sqlc.arg(subscription_id),sqlc.arg(effect_kind),sqlc.arg(expected_subscription_version),sqlc.narg(target_term_id),sqlc.arg(plan_key),sqlc.arg(catalogue_version),sqlc.arg(cadence),sqlc.narg(price_minor_units),sqlc.narg(additional_licence_monthly_minor_units),sqlc.narg(bid_workspace_limit),sqlc.narg(included_licence_limit),sqlc.narg(storage_limit_bytes),sqlc.narg(document_version_limit),sqlc.narg(ai_credit_limit),sqlc.narg(concurrent_ai_job_limit),sqlc.arg(commercial_period_start),sqlc.arg(commercial_period_end),sqlc.narg(usage_period_start),sqlc.narg(usage_period_end),sqlc.narg(additional_licence_quantity),sqlc.arg(snapshot_hash),sqlc.arg(created_at));

-- name: InsertInvoiceFinancialCommand :exec
INSERT INTO commercial.financial_commands(id,actor_user_id,tenant_id,operation,key_hash,request_hash,target_id,response_state,response_version,response_status,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_user_id),sqlc.arg(tenant_id),'ISSUE_INVOICE',sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(invoice_id),'ISSUED',1,201,sqlc.arg(at),sqlc.arg(at));

-- name: GetCommercialInvoice :one
SELECT id,tenant_id,subscription_id,purchase_operation_id,invoice_number,state,version,purchase_kind,quantity,plan_key,catalogue_version,cadence,term_start,term_end,
 tax_profile_id,tax_profile_hash,arithmetic_profile,currency,subtotal_minor_units,credit_minor_units,taxable_minor_units,tax_minor_units,total_minor_units,outstanding_minor_units,
 seller_legal_name,seller_tax_identifier,seller_billing_address,customer_legal_name,customer_tax_identifier,customer_billing_address,billing_snapshot_version,billing_snapshot_hash,quote_hash,
 issued_by,issued_at,due_at
FROM commercial.invoices WHERE id=sqlc.arg(id) AND tenant_id=sqlc.arg(tenant_id);

-- name: ListCommercialInvoiceLines :many
SELECT id,line_number,line_kind,description_code,quantity,unit_minor_units,proration_numerator,proration_denominator,net_minor_units,tax_code,tax_minor_units,gross_minor_units,calculation_hash
FROM commercial.invoice_lines WHERE invoice_id=sqlc.arg(invoice_id) ORDER BY line_number;
