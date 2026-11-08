-- name: ApplyPlatformInvitedName :execrows
UPDATE identity.users SET first_name=CASE WHEN coalesce(first_name,'')='' THEN sqlc.arg(first_name)::text ELSE first_name END,last_name=CASE WHEN coalesce(last_name,'')='' THEN sqlc.arg(last_name)::text ELSE last_name END,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(user_id) AND disabled_at IS NULL AND email_verified_at IS NOT NULL;
