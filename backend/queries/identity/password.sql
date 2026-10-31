-- name: InsertPasswordUser :exec
INSERT INTO identity.users (
    id, email, email_lookup, first_name, last_name,
    email_verified_at, disabled_at, auth_epoch, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(email), sqlc.arg(email_lookup),
    sqlc.arg(first_name), sqlc.arg(last_name), NULL, NULL,
    1, sqlc.arg(created_at), sqlc.arg(created_at)
);

-- name: InsertPasswordIdentity :exec
INSERT INTO identity.authentication_identities (
    user_id, password_phc, credential_version,
    password_changed_at, created_at, updated_at
) VALUES (
    sqlc.arg(user_id), sqlc.arg(password_phc), 1,
    sqlc.arg(created_at), sqlc.arg(created_at), sqlc.arg(created_at)
);

-- name: InsertUserRegisteredAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, NULL, 'UserSelfRegistered', 'user', sqlc.arg(target_id),
    sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    '{}'::jsonb, NULL, sqlc.arg(occurred_at)
);

-- name: RehashPasswordCredential :execrows
UPDATE identity.authentication_identities
SET password_phc = sqlc.arg(password_phc),
    credential_version = credential_version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE user_id = sqlc.arg(user_id)
  AND credential_version = sqlc.arg(expected_credential_version);

-- name: GetPasswordChangePrincipalForUpdate :one
SELECT u.auth_epoch, a.credential_version
FROM identity.users AS u
JOIN identity.authentication_identities AS a ON a.user_id = u.id
JOIN identity.sessions AS s
  ON s.id = sqlc.arg(session_id)
 AND s.user_id = u.id
WHERE u.id = sqlc.arg(user_id)
  AND u.auth_epoch = sqlc.arg(expected_auth_epoch)
  AND a.credential_version = sqlc.arg(expected_credential_version)
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND s.auth_epoch = u.auth_epoch
  AND s.revoked_at IS NULL
  AND s.idle_expires_at > sqlc.arg(observed_at)
  AND s.absolute_expires_at > sqlc.arg(observed_at)
FOR UPDATE OF u, a, s;

-- name: ChangePasswordCredential :execrows
UPDATE identity.authentication_identities
SET password_phc = sqlc.arg(password_phc),
    credential_version = credential_version + 1,
    password_changed_at = sqlc.arg(changed_at),
    updated_at = sqlc.arg(changed_at)
WHERE user_id = sqlc.arg(user_id)
  AND credential_version = sqlc.arg(expected_credential_version);

-- name: AdvancePasswordChangeAuthEpoch :execrows
UPDATE identity.users
SET auth_epoch = auth_epoch + 1,
    updated_at = sqlc.arg(changed_at)
WHERE id = sqlc.arg(user_id)
  AND auth_epoch = sqlc.arg(expected_auth_epoch);

-- name: InsertPasswordChangedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.arg(actor_id), 'PasswordChanged', 'user',
    sqlc.arg(actor_id), sqlc.arg(occurred_at), 'SUCCEEDED',
    sqlc.arg(correlation_id),
    jsonb_build_object('credential_version', sqlc.arg(credential_version)::bigint,
                       'auth_epoch', sqlc.arg(auth_epoch)::bigint),
    1, sqlc.arg(occurred_at)
);

-- name: GetCurrentAccountRecoveryPreparation :one
SELECT (u.email_verified_at IS NOT NULL)::boolean AS email_verified,(u.disabled_at IS NOT NULL)::boolean AS account_disabled,
 EXISTS(SELECT 1 FROM identity.authentication_identities p WHERE p.user_id=u.id) AS password_credential_present,
 EXISTS(SELECT 1 FROM identity.external_identities e WHERE e.user_id=u.id AND e.provider='GOOGLE') AS google_identity_present
FROM identity.users u WHERE u.id=sqlc.arg(user_id);

-- name: GetPasswordLoginRecord :one
SELECT u.id, u.email, u.email_verified_at, u.disabled_at, u.auth_epoch,
       a.password_phc, a.credential_version, a.password_changed_at
FROM identity.users AS u
JOIN identity.authentication_identities AS a ON a.user_id = u.id
WHERE u.email_lookup = sqlc.arg(email_lookup)
LIMIT 1;
