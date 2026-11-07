-- name: ListPlatformNotificationAttempts :many
SELECT COALESCE(a.id,n.id)::uuid AS id, n.category, COALESCE(a.attempt_number,0)::integer AS attempt_number,
 COALESCE(a.started_at,n.created_at)::timestamptz AS started_at,
 r.id AS result_id, r.correlation_id AS result_correlation_id, r.disposition, r.failure_code, r.finished_at
FROM notification.notifications n
LEFT JOIN notification.delivery_attempts a ON a.notification_id=n.id
LEFT JOIN notification.delivery_attempt_results r ON r.attempt_id=a.id
-- Platform-authorized metadata only: no message body, token or recipient data.
WHERE (sqlc.narg(cursor_at)::timestamptz IS NULL OR
 (COALESCE(a.started_at,n.created_at),COALESCE(a.id,n.id))<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY COALESCE(a.started_at,n.created_at) DESC,COALESCE(a.id,n.id) DESC
LIMIT sqlc.arg(page_limit);

-- name: InsertPlatformNotificationHistoryReadAudit :exec
INSERT INTO bidora_control_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformNotificationHistoryRead','notification_history_page',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));
