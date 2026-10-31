-- name: InsertPesaPalVersion :exec
INSERT INTO platform_settings.pesapal_versions(version_id,enabled,environment,merchant_name,credentials_secret_ref,notification_id)VALUES(sqlc.arg(version_id),sqlc.arg(enabled),sqlc.arg(environment),sqlc.arg(merchant_name),sqlc.arg(credentials_secret_ref),sqlc.narg(notification_id));

-- name: GetPesaPalConfiguration :one
SELECT v.id,v.version,v.state,v.configuration_hash,p.enabled,p.environment,p.merchant_name,p.credentials_secret_ref,p.notification_id FROM platform_settings.integration_versions v JOIN platform_settings.pesapal_versions p ON p.version_id=v.id WHERE v.id=sqlc.arg(id) AND v.setting_kind='PESAPAL';

-- name: GetActivePesaPalVersionID :one
SELECT active_version_id FROM platform_settings.integration_heads WHERE setting_kind='PESAPAL' AND active_version_id IS NOT NULL;
