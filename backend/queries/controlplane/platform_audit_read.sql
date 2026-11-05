-- name: ListPlatformAuditActions :many
SELECT id,actor_id,action,occurred_at,module,affected_record_id,result,administrator_name,affected_record_name,changes::jsonb AS changes,
 CASE WHEN sqlc.arg(include_metadata)::boolean THEN coalesce(host(ip_address),'') ELSE '' END::text AS ip_address,
 CASE WHEN sqlc.arg(include_metadata)::boolean THEN coalesce(browser,'') ELSE '' END::text AS browser
FROM bidora_control_audit.platform_action_details_v1
WHERE (sqlc.narg(actor_filter)::uuid IS NULL OR actor_id=sqlc.narg(actor_filter)::uuid)
 AND (sqlc.narg(action_filter)::text IS NULL OR action=sqlc.narg(action_filter)::text)
 AND (sqlc.narg(module_filter)::text IS NULL OR module=sqlc.narg(module_filter)::text)
 AND (sqlc.narg(result_filter)::text IS NULL OR result=sqlc.narg(result_filter)::text)
 AND (sqlc.narg(from_at)::timestamptz IS NULL OR occurred_at>=sqlc.narg(from_at)::timestamptz)
 AND (sqlc.narg(before_at)::timestamptz IS NULL OR occurred_at<sqlc.narg(before_at)::timestamptz)
 AND (sqlc.arg(search_term)::text='' OR position(lower(sqlc.arg(search_term)::text) in lower(action))>0
   OR position(lower(sqlc.arg(search_term)::text) in lower(module))>0
   OR position(lower(sqlc.arg(search_term)::text) in lower(administrator_name))>0
   OR position(lower(sqlc.arg(search_term)::text) in lower(affected_record_name))>0
   OR position(lower(sqlc.arg(search_term)::text) in lower(coalesce(actor_id::text,'')))>0
   OR position(lower(sqlc.arg(search_term)::text) in lower(coalesce(affected_record_id::text,'')))>0)
 AND (sqlc.narg(cursor_at)::timestamptz IS NULL
   OR (sqlc.arg(sort_order)::text='DESC' AND (occurred_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
   OR (sqlc.arg(sort_order)::text='ASC' AND (occurred_at,id)>(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid)))
ORDER BY
 CASE WHEN sqlc.arg(sort_order)::text='ASC' THEN occurred_at END ASC,
 CASE WHEN sqlc.arg(sort_order)::text='ASC' THEN id END ASC,
 CASE WHEN sqlc.arg(sort_order)::text='DESC' THEN occurred_at END DESC,
 CASE WHEN sqlc.arg(sort_order)::text='DESC' THEN id END DESC
LIMIT sqlc.arg(page_limit);

-- name: GetPlatformAuditAction :one
SELECT id,actor_id,action,occurred_at,module,affected_record_id,result,administrator_name,affected_record_name,changes::jsonb AS changes,
 CASE WHEN sqlc.arg(include_metadata)::boolean THEN coalesce(host(ip_address),'') ELSE '' END::text AS ip_address,
 CASE WHEN sqlc.arg(include_metadata)::boolean THEN coalesce(browser,'') ELSE '' END::text AS browser
FROM bidora_control_audit.platform_action_details_v1 WHERE id=sqlc.arg(id);

-- name: InsertPlatformAuditExport :exec
INSERT INTO bidora_control_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'PlatformControlAuditExport','platform_audit_export',sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),'{}'::jsonb,1,sqlc.arg(at));

-- name: InsertPlatformAuditPageRead :exec
INSERT INTO bidora_control_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),
 CASE WHEN sqlc.arg(metadata_access)::boolean THEN 'RestrictedRequestMetadataRead' ELSE 'PlatformControlAuditPageRead' END,
 CASE WHEN sqlc.arg(metadata_access)::boolean THEN 'platform_regional_policy' ELSE 'platform_audit_page' END,
 sqlc.arg(id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),'{}'::jsonb,1,sqlc.arg(at));
