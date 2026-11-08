-- name: LockPlatformBootstrapIdentityCandidate :one
SELECT u.id, u.auth_epoch, u.email_lookup
FROM identity.users AS u
WHERE u.id = sqlc.arg(user_id)
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND u.account_type IS NULL
  AND EXISTS (SELECT 1 FROM identity.sessions s WHERE s.user_id=u.id AND s.auth_epoch=u.auth_epoch
    AND s.revoked_at IS NULL AND s.assurance_method='EMAIL_OTP' AND s.assurance_level IN ('AAL1','AAL2')
    AND s.idle_expires_at>CURRENT_TIMESTAMP AND s.absolute_expires_at>CURRENT_TIMESTAMP
    AND s.assured_at<=CURRENT_TIMESTAMP AND s.assured_at>=CURRENT_TIMESTAMP-interval '15 minutes')
FOR UPDATE OF u;

-- name: PromotePlatformIdentity :execrows
UPDATE identity.users
SET account_type = 'PLATFORM_ADMIN', auth_epoch = auth_epoch + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(user_id)
  AND account_type IS NULL
  AND auth_epoch = sqlc.arg(expected_auth_epoch)
  AND email_verified_at IS NOT NULL
  AND disabled_at IS NULL;

-- name: RevokePlatformIdentitySessions :execrows
UPDATE identity.sessions
SET revoked_at = sqlc.arg(at)
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL;

-- name: GetPlatformIdentitySessionFacts :one
SELECT u.id AS user_id, u.auth_epoch, u.email_lookup, s.assurance_level, s.assurance_method,
       s.assured_at, s.idle_expires_at, s.absolute_expires_at
FROM identity.users AS u
JOIN identity.sessions AS s ON s.user_id = u.id AND s.auth_epoch = u.auth_epoch
WHERE u.id = sqlc.arg(user_id) AND u.account_type = 'PLATFORM_ADMIN'
  AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL
  AND s.id = sqlc.arg(session_id) AND s.revoked_at IS NULL
  AND s.idle_expires_at > sqlc.arg(observed_at) AND s.absolute_expires_at > sqlc.arg(observed_at)
  AND (EXISTS (SELECT 1 FROM identity.mfa_factors AS f WHERE f.user_id = u.id AND f.state = 'ACTIVE')
       OR (s.assurance_level='AAL2' AND s.assurance_method='EMAIL_OTP'));

-- name: LockPlatformInviteeIdentity :one
SELECT u.id, u.auth_epoch, u.email_lookup
FROM identity.users AS u
WHERE u.id = sqlc.arg(user_id) AND u.email_lookup = sqlc.arg(email_lookup)
  AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND u.account_type IS NULL
  AND EXISTS (
      SELECT 1 FROM identity.sessions AS s
      WHERE s.id = sqlc.arg(session_id) AND s.user_id = u.id AND s.auth_epoch = u.auth_epoch
        AND s.revoked_at IS NULL AND s.idle_expires_at > sqlc.arg(observed_at)
        AND s.absolute_expires_at > sqlc.arg(observed_at)
        AND s.assurance_level IN ('AAL1','AAL2') AND s.assurance_method = 'EMAIL_OTP'
        AND s.assured_at <= sqlc.arg(observed_at)
        AND s.assured_at >= sqlc.arg(observed_at) - interval '15 minutes'
  )
FOR UPDATE OF u;

-- name: CountEffectivePlatformIdentities :one
SELECT count(*)::bigint
FROM identity.users AS u
WHERE u.id = ANY(sqlc.arg(user_ids)::uuid[])
  AND u.account_type = 'PLATFORM_ADMIN'
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND (EXISTS (SELECT 1 FROM identity.mfa_factors AS f WHERE f.user_id = u.id AND f.state = 'ACTIVE')
    OR EXISTS (SELECT 1 FROM identity.sessions s WHERE s.user_id=u.id AND s.auth_epoch=u.auth_epoch AND s.revoked_at IS NULL AND s.assurance_level='AAL2' AND s.assurance_method='EMAIL_OTP' AND s.idle_expires_at>CURRENT_TIMESTAMP AND s.absolute_expires_at>CURRENT_TIMESTAMP));

-- name: FindPlatformAdministratorByEmailLookup :one
SELECT id FROM identity.users
WHERE email_lookup = sqlc.arg(email_lookup)
  AND account_type = 'PLATFORM_ADMIN' AND email_verified_at IS NOT NULL;

-- name: ListPlatformAdministratorIdentitySummaries :many
SELECT u.id, u.email,coalesce(u.first_name,'')::text AS first_name,coalesce(u.last_name,'')::text AS last_name, u.disabled_at,
       EXISTS (SELECT 1 FROM identity.mfa_factors AS f WHERE f.user_id = u.id AND f.state = 'ACTIVE') AS totp_enrolled,
       EXISTS (SELECT 1 FROM identity.sessions s WHERE s.user_id=u.id AND s.auth_epoch=u.auth_epoch AND s.revoked_at IS NULL AND s.assurance_level='AAL2' AND s.assurance_method='EMAIL_OTP' AND s.idle_expires_at>CURRENT_TIMESTAMP AND s.absolute_expires_at>CURRENT_TIMESTAMP) AS email_otp_verified
FROM identity.users AS u
WHERE u.id = ANY(sqlc.arg(user_ids)::uuid[])
  AND u.account_type = 'PLATFORM_ADMIN' AND u.email_verified_at IS NOT NULL
ORDER BY u.id;

-- name: LockPlatformAdministratorIdentity :one
SELECT id, email, email_lookup, auth_epoch, disabled_at
FROM identity.users
WHERE id = sqlc.arg(user_id)
  AND account_type = 'PLATFORM_ADMIN'
  AND email_verified_at IS NOT NULL
FOR UPDATE;

-- name: AdvancePlatformIdentityEpoch :execrows
UPDATE identity.users
SET auth_epoch = auth_epoch + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(user_id) AND account_type = 'PLATFORM_ADMIN'
  AND auth_epoch = sqlc.arg(expected_auth_epoch);

-- name: SetPlatformIdentityDisabled :execrows
UPDATE identity.users
SET disabled_at = sqlc.arg(at), auth_epoch = auth_epoch + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(user_id) AND account_type = 'PLATFORM_ADMIN'
  AND email_verified_at IS NOT NULL AND disabled_at IS NULL
  AND auth_epoch = sqlc.arg(expected_auth_epoch);

-- name: SetPlatformIdentityActive :execrows
UPDATE identity.users
SET disabled_at = NULL, auth_epoch = auth_epoch + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(user_id) AND account_type = 'PLATFORM_ADMIN'
  AND email_verified_at IS NOT NULL AND disabled_at IS NOT NULL
  AND auth_epoch = sqlc.arg(expected_auth_epoch);
