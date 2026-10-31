-- name: LockCommercialTrialIdentity :one
SELECT id FROM identity.users
WHERE id=sqlc.arg(user_id) AND account_type='TENDERPRENEUR' AND email_verified_at IS NOT NULL AND disabled_at IS NULL
FOR KEY SHARE;
