-- name: LockIntegrationHead :one
SELECT setting_kind,next_sequence,active_version_id,generation FROM platform_settings.integration_heads WHERE setting_kind=sqlc.arg(setting_kind) FOR UPDATE;

-- name: AdvanceIntegrationSequence :execrows
UPDATE platform_settings.integration_heads SET next_sequence=next_sequence+1,updated_at=sqlc.arg(at) WHERE setting_kind=sqlc.arg(setting_kind) AND next_sequence=sqlc.arg(expected_sequence);

-- name: InsertIntegrationVersion :exec
INSERT INTO platform_settings.integration_versions(id,setting_kind,sequence,state,version,configuration_hash,created_by,creation_reason,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(setting_kind),sqlc.arg(sequence),'DRAFT',1,sqlc.arg(configuration_hash),sqlc.arg(created_by),sqlc.arg(reason),sqlc.arg(at),sqlc.arg(at));

-- name: GetVerifiedPlatformSettingTestEmail :one
SELECT u.email
FROM identity.users u
JOIN platform_administration.administrator_profiles p ON p.user_id=u.id
WHERE u.id=sqlc.arg(user_id) AND u.disabled_at IS NULL AND u.email_verified_at IS NOT NULL
  AND EXISTS(SELECT 1 FROM platform_administration.role_assignments r WHERE r.user_id=u.id AND r.state='ACTIVE')
LIMIT 1;

-- name: InsertSMTPVersion :exec
INSERT INTO platform_settings.smtp_versions(version_id,host,port,credential_secret_ref,trust_roots_secret_ref,sender_name,sender_address,reply_to_address,connect_timeout_seconds,command_timeout_seconds,delivery_timeout_seconds,max_concurrency,starttls_required)
VALUES(sqlc.arg(version_id),sqlc.arg(host),sqlc.arg(port),sqlc.arg(credential_secret_ref),sqlc.arg(trust_roots_secret_ref),sqlc.arg(sender_name),sqlc.arg(sender_address),sqlc.narg(reply_to_address),sqlc.arg(connect_timeout_seconds),sqlc.arg(command_timeout_seconds),sqlc.arg(delivery_timeout_seconds),sqlc.arg(max_concurrency),true);

-- name: InsertOpenAIVersion :exec
INSERT INTO platform_settings.openai_versions(version_id,endpoint,model,api_key_secret_ref,trust_roots_secret_ref,store_responses,timeout_seconds,max_concurrency,enabled)
VALUES(sqlc.arg(version_id),'https://api.openai.com/v1/responses',sqlc.arg(model),sqlc.arg(api_key_secret_ref),sqlc.arg(trust_roots_secret_ref),false,sqlc.arg(timeout_seconds),sqlc.arg(max_concurrency),sqlc.arg(enabled));

-- name: GetActiveOpenAIConfiguration :one
SELECT v.id,v.version,v.configuration_hash,o.model,o.api_key_secret_ref,o.trust_roots_secret_ref,o.timeout_seconds,o.max_concurrency,o.enabled
FROM platform_settings.integration_heads h JOIN platform_settings.integration_versions v ON v.id=h.active_version_id AND v.setting_kind='OPENAI' AND v.state='ACTIVE' JOIN platform_settings.openai_versions o ON o.version_id=v.id WHERE h.setting_kind='OPENAI';

-- name: GetIntegrationVersion :one
SELECT v.id,v.setting_kind,v.sequence,v.state,v.version,v.configuration_hash,v.latest_test_id,v.created_by,v.created_at,v.updated_at,v.activated_at,v.superseded_at,
 s.host,s.port,s.sender_name,s.sender_address,s.reply_to_address,s.connect_timeout_seconds,s.command_timeout_seconds,s.delivery_timeout_seconds,s.max_concurrency AS smtp_max_concurrency,
 o.model,o.timeout_seconds,o.max_concurrency AS openai_max_concurrency,o.enabled AS openai_enabled,
 p.enabled AS pesapal_enabled,p.environment,p.merchant_name,p.notification_id,COALESCE(p.credentials_secret_ref LIKE 'dbpesapal:v1:%',false)::boolean AS pesapal_credentials_configured
FROM platform_settings.integration_versions v LEFT JOIN platform_settings.smtp_versions s ON s.version_id=v.id LEFT JOIN platform_settings.openai_versions o ON o.version_id=v.id LEFT JOIN platform_settings.pesapal_versions p ON p.version_id=v.id
WHERE v.id=sqlc.arg(id);

-- name: GetActiveSMTPDeliveryConfiguration :one
SELECT v.id,v.version,v.configuration_hash,s.host,s.port,s.credential_secret_ref,s.trust_roots_secret_ref,
 s.sender_name,s.sender_address,s.reply_to_address,s.connect_timeout_seconds,s.command_timeout_seconds,s.delivery_timeout_seconds,s.max_concurrency
FROM platform_settings.integration_heads h
JOIN platform_settings.integration_versions v ON v.id=h.active_version_id AND v.setting_kind='SMTP' AND v.state='ACTIVE'
JOIN platform_settings.smtp_versions s ON s.version_id=v.id
WHERE h.setting_kind='SMTP';

-- name: GetActiveIntegrationVersion :one
SELECT v.id,v.setting_kind,v.sequence,v.state,v.version,v.configuration_hash,v.latest_test_id,v.created_by,v.created_at,v.updated_at,v.activated_at,v.superseded_at,
 s.host,s.port,s.sender_name,s.sender_address,s.reply_to_address,s.connect_timeout_seconds,s.command_timeout_seconds,s.delivery_timeout_seconds,s.max_concurrency AS smtp_max_concurrency,
 o.model,o.timeout_seconds,o.max_concurrency AS openai_max_concurrency,o.enabled AS openai_enabled,
 p.enabled AS pesapal_enabled,p.environment,p.merchant_name,p.notification_id,COALESCE(p.credentials_secret_ref LIKE 'dbpesapal:v1:%',false)::boolean AS pesapal_credentials_configured
FROM platform_settings.integration_heads h
JOIN platform_settings.integration_versions v ON v.id=h.active_version_id AND v.setting_kind=h.setting_kind AND v.state='ACTIVE'
LEFT JOIN platform_settings.smtp_versions s ON s.version_id=v.id
LEFT JOIN platform_settings.openai_versions o ON o.version_id=v.id LEFT JOIN platform_settings.pesapal_versions p ON p.version_id=v.id
WHERE h.setting_kind=sqlc.arg(setting_kind);

-- name: GetIntegrationVersionInternal :one
SELECT v.id,v.setting_kind,v.sequence,v.state,v.version,v.configuration_hash,v.latest_test_id,v.created_by,v.created_at,v.updated_at,v.activated_at,v.superseded_at,
 s.host,s.port,s.credential_secret_ref,s.trust_roots_secret_ref AS smtp_trust_roots_secret_ref,s.sender_name,s.sender_address,s.reply_to_address,s.connect_timeout_seconds,s.command_timeout_seconds,s.delivery_timeout_seconds,s.max_concurrency AS smtp_max_concurrency,
 o.model,o.api_key_secret_ref,o.trust_roots_secret_ref AS openai_trust_roots_secret_ref,o.timeout_seconds,o.max_concurrency AS openai_max_concurrency,o.enabled AS openai_enabled,
 p.enabled AS pesapal_enabled,p.environment,p.merchant_name,p.notification_id,p.credentials_secret_ref
FROM platform_settings.integration_versions v LEFT JOIN platform_settings.smtp_versions s ON s.version_id=v.id LEFT JOIN platform_settings.openai_versions o ON o.version_id=v.id LEFT JOIN platform_settings.pesapal_versions p ON p.version_id=v.id
WHERE v.id=sqlc.arg(id);

-- name: ListIntegrationVersions :many
SELECT v.id,v.setting_kind,v.sequence,v.state,v.version,v.latest_test_id,v.created_at,v.updated_at,v.activated_at,v.superseded_at,
 s.host,s.port,s.sender_name,s.sender_address,s.reply_to_address,s.connect_timeout_seconds,s.command_timeout_seconds,s.delivery_timeout_seconds,s.max_concurrency AS smtp_max_concurrency,
 o.model,o.timeout_seconds,o.max_concurrency AS openai_max_concurrency,o.enabled AS openai_enabled,
 p.enabled AS pesapal_enabled,p.environment,p.merchant_name,p.notification_id,COALESCE(p.credentials_secret_ref LIKE 'dbpesapal:v1:%',false)::boolean AS pesapal_credentials_configured
FROM platform_settings.integration_versions v LEFT JOIN platform_settings.smtp_versions s ON s.version_id=v.id LEFT JOIN platform_settings.openai_versions o ON o.version_id=v.id LEFT JOIN platform_settings.pesapal_versions p ON p.version_id=v.id
WHERE v.setting_kind=sqlc.arg(setting_kind) AND (sqlc.narg(before_sequence)::bigint IS NULL OR v.sequence<sqlc.narg(before_sequence)) ORDER BY v.sequence DESC LIMIT sqlc.arg(result_limit);

-- name: LockIntegrationVersion :one
SELECT id,setting_kind,sequence,state,version,configuration_hash,latest_test_id,created_by,created_at,updated_at,activated_at,superseded_at FROM platform_settings.integration_versions WHERE id=sqlc.arg(id) FOR UPDATE;

-- name: CountIntegrationTests :one
SELECT count(*) FROM platform_settings.integration_test_attempts WHERE version_id=sqlc.arg(version_id);

-- name: InsertIntegrationTest :exec
INSERT INTO platform_settings.integration_test_attempts(id,version_id,attempt,outcome,result_code,applicability_hash,tested_by,reason,occurred_at)
VALUES(sqlc.arg(id),sqlc.arg(version_id),sqlc.arg(attempt),sqlc.arg(outcome),sqlc.arg(result_code),sqlc.arg(applicability_hash),sqlc.arg(tested_by),sqlc.arg(reason),sqlc.arg(occurred_at));

-- name: ApplyIntegrationTestResult :execrows
UPDATE platform_settings.integration_versions SET state=sqlc.arg(state),version=version+1,latest_test_id=sqlc.arg(test_id),updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND state IN('DRAFT','TEST_PASSED','TEST_FAILED','SUPERSEDED');

-- name: GetIntegrationTest :one
SELECT id,version_id,attempt,outcome,result_code,applicability_hash,tested_by,reason,occurred_at FROM platform_settings.integration_test_attempts WHERE id=sqlc.arg(id);

-- name: InsertIntegrationChange :exec
INSERT INTO platform_settings.integration_changes(id,setting_kind,target_version_id,action,state,version,target_prior_state,target_test_id,expected_active_version_id,maker_user_id,proposal_reason,created_at,expires_at)
VALUES(sqlc.arg(id),sqlc.arg(setting_kind),sqlc.arg(target_version_id),sqlc.arg(action),'PENDING_APPROVAL',1,sqlc.arg(target_prior_state),sqlc.arg(target_test_id),sqlc.narg(expected_active_version_id),sqlc.arg(maker_user_id),sqlc.arg(reason),sqlc.arg(created_at),sqlc.arg(expires_at));

-- name: MarkIntegrationVersionPending :execrows
UPDATE platform_settings.integration_versions SET state='PENDING_APPROVAL',version=version+1,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND state=sqlc.arg(expected_state);

-- name: LockIntegrationChange :one
SELECT id,setting_kind,target_version_id,action,state,version,target_prior_state,target_test_id,expected_active_version_id,maker_user_id,checker_user_id,proposal_reason,decision_reason,created_at,expires_at,decided_at
FROM platform_settings.integration_changes WHERE id=sqlc.arg(id) FOR UPDATE;

-- name: GetIntegrationChange :one
SELECT id,setting_kind,target_version_id,action,state,version,target_prior_state,target_test_id,expected_active_version_id,maker_user_id,checker_user_id,created_at,expires_at,decided_at
FROM platform_settings.integration_changes WHERE id=sqlc.arg(id);

-- name: RestoreIntegrationVersionAfterDisposition :execrows
UPDATE platform_settings.integration_versions SET state=sqlc.arg(state),version=version+1,updated_at=sqlc.arg(at) WHERE id=sqlc.arg(id) AND state='PENDING_APPROVAL';

-- name: RejectIntegrationChange :execrows
UPDATE platform_settings.integration_changes SET state='REJECTED',version=version+1,checker_user_id=sqlc.arg(checker_user_id),decision_reason=sqlc.arg(reason),decided_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='PENDING_APPROVAL' AND version=sqlc.arg(expected_version);

-- name: ExpireIntegrationChange :execrows
UPDATE platform_settings.integration_changes SET state='EXPIRED',version=version+1,decision_reason=sqlc.arg(reason),decided_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='PENDING_APPROVAL' AND version=sqlc.arg(expected_version);

-- name: SupersedeActiveIntegrationVersion :execrows
UPDATE platform_settings.integration_versions SET state='SUPERSEDED',version=version+1,superseded_at=sqlc.arg(at),updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='ACTIVE';

-- name: ActivateIntegrationVersion :execrows
UPDATE platform_settings.integration_versions SET state='ACTIVE',version=version+1,activated_at=sqlc.arg(at),superseded_at=NULL,updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='PENDING_APPROVAL';

-- name: AdvanceIntegrationHead :execrows
UPDATE platform_settings.integration_heads SET active_version_id=sqlc.arg(active_version_id),generation=generation+1,updated_at=sqlc.arg(at)
WHERE setting_kind=sqlc.arg(setting_kind) AND generation=sqlc.arg(expected_generation) AND active_version_id IS NOT DISTINCT FROM sqlc.narg(expected_active_version_id);

-- name: ApproveIntegrationChange :execrows
UPDATE platform_settings.integration_changes SET state='APPROVED_EXECUTED',version=version+1,checker_user_id=sqlc.arg(checker_user_id),decision_reason=sqlc.arg(reason),decided_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='PENDING_APPROVAL' AND version=sqlc.arg(expected_version);

-- name: InsertIntegrationActivationHistory :exec
INSERT INTO platform_settings.integration_activation_history(id,setting_kind,version_id,previous_version_id,change_id,action,generation,activated_by,activated_at)
VALUES(sqlc.arg(id),sqlc.arg(setting_kind),sqlc.arg(version_id),sqlc.narg(previous_version_id),sqlc.arg(change_id),sqlc.arg(action),sqlc.arg(generation),sqlc.arg(activated_by),sqlc.arg(activated_at));

-- name: GetIntegrationCommand :one
SELECT request_hash,state,target_id,response_state,response_version,response_status FROM platform_settings.integration_commands WHERE actor_user_id=sqlc.arg(actor_user_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash);

-- name: InsertIntegrationCommand :exec
INSERT INTO platform_settings.integration_commands(id,actor_user_id,operation,key_hash,request_hash,state,target_id,response_state,response_version,response_status,created_at,completed_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_user_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),'COMPLETED',sqlc.arg(target_id),sqlc.arg(response_state),sqlc.arg(response_version),sqlc.arg(response_status),sqlc.arg(at),sqlc.arg(at));

-- name: BeginIntegrationTestCommand :exec
INSERT INTO platform_settings.integration_commands(id,actor_user_id,operation,key_hash,request_hash,state,target_id,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_user_id),'TEST_VERSION',sqlc.arg(key_hash),sqlc.arg(request_hash),'PENDING',sqlc.arg(target_id),sqlc.arg(at));

-- name: CompleteIntegrationTestCommand :execrows
UPDATE platform_settings.integration_commands SET state='COMPLETED',response_state=sqlc.arg(response_state),response_version=sqlc.arg(response_version),response_status=200,completed_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND state='PENDING';
