-- name: ListNotificationTemplates :many
SELECT * FROM notification.templates ORDER BY category,template_key LIMIT 100;
-- name: GetNotificationTemplate :one
SELECT * FROM notification.templates WHERE template_key=sqlc.arg(template_key);
-- name: LockNotificationTemplate :one
SELECT * FROM notification.templates WHERE template_key=sqlc.arg(template_key) FOR UPDATE;
-- name: UpdateNotificationTemplate :execrows
UPDATE notification.templates SET name=sqlc.arg(name),subject=sqlc.arg(subject),body=sqlc.arg(body),enabled=sqlc.arg(enabled),version=version+1,updated_at=sqlc.arg(at),updated_by=sqlc.arg(actor_id) WHERE template_key=sqlc.arg(template_key) AND version=sqlc.arg(expected_version);
-- name: InsertNotificationTemplateVersion :exec
INSERT INTO notification.template_versions(template_key,version,name,subject,body,enabled,created_at,created_by) VALUES(sqlc.arg(template_key),sqlc.arg(version),sqlc.arg(name),sqlc.arg(subject),sqlc.arg(body),sqlc.arg(enabled),sqlc.arg(at),sqlc.arg(actor_id));
-- name: ListNotificationTemplateVersions :many
SELECT v.*,t.category,t.channel,t.mandatory FROM notification.template_versions v JOIN notification.templates t USING(template_key) WHERE v.template_key=sqlc.arg(template_key) ORDER BY v.version DESC LIMIT 25;
-- name: GetNotificationTemplateCommand :one
SELECT * FROM notification.template_commands WHERE actor_id=sqlc.arg(actor_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);
-- name: InsertNotificationTemplateCommand :exec
INSERT INTO notification.template_commands(actor_id,operation,key_hash,request_hash,template_key,result_version,state,result_code,created_at) VALUES(sqlc.arg(actor_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(template_key),sqlc.arg(result_version),sqlc.arg(state),sqlc.arg(result_code),sqlc.arg(at));
-- name: CompleteNotificationTemplateTest :execrows
UPDATE notification.template_commands SET state='COMPLETED',result_code=sqlc.arg(result_code) WHERE actor_id=sqlc.arg(actor_id) AND operation='TEST' AND key_hash=sqlc.arg(key_hash) AND state='PENDING';
-- name: CountRecentNotificationTemplateTests :one
SELECT count(*)::bigint FROM notification.template_commands WHERE actor_id=sqlc.arg(actor_id) AND operation='TEST' AND created_at>sqlc.arg(after)::timestamptz;
-- name: AppendNotificationTemplateAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),sqlc.arg(action),'notification_template',sqlc.arg(target_id)::uuid,sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('template_key',sqlc.arg(template_key)::text,'version',sqlc.arg(version)::bigint,'result',sqlc.arg(result)::text),1,sqlc.arg(at));
-- name: AppendNotificationTemplateOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'notification_template',sqlc.arg(target_id)::uuid,sqlc.arg(version),sqlc.arg(action),1,'INTERNAL',jsonb_build_object('template_key',sqlc.arg(template_key)::text,'version',sqlc.arg(version)::bigint),sqlc.arg(at),sqlc.arg(at),sqlc.arg(at));
