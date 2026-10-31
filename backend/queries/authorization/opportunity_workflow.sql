-- name: ListActiveContinuityMembers :many
SELECT m.id AS membership_id,m.user_id,m.context_generation
FROM tenancy.memberships m JOIN identity.users u ON u.id=m.user_id
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.state='ACTIVE' AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL
ORDER BY m.id LIMIT 201;

-- name: AuthorizeOpportunityWorkflow :one
SELECT actor.id AS actor_membership_id,t.assignment_generation
FROM tenancy.memberships actor
JOIN identity.users actor_user ON actor_user.id=actor.user_id
JOIN bidora_tenancy.tenants t ON t.id=actor.tenant_id
WHERE actor.user_id=sqlc.arg(actor_user_id) AND actor.tenant_id=sqlc.arg(tenant_id)
  AND actor.context_generation=sqlc.arg(context_generation) AND actor.state='ACTIVE'
  AND actor_user.email_verified_at IS NOT NULL AND actor_user.disabled_at IS NULL
  AND t.tenant_type='TENDERPRENEUR' AND t.state='ACTIVE'
  AND EXISTS (SELECT 1 FROM tenancy.role_grants g WHERE g.membership_id=actor.id AND g.capability=sqlc.arg(capability) AND g.effect='ALLOW' AND g.revoked_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM tenancy.role_grants g WHERE g.membership_id=actor.id AND g.capability=sqlc.arg(capability) AND g.effect='DENY' AND g.revoked_at IS NULL);

-- name: AuthorizeCurrentBackgroundCapability :one
SELECT actor.id AS actor_membership_id,t.assignment_generation
FROM tenancy.memberships actor
JOIN identity.users actor_user ON actor_user.id=actor.user_id
JOIN bidora_tenancy.tenants t ON t.id=actor.tenant_id
WHERE actor.user_id=sqlc.arg(actor_user_id) AND actor.tenant_id=sqlc.arg(tenant_id)
  AND actor.state='ACTIVE' AND actor_user.email_verified_at IS NOT NULL AND actor_user.disabled_at IS NULL
  AND t.tenant_type='TENDERPRENEUR' AND t.state='ACTIVE'
  AND EXISTS (SELECT 1 FROM tenancy.role_grants g WHERE g.membership_id=actor.id AND g.capability=sqlc.arg(capability) AND g.effect='ALLOW' AND g.revoked_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM tenancy.role_grants g WHERE g.membership_id=actor.id AND g.capability=sqlc.arg(capability) AND g.effect='DENY' AND g.revoked_at IS NULL);

-- name: LockCurrentBackgroundCapability :one
SELECT actor.id AS actor_membership_id,t.assignment_generation
FROM tenancy.memberships actor
JOIN identity.users actor_user ON actor_user.id=actor.user_id
JOIN bidora_tenancy.tenants t ON t.id=actor.tenant_id
WHERE actor.user_id=sqlc.arg(actor_user_id) AND actor.tenant_id=sqlc.arg(tenant_id)
  AND actor.state='ACTIVE' AND actor_user.email_verified_at IS NOT NULL AND actor_user.disabled_at IS NULL
  AND t.tenant_type='TENDERPRENEUR' AND t.state='ACTIVE'
  AND EXISTS (SELECT 1 FROM tenancy.role_grants g WHERE g.membership_id=actor.id AND g.capability=sqlc.arg(capability) AND g.effect='ALLOW' AND g.revoked_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM tenancy.role_grants g WHERE g.membership_id=actor.id AND g.capability=sqlc.arg(capability) AND g.effect='DENY' AND g.revoked_at IS NULL)
FOR UPDATE OF actor,actor_user,t;

-- name: AuthorizeWorkspaceOwner :one
SELECT owner.id
FROM tenancy.memberships owner
JOIN identity.users owner_user ON owner_user.id=owner.user_id
WHERE owner.user_id=sqlc.arg(owner_user_id) AND owner.tenant_id=sqlc.arg(tenant_id)
  AND owner.state='ACTIVE' AND owner_user.email_verified_at IS NOT NULL AND owner_user.disabled_at IS NULL
  AND EXISTS (SELECT 1 FROM tenancy.role_grants a WHERE a.membership_id=owner.id AND a.capability='tenderpreneur.bid.read' AND a.effect='ALLOW' AND a.revoked_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM tenancy.role_grants d WHERE d.membership_id=owner.id AND d.capability='tenderpreneur.bid.read' AND d.effect='DENY' AND d.revoked_at IS NULL);
