-- name: ListFilteredPlatformAIUsage :many
SELECT r.id,r.tenant_id,t.name AS tenant_name,r.kind,r.actual_credits,r.settled_at
FROM commercial.ai_usage_reservations r JOIN bidora_tenancy.tenants t ON t.id=r.tenant_id
WHERE r.status='SETTLED' AND r.actual_credits IS NOT NULL AND r.settled_at IS NOT NULL
 AND (sqlc.narg(tenant_filter)::uuid IS NULL OR r.tenant_id=sqlc.narg(tenant_filter)::uuid)
 AND (sqlc.narg(kind_filter)::text IS NULL OR r.kind=sqlc.narg(kind_filter)::text)
 AND (sqlc.narg(from_at)::timestamptz IS NULL OR r.settled_at>=sqlc.narg(from_at)::timestamptz)
 AND (sqlc.narg(before_at)::timestamptz IS NULL OR r.settled_at<sqlc.narg(before_at)::timestamptz)
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (r.settled_at,r.id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY r.settled_at DESC,r.id DESC LIMIT sqlc.arg(page_limit);
