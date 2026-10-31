-- name: GetTenantCommercialPaymentView :one
SELECT p.id,p.invoice_id,p.state,p.version,p.currency,p.amount_minor_units,p.reconciliation_state,p.updated_at,r.id AS receipt_id,e.effect_kind
FROM commercial.payment_operations p
LEFT JOIN commercial.receipts r ON r.payment_id=p.id
LEFT JOIN commercial.payment_settlement_effects e ON e.payment_id=p.id
WHERE p.id=sqlc.arg(payment_id) AND p.tenant_id=sqlc.arg(tenant_id);

-- name: GetTenantCommercialReceipt :one
SELECT r.id,r.payment_id,r.invoice_id,r.receipt_number,r.currency,r.amount_minor_units,r.issued_at,r.content_hash
FROM commercial.receipts r JOIN commercial.invoices i ON i.id=r.invoice_id
WHERE r.id=sqlc.arg(receipt_id) AND i.tenant_id=sqlc.arg(tenant_id);

-- name: GetTenantCommercialCreditNote :one
SELECT n.id,n.invoice_id,n.change_id,n.credit_number,n.state,n.version,n.reason_code,n.currency,n.net_minor_units,n.tax_minor_units,n.total_minor_units,n.issued_at,n.content_hash
FROM commercial.credit_notes n JOIN commercial.invoices i ON i.id=n.invoice_id
WHERE n.id=sqlc.arg(credit_note_id) AND i.tenant_id=sqlc.arg(tenant_id);

-- name: GetPlatformFinancialAdjustment :one
SELECT c.id,c.tenant_id,c.invoice_id,c.payment_id,c.action,c.state,c.version,c.reason_code,c.amount_minor_units,c.net_minor_units,c.tax_minor_units,c.entitlement_disposition,c.maker_user_id,c.checker_user_id,c.proposal_reason,c.decision_reason,c.created_at,c.expires_at,c.decided_at,
 n.id AS credit_note_id,n.credit_number,n.state AS credit_note_state,r.id AS refund_id,r.state AS refund_state
FROM commercial.financial_changes c
LEFT JOIN commercial.credit_notes n ON n.change_id=c.id
LEFT JOIN commercial.refund_operations r ON r.change_id=c.id
WHERE c.id=sqlc.arg(change_id);

-- name: GetPlatformPaymentReversal :one
SELECT e.payment_id,p.tenant_id,e.invoice_id,e.subscription_id,e.settlement_observation_id,e.reversal_observation_id,e.access_hold_id,e.prior_payment_version,e.resulting_payment_version,e.prior_invoice_version,e.resulting_invoice_version,e.prior_subscription_version,e.resulting_subscription_version,e.amount_minor_units,e.currency,e.provider_observed_at,e.recorded_at,e.applied_at,
 p.state AS payment_state,i.state AS invoice_state,s.state AS subscription_state,h.access_mode,h.state AS hold_state
FROM commercial.payment_reversal_effects e
JOIN commercial.payment_operations p ON p.id=e.payment_id
JOIN commercial.invoices i ON i.id=e.invoice_id
JOIN commercial.subscriptions s ON s.id=e.subscription_id
JOIN commercial.subscription_access_holds h ON h.id=e.access_hold_id
WHERE e.payment_id=sqlc.arg(payment_id);
