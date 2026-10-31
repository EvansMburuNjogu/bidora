-- name: GetActiveMFAFactor :one
SELECT id,user_id,secret_ciphertext,secret_nonce,key_version,last_accepted_counter,version FROM identity.mfa_factors WHERE user_id=sqlc.arg(user_id) AND state='ACTIVE';
-- name: GetPendingMFAFactor :one
SELECT id,user_id,secret_ciphertext,secret_nonce,key_version,last_accepted_counter,version,pending_expires_at,failed_attempts FROM identity.mfa_factors WHERE user_id=sqlc.arg(user_id) AND state='PENDING' AND failed_attempts<5;
-- name: CancelPendingMFAFactor :exec
UPDATE identity.mfa_factors SET state='CANCELLED',replaced_at=sqlc.arg(at),version=version+1 WHERE user_id=sqlc.arg(user_id) AND state='PENDING';
-- name: InsertPendingMFAFactor :exec
INSERT INTO identity.mfa_factors(id,user_id,state,algorithm,digits,period_seconds,secret_ciphertext,secret_nonce,key_version,last_accepted_counter,created_at,pending_expires_at,version) VALUES(sqlc.arg(id),sqlc.arg(user_id),'PENDING','TOTP_SHA1',6,30,sqlc.arg(ciphertext),sqlc.arg(nonce),sqlc.arg(key_version),-1,sqlc.arg(created_at)::timestamptz,sqlc.arg(created_at)::timestamptz+interval '10 minutes',1);
-- name: MFAEnrollmentAttemptDenied :one
SELECT EXISTS(SELECT 1 FROM identity.mfa_factors WHERE user_id=sqlc.arg(user_id) AND failed_attempts=5 AND denied_until>sqlc.arg(at));
-- name: RecordMFAEnrollmentFailure :execrows
UPDATE identity.mfa_factors SET failed_attempts=failed_attempts+1,state=CASE WHEN failed_attempts+1>=5 THEN 'CANCELLED' ELSE state END,replaced_at=CASE WHEN failed_attempts+1>=5 THEN sqlc.arg(at) ELSE replaced_at END,denied_until=CASE WHEN failed_attempts+1>=5 THEN sqlc.arg(at)+interval '10 minutes' ELSE denied_until END,version=version+1 WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='PENDING' AND pending_expires_at>sqlc.arg(at) AND failed_attempts<5;
-- name: ActivateMFAFactor :execrows
UPDATE identity.mfa_factors SET state='ACTIVE',activated_at=sqlc.arg(at),last_accepted_counter=sqlc.arg(counter),version=version+1 WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='PENDING' AND version=sqlc.arg(expected_version) AND pending_expires_at>sqlc.arg(at) AND last_accepted_counter<sqlc.arg(counter);
-- name: AdvanceMFAFactorCounter :execrows
UPDATE identity.mfa_factors SET last_accepted_counter=sqlc.arg(counter),version=version+1 WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='ACTIVE' AND version=sqlc.arg(expected_version) AND last_accepted_counter<sqlc.arg(counter);
-- name: ReplaceActiveMFAFactor :execrows
UPDATE identity.mfa_factors SET state='REPLACED',replaced_at=sqlc.arg(at),version=version+1 WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='ACTIVE' AND version=sqlc.arg(expected_version);
-- name: TouchActiveMFAFactor :execrows
UPDATE identity.mfa_factors SET version=version+1 WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='ACTIVE' AND version=sqlc.arg(expected_version);
-- name: InvalidateMFARecoveryCodes :exec
UPDATE identity.mfa_recovery_codes SET invalidated_at=sqlc.arg(at) WHERE user_id=sqlc.arg(user_id) AND consumed_at IS NULL AND invalidated_at IS NULL;
-- name: InsertMFARecoveryCode :exec
INSERT INTO identity.mfa_recovery_codes(id,factor_id,user_id,code_hash,created_at) VALUES(sqlc.arg(id),sqlc.arg(factor_id),sqlc.arg(user_id),sqlc.arg(code_hash),sqlc.arg(created_at));
-- name: CheckMFARecoveryCode :one
SELECT EXISTS(SELECT 1 FROM identity.mfa_recovery_codes WHERE factor_id=sqlc.arg(factor_id) AND user_id=sqlc.arg(user_id) AND code_hash=sqlc.arg(code_hash) AND consumed_at IS NULL AND invalidated_at IS NULL);
-- name: ConsumeMFARecoveryCode :execrows
UPDATE identity.mfa_recovery_codes SET consumed_at=sqlc.arg(at) WHERE factor_id=sqlc.arg(factor_id) AND user_id=sqlc.arg(user_id) AND code_hash=sqlc.arg(code_hash) AND consumed_at IS NULL AND invalidated_at IS NULL;
-- name: LockMFAUserSession :one
SELECT u.auth_epoch FROM identity.users u JOIN identity.sessions s ON s.user_id=u.id WHERE u.id=sqlc.arg(user_id) AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL AND u.auth_epoch=sqlc.arg(auth_epoch) AND s.id=sqlc.arg(session_id) AND s.auth_epoch=u.auth_epoch AND s.revoked_at IS NULL AND s.idle_expires_at>sqlc.arg(at) AND s.absolute_expires_at>sqlc.arg(at) FOR UPDATE OF u,s;
-- name: AdvanceMFAAuthEpoch :execrows
UPDATE identity.users SET auth_epoch=auth_epoch+1,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(id) AND auth_epoch=sqlc.arg(expected_auth_epoch);
-- name: RevokeMFAUserSessions :exec
UPDATE identity.sessions SET revoked_at=sqlc.arg(at) WHERE user_id=sqlc.arg(user_id) AND revoked_at IS NULL;
-- name: RevokeMFACurrentSession :execrows
UPDATE identity.sessions SET revoked_at=sqlc.arg(at) WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND revoked_at IS NULL;
-- name: CancelMFALoginTransactions :exec
UPDATE identity.mfa_transactions SET consumed_at=sqlc.arg(at) WHERE user_id=sqlc.arg(user_id) AND purpose IN ('MFA_LOGIN','EMAIL_LOGIN') AND consumed_at IS NULL;
-- name: InsertEmailLoginTransaction :exec
INSERT INTO identity.mfa_transactions(id,user_id,purpose,token_hash,browser_binding_hash,primary_method,auth_epoch,attempts,expires_at,created_at,email_code_mac) VALUES(sqlc.arg(id),sqlc.arg(user_id),'EMAIL_LOGIN',sqlc.arg(token_hash),sqlc.arg(browser_binding_hash),sqlc.arg(primary_method),sqlc.arg(auth_epoch),0,sqlc.arg(at)::timestamptz+interval '5 minutes',sqlc.arg(at)::timestamptz,sqlc.arg(email_code_mac));
-- name: InsertTOTPLoginTransaction :exec
INSERT INTO identity.mfa_transactions(id,user_id,purpose,token_hash,browser_binding_hash,primary_method,auth_epoch,attempts,expires_at,created_at,email_code_mac) VALUES(sqlc.arg(id),sqlc.arg(user_id),'MFA_LOGIN',sqlc.arg(token_hash),sqlc.arg(browser_binding_hash),sqlc.arg(primary_method),sqlc.arg(auth_epoch),0,sqlc.arg(at)::timestamptz+interval '5 minutes',sqlc.arg(at)::timestamptz,NULL);
-- name: GetMFALoginChallenge :one
SELECT t.id AS transaction_id,t.user_id,t.purpose,t.auth_epoch,t.primary_method,t.attempts,t.expires_at,t.email_code_mac,u.email,
 f.id AS factor_id,f.secret_ciphertext,f.secret_nonce,f.key_version,f.last_accepted_counter,f.version AS factor_version
FROM identity.mfa_transactions t
JOIN identity.users u ON u.id=t.user_id AND u.auth_epoch=t.auth_epoch AND u.disabled_at IS NULL AND u.email_verified_at IS NOT NULL
LEFT JOIN identity.mfa_factors f ON f.user_id=t.user_id AND f.state='ACTIVE' AND t.purpose='MFA_LOGIN'
WHERE t.token_hash=sqlc.arg(token_hash) AND t.browser_binding_hash=sqlc.arg(browser_binding_hash) AND t.consumed_at IS NULL AND t.expires_at>sqlc.arg(at) AND t.attempts<5
 AND ((t.purpose='EMAIL_LOGIN' AND octet_length(t.email_code_mac)=32) OR (t.purpose='MFA_LOGIN' AND t.email_code_mac IS NULL AND f.id IS NOT NULL));
-- name: MFALoginAttemptDenied :one
SELECT EXISTS(SELECT 1 FROM identity.mfa_transactions WHERE token_hash=sqlc.arg(token_hash) AND browser_binding_hash=sqlc.arg(browser_binding_hash) AND attempts=5 AND denied_until>sqlc.arg(at));
-- name: RecordMFALoginFailure :execrows
UPDATE identity.mfa_transactions SET attempts=attempts+1,consumed_at=CASE WHEN attempts+1>=5 THEN sqlc.arg(at) ELSE NULL END,denied_until=CASE WHEN attempts+1>=5 THEN sqlc.arg(at)+interval '10 minutes' ELSE denied_until END WHERE id=sqlc.arg(id) AND consumed_at IS NULL AND expires_at>sqlc.arg(at) AND attempts<5;
-- name: ConsumeMFALoginTransaction :execrows
UPDATE identity.mfa_transactions SET consumed_at=sqlc.arg(at) WHERE id=sqlc.arg(id) AND consumed_at IS NULL AND expires_at>sqlc.arg(at) AND attempts<5;
-- name: CancelEmailLoginTransaction :exec
UPDATE identity.mfa_transactions SET consumed_at=sqlc.arg(at) WHERE purpose IN ('MFA_LOGIN','EMAIL_LOGIN') AND token_hash=sqlc.arg(token_hash) AND browser_binding_hash=sqlc.arg(browser_binding_hash) AND consumed_at IS NULL;
-- name: InsertMFAAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),sqlc.arg(action),'mfa_factor',sqlc.arg(factor_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('assurance_level','AAL2','assurance_method',sqlc.arg(method)::text),1,sqlc.arg(occurred_at));
-- name: InsertMFAOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'mfa_factor',sqlc.arg(factor_id),sqlc.arg(version),sqlc.arg(event_type),1,'INTERNAL',jsonb_build_object('factor_id',sqlc.arg(factor_id)::uuid,'user_id',sqlc.arg(actor_id)::uuid,'assurance_level','AAL2','assurance_method',sqlc.arg(method)::text),sqlc.arg(occurred_at),NULL,0,sqlc.arg(occurred_at),sqlc.arg(occurred_at));
-- name: GetPlatformEmailLoginPolicy :one
SELECT COALESCE(account_type='PLATFORM_ADMIN',false)::boolean AS platform_admin FROM identity.users
WHERE id=sqlc.arg(user_id) AND auth_epoch=sqlc.arg(auth_epoch) AND email_verified_at IS NOT NULL AND disabled_at IS NULL;
-- name: RevokePreOTPAdministratorSessions :exec
UPDATE identity.sessions SET revoked_at=sqlc.arg(at) WHERE user_id=sqlc.arg(user_id) AND assurance_level='AAL1' AND revoked_at IS NULL;
-- name: LockEmailLoginSessionPolicy :one
SELECT COALESCE(u.account_type='PLATFORM_ADMIN',false)::boolean AS platform_admin,t.primary_method
FROM identity.users u JOIN identity.mfa_transactions t ON t.user_id=u.id AND t.auth_epoch=u.auth_epoch
WHERE u.id=sqlc.arg(user_id) AND u.auth_epoch=sqlc.arg(auth_epoch) AND u.email_verified_at IS NOT NULL AND u.disabled_at IS NULL
AND t.id=sqlc.arg(transaction_id) AND t.purpose='EMAIL_LOGIN' AND t.consumed_at IS NULL AND t.expires_at>sqlc.arg(at) AND t.attempts<5
FOR UPDATE OF u,t;
-- name: InsertEmailLoginDeliveryAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'EmailLoginCodeSMTPAccepted','authentication',sqlc.arg(transaction_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(transaction_id),'{"provider_acceptance":true,"inbox_delivery_confirmed":false}'::jsonb,1,sqlc.arg(at));
-- name: InsertEmailLoginAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'EmailLoginCodeVerified','session',sqlc.arg(session_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('assurance_level',sqlc.arg(assurance_level)::text,'assurance_method','EMAIL_OTP'),1,sqlc.arg(occurred_at));
-- name: InsertEmailLoginOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,published_at,attempts,next_attempt_at,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'authentication',sqlc.arg(transaction_id),1,'identity.email_login.completed.v1',1,'INTERNAL',jsonb_build_object('user_id',sqlc.arg(actor_id)::uuid,'assurance_level',sqlc.arg(assurance_level)::text,'assurance_method','EMAIL_OTP'),sqlc.arg(occurred_at),NULL,0,sqlc.arg(occurred_at),sqlc.arg(occurred_at));
