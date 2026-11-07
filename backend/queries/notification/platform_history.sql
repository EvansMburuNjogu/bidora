-- name: ListPlatformNotificationAttempts :many
SELECT COALESCE(a.id,n.id)::uuid AS id,n.id AS notification_id,n.category,
 COALESCE(a.attempt_number,0)::integer AS attempt_number,COALESCE(a.started_at,n.created_at)::timestamptz AS started_at,
 r.id AS result_id,r.correlation_id AS result_correlation_id,r.disposition,r.failure_code,r.finished_at,
 m.key_version,m.nonce,m.ciphertext,
 COALESCE(CASE WHEN n.created_at>=CURRENT_TIMESTAMP-make_interval(days=>COALESCE((SELECT restricted_metadata_retention_days FROM platform_settings.operational_policy WHERE singleton),90)) THEN u.email_lookup ELSE NULL END,'')::text AS current_recipient,
 n.recipient_fingerprint
FROM notification.notifications n
LEFT JOIN notification.delivery_attempts a ON a.notification_id=n.id
LEFT JOIN notification.delivery_attempt_results r ON r.attempt_id=a.id
LEFT JOIN notification.message_snapshots m ON m.notification_id=n.id AND m.expires_at>CURRENT_TIMESTAMP
 AND m.created_at>=CURRENT_TIMESTAMP-make_interval(days=>COALESCE((SELECT restricted_metadata_retention_days FROM platform_settings.operational_policy WHERE singleton),90))
LEFT JOIN identity.users u ON u.id=n.recipient_user_id
WHERE (sqlc.narg(cursor_at)::timestamptz IS NULL OR
 (COALESCE(a.started_at,n.created_at),COALESCE(a.id,n.id))<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
 AND (sqlc.arg(category)::text='' OR n.category=sqlc.arg(category)::text)
 AND (sqlc.arg(status)::text='' OR
  (sqlc.arg(status)::text='NOT_ATTEMPTED' AND a.id IS NULL) OR
  (sqlc.arg(status)::text='PENDING' AND a.id IS NOT NULL AND r.id IS NULL) OR
  r.disposition=sqlc.arg(status)::text)
 AND (sqlc.narg(date_from)::timestamptz IS NULL OR COALESCE(a.started_at,n.created_at)>=sqlc.narg(date_from)::timestamptz)
 AND (sqlc.narg(date_to)::timestamptz IS NULL OR COALESCE(a.started_at,n.created_at)<sqlc.narg(date_to)::timestamptz)
ORDER BY COALESCE(a.started_at,n.created_at) DESC,COALESCE(a.id,n.id) DESC LIMIT sqlc.arg(page_limit);

-- name: InsertPlatformNotificationHistoryReadAudit :exec
INSERT INTO bidora_control_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformNotificationHistoryRead','notification_history_page',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));
