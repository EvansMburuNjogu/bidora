-- Fixed capture-contributor revocation. Administrator and arbitrary role
-- mutation are structurally excluded by the target grant predicates.

-- name: AuthorizeMembershipManagement :one
SELECT actor.id
FROM tenancy.memberships AS actor
JOIN identity.users AS actor_user ON actor_user.id = actor.user_id
JOIN bidora_tenancy.tenants AS t ON t.id = actor.tenant_id
WHERE actor.user_id = sqlc.arg(actor_user_id)
  AND actor.tenant_id = sqlc.arg(tenant_id)
  AND actor.context_generation = sqlc.arg(actor_context_generation)
  AND actor.state = 'ACTIVE'
  AND actor_user.email_verified_at IS NOT NULL
  AND actor_user.disabled_at IS NULL
  AND t.tenant_type = 'TENDERPRENEUR'
  AND t.state = 'ACTIVE'
  AND EXISTS (
      SELECT 1 FROM tenancy.role_grants AS manager_allow
      WHERE manager_allow.membership_id = actor.id
        AND manager_allow.capability = 'tenant.memberships.manage'
        AND manager_allow.effect = 'ALLOW'
        AND manager_allow.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM tenancy.role_grants AS manager_deny
      WHERE manager_deny.membership_id = actor.id
        AND manager_deny.capability = 'tenant.memberships.manage'
        AND manager_deny.effect = 'DENY'
        AND manager_deny.revoked_at IS NULL
  );

-- name: ListCaptureContributorMemberships :many
SELECT target.id AS membership_id, target_user.email,
       target.context_generation
FROM tenancy.memberships AS target
JOIN identity.users AS target_user ON target_user.id = target.user_id
WHERE target.tenant_id = sqlc.arg(tenant_id)
  AND target.state = 'ACTIVE'
  AND EXISTS (
      SELECT 1 FROM tenancy.role_grants AS capture_grant
      WHERE capture_grant.membership_id = target.id
        AND capture_grant.role_key = 'TENDERPRENEUR_CAPTURE_CONTRIBUTOR'
        AND capture_grant.capability = 'tenderpreneur.opportunity.capture_external'
        AND capture_grant.effect = 'ALLOW'
        AND capture_grant.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM tenancy.role_grants AS other_grant
      WHERE other_grant.membership_id = target.id
        AND other_grant.revoked_at IS NULL
        AND NOT (
            other_grant.role_key = 'TENDERPRENEUR_CAPTURE_CONTRIBUTOR'
            AND other_grant.capability = 'tenderpreneur.opportunity.capture_external'
            AND other_grant.effect = 'ALLOW'
        )
  )
ORDER BY target_user.email_lookup, target.id;

-- name: LockRevocableCaptureContributor :one
SELECT target.id AS target_membership_id, target.user_id AS target_user_id,
       target.context_generation AS target_context_generation
FROM tenancy.memberships AS actor
JOIN identity.users AS actor_user ON actor_user.id = actor.user_id
JOIN bidora_tenancy.tenants AS t ON t.id = actor.tenant_id
JOIN tenancy.memberships AS target
  ON target.id = sqlc.arg(target_membership_id)
 AND target.tenant_id = actor.tenant_id
WHERE actor.user_id = sqlc.arg(actor_user_id)
  AND actor.tenant_id = sqlc.arg(tenant_id)
  AND actor.context_generation = sqlc.arg(actor_context_generation)
  AND actor.state = 'ACTIVE'
  AND actor_user.email_verified_at IS NOT NULL
  AND actor_user.disabled_at IS NULL
  AND t.tenant_type = 'TENDERPRENEUR'
  AND t.state = 'ACTIVE'
  AND target.state = 'ACTIVE'
  AND EXISTS (
      SELECT 1 FROM tenancy.role_grants AS manager_allow
      WHERE manager_allow.membership_id = actor.id
        AND manager_allow.capability = 'tenant.memberships.manage'
        AND manager_allow.effect = 'ALLOW'
        AND manager_allow.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM tenancy.role_grants AS manager_deny
      WHERE manager_deny.membership_id = actor.id
        AND manager_deny.capability = 'tenant.memberships.manage'
        AND manager_deny.effect = 'DENY'
        AND manager_deny.revoked_at IS NULL
  )
  AND EXISTS (
      SELECT 1 FROM tenancy.role_grants AS capture_grant
      WHERE capture_grant.membership_id = target.id
        AND capture_grant.role_key = 'TENDERPRENEUR_CAPTURE_CONTRIBUTOR'
        AND capture_grant.capability = 'tenderpreneur.opportunity.capture_external'
        AND capture_grant.effect = 'ALLOW'
        AND capture_grant.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM tenancy.role_grants AS other_grant
      WHERE other_grant.membership_id = target.id
        AND other_grant.revoked_at IS NULL
        AND NOT (
            other_grant.role_key = 'TENDERPRENEUR_CAPTURE_CONTRIBUTOR'
            AND other_grant.capability = 'tenderpreneur.opportunity.capture_external'
            AND other_grant.effect = 'ALLOW'
        )
  )
FOR UPDATE OF actor, target;

-- name: RevokeCaptureContributorMembership :execrows
UPDATE tenancy.memberships
SET state = 'REVOKED',
    context_generation = context_generation + 1,
    updated_at = sqlc.arg(revoked_at)
WHERE id = sqlc.arg(id)
  AND state = 'ACTIVE';

-- name: RevokeCaptureContributorGrant :execrows
UPDATE tenancy.role_grants
SET revoked_at = sqlc.arg(revoked_at)
WHERE membership_id = sqlc.arg(membership_id)
  AND role_key = 'TENDERPRENEUR_CAPTURE_CONTRIBUTOR'
  AND capability = 'tenderpreneur.opportunity.capture_external'
  AND effect = 'ALLOW'
  AND revoked_at IS NULL;

-- name: GetRevokedMembershipGeneration :one
SELECT context_generation
FROM tenancy.memberships
WHERE id = sqlc.arg(id)
  AND state = 'REVOKED';

-- name: InsertMembershipRevokedAudit :exec
INSERT INTO bidora_control_audit.audit_events (
    id, tenant_id, actor_id, action, target_type, target_id,
    occurred_at, outcome, correlation_id, change_summary, policy_version, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'MembershipRevoked', 'membership', sqlc.arg(target_id),
    sqlc.arg(occurred_at), 'SUCCEEDED', sqlc.arg(correlation_id),
    jsonb_build_object('context_generation', sqlc.arg(context_generation)::bigint),
    1, sqlc.arg(occurred_at)
);

-- name: InsertMembershipRevokedOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events (
    id, tenant_id, actor_id, aggregate_type, aggregate_id, aggregate_version,
    event_type, schema_version, classification, payload,
    occurred_at, published_at, attempts, next_attempt_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(tenant_id), sqlc.arg(actor_id),
    'membership', sqlc.arg(aggregate_id), sqlc.arg(aggregate_version),
    'membership.revoked.v1', 1, 'INTERNAL',
    jsonb_build_object(
        'membership_id', sqlc.arg(aggregate_id)::uuid,
        'tenant_id', sqlc.arg(tenant_id)::uuid,
        'context_generation', sqlc.arg(aggregate_version)::bigint
    ),
    sqlc.arg(occurred_at), NULL, 0, sqlc.arg(occurred_at), sqlc.arg(occurred_at)
);
