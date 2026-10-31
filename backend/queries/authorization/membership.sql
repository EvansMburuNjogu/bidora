-- name: ListActiveMembershipCapabilities :many
SELECT m.id AS membership_id, m.tenant_id, t.tenant_type, t.name AS tenant_name,
       m.context_generation,
       COALESCE(
           array_agg(DISTINCT allow_grant.capability ORDER BY allow_grant.capability)
               FILTER (WHERE allow_grant.capability IS NOT NULL),
           ARRAY[]::text[]
       )::text[] AS capabilities
FROM tenancy.memberships AS m
JOIN identity.users AS u ON u.id = m.user_id
JOIN bidora_tenancy.tenants AS t ON t.id = m.tenant_id
LEFT JOIN tenancy.role_grants AS allow_grant
  ON allow_grant.membership_id = m.id
 AND allow_grant.effect = 'ALLOW'
 AND allow_grant.revoked_at IS NULL
 AND NOT EXISTS (
     SELECT 1 FROM tenancy.role_grants AS deny_grant
     WHERE deny_grant.membership_id = m.id
       AND deny_grant.capability = allow_grant.capability
       AND deny_grant.effect = 'DENY'
       AND deny_grant.revoked_at IS NULL
 )
WHERE m.user_id = sqlc.arg(user_id)
  AND m.state = 'ACTIVE'
  AND u.email_verified_at IS NOT NULL
  AND u.disabled_at IS NULL
  AND t.state = 'ACTIVE'
GROUP BY m.id, m.tenant_id, t.tenant_type, t.name, m.context_generation
ORDER BY t.name, m.tenant_id, m.id;

-- name: AuthorizeExternalOpportunityCapture :one
SELECT m.id AS membership_id,
       m.context_generation,
       t.id AS tenant_id,
       t.assignment_generation
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
  AND t.assignment_generation > 0
  AND EXISTS (
      SELECT 1 FROM tenancy.role_grants AS allow_grant
      WHERE allow_grant.membership_id = m.id
        AND allow_grant.capability = 'tenderpreneur.opportunity.capture_external'
        AND allow_grant.effect = 'ALLOW'
        AND allow_grant.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM tenancy.role_grants AS deny_grant
      WHERE deny_grant.membership_id = m.id
        AND deny_grant.capability = 'tenderpreneur.opportunity.capture_external'
        AND deny_grant.effect = 'DENY'
        AND deny_grant.revoked_at IS NULL
  )
LIMIT 1;
