-- name: InsertNotificationMessage :exec
INSERT INTO notification.message_snapshots(notification_id,key_version,nonce,ciphertext,expires_at)
VALUES(sqlc.arg(notification_id),sqlc.arg(key_version),sqlc.arg(nonce),sqlc.arg(ciphertext),
 CURRENT_TIMESTAMP+make_interval(days=>COALESCE((SELECT restricted_metadata_retention_days FROM platform_settings.operational_policy WHERE singleton),90)))
ON CONFLICT(notification_id) DO NOTHING;

-- name: PurgeExpiredNotificationMessages :exec
DELETE FROM notification.message_snapshots WHERE notification_id IN (
 SELECT notification_id FROM notification.message_snapshots
 WHERE expires_at<=CURRENT_TIMESTAMP OR created_at<CURRENT_TIMESTAMP-make_interval(days=>COALESCE((SELECT restricted_metadata_retention_days FROM platform_settings.operational_policy WHERE singleton),90))
 ORDER BY expires_at LIMIT 1000
);
