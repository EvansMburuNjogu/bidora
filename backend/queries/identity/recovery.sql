-- name: GetPasswordRecoveryCandidate :one
SELECT u.id,u.email FROM identity.users u JOIN identity.authentication_identities p ON p.user_id=u.id
WHERE u.email_lookup=sqlc.arg(email_lookup) AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL;
-- name: InvalidateActivePasswordRecoveryChallenges :exec
UPDATE identity.password_recovery_challenges SET consumed_at=sqlc.arg(at) WHERE user_id=sqlc.arg(user_id) AND consumed_at IS NULL;
-- name: InsertPasswordRecoveryChallenge :exec
INSERT INTO identity.password_recovery_challenges(id,user_id,purpose,token_hash,expires_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(user_id),'PASSWORD_RECOVERY',sqlc.arg(token_hash),sqlc.arg(created_at)::timestamptz+interval '30 minutes',sqlc.arg(created_at)::timestamptz);
-- name: InsertPasswordRecoveryRequestedAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),NULL,sqlc.arg(user_id),'PasswordRecoveryRequested','password_recovery_challenge',sqlc.arg(challenge_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('purpose','PASSWORD_RECOVERY','expires_in_seconds',1800),1,sqlc.arg(at));
-- name: LockValidPasswordRecoveryChallenge :one
SELECT c.id AS challenge_id,c.user_id,p.credential_version,u.auth_epoch
FROM identity.password_recovery_challenges c JOIN identity.users u ON u.id=c.user_id JOIN identity.authentication_identities p ON p.user_id=u.id
WHERE c.token_hash=sqlc.arg(token_hash) AND c.purpose='PASSWORD_RECOVERY' AND c.consumed_at IS NULL AND c.expires_at>sqlc.arg(at)
 AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL FOR UPDATE OF c,u,p;
-- name: ConsumePasswordRecoveryChallenge :execrows
UPDATE identity.password_recovery_challenges SET consumed_at=sqlc.arg(at) WHERE id=sqlc.arg(id) AND consumed_at IS NULL;
-- name: ResetPasswordCredential :execrows
UPDATE identity.authentication_identities SET password_phc=sqlc.arg(password_phc),credential_version=credential_version+1,password_changed_at=sqlc.arg(at),updated_at=sqlc.arg(at)
WHERE user_id=sqlc.arg(user_id) AND credential_version=sqlc.arg(expected_credential_version);
-- name: AdvancePasswordRecoveryAuthEpoch :execrows
UPDATE identity.users SET auth_epoch=auth_epoch+1,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(user_id) AND auth_epoch=sqlc.arg(expected_auth_epoch);
-- name: InsertPasswordRecoveredAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),NULL,sqlc.arg(user_id),'PasswordRecovered','user',sqlc.arg(user_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('sessions_revoked',true,'auth_epoch_advanced',true),1,sqlc.arg(at));
