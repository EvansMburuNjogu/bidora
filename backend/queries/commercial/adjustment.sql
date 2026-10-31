-- name: ResolveAdjustmentInvoiceTenant :one
SELECT tenant_id FROM commercial.invoices WHERE id=sqlc.arg(invoice_id);

-- name: ResolveAdjustmentChangeTenant :one
SELECT tenant_id FROM commercial.financial_changes WHERE id=sqlc.arg(change_id);

-- name: GetAdjustmentCommand :one
SELECT request_hash,target_id,response_state,response_version FROM commercial.financial_commands WHERE actor_user_id=sqlc.arg(actor_user_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);

-- name: LockAdjustmentCandidate :one
SELECT i.id AS invoice_id,i.tenant_id,i.version AS invoice_version,i.tax_profile_id,i.tax_profile_hash,i.taxable_minor_units,i.tax_minor_units,i.total_minor_units,i.credit_minor_units,i.outstanding_minor_units,
 p.id AS payment_id,p.version AS payment_version,p.provider,p.amount_minor_units,p.state AS payment_state,e.settled_at,s.effect_kind AS entitlement_kind,
 t.version AS tax_profile_version,t.country,t.currency,t.tax_code,t.display_label,t.rate_numerator,t.rate_denominator,t.rounding_profile,t.effective_from,t.effective_until,t.configuration_hash,t.legal_review_reference_hash,
 COALESCE((SELECT sum(c.total_minor_units) FROM commercial.credit_notes c WHERE c.invoice_id=i.id AND c.state<>'VOID'),0)::bigint AS prior_credit_minor_units
FROM commercial.invoices i JOIN commercial.payment_operations p ON p.invoice_id=i.id JOIN commercial.payment_settlement_effects e ON e.payment_id=p.id JOIN commercial.invoice_entitlement_snapshots s ON s.invoice_id=i.id JOIN commercial.tax_profiles t ON t.id=i.tax_profile_id
WHERE i.id=sqlc.arg(invoice_id) AND i.tenant_id=sqlc.arg(tenant_id) AND i.state='PAID' AND p.state='SETTLED' AND (SELECT count(*) FROM commercial.payment_operations exact_payment WHERE exact_payment.invoice_id=i.id AND exact_payment.state='SETTLED')=1
FOR UPDATE OF i,p;

-- name: ExpirePendingFinancialChange :one
UPDATE commercial.financial_changes SET state='EXPIRED',version=version+1,decided_at=sqlc.arg(at)
WHERE tenant_id=sqlc.arg(tenant_id) AND invoice_id=sqlc.arg(invoice_id) AND state='PENDING_APPROVAL' AND expires_at<sqlc.arg(at)
RETURNING id,payment_id,action,reason_code,entitlement_disposition,amount_minor_units,version;

-- name: InsertFinancialChange :exec
INSERT INTO commercial.financial_changes(id,tenant_id,invoice_id,payment_id,action,state,version,reason_code,amount_minor_units,net_minor_units,tax_minor_units,tax_profile_id,tax_profile_hash,entitlement_disposition,maker_user_id,maker_authority_hash,proposal_reason,expected_invoice_version,expected_payment_version,request_hash,created_at,expires_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(invoice_id),sqlc.arg(payment_id),sqlc.arg(action),'PENDING_APPROVAL',1,sqlc.arg(reason_code),sqlc.arg(amount_minor_units),sqlc.arg(net_minor_units),sqlc.arg(tax_minor_units),sqlc.arg(tax_profile_id),sqlc.arg(tax_profile_hash),sqlc.arg(entitlement_disposition),sqlc.arg(maker_user_id),sqlc.arg(maker_authority_hash),sqlc.arg(proposal_reason),sqlc.arg(expected_invoice_version),sqlc.arg(expected_payment_version),sqlc.arg(request_hash),sqlc.arg(at),sqlc.arg(expires_at));

-- name: LockFinancialChange :one
SELECT c.id,c.tenant_id,c.invoice_id,c.payment_id,c.action,c.state,c.version,c.reason_code,c.amount_minor_units,c.net_minor_units,c.tax_minor_units,c.tax_profile_id,c.tax_profile_hash,c.entitlement_disposition,c.maker_user_id,c.checker_user_id,c.maker_authority_hash,c.proposal_reason,c.expected_invoice_version,c.expected_payment_version,c.request_hash,c.created_at,c.expires_at,
 (SELECT n.id FROM commercial.credit_notes n WHERE n.change_id=c.id) AS credit_note_id,COALESCE((SELECT n.credit_number FROM commercial.credit_notes n WHERE n.change_id=c.id),''::text)::text AS credit_number,(SELECT r.id FROM commercial.refund_operations r WHERE r.change_id=c.id) AS refund_id,
 i.version AS invoice_version,i.state AS invoice_state,i.total_minor_units,i.credit_minor_units,p.version AS payment_version,p.state AS payment_state,p.provider,p.amount_minor_units AS payment_amount_minor_units,
 COALESCE((SELECT sum(n.total_minor_units) FROM commercial.credit_notes n WHERE n.invoice_id=i.id AND n.state<>'VOID'),0)::bigint AS prior_credit_minor_units
FROM commercial.financial_changes c JOIN commercial.invoices i ON i.id=c.invoice_id JOIN commercial.payment_operations p ON p.id=c.payment_id
WHERE c.id=sqlc.arg(change_id) AND c.tenant_id=sqlc.arg(tenant_id) FOR UPDATE OF c,i,p;

-- name: DecideFinancialChange :execrows
UPDATE commercial.financial_changes SET state=sqlc.arg(state),version=version+1,checker_user_id=sqlc.arg(checker_user_id),checker_authority_hash=sqlc.arg(checker_authority_hash),decision_reason=sqlc.arg(decision_reason),decided_at=sqlc.arg(at)
WHERE id=sqlc.arg(change_id) AND state='PENDING_APPROVAL' AND version=sqlc.arg(expected_version);

-- name: InsertFinancialChangeDecision :exec
INSERT INTO commercial.financial_change_decisions(id,change_id,decision,checker_user_id,expected_change_version,reason,authority_hash,decided_at)
VALUES(sqlc.arg(id),sqlc.arg(change_id),sqlc.arg(decision),sqlc.arg(checker_user_id),sqlc.arg(expected_change_version),sqlc.arg(reason),sqlc.arg(authority_hash),sqlc.arg(at));

-- name: InsertCreditNote :exec
INSERT INTO commercial.credit_notes(id,invoice_id,change_id,credit_number,state,version,reason_code,currency,net_minor_units,tax_minor_units,total_minor_units,original_tax_profile_id,issued_by,issued_at,content_hash)
VALUES(sqlc.arg(id),sqlc.arg(invoice_id),sqlc.arg(change_id),sqlc.arg(credit_number),sqlc.arg(state),1,sqlc.arg(reason_code),'KES',sqlc.arg(net_minor_units),sqlc.arg(tax_minor_units),sqlc.arg(total_minor_units),sqlc.arg(tax_profile_id),sqlc.arg(issued_by),sqlc.arg(at),sqlc.arg(content_hash));

-- name: InsertRefundIntent :exec
INSERT INTO commercial.refund_operations(id,change_id,payment_id,credit_note_id,provider,state,version,amount_minor_units,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(change_id),sqlc.arg(payment_id),sqlc.arg(credit_note_id),sqlc.arg(provider),'APPROVED',1,sqlc.arg(amount_minor_units),sqlc.arg(at),sqlc.arg(at));

-- name: MarkPaymentRefundPending :execrows
UPDATE commercial.payment_operations SET state='REFUND_PENDING',version=version+1,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(payment_id) AND state='SETTLED' AND version=sqlc.arg(expected_version);

-- name: InsertAdjustmentCommand :exec
INSERT INTO commercial.financial_commands(id,actor_user_id,tenant_id,operation,key_hash,request_hash,target_id,response_state,response_version,response_status,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_user_id),sqlc.arg(tenant_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(target_id),sqlc.arg(response_state),sqlc.arg(response_version),sqlc.arg(response_status),sqlc.arg(at),sqlc.arg(at));
