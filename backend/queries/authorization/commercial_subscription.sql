-- name: CommercialSubscriptionCapabilityAllowed :one
SELECT EXISTS(SELECT 1 FROM tenancy.role_grants allow_grant WHERE allow_grant.membership_id=sqlc.arg(target_membership_id) AND allow_grant.capability=sqlc.arg(capability) AND allow_grant.effect='ALLOW' AND allow_grant.revoked_at IS NULL)
AND NOT EXISTS(SELECT 1 FROM tenancy.role_grants deny_grant WHERE deny_grant.membership_id=sqlc.arg(target_membership_id) AND deny_grant.capability=sqlc.arg(capability) AND deny_grant.effect='DENY' AND deny_grant.revoked_at IS NULL) AS allowed;
