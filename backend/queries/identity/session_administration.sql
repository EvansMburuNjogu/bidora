-- name: GetSessionRevocationCommand :one
SELECT request_hash,response_status FROM identity.session_revocation_commands WHERE actor_user_id=sqlc.arg(actor_user_id) AND key_hash=sqlc.arg(key_hash);
-- name: LockOtherActiveUserSession :one
SELECT s.id FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id WHERE s.id=sqlc.arg(target_session_id) AND s.user_id=sqlc.arg(user_id) AND s.id<>sqlc.arg(current_session_id) AND s.auth_epoch=sqlc.arg(expected_auth_epoch) AND u.auth_epoch=s.auth_epoch AND s.revoked_at IS NULL AND s.idle_expires_at>sqlc.arg(observed_at) AND s.absolute_expires_at>sqlc.arg(observed_at) FOR UPDATE OF u,s;
-- name: RevokeOwnedSession :execrows
UPDATE identity.sessions SET revoked_at=sqlc.arg(revoked_at) WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND revoked_at IS NULL;
-- name: ClaimSessionRevocationCommand :execrows
INSERT INTO identity.session_revocation_commands(id,actor_user_id,key_hash,request_hash,response_status,created_at,completed_at) VALUES(sqlc.arg(id),sqlc.arg(actor_user_id),sqlc.arg(key_hash),sqlc.arg(request_hash),204,sqlc.arg(completed_at),sqlc.arg(completed_at)) ON CONFLICT(actor_user_id,key_hash) DO NOTHING;
-- name: InsertOtherSessionRevokedAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'OtherSessionRevoked','session',sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),'{}'::jsonb,1,sqlc.arg(occurred_at));
-- name: InsertOtherSessionRevokedOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'session',sqlc.arg(target_id),1,'identity.session.revoked_by_owner.v1',1,'INTERNAL',jsonb_build_object('session_id',sqlc.arg(target_id)::uuid,'user_id',sqlc.arg(actor_id)::uuid),sqlc.arg(occurred_at),NULL,0,sqlc.arg(occurred_at),sqlc.arg(occurred_at));
-- name: LockCurrentSessionForRevokeAll :one
SELECT u.auth_epoch,s.id FROM identity.users u JOIN identity.sessions s ON s.user_id=u.id WHERE u.id=sqlc.arg(user_id) AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND u.auth_epoch=sqlc.arg(expected_auth_epoch) AND s.id=sqlc.arg(current_session_id) AND s.auth_epoch=u.auth_epoch AND s.revoked_at IS NULL AND s.idle_expires_at>sqlc.arg(observed_at) AND s.absolute_expires_at>sqlc.arg(observed_at) FOR UPDATE OF u,s;
-- name: CountActiveCurrentEpochSessions :one
SELECT count(*)::bigint FROM identity.sessions WHERE user_id=sqlc.arg(user_id) AND auth_epoch=sqlc.arg(auth_epoch) AND revoked_at IS NULL AND idle_expires_at>sqlc.arg(observed_at) AND absolute_expires_at>sqlc.arg(observed_at);
-- name: AdvanceUserAuthEpochForSessionRevocation :execrows
UPDATE identity.users SET auth_epoch=auth_epoch+1,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(id) AND auth_epoch=sqlc.arg(expected_auth_epoch);
-- name: RevokeActiveCurrentEpochSessions :execrows
UPDATE identity.sessions SET revoked_at=sqlc.arg(at) WHERE user_id=sqlc.arg(user_id) AND auth_epoch=sqlc.arg(auth_epoch) AND revoked_at IS NULL AND idle_expires_at>sqlc.arg(at) AND absolute_expires_at>sqlc.arg(at);
-- name: InsertAllSessionsRevokedAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'AllSessionsRevoked','user',sqlc.arg(actor_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('auth_epoch',sqlc.arg(auth_epoch)::bigint,'revoked_session_count',sqlc.arg(revoked_session_count)::integer),1,sqlc.arg(occurred_at));
-- name: InsertAllSessionsRevokedOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'user_security',sqlc.arg(actor_id),sqlc.arg(auth_epoch),'identity.sessions.revoked_all.v1',1,'INTERNAL',jsonb_build_object('user_id',sqlc.arg(actor_id)::uuid,'auth_epoch',sqlc.arg(auth_epoch)::bigint,'revoked_session_count',sqlc.arg(revoked_session_count)::integer),sqlc.arg(occurred_at),NULL,0,sqlc.arg(occurred_at),sqlc.arg(occurred_at));
