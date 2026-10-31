-- name: PlatformIdentityHasTenantTies :one
SELECT EXISTS (
    SELECT 1 FROM tenancy.memberships AS m WHERE m.user_id = sqlc.arg(user_id)
) OR EXISTS (
    SELECT 1 FROM tenancy.invitations AS i
    WHERE i.email_lookup = sqlc.arg(email_lookup)
      AND i.invalidated_at IS NULL
      AND i.consumed_at IS NULL
      AND i.expires_at > sqlc.arg(observed_at)
) AS has_tenant_ties;

-- name: FinancialActorHasAffectedTenantTie :one
SELECT EXISTS(SELECT 1 FROM tenancy.memberships WHERE user_id=sqlc.arg(user_id) AND tenant_id=sqlc.arg(tenant_id)) AS has_tie;
