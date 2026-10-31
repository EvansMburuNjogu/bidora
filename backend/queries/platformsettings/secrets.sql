-- name: InsertSMTPSecret :exec
INSERT INTO platform_settings.smtp_secrets(id,secret_kind,key_version,nonce,ciphertext,created_by,created_at) VALUES(sqlc.arg(id),sqlc.arg(secret_kind),sqlc.arg(key_version),sqlc.arg(nonce),sqlc.arg(ciphertext),sqlc.arg(created_by),sqlc.arg(created_at));
-- name: GetSMTPSecret :one
SELECT secret_kind,key_version,nonce,ciphertext FROM platform_settings.smtp_secrets WHERE id=sqlc.arg(id) AND retired_at IS NULL;
