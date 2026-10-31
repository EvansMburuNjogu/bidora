-- +goose Up
-- +goose StatementBegin
-- Correct the legacy unnamed run-shape checks so entitlement-gated states added
-- by migrations 31 and 32 are reachable without weakening terminal evidence.
SET lock_timeout='1s'; SET statement_timeout='5s';

ALTER TABLE ai.response_draft_runs
 DROP CONSTRAINT response_draft_runs_check,
 ADD CONSTRAINT response_draft_runs_shape CHECK(
   (state IN ('QUEUED','MEASURING','PENDING_RESERVATION','READY_FOR_PROVIDER') AND provider_response_id IS NULL AND failure_code IS NULL AND started_at IS NULL AND completed_at IS NULL)
   OR (state='RUNNING' AND provider_response_id IS NULL AND failure_code IS NULL AND started_at IS NOT NULL AND completed_at IS NULL)
   OR (state='SUCCEEDED' AND provider_response_id IS NOT NULL AND failure_code IS NULL AND started_at IS NOT NULL AND completed_at IS NOT NULL)
   OR (state='FAILED' AND provider_response_id IS NULL AND failure_code IS NOT NULL AND completed_at IS NOT NULL)
   OR (state='CANCELLED' AND provider_response_id IS NULL AND completed_at IS NOT NULL)
 );

ALTER TABLE ai.proposal_draft_runs
 DROP CONSTRAINT proposal_draft_runs_check,
 ADD CONSTRAINT proposal_draft_runs_shape CHECK(
   (state IN ('QUEUED','PENDING_RESERVATION','READY_FOR_PROVIDER') AND provider_response_id IS NULL AND failure_code IS NULL AND started_at IS NULL AND completed_at IS NULL)
   OR (state='RUNNING' AND provider_response_id IS NULL AND failure_code IS NULL AND started_at IS NOT NULL AND completed_at IS NULL)
   OR (state='SUCCEEDED' AND provider_response_id IS NOT NULL AND failure_code IS NULL AND started_at IS NOT NULL AND completed_at IS NOT NULL)
   OR (state='FAILED' AND provider_response_id IS NULL AND failure_code IS NOT NULL AND completed_at IS NOT NULL)
   OR (state='CANCELLED' AND provider_response_id IS NULL AND completed_at IS NOT NULL)
 );

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=33,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=32;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 32'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM ai.response_draft_runs WHERE state IN ('MEASURING','PENDING_RESERVATION','READY_FOR_PROVIDER') OR checkpoint_id IS NOT NULL)
    OR EXISTS(SELECT 1 FROM ai.proposal_draft_runs WHERE state IN ('PENDING_RESERVATION','READY_FOR_PROVIDER') OR checkpoint_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot roll back retained generation entitlement evidence';
 END IF;
END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=32,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=33;
ALTER TABLE ai.response_draft_runs DROP CONSTRAINT response_draft_runs_shape,
 ADD CONSTRAINT response_draft_runs_check CHECK((state='QUEUED' AND started_at IS NULL AND completed_at IS NULL) OR (state='RUNNING' AND started_at IS NOT NULL AND completed_at IS NULL) OR (state='SUCCEEDED' AND provider_response_id IS NOT NULL AND completed_at IS NOT NULL) OR (state='FAILED' AND failure_code IS NOT NULL AND completed_at IS NOT NULL) OR (state='CANCELLED' AND completed_at IS NOT NULL));
ALTER TABLE ai.proposal_draft_runs DROP CONSTRAINT proposal_draft_runs_shape,
 ADD CONSTRAINT proposal_draft_runs_check CHECK((state='QUEUED' AND started_at IS NULL AND completed_at IS NULL) OR (state='RUNNING' AND started_at IS NOT NULL AND completed_at IS NULL) OR (state='SUCCEEDED' AND provider_response_id IS NOT NULL AND completed_at IS NOT NULL) OR (state='FAILED' AND failure_code IS NOT NULL AND completed_at IS NOT NULL) OR (state='CANCELLED' AND completed_at IS NOT NULL));
-- +goose StatementEnd
