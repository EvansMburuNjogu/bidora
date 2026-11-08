-- name: GetPendingIntegrationChangeForVersion :one
SELECT id FROM platform_settings.integration_changes
WHERE target_version_id=sqlc.arg(version_id) AND state='PENDING_APPROVAL'
ORDER BY created_at DESC,id DESC LIMIT 1;
