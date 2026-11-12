-- Accessible only through an explicitly granted verifier-login function.
-- An assertion is not independent proof of external effects.
-- name: RecordProvisioningCapacity :one
SELECT observation_id::uuid AS observation_id,expires_at::timestamptz AS expires_at
FROM tenancy.record_provisioning_capacity(sqlc.arg(observation_id)::uuid,sqlc.arg(cluster_id)::uuid,sqlc.arg(quota_version)::bigint,sqlc.arg(cluster_version)::bigint,sqlc.arg(available_slots)::integer,sqlc.arg(evidence_digest)::bytea);

-- name: IssueProvisioningAttestation :one
SELECT attestation_id::uuid AS attestation_id,expires_at::timestamptz AS expires_at
FROM tenancy.issue_provisioning_attestation(sqlc.arg(attestation_id)::uuid,sqlc.arg(job_id)::uuid,sqlc.arg(operation_id)::uuid,sqlc.arg(tenant_id)::uuid,sqlc.arg(reservation_id)::uuid,sqlc.arg(cluster_id)::uuid,sqlc.arg(reservation_generation)::bigint,sqlc.arg(lease_token_hash)::bytea,sqlc.arg(next_checkpoint)::text,sqlc.arg(verification_kind)::text,sqlc.arg(verification_digest)::bytea);
