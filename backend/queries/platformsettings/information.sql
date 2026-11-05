-- name: GetPlatformInformation :one
SELECT platform_name, contact_phone, support_email, website, version, updated_at
FROM platform_settings.platform_information
WHERE id='00000000-0000-4000-8000-000000000035'::uuid;

-- name: LockPlatformInformation :one
SELECT platform_name, contact_phone, support_email, website, version, updated_at
FROM platform_settings.platform_information
WHERE id='00000000-0000-4000-8000-000000000035'::uuid FOR UPDATE;

-- name: GetPlatformInformationCommand :one
SELECT request_hash, result_version FROM platform_settings.platform_information_commands
WHERE actor_id=sqlc.arg(actor_id)::uuid AND key_hash=sqlc.arg(key_hash)::bytea;

-- name: UpdatePlatformInformation :execrows
UPDATE platform_settings.platform_information
SET platform_name=sqlc.arg(platform_name)::text,contact_phone=sqlc.arg(contact_phone)::text,
    support_email=sqlc.arg(support_email)::text,website=sqlc.arg(website)::text,
    version=version+1,updated_at=sqlc.arg(updated_at)::timestamptz,updated_by=sqlc.arg(updated_by)::uuid
WHERE id='00000000-0000-4000-8000-000000000035'::uuid AND version=sqlc.arg(expected_version)::bigint;

-- name: InsertPlatformInformationCommand :exec
INSERT INTO platform_settings.platform_information_commands(actor_id,key_hash,request_hash,result_version)
VALUES(sqlc.arg(actor_id)::uuid,sqlc.arg(key_hash)::bytea,sqlc.arg(request_hash)::bytea,sqlc.arg(result_version)::bigint);

-- name: AppendPlatformInformationAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id)::uuid,NULL,sqlc.arg(actor_id)::uuid,'PlatformInformationUpdated','platform_information','00000000-0000-4000-8000-000000000035'::uuid,sqlc.arg(at)::timestamptz,'SUCCEEDED',sqlc.arg(correlation_id)::uuid,jsonb_build_object('changes',sqlc.arg(changes)::jsonb,'version',sqlc.arg(version)::bigint),1,sqlc.arg(at)::timestamptz);

-- name: AppendPlatformInformationOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id)::uuid,NULL,sqlc.arg(actor_id)::uuid,'platform_information','00000000-0000-4000-8000-000000000035'::uuid,sqlc.arg(version)::bigint,'PlatformInformationUpdated',1,'INTERNAL',jsonb_build_object('version',sqlc.arg(version)::bigint),sqlc.arg(at)::timestamptz,sqlc.arg(at)::timestamptz,sqlc.arg(at)::timestamptz);
