-- Worker accesses fixed database functions only. The authenticated PostgreSQL
-- login is resolved to its executor in the database, not trusted from input.
-- Checkpoint advancement and operator/reaper recovery remain disabled until
-- an independently authorized verifier/attestation boundary exists.

-- name: ClaimProvisioningJob :one
SELECT id::uuid AS id,operation_id::uuid AS operation_id,tenant_id::uuid AS tenant_id,checkpoint::text AS checkpoint,attempts::integer AS attempts,lease_until::timestamptz AS lease_until,executor_id::uuid AS executor_id FROM tenancy.claim_provisioning_job(sqlc.arg(lease_token)::uuid,sqlc.arg(event_id)::uuid,sqlc.arg(lease_seconds)::integer);

-- name: ReserveProvisioningPlacement :one
SELECT reservation_id::uuid AS reservation_id,cluster_id::uuid AS cluster_id,generation::bigint AS generation,target_digest::bytea AS target_digest
FROM tenancy.reserve_provisioning_placement(sqlc.arg(reservation_id)::uuid,sqlc.arg(job_id)::uuid,sqlc.arg(operation_id)::uuid,sqlc.arg(tenant_id)::uuid,sqlc.arg(lease_token)::uuid);

-- name: RenewProvisioningLease :one
SELECT tenancy.renew_provisioning_lease(sqlc.arg(job_id)::uuid,sqlc.arg(operation_id)::uuid,sqlc.arg(tenant_id)::uuid,sqlc.arg(lease_token)::uuid,sqlc.arg(lease_seconds)::integer)::boolean AS renewed;

-- name: HoldProvisioningJobForReview :one
SELECT tenancy.hold_provisioning_job(sqlc.arg(job_id)::uuid,sqlc.arg(operation_id)::uuid,sqlc.arg(tenant_id)::uuid,sqlc.arg(lease_token)::uuid,sqlc.arg(event_id)::uuid,sqlc.arg(failure_code)::text)::boolean AS held;
