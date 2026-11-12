-- name: ListOrganizationReferences :many
SELECT kind,code,label,enabled,version,updated_at
FROM platform_settings.organization_reference_catalogue
WHERE kind=sqlc.arg(kind) AND (NOT sqlc.arg(active_only)::boolean OR enabled)
ORDER BY label,code LIMIT 500;

-- name: LockOrganizationReference :one
SELECT kind,code,label,enabled,version,updated_at
FROM platform_settings.organization_reference_catalogue
WHERE kind=sqlc.arg(kind) AND code=sqlc.arg(code)
FOR UPDATE;

-- name: FenceActiveOrganizationReference :one
SELECT code FROM platform_settings.organization_reference_catalogue
WHERE kind=sqlc.arg(kind) AND code=sqlc.arg(code) AND enabled
FOR SHARE;

-- name: CreateOrganizationReference :execrows
INSERT INTO platform_settings.organization_reference_catalogue(kind,code,label,enabled,version,updated_by,updated_at)
VALUES(sqlc.arg(kind),sqlc.arg(code),sqlc.arg(label),sqlc.arg(enabled),1,sqlc.arg(actor_id),sqlc.arg(at))
ON CONFLICT DO NOTHING;

-- name: UpdateOrganizationReference :execrows
UPDATE platform_settings.organization_reference_catalogue
SET label=sqlc.arg(label),enabled=sqlc.arg(enabled),version=version+1,updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(at)
WHERE kind=sqlc.arg(kind) AND code=sqlc.arg(code) AND version=sqlc.arg(expected_version);

-- name: ReadOrganizationReferenceCommand :one
SELECT request_hash,kind,code,result_version FROM platform_settings.organization_reference_commands
WHERE actor_id=sqlc.arg(actor_id) AND key_hash=sqlc.arg(key_hash);

-- name: InsertOrganizationReferenceCommand :exec
INSERT INTO platform_settings.organization_reference_commands(actor_id,key_hash,request_hash,kind,code,result_version,created_at)
VALUES(sqlc.arg(actor_id),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(kind),sqlc.arg(code),sqlc.arg(result_version),sqlc.arg(at));
