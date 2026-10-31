-- name: GetGoogleIdentityForUpdate :one
SELECT e.user_id, u.auth_epoch, u.disabled_at, u.email, e.email_at_link
FROM identity.external_identities AS e
JOIN identity.users AS u ON u.id = e.user_id
WHERE e.provider = 'GOOGLE' AND e.provider_subject = sqlc.arg(provider_subject)
FOR UPDATE OF e;

-- name: GetUserByEmailLookupForGoogleLink :one
SELECT id
FROM identity.users
WHERE email_lookup = sqlc.arg(email_lookup)
LIMIT 1;

-- name: InsertGoogleUser :exec
INSERT INTO identity.users (
    id, email, email_lookup, email_verified_at, disabled_at,
    auth_epoch, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(email), sqlc.arg(email_lookup), sqlc.arg(created_at),
    NULL, 1, sqlc.arg(created_at), sqlc.arg(created_at)
);

-- name: InsertGoogleIdentity :exec
INSERT INTO identity.external_identities (
    id, user_id, provider, provider_subject, email_at_link,
    created_at, last_authenticated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), 'GOOGLE', sqlc.arg(provider_subject),
    sqlc.arg(email_at_link), sqlc.arg(created_at), sqlc.arg(created_at)
);

-- name: TouchGoogleIdentity :execrows
UPDATE identity.external_identities
SET last_authenticated_at = sqlc.arg(authenticated_at)
WHERE provider = 'GOOGLE'
  AND provider_subject = sqlc.arg(provider_subject)
  AND user_id = sqlc.arg(user_id);

-- name: GetGoogleLinkPrincipalForUpdate :one
SELECT u.auth_epoch, a.credential_version
FROM identity.users AS u
JOIN identity.authentication_identities AS a ON a.user_id = u.id
JOIN identity.sessions AS s ON s.id = sqlc.arg(session_id) AND s.user_id = u.id
WHERE u.id = sqlc.arg(user_id)
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND u.auth_epoch = sqlc.arg(expected_auth_epoch)
  AND a.credential_version = sqlc.arg(expected_credential_version)
  AND s.auth_epoch = u.auth_epoch
  AND s.revoked_at IS NULL
  AND s.idle_expires_at > sqlc.arg(observed_at)
  AND s.absolute_expires_at > sqlc.arg(observed_at)
FOR UPDATE OF u, a, s;

-- name: GetGoogleIdentityForUserForUpdate :one
SELECT id
FROM identity.external_identities
WHERE user_id = sqlc.arg(user_id) AND provider = 'GOOGLE'
FOR UPDATE;

-- name: AdvanceGoogleLinkAuthEpoch :execrows
UPDATE identity.users
SET auth_epoch = auth_epoch + 1, updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(user_id)
  AND auth_epoch = sqlc.arg(expected_auth_epoch)
  AND auth_epoch < 9223372036854775807;

-- name: InsertGoogleIdentityLinkedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.arg(actor_id), 'GoogleIdentityLinked', 'external_identity', sqlc.arg(target_id),
    sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    '{}'::jsonb, NULL, sqlc.arg(occurred_at)
);

-- name: InsertGoogleUserRegisteredAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, NULL, 'GoogleUserRegistered', 'user', sqlc.arg(target_id),
    sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    '{}'::jsonb, NULL, sqlc.arg(occurred_at)
);
