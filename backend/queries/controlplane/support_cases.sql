-- name: SupportTenantExists :one
SELECT EXISTS(SELECT 1 FROM bidora_tenancy.tenants WHERE id=sqlc.arg(tenant_id))::boolean;

-- name: SupportAssigneeEligible :one
SELECT EXISTS(
 SELECT 1 FROM platform_administration.administrator_profiles p
 JOIN identity.users u ON u.id=p.user_id AND u.disabled_at IS NULL
 JOIN platform_administration.role_assignments r ON r.user_id=p.user_id AND r.role_key='SUPPORT_ADMINISTRATOR' AND r.state='ACTIVE'
 WHERE p.user_id=sqlc.arg(user_id)
)::boolean;

-- name: ListSupportCases :many
SELECT id,tenant_id,title,description,state,assigned_to,created_by,version,created_at,updated_at
FROM bidora_support.cases
WHERE (sqlc.narg(cursor_at)::timestamptz IS NULL OR (updated_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY updated_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: GetSupportCase :one
SELECT id,tenant_id,title,description,state,assigned_to,created_by,version,created_at,updated_at
FROM bidora_support.cases WHERE id=sqlc.arg(id);

-- name: GetSupportCaseForUpdate :one
SELECT id,tenant_id,title,description,state,assigned_to,created_by,version,created_at,updated_at
FROM bidora_support.cases WHERE id=sqlc.arg(id) FOR UPDATE;

-- name: ListSupportCaseMessages :many
SELECT id,actor_id,body,created_at FROM bidora_support.case_messages
WHERE case_id=sqlc.arg(case_id) ORDER BY created_at,id LIMIT 101;

-- name: CountSupportCaseMessages :one
SELECT count(*)::bigint FROM bidora_support.case_messages WHERE case_id=sqlc.arg(case_id);

-- name: InsertSupportCase :exec
INSERT INTO bidora_support.cases(id,tenant_id,title,description,state,assigned_to,created_by,version,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(tenant_id),sqlc.arg(title),sqlc.arg(description),'OPEN',NULL,sqlc.arg(created_by),1,sqlc.arg(at),sqlc.arg(at));

-- name: UpdateSupportCase :execrows
UPDATE bidora_support.cases SET state=sqlc.arg(state),assigned_to=sqlc.narg(assigned_to),version=version+1,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version);

-- name: InsertSupportCaseMessage :exec
INSERT INTO bidora_support.case_messages(id,case_id,actor_id,body,created_at)
VALUES(sqlc.arg(id),sqlc.arg(case_id),sqlc.arg(actor_id),sqlc.arg(body),sqlc.arg(at));

-- name: InsertSupportCaseEvent :exec
INSERT INTO bidora_support.case_events(id,case_id,actor_id,action,version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(case_id),sqlc.arg(actor_id),sqlc.arg(action),sqlc.arg(version),sqlc.arg(at));

-- name: GetSupportCaseCommand :one
SELECT request_hash,case_id,result_version FROM bidora_support.case_commands
WHERE actor_id=sqlc.arg(actor_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);

-- name: InsertSupportCaseCommand :exec
INSERT INTO bidora_support.case_commands(actor_id,operation,key_hash,request_hash,case_id,result_version,created_at)
VALUES(sqlc.arg(actor_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(case_id),sqlc.arg(result_version),sqlc.arg(at));

-- name: InsertSupportCaseAudit :exec
INSERT INTO bidora_control_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),sqlc.arg(target_type),sqlc.arg(case_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary),1,sqlc.arg(at));
