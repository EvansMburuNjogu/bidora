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
WITH policy AS (SELECT sqlc.arg(policy_idle_seconds)::integer AS idle_seconds,sqlc.arg(policy_absolute_seconds)::integer AS absolute_seconds)
SELECT s.id,s.rotated_from_session_id,s.created_at,s.last_seen_at,
 LEAST(s.idle_expires_at,CASE WHEN s.idle_timeout_seconds<=1800 THEN s.last_seen_at+make_interval(secs=>p.idle_seconds) ELSE s.idle_expires_at END,CASE WHEN s.idle_timeout_seconds<=1800 THEN s.created_at+make_interval(secs=>p.absolute_seconds) ELSE s.absolute_expires_at END)::timestamptz AS idle_expires_at,
 LEAST(s.absolute_expires_at,CASE WHEN s.idle_timeout_seconds<=1800 THEN s.created_at+make_interval(secs=>p.absolute_seconds) ELSE s.absolute_expires_at END)::timestamptz AS absolute_expires_at
FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id CROSS JOIN policy p
WHERE s.user_id=sqlc.arg(user_id) AND s.auth_epoch=u.auth_epoch AND s.revoked_at IS NULL
 AND s.idle_expires_at>sqlc.arg(observed_at) AND s.absolute_expires_at>sqlc.arg(observed_at)
 AND (s.idle_timeout_seconds>1800 OR (s.last_seen_at+make_interval(secs=>p.idle_seconds)>sqlc.arg(observed_at) AND s.created_at+make_interval(secs=>p.absolute_seconds)>sqlc.arg(observed_at)))
ORDER BY s.created_at DESC,s.id DESC LIMIT 101;

-- name: GetActiveSession :one
WITH policy AS (SELECT sqlc.arg(policy_idle_seconds)::integer AS idle_seconds,sqlc.arg(policy_absolute_seconds)::integer AS absolute_seconds)
SELECT s.id, s.user_id, s.csrf_hash, s.auth_epoch,
       LEAST(s.idle_expires_at,CASE WHEN s.idle_timeout_seconds<=1800 THEN s.last_seen_at+make_interval(secs=>p.idle_seconds) ELSE s.idle_expires_at END,CASE WHEN s.idle_timeout_seconds<=1800 THEN s.created_at+make_interval(secs=>p.absolute_seconds) ELSE s.absolute_expires_at END)::timestamptz AS idle_expires_at,
       LEAST(s.absolute_expires_at,CASE WHEN s.idle_timeout_seconds<=1800 THEN s.created_at+make_interval(secs=>p.absolute_seconds) ELSE s.absolute_expires_at END)::timestamptz AS absolute_expires_at,
       s.assurance_level,s.assured_at,s.assurance_method
FROM identity.sessions AS s
JOIN identity.users AS u ON u.id = s.user_id
CROSS JOIN policy p
WHERE s.token_hash = sqlc.arg(token_hash)
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND u.auth_epoch = s.auth_epoch
  AND s.revoked_at IS NULL
  AND s.idle_expires_at > sqlc.arg(observed_at)
  AND s.absolute_expires_at > sqlc.arg(observed_at)
  AND (s.idle_timeout_seconds>1800 OR (s.last_seen_at+make_interval(secs=>p.idle_seconds)>sqlc.arg(observed_at) AND s.created_at+make_interval(secs=>p.absolute_seconds)>sqlc.arg(observed_at)));

-- Touch only after browser/request policy accepts the request. Validation alone
-- must not let rejected cross-site traffic extend idle lifetime.
-- name: TouchActiveSession :execrows
WITH policy AS (SELECT sqlc.arg(policy_idle_seconds)::integer AS idle_seconds,sqlc.arg(policy_absolute_seconds)::integer AS absolute_seconds)
UPDATE identity.sessions AS s
SET last_seen_at = sqlc.arg(observed_at),
    idle_expires_at = LEAST(sqlc.arg(observed_at) + make_interval(secs=>CASE WHEN s.idle_timeout_seconds<=1800 THEN LEAST(s.idle_timeout_seconds,p.idle_seconds) ELSE s.idle_timeout_seconds END), s.absolute_expires_at,
      CASE WHEN s.idle_timeout_seconds<=1800 THEN s.created_at+make_interval(secs=>p.absolute_seconds) ELSE s.absolute_expires_at END)
FROM policy p
WHERE s.id = sqlc.arg(id)
  AND s.revoked_at IS NULL
  AND s.idle_expires_at > sqlc.arg(observed_at)
  AND s.absolute_expires_at > sqlc.arg(observed_at)
  AND s.last_seen_at <= sqlc.arg(observed_at)
  AND (s.idle_timeout_seconds>1800 OR (s.last_seen_at+make_interval(secs=>p.idle_seconds)>sqlc.arg(observed_at) AND s.created_at+make_interval(secs=>p.absolute_seconds)>sqlc.arg(observed_at)));

-- Policy changes invalidate ordinary sessions atomically; no restoration when
-- a later policy relaxes. Verified administrator profiles remain governed.
-- name: RevokeOrdinarySessionsForPolicy :execrows
UPDATE identity.sessions SET revoked_at=sqlc.arg(at)::timestamptz
WHERE revoked_at IS NULL AND idle_timeout_seconds<=1800;

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
