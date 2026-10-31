-- name: LockGoogleHead :one
SELECT version_id,version FROM platform_settings.google_head WHERE id=true FOR UPDATE;
-- name: LockGoogleHeadShared :one
SELECT version_id,version FROM platform_settings.google_head WHERE id=true FOR SHARE;
-- name: GetGoogleConfiguration :one
SELECT v.* FROM platform_settings.google_head h JOIN platform_settings.google_versions v ON v.id=h.version_id WHERE h.id=true;
-- name: GetGoogleVersion :one
SELECT * FROM platform_settings.google_versions WHERE id=sqlc.arg(id)::uuid;
-- name: ListGoogleVersions :many
SELECT * FROM platform_settings.google_versions ORDER BY version DESC LIMIT 25;
-- name: InsertGoogleVersion :exec
INSERT INTO platform_settings.google_versions(id,version,enabled,allow_signup,client_id,allowed_domains,secret_id,key_version,nonce,ciphertext,created_at,created_by) VALUES(sqlc.arg(id),sqlc.arg(version),sqlc.arg(enabled),sqlc.arg(allow_signup),sqlc.arg(client_id),sqlc.arg(allowed_domains),sqlc.narg(secret_id),sqlc.narg(key_version),sqlc.narg(nonce),sqlc.narg(ciphertext),sqlc.arg(at),sqlc.arg(actor_id));
-- name: AdvanceGoogleHead :execrows
UPDATE platform_settings.google_head SET version_id=sqlc.arg(version_id),version=version+1 WHERE id=true AND version=sqlc.arg(expected_version);
-- name: GetGoogleCommand :one
SELECT * FROM platform_settings.google_commands WHERE actor_id=sqlc.arg(actor_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);
-- name: InsertGoogleCommand :exec
INSERT INTO platform_settings.google_commands(actor_id,operation,key_hash,request_hash,version_id,result_version,state,result_code) VALUES(sqlc.arg(actor_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(version_id),sqlc.arg(result_version),sqlc.arg(state),sqlc.arg(result_code));
-- name: CompleteGoogleTestCommand :execrows
UPDATE platform_settings.google_commands SET state='COMPLETED',result_code=sqlc.arg(result_code) WHERE actor_id=sqlc.arg(actor_id) AND operation='TEST' AND key_hash=sqlc.arg(key_hash) AND state='PENDING';
-- name: InsertGoogleTest :exec
INSERT INTO platform_settings.google_tests(id,version_id,actor_id,status,created_at) VALUES(sqlc.arg(id),sqlc.arg(version_id),sqlc.arg(actor_id),sqlc.arg(status),sqlc.arg(at));
-- name: CountRecentGoogleTests :one
SELECT count(*)::bigint FROM platform_settings.google_commands WHERE actor_id=sqlc.arg(actor_id) AND operation='TEST' AND created_at>sqlc.arg(after)::timestamptz;
-- name: LatestGoogleTest :one
SELECT status FROM platform_settings.google_tests WHERE version_id=sqlc.arg(version_id) ORDER BY created_at DESC LIMIT 1;
-- name: AppendGoogleSettingsAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),sqlc.arg(action),'platform_google_configuration',sqlc.arg(target_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('version',sqlc.arg(version)::bigint,'result',sqlc.arg(result)::text),1,sqlc.arg(at));
-- name: AppendGoogleSettingsOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'platform_google_configuration',sqlc.arg(target_id),sqlc.arg(version),sqlc.arg(action),1,'INTERNAL',jsonb_build_object('version',sqlc.arg(version)::bigint),sqlc.arg(at),sqlc.arg(at),sqlc.arg(at));
