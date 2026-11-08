-- name: LookupPlatformOrganizations :many
SELECT id,name AS label FROM bidora_tenancy.tenants
WHERE tenant_type='TENDERPRENEUR' AND (sqlc.arg(search)::text='' OR strpos(lower(name),lower(sqlc.arg(search)::text))>0 OR id::text=lower(sqlc.arg(search)::text))
ORDER BY lower(name),id LIMIT 26;

-- name: LookupPlatformSupportAssignees :many
SELECT u.id,coalesce(nullif(btrim(concat_ws(' ',u.first_name,u.last_name)),''),u.email)::text AS label
FROM platform_administration.administrator_profiles p JOIN identity.users u ON u.id=p.user_id AND u.disabled_at IS NULL
WHERE EXISTS(SELECT 1 FROM platform_administration.role_assignments r WHERE r.user_id=p.user_id AND r.role_key='SUPPORT_ADMINISTRATOR' AND r.state='ACTIVE')
AND (sqlc.arg(search)::text='' OR strpos(lower(concat_ws(' ',u.first_name,u.last_name,u.email)),lower(sqlc.arg(search)::text))>0 OR u.id::text=lower(sqlc.arg(search)::text))
ORDER BY lower(u.email),u.id LIMIT 26;

-- name: InsertPlatformReferenceReadAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformDirectoryReferenceRead','platform_reference',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),'{}',1,sqlc.arg(at));
