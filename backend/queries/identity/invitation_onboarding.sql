-- name: InvitedAccountExists :one
SELECT EXISTS(SELECT 1 FROM identity.users WHERE email_lookup=sqlc.arg(email_lookup)::text);

-- name: VerifyNewInvitedIdentity :execrows
UPDATE identity.users SET email_verified_at=sqlc.arg(at),account_type='PLATFORM_ADMIN',auth_epoch=auth_epoch+1,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(user_id) AND account_type IS NULL AND email_verified_at IS NULL AND disabled_at IS NULL;
