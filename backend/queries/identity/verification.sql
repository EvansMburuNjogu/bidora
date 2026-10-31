-- Issuance requires target lock + Invalidate + Insert + audit in one control
-- transaction. Generated methods are persistence primitives, not authorization.

-- name: LockEmailVerificationIssuanceTarget :one
SELECT email
FROM identity.users
WHERE id = sqlc.arg(user_id)
  AND email_verified_at IS NULL
  AND disabled_at IS NULL
FOR UPDATE;

-- name: InvalidateEmailVerificationChallenges :execrows
UPDATE identity.challenges
SET invalidated_at = sqlc.arg(invalidated_at)
WHERE user_id = sqlc.arg(user_id)
  AND purpose = 'EMAIL_VERIFY'
  AND consumed_at IS NULL
  AND invalidated_at IS NULL;

-- name: InsertEmailVerificationChallenge :exec
INSERT INTO identity.challenges (
    id, user_id, purpose, token_hash, expires_at, consumed_at,
    invalidated_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), 'EMAIL_VERIFY', sqlc.arg(token_hash),
    sqlc.arg(expires_at), NULL, NULL, sqlc.arg(created_at)
);

-- name: InsertEmailVerificationIssuedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.narg(actor_id), 'EmailVerificationIssued', 'user',
    sqlc.arg(target_id), sqlc.arg(occurred_at), 'SUCCEEDED',
    sqlc.arg(correlation_id), '{}'::jsonb, NULL, sqlc.arg(occurred_at)
);

-- name: ConsumeEmailVerificationChallenge :one
WITH consumed AS (
    UPDATE identity.challenges
    SET consumed_at = sqlc.arg(consumed_at)
    WHERE token_hash = sqlc.arg(token_hash)
      AND purpose = 'EMAIL_VERIFY'
      AND consumed_at IS NULL
      AND invalidated_at IS NULL
      AND expires_at > sqlc.arg(consumed_at)
    RETURNING user_id
)
UPDATE identity.users AS u
SET email_verified_at = COALESCE(u.email_verified_at, sqlc.arg(consumed_at)),
    updated_at = sqlc.arg(consumed_at)
FROM consumed
WHERE u.id = consumed.user_id
RETURNING u.id, u.email_verified_at, u.auth_epoch;

-- name: InsertEmailVerifiedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.arg(actor_id), 'EmailVerified', 'user',
    sqlc.arg(actor_id), sqlc.arg(occurred_at), 'SUCCEEDED',
    sqlc.arg(correlation_id), '{}'::jsonb, NULL, sqlc.arg(occurred_at)
);
