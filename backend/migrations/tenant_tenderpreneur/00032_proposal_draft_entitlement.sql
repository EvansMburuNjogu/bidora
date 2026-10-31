-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE ai.proposal_draft_runs
 DROP CONSTRAINT proposal_draft_runs_state_check,
 ADD CONSTRAINT proposal_draft_runs_state_check CHECK(state IN ('QUEUED','PENDING_RESERVATION','READY_FOR_PROVIDER','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
 ADD COLUMN input_characters bigint CHECK(input_characters>0),
 ADD COLUMN estimated_credits bigint CHECK(estimated_credits>0),
 ADD COLUMN checkpoint_id uuid UNIQUE REFERENCES bidora_entitlement.ai_usage_checkpoints(id) ON DELETE RESTRICT,
 ADD COLUMN entitlement_accounting_version smallint NOT NULL DEFAULT 1 CHECK(entitlement_accounting_version IN(1,2));
ALTER TABLE ai.proposal_draft_runs ALTER COLUMN entitlement_accounting_version SET DEFAULT 2,
 ADD CONSTRAINT proposal_draft_entitlement_shape CHECK(
  (entitlement_accounting_version=1 AND state IN('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELLED') AND input_characters IS NULL AND estimated_credits IS NULL AND checkpoint_id IS NULL)
  OR (entitlement_accounting_version=2 AND state IN('PENDING_RESERVATION','READY_FOR_PROVIDER','RUNNING','SUCCEEDED','FAILED','CANCELLED') AND input_characters IS NOT NULL AND estimated_credits IS NOT NULL AND checkpoint_id IS NOT NULL)
 );
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=32,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=31;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 31'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM ai.proposal_draft_runs WHERE checkpoint_id IS NOT NULL) THEN RAISE EXCEPTION 'cannot roll back retained proposal draft entitlement evidence'; END IF; END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=31,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=32;
ALTER TABLE ai.proposal_draft_runs DROP CONSTRAINT proposal_draft_entitlement_shape,DROP COLUMN entitlement_accounting_version,DROP COLUMN checkpoint_id,DROP COLUMN estimated_credits,DROP COLUMN input_characters,DROP CONSTRAINT proposal_draft_runs_state_check,ADD CONSTRAINT proposal_draft_runs_state_check CHECK(state IN ('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELLED'));
-- +goose StatementEnd
