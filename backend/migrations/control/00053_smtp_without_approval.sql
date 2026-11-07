-- +goose Up
SET lock_timeout='5s';
SET statement_timeout='30s';
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM bidora_control_meta.schema_compatibility WHERE migration_set='control_registry' AND schema_version=52) THEN RAISE EXCEPTION 'control schema 52 required';END IF;END $$;
-- SMTP changes are direct Owner operations, not fabricated checker approvals.
ALTER TABLE platform_settings.integration_activation_history
 ALTER COLUMN change_id DROP NOT NULL,
 ALTER COLUMN activated_by DROP NOT NULL,
 ADD COLUMN activation_method text NOT NULL DEFAULT 'APPROVAL' CHECK(activation_method IN('APPROVAL','DIRECT','MIGRATION')),
 ADD CONSTRAINT integration_activation_provenance CHECK(
  (activation_method='APPROVAL' AND change_id IS NOT NULL AND activated_by IS NOT NULL)
  OR (setting_kind='SMTP' AND activation_method='DIRECT' AND change_id IS NULL AND activated_by IS NOT NULL)
  OR (setting_kind='SMTP' AND activation_method='MIGRATION' AND change_id IS NULL AND activated_by IS NULL));
-- The operator explicitly authorised removal of SMTP approval. Promote only
-- the existing saved SMTP revision with its exact successful connection proof;
-- never use deployment SMTP values or invent a human approver.
-- +goose StatementBegin
DO $$ DECLARE candidate uuid; prior uuid; next_generation bigint; stamp timestamptz:=CURRENT_TIMESTAMP; BEGIN
 SELECT active_version_id,generation INTO prior,next_generation FROM platform_settings.integration_heads WHERE setting_kind='SMTP' FOR UPDATE;
 IF prior IS NULL THEN
  SELECT v.id INTO candidate FROM platform_settings.integration_versions v JOIN platform_settings.integration_test_attempts t ON t.id=v.latest_test_id AND t.version_id=v.id
  WHERE v.setting_kind='SMTP' AND v.state IN('TEST_PASSED','PENDING_APPROVAL') AND t.outcome='PASSED' AND t.result_code='SMTP_STARTTLS_VERIFIED'
   AND t.applicability_hash=sha256(convert_to('platform-integration-test:v1:','UTF8')||v.configuration_hash)
  ORDER BY v.sequence DESC LIMIT 1 FOR UPDATE OF v;
  IF candidate IS NOT NULL THEN
   UPDATE platform_settings.integration_versions SET state='ACTIVE',version=version+1,activated_at=stamp,updated_at=stamp WHERE id=candidate;
   UPDATE platform_settings.integration_heads SET active_version_id=candidate,generation=generation+1,updated_at=stamp WHERE setting_kind='SMTP';
   INSERT INTO platform_settings.integration_activation_history(id,setting_kind,version_id,previous_version_id,change_id,action,generation,activated_by,activated_at,activation_method) VALUES(gen_random_uuid(),'SMTP',candidate,NULL,NULL,'ACTIVATE',next_generation+1,NULL,stamp,'MIGRATION');
   INSERT INTO bidora_control_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,created_at) VALUES(gen_random_uuid(),NULL,'PlatformSMTPApprovalRemoved','platform_integration_setting',candidate,stamp,'SUCCEEDED',gen_random_uuid(),jsonb_build_object('activation_method','MIGRATION','approval_required',false),stamp);
  END IF;
 END IF;
 UPDATE platform_settings.integration_changes SET state='EXPIRED',version=version+1,decision_reason='SMTP approval removed; configuration is managed directly by the Owner',decided_at=stamp WHERE setting_kind='SMTP' AND state='PENDING_APPROVAL';
 UPDATE platform_settings.integration_versions SET state='SUPERSEDED',version=version+1,updated_at=stamp,superseded_at=stamp WHERE setting_kind='SMTP' AND state='PENDING_APPROVAL';
END $$;
-- +goose StatementEnd
UPDATE bidora_control_meta.schema_compatibility SET schema_version=53,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=52;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'direct SMTP settings require forward recovery';END $$;
-- +goose StatementEnd
