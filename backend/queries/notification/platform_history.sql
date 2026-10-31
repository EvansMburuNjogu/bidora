-- name: ListPlatformNotificationAttempts :many
SELECT a.id, n.category, a.attempt_number, a.started_at,
 r.id AS result_id, r.correlation_id AS result_correlation_id, r.disposition, r.failure_code, r.finished_at
FROM notification.delivery_attempts a
JOIN notification.notifications n ON n.id=a.notification_id
LEFT JOIN notification.delivery_attempt_results r ON r.attempt_id=a.id
WHERE n.tenant_id IS NULL
 AND n.category IN ('PLATFORM_INVITATION','PLATFORM_SESSION_REVOCATION','PLATFORM_IDENTITY_STATE_CHANGE','PLATFORM_ROLE_CHANGE','PLATFORM_TEMPLATE_TEST')
 AND n.source_type=n.category
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (a.started_at,a.id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY a.started_at DESC,a.id DESC
LIMIT sqlc.arg(page_limit);

-- name: InsertPlatformNotificationHistoryReadAudit :exec
INSERT INTO bidora_control_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformNotificationHistoryRead','notification_history_page',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));
