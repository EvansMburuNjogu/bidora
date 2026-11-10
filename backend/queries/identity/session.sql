-- name: InsertSession :exec
INSERT INTO identity.sessions (
    id, user_id, token_hash, csrf_hash, auth_epoch, rotated_from_session_id,
    created_at, last_seen_at, idle_expires_at, absolute_expires_at, revoked_at,
    assurance_level, assured_at, assurance_method, idle_timeout_seconds, absolute_timeout_seconds
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(csrf_hash),
    sqlc.arg(auth_epoch), sqlc.narg(rotated_from_session_id),
    sqlc.arg(created_at)::timestamptz, sqlc.arg(created_at)::timestamptz,
    sqlc.arg(idle_expires_at), sqlc.arg(absolute_expires_at), NULL,
    sqlc.arg(assurance_level), sqlc.arg(assured_at), sqlc.arg(assurance_method),
    sqlc.arg(idle_timeout_seconds), sqlc.arg(absolute_timeout_seconds)
);

-- name: InsertLoginAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.arg(actor_id), 'UserLoggedIn', 'session',
    sqlc.arg(target_id), sqlc.arg(occurred_at), 'SUCCEEDED',
    sqlc.arg(correlation_id), sqlc.arg(change_summary), NULL, sqlc.arg(occurred_at)
);

-- name: ListCurrentUserActiveSessions :many
WITH policy AS (SELECT sqlc.arg(policy_idle_seconds)::integer AS idle_seconds,sqlc.arg(policy_absolute_seconds)::integer AS absolute_seconds,sqlc.arg(admin_idle_seconds)::integer AS admin_idle_seconds,sqlc.arg(admin_absolute_seconds)::integer AS admin_absolute_seconds)
SELECT s.id,s.rotated_from_session_id,s.created_at,s.last_seen_at,LEAST(s.idle_expires_at,s.last_seen_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_idle_seconds ELSE p.idle_seconds END),s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))::timestamptz AS idle_expires_at,LEAST(s.absolute_expires_at,s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))::timestamptz AS absolute_expires_at
FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id CROSS JOIN policy p
WHERE u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND u.auth_epoch=s.auth_epoch AND s.revoked_at IS NULL
 AND LEAST(s.idle_expires_at,s.last_seen_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_idle_seconds ELSE p.idle_seconds END),s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))>sqlc.arg(observed_at) AND LEAST(s.absolute_expires_at,s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))>sqlc.arg(observed_at) AND s.user_id=sqlc.arg(user_id)
ORDER BY s.created_at DESC,s.id DESC LIMIT 101;

-- name: GetActiveSession :one
WITH policy AS (SELECT sqlc.arg(policy_idle_seconds)::integer AS idle_seconds,sqlc.arg(policy_absolute_seconds)::integer AS absolute_seconds,sqlc.arg(admin_idle_seconds)::integer AS admin_idle_seconds,sqlc.arg(admin_absolute_seconds)::integer AS admin_absolute_seconds)
SELECT s.id,s.user_id,s.csrf_hash,s.auth_epoch,LEAST(s.idle_expires_at,s.last_seen_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_idle_seconds ELSE p.idle_seconds END),s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))::timestamptz AS idle_expires_at,LEAST(s.absolute_expires_at,s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))::timestamptz AS absolute_expires_at,s.assurance_level,s.assured_at,s.assurance_method,s.created_at AS authenticated_at
FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id CROSS JOIN policy p
WHERE u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND u.auth_epoch=s.auth_epoch AND s.revoked_at IS NULL
 AND LEAST(s.idle_expires_at,s.last_seen_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_idle_seconds ELSE p.idle_seconds END),s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))>sqlc.arg(observed_at) AND LEAST(s.absolute_expires_at,s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))>sqlc.arg(observed_at) AND s.token_hash=sqlc.arg(token_hash);

-- Touch only accepted requests; rejected traffic cannot prolong sessions.
-- name: TouchActiveSession :execrows
WITH policy AS (SELECT sqlc.arg(policy_idle_seconds)::integer AS idle_seconds,sqlc.arg(policy_absolute_seconds)::integer AS absolute_seconds,sqlc.arg(admin_idle_seconds)::integer AS admin_idle_seconds,sqlc.arg(admin_absolute_seconds)::integer AS admin_absolute_seconds)
UPDATE identity.sessions s SET last_seen_at=GREATEST(s.last_seen_at,sqlc.arg(observed_at)),idle_expires_at=LEAST(GREATEST(s.last_seen_at,sqlc.arg(observed_at))+make_interval(secs=>LEAST(s.idle_timeout_seconds,CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_idle_seconds ELSE p.idle_seconds END)),s.absolute_expires_at,s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))
FROM identity.users u CROSS JOIN policy p WHERE u.id=s.user_id AND s.id=sqlc.arg(id) AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND u.auth_epoch=s.auth_epoch AND s.revoked_at IS NULL AND LEAST(s.idle_expires_at,s.last_seen_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_idle_seconds ELSE p.idle_seconds END),s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))>sqlc.arg(observed_at) AND LEAST(s.absolute_expires_at,s.created_at+make_interval(secs=>CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN p.admin_absolute_seconds ELSE p.absolute_seconds END))>sqlc.arg(observed_at);

-- name: RevokeOrdinarySessionsForPolicy :execrows
UPDATE identity.sessions SET revoked_at=sqlc.arg(at)::timestamptz
WHERE revoked_at IS NULL AND idle_timeout_seconds<=1800;

-- name: TightenSessionsForPolicy :execrows
WITH limits AS (
 SELECT s.id,CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN sqlc.arg(admin_idle_seconds)::integer ELSE sqlc.arg(idle_seconds)::integer END AS idle_seconds,
 CASE WHEN u.account_type='PLATFORM_ADMIN' AND s.assurance_level='AAL2' AND s.assurance_method IN ('EMAIL_OTP','TOTP','RECOVERY_CODE') THEN sqlc.arg(admin_absolute_seconds)::integer ELSE sqlc.arg(absolute_seconds)::integer END AS absolute_seconds
 FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id WHERE s.revoked_at IS NULL
)
UPDATE identity.sessions s SET idle_timeout_seconds=LEAST(s.idle_timeout_seconds,l.idle_seconds),absolute_timeout_seconds=LEAST(s.absolute_timeout_seconds,l.absolute_seconds),absolute_expires_at=LEAST(s.absolute_expires_at,s.created_at+make_interval(secs=>l.absolute_seconds)),idle_expires_at=LEAST(s.idle_expires_at,s.last_seen_at+make_interval(secs=>l.idle_seconds),s.absolute_expires_at,s.created_at+make_interval(secs=>l.absolute_seconds))
FROM limits l WHERE l.id=s.id;

-- name: RevokeExpiredSessionsForPolicy :execrows
UPDATE identity.sessions s SET revoked_at=GREATEST(sqlc.arg(at)::timestamptz,s.created_at,s.last_seen_at)
FROM identity.users u WHERE u.id=s.user_id AND s.revoked_at IS NULL AND
 (s.idle_expires_at<=sqlc.arg(at)::timestamptz OR s.absolute_expires_at<=sqlc.arg(at)::timestamptz OR s.auth_epoch<>u.auth_epoch OR u.disabled_at IS NOT NULL OR u.email_verified_at IS NULL);

-- name: RevokeExcessSessionsForPolicy :execrows
WITH ranked AS (
 SELECT s.id,row_number() OVER (PARTITION BY s.user_id ORDER BY (s.id=sqlc.arg(current_session_id)::uuid) DESC,s.created_at DESC,s.id DESC) AS rank
 FROM identity.sessions s WHERE s.revoked_at IS NULL AND s.idle_expires_at>sqlc.arg(at)::timestamptz AND s.absolute_expires_at>sqlc.arg(at)::timestamptz
)
UPDATE identity.sessions s SET revoked_at=GREATEST(sqlc.arg(at)::timestamptz,s.created_at,s.last_seen_at)
FROM ranked r WHERE s.id=r.id AND r.rank>1 AND NOT sqlc.arg(allow_multiple_sessions)::boolean;

-- name: IsPlatformAdministrator :one
SELECT EXISTS(SELECT 1 FROM identity.users WHERE id=sqlc.arg(user_id) AND account_type='PLATFORM_ADMIN' AND disabled_at IS NULL AND email_verified_at IS NOT NULL);

-- name: InsertLogoutAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.arg(actor_id), 'UserLoggedOut', 'session',
    sqlc.arg(target_id), sqlc.arg(occurred_at), 'SUCCEEDED',
    sqlc.arg(correlation_id), '{}'::jsonb, NULL, sqlc.arg(occurred_at)
);

-- name: RevokeSessionByToken :execrows
UPDATE identity.sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE token_hash = sqlc.arg(token_hash)
  AND revoked_at IS NULL;

-- name: RevokeSessionByID :execrows
UPDATE identity.sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id)
  AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :execrows
UPDATE identity.sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE user_id = sqlc.arg(user_id)
  AND revoked_at IS NULL;
