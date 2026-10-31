-- Approved fixed-role invitation acceptance primitives. The application owns
-- transaction ordering; generated methods are not authorization on their own.

-- name: ListPendingInvitations :many
SELECT i.id, i.email, i.role_key, i.created_at, i.expires_at
FROM tenancy.invitations AS i
WHERE i.tenant_id = sqlc.arg(tenant_id)
  AND i.invalidated_at IS NULL
  AND i.consumed_at IS NULL
  AND i.expires_at > sqlc.arg(observed_at)
  AND (
      sqlc.arg(cursor_created_at)::timestamptz IS NULL
      OR (i.created_at, i.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
  )
ORDER BY i.created_at DESC, i.id DESC
LIMIT sqlc.arg(max_rows);

-- name: LockAuthorizedInvitationIssuer :one
SELECT m.id AS membership_id
FROM tenancy.memberships AS m
JOIN identity.users AS u ON u.id = m.user_id
JOIN bidora_tenancy.tenants AS t ON t.id = m.tenant_id
WHERE m.user_id = sqlc.arg(user_id)
  AND m.tenant_id = sqlc.arg(tenant_id)
  AND m.context_generation = sqlc.arg(context_generation)
  AND m.state = 'ACTIVE'
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND t.tenant_type = 'TENDERPRENEUR'
  AND t.state = 'ACTIVE'
  AND EXISTS (
      SELECT 1 FROM tenancy.role_grants AS allow_grant
      WHERE allow_grant.membership_id = m.id
        AND allow_grant.capability = 'tenant.memberships.manage'
        AND allow_grant.effect = 'ALLOW'
        AND allow_grant.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM tenancy.role_grants AS deny_grant
      WHERE deny_grant.membership_id = m.id
        AND deny_grant.capability = 'tenant.memberships.manage'
        AND deny_grant.effect = 'DENY'
        AND deny_grant.revoked_at IS NULL
  )
FOR UPDATE OF m;

-- name: InvalidateActiveTenantInvitation :execrows
UPDATE tenancy.invitations
SET invalidated_at = sqlc.arg(invalidated_at)
WHERE tenant_id = sqlc.arg(tenant_id)
  AND email_lookup = sqlc.arg(email_lookup)
  AND invalidated_at IS NULL
  AND consumed_at IS NULL;

-- name: InsertTenantInvitation :exec
INSERT INTO tenancy.invitations (
    id, tenant_id, email, email_lookup, token_hash, role_key, capability,
    created_by, created_at, expires_at, invalidated_at, consumed_at, accepted_by
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(email), sqlc.arg(email_lookup),
    sqlc.arg(token_hash), sqlc.arg(role_key),
    'ROLE_BUNDLE_V2', sqlc.arg(created_by),
    sqlc.arg(created_at), sqlc.arg(expires_at), NULL, NULL, NULL
);

-- name: InsertInvitationIssuedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'InvitationIssued', 'invitation', sqlc.arg(target_id),
    sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    jsonb_build_object('role_key', sqlc.arg(role_key)::text),
    1, sqlc.arg(occurred_at)
);

-- name: InsertInvitationIssuedOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events (
    id, tenant_id, actor_id, aggregate_type, aggregate_id, aggregate_version,
    event_type, schema_version, classification, payload,
    occurred_at, published_at, attempts, next_attempt_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'invitation', sqlc.arg(aggregate_id), 1,
    'invitation.issued.v1', 1, 'INTERNAL',
    jsonb_build_object(
        'invitation_id', sqlc.arg(aggregate_id)::uuid,
        'tenant_id', sqlc.arg(tenant_id)::uuid,
        'role_key', sqlc.arg(role_key)::text
    ),
    sqlc.arg(occurred_at), NULL, 0, sqlc.arg(occurred_at), sqlc.arg(occurred_at)
);

-- name: LockCancelableInvitation :one
SELECT i.id, i.role_key
FROM tenancy.memberships AS m
JOIN identity.users AS u ON u.id = m.user_id
JOIN bidora_tenancy.tenants AS t ON t.id = m.tenant_id
JOIN tenancy.invitations AS i
  ON i.id = sqlc.arg(invitation_id)
 AND i.tenant_id = m.tenant_id
WHERE m.user_id = sqlc.arg(actor_id)
  AND m.tenant_id = sqlc.arg(tenant_id)
  AND m.context_generation = sqlc.arg(context_generation)
  AND m.state = 'ACTIVE'
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND t.tenant_type = 'TENDERPRENEUR'
  AND t.state = 'ACTIVE'
  AND i.invalidated_at IS NULL
  AND i.consumed_at IS NULL
  AND i.expires_at > sqlc.arg(cancelled_at)
  AND EXISTS (
      SELECT 1 FROM tenancy.role_grants AS allow_grant
      WHERE allow_grant.membership_id = m.id
        AND allow_grant.capability = 'tenant.memberships.manage'
        AND allow_grant.effect = 'ALLOW'
        AND allow_grant.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM tenancy.role_grants AS deny_grant
      WHERE deny_grant.membership_id = m.id
        AND deny_grant.capability = 'tenant.memberships.manage'
        AND deny_grant.effect = 'DENY'
        AND deny_grant.revoked_at IS NULL
  )
FOR UPDATE OF m, i;

-- name: CancelInvitation :execrows
UPDATE tenancy.invitations
SET invalidated_at = sqlc.arg(cancelled_at)
WHERE id = sqlc.arg(id)
  AND invalidated_at IS NULL
  AND consumed_at IS NULL;

-- name: InsertInvitationCancelledAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'InvitationCancelled', 'invitation', sqlc.arg(target_id),
    sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    jsonb_build_object('role_key', sqlc.arg(role_key)::text),
    1, sqlc.arg(occurred_at)
);

-- name: InsertInvitationCancelledOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events (
    id, tenant_id, actor_id, aggregate_type, aggregate_id, aggregate_version,
    event_type, schema_version, classification, payload,
    occurred_at, published_at, attempts, next_attempt_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'invitation', sqlc.arg(aggregate_id), 2,
    'invitation.cancelled.v1', 1, 'INTERNAL',
    jsonb_build_object(
        'invitation_id', sqlc.arg(aggregate_id)::uuid,
        'tenant_id', sqlc.arg(tenant_id)::uuid,
        'role_key', sqlc.arg(role_key)::text
    ),
    sqlc.arg(occurred_at), NULL, 0, sqlc.arg(occurred_at), sqlc.arg(occurred_at)
);

-- name: LockInvitationForAcceptance :one
SELECT i.id, i.tenant_id, i.role_key, i.capability
FROM tenancy.invitations AS i
JOIN identity.users AS u
  ON u.id = sqlc.arg(user_id)
 AND u.email_lookup = i.email_lookup
JOIN bidora_tenancy.tenants AS t
  ON t.id = i.tenant_id
WHERE i.token_hash = sqlc.arg(token_hash)
  AND i.invalidated_at IS NULL
  AND i.consumed_at IS NULL
  AND i.expires_at > sqlc.arg(accepted_at)
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND t.tenant_type = 'TENDERPRENEUR'
  AND t.state = 'ACTIVE'
FOR UPDATE OF i;

-- name: InsertInvitedMembership :exec
INSERT INTO tenancy.memberships (
    id, user_id, tenant_id, state, context_generation, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(tenant_id), 'ACTIVE', 1,
    sqlc.arg(created_at), sqlc.arg(created_at)
);

-- name: InsertInvitationCapabilityGrant :exec
INSERT INTO tenancy.role_grants (
    id, membership_id, role_key, capability, effect, created_by, created_at, revoked_at
) VALUES (
    sqlc.arg(id), sqlc.arg(membership_id), sqlc.arg(role_key),
    sqlc.arg(capability), 'ALLOW',
    sqlc.arg(created_by), sqlc.arg(created_at), NULL
);

-- name: ConsumeTenantInvitation :execrows
UPDATE tenancy.invitations
SET consumed_at = sqlc.arg(consumed_at), accepted_by = sqlc.arg(accepted_by)
WHERE id = sqlc.arg(id)
  AND invalidated_at IS NULL
  AND consumed_at IS NULL;

-- name: GetAcceptedMembershipGeneration :one
SELECT context_generation
FROM tenancy.memberships
WHERE id = sqlc.arg(id)
  AND state = 'ACTIVE';

-- name: InsertInvitationAcceptedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'InvitationAccepted', 'membership', sqlc.arg(target_id),
    sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    jsonb_build_object('context_generation', sqlc.arg(context_generation)::bigint, 'role_key', sqlc.arg(role_key)::text),
    1, sqlc.arg(occurred_at)
);

-- name: InsertMembershipGrantedOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events (
    id, tenant_id, actor_id, aggregate_type, aggregate_id, aggregate_version,
    event_type, schema_version, classification, payload,
    occurred_at, published_at, attempts, next_attempt_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'membership', sqlc.arg(aggregate_id), sqlc.arg(aggregate_version),
    'membership.granted.v1', 1, 'INTERNAL',
    jsonb_build_object(
        'membership_id', sqlc.arg(aggregate_id)::uuid,
        'tenant_id', sqlc.arg(tenant_id)::uuid,
        'context_generation', sqlc.arg(aggregate_version)::bigint,
        'role_key', sqlc.arg(role_key)::text
    ),
    sqlc.arg(occurred_at), NULL, 0, sqlc.arg(occurred_at), sqlc.arg(occurred_at)
);
