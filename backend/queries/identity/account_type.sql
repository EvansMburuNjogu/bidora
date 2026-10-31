-- name: GetUserAccountType :one
SELECT account_type
FROM identity.users
WHERE id = sqlc.arg(user_id)
  AND email_verified_at IS NOT NULL
  AND disabled_at IS NULL;

-- name: LockUserAccountTypeForSelection :one
SELECT account_type
FROM identity.users
WHERE id = sqlc.arg(user_id)
  AND email_verified_at IS NOT NULL
  AND disabled_at IS NULL
FOR UPDATE;

-- name: GetAccountTypeSelectionCommand :one
SELECT request_hash, selected_account_type, response_status
FROM identity.account_type_selection_commands
WHERE actor_user_id = sqlc.arg(actor_user_id)
  AND key_hash = sqlc.arg(key_hash);

-- name: ClaimAccountTypeSelectionCommand :execrows
INSERT INTO identity.account_type_selection_commands (
    id, actor_user_id, key_hash, request_hash, selected_account_type,
    response_status, created_at, completed_at
) VALUES (
    sqlc.arg(id), sqlc.arg(actor_user_id), sqlc.arg(key_hash), sqlc.arg(request_hash),
    sqlc.arg(selected_account_type), 204, sqlc.arg(completed_at), sqlc.arg(completed_at)
)
ON CONFLICT (actor_user_id, key_hash) DO NOTHING;

-- name: SelectUserAccountType :execrows
UPDATE identity.users
SET account_type = sqlc.arg(account_type),
    updated_at = sqlc.arg(selected_at)
WHERE id = sqlc.arg(user_id)
  AND account_type IS NULL
  AND email_verified_at IS NOT NULL
  AND disabled_at IS NULL;

-- name: InsertAccountTypeSelectedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id, occurred_at,
    outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.arg(actor_id), 'AccountTypeSelected', 'user',
    sqlc.arg(actor_id), sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    jsonb_build_object('account_type', sqlc.arg(account_type)::text), 1, sqlc.arg(occurred_at)
);

-- name: InsertAccountTypeSelectedOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events (
    id, tenant_id, actor_id, aggregate_type, aggregate_id, aggregate_version,
    event_type, schema_version, classification, payload, occurred_at,
    published_at, attempts, next_attempt_at, created_at
) VALUES (
    sqlc.arg(id), NULL, sqlc.arg(actor_id), 'identity_user', sqlc.arg(actor_id), 1,
    'identity.account_type.selected.v1', 1, 'INTERNAL',
    jsonb_build_object('user_id', sqlc.arg(actor_id)::uuid, 'account_type', sqlc.arg(account_type)::text),
    sqlc.arg(occurred_at), NULL, 0, sqlc.arg(occurred_at), sqlc.arg(occurred_at)
);
