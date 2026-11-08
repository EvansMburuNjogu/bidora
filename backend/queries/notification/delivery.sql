-- name: GetNotificationByIdempotencyHash :one
SELECT id,tenant_id,actor_id,recipient_user_id,category,template_key,template_version,source_type,source_id,
 recipient_fingerprint,content_fingerprint,fingerprint_key_version,idempotency_key_hash,correlation_id,created_at
FROM notification.notifications WHERE idempotency_key_hash=sqlc.arg(idempotency_key_hash);

-- name: InsertNotification :exec
INSERT INTO notification.notifications
(id,tenant_id,actor_id,recipient_user_id,category,template_key,template_version,source_type,source_id,
 recipient_fingerprint,content_fingerprint,fingerprint_key_version,idempotency_key_hash,correlation_id,created_at)
VALUES(sqlc.arg(id),sqlc.narg(tenant_id),sqlc.narg(actor_id),sqlc.narg(recipient_user_id),sqlc.arg(category),sqlc.arg(template_key),sqlc.arg(template_version),sqlc.arg(source_type),sqlc.arg(source_id),
 sqlc.arg(recipient_fingerprint),sqlc.arg(content_fingerprint),sqlc.arg(fingerprint_key_version),sqlc.arg(idempotency_key_hash),sqlc.arg(correlation_id),sqlc.arg(created_at));

-- name: GetNotification :one
SELECT id,tenant_id,actor_id,recipient_user_id,category,template_key,template_version,source_type,source_id,
 recipient_fingerprint,content_fingerprint,fingerprint_key_version,idempotency_key_hash,correlation_id,created_at
FROM notification.notifications WHERE id=sqlc.arg(id);

-- name: LockNotificationForDelivery :exec
-- Serialize starts/results without UPDATE privileges on append-only evidence.
SELECT pg_advisory_xact_lock(hashtextextended('bidora:notification:' || sqlc.arg(id)::uuid::text, 0));

-- name: GetDeliveryAttemptProgress :one
SELECT count(a.id)::integer AS attempt_count,
 count(a.id) FILTER(WHERE r.id IS NULL)::integer AS unfinished_count,
 count(a.id) FILTER(WHERE r.disposition='SMTP_ACCEPTED')::integer AS accepted_count,
 count(a.id) FILTER(WHERE r.disposition='AMBIGUOUS')::integer AS ambiguous_count
FROM notification.delivery_attempts a
LEFT JOIN notification.delivery_attempt_results r ON r.attempt_id=a.id
WHERE a.notification_id=sqlc.arg(notification_id);

-- name: IsActiveSMTPVersion :one
SELECT EXISTS(
 SELECT 1 FROM platform_settings.integration_versions v
 JOIN platform_settings.integration_heads h ON h.setting_kind=v.setting_kind AND h.active_version_id=v.id
 WHERE v.id=sqlc.arg(version_id) AND v.setting_kind='SMTP' AND v.state='ACTIVE'
);

-- name: GetDeliveryAttempt :one
SELECT id,notification_id,attempt_number,smtp_version_id,correlation_id,started_at
FROM notification.delivery_attempts WHERE id=sqlc.arg(id);

-- name: GetDeliveryAttemptByNumber :one
SELECT id,notification_id,attempt_number,smtp_version_id,correlation_id,started_at
FROM notification.delivery_attempts WHERE notification_id=sqlc.arg(notification_id) AND attempt_number=sqlc.arg(attempt_number);

-- name: InsertDeliveryAttempt :exec
INSERT INTO notification.delivery_attempts(id,notification_id,attempt_number,smtp_version_id,correlation_id,started_at)
VALUES(sqlc.arg(id),sqlc.arg(notification_id),sqlc.arg(attempt_number),sqlc.arg(smtp_version_id),sqlc.arg(correlation_id),sqlc.arg(started_at));

-- name: GetDeliveryAttemptResult :one
SELECT id,attempt_id,disposition,failure_code,correlation_id,finished_at
FROM notification.delivery_attempt_results WHERE attempt_id=sqlc.arg(attempt_id);

-- name: InsertDeliveryAttemptResult :exec
INSERT INTO notification.delivery_attempt_results(id,attempt_id,disposition,failure_code,correlation_id,finished_at)
VALUES(sqlc.arg(id),sqlc.arg(attempt_id),sqlc.arg(disposition),sqlc.narg(failure_code),sqlc.arg(correlation_id),sqlc.arg(finished_at));

-- name: InsertNotificationAudit :exec
INSERT INTO bidora_control_audit.audit_events
(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.narg(tenant_id),sqlc.narg(actor_id),sqlc.arg(action),'notification',sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(change_summary),1,sqlc.arg(occurred_at));

-- name: InsertNotificationOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events
(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.narg(tenant_id),sqlc.narg(actor_id),'notification',sqlc.arg(aggregate_id),sqlc.arg(aggregate_version),sqlc.arg(event_type),1,'INTERNAL',sqlc.arg(payload),sqlc.arg(occurred_at),NULL,0,sqlc.arg(occurred_at),sqlc.arg(occurred_at));
