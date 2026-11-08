-- name: ReadPlatformOverview :one
SELECT
 CASE WHEN sqlc.arg(tenants_allowed)::boolean THEN (SELECT count(*)::text FROM bidora_tenancy.tenants) ELSE ''::text END AS tenderpreneurs,
 CASE WHEN sqlc.arg(subscriptions_allowed)::boolean THEN
  (SELECT count(*)::text FROM commercial.subscriptions s JOIN commercial.subscription_terms t ON t.id=s.current_term_id AND t.subscription_id=s.id
   WHERE s.state IN ('ACTIVE','TRIAL_ACTIVE') AND t.commercial_period_start<=sqlc.arg(observed_at)::timestamptz AND t.commercial_period_end>sqlc.arg(observed_at)::timestamptz
   AND NOT EXISTS(SELECT 1 FROM commercial.subscription_access_holds h WHERE h.subscription_id=s.id AND h.state='ACTIVE'))
 ELSE ''::text END AS active_subscriptions,
 CASE WHEN sqlc.arg(invoices_allowed)::boolean THEN
  (SELECT coalesce(sum(outstanding_minor_units),0)::text FROM commercial.invoices WHERE currency='KES' AND state<>'VOID' AND outstanding_minor_units>0)
 ELSE ''::text END AS outstanding_minor_units,
 CASE WHEN sqlc.arg(support_allowed)::boolean THEN (SELECT count(*)::text FROM bidora_support.cases WHERE state='OPEN') ELSE ''::text END AS open_support_cases,
 CASE WHEN sqlc.arg(invoices_allowed)::boolean THEN (SELECT coalesce(sum(total_minor_units),0)::text FROM commercial.invoices WHERE currency='KES' AND state<>'VOID') ELSE ''::text END AS invoiced_minor_units,
 CASE WHEN sqlc.arg(invoices_allowed)::boolean THEN (SELECT count(*)::text FROM commercial.invoices WHERE state='PAID') ELSE ''::text END AS paid_invoices,
 CASE WHEN sqlc.arg(payments_allowed)::boolean THEN (SELECT count(*)::text FROM commercial.payment_operations WHERE state='FAILED') ELSE ''::text END AS failed_payments;

-- name: ReadPlatformMonthlyInvoicing :many
WITH months AS (SELECT generate_series(date_trunc('month',sqlc.arg(observed_at)::timestamptz AT TIME ZONE 'UTC')-interval '5 months',date_trunc('month',sqlc.arg(observed_at)::timestamptz AT TIME ZONE 'UTC'),interval '1 month') AS month)
SELECT to_char(m.month,'YYYY-MM')::text AS month,coalesce(sum(i.total_minor_units),0)::text AS total_minor_units
FROM months m LEFT JOIN commercial.invoices i ON i.currency='KES' AND i.state<>'VOID' AND i.issued_at>=m.month AT TIME ZONE 'UTC' AND i.issued_at<(m.month+interval '1 month') AT TIME ZONE 'UTC'
GROUP BY m.month ORDER BY m.month;

-- name: InsertPlatformOverviewReadAudit :exec
INSERT INTO bidora_control_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformOverviewRead','platform_overview',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),'{}',1,sqlc.arg(at));
