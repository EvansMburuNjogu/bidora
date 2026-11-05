-- name: GetPlatformSessionPolicy :one
SELECT idle_seconds,absolute_seconds,version FROM platform_settings.session_policy WHERE id=true;
-- name: LockPlatformSessionPolicy :one
SELECT idle_seconds,absolute_seconds,version FROM platform_settings.session_policy WHERE id=true FOR UPDATE;
-- name: UpdatePlatformSessionPolicy :execrows
UPDATE platform_settings.session_policy SET idle_seconds=sqlc.arg(idle_seconds),absolute_seconds=sqlc.arg(absolute_seconds),version=version+1,updated_at=sqlc.arg(at)::timestamptz,updated_by=sqlc.arg(actor_id)::uuid WHERE id=true AND version=sqlc.arg(expected_version);
-- name: GetPlatformSessionPolicyCommand :one
SELECT request_hash,result_version FROM platform_settings.session_policy_commands WHERE actor_id=sqlc.arg(actor_id)::uuid AND key_hash=sqlc.arg(key_hash)::bytea;
-- name: InsertPlatformSessionPolicyCommand :exec
INSERT INTO platform_settings.session_policy_commands(actor_id,key_hash,request_hash,result_version) VALUES(sqlc.arg(actor_id)::uuid,sqlc.arg(key_hash)::bytea,sqlc.arg(request_hash)::bytea,sqlc.arg(result_version));
-- name: AppendPlatformSessionPolicyAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id)::uuid,NULL,sqlc.arg(actor_id)::uuid,'PlatformSessionPolicyUpdated','platform_session_policy','00000000-0000-4000-8000-000000000038'::uuid,sqlc.arg(at)::timestamptz,'SUCCEEDED',sqlc.arg(correlation_id)::uuid,jsonb_build_object('version',sqlc.arg(version)::bigint,'ordinary_sessions_revoked',sqlc.arg(revoked)::bigint,'changes',sqlc.arg(changes)::jsonb),1,sqlc.arg(at)::timestamptz);
-- name: AppendPlatformSessionPolicyOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id)::uuid,NULL,sqlc.arg(actor_id)::uuid,'platform_session_policy','00000000-0000-4000-8000-000000000038'::uuid,sqlc.arg(version)::bigint,'PlatformSessionPolicyUpdated',1,'INTERNAL',jsonb_build_object('version',sqlc.arg(version)::bigint),sqlc.arg(at)::timestamptz,sqlc.arg(at)::timestamptz,sqlc.arg(at)::timestamptz);
