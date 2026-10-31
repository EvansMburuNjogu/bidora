-- +goose Up
-- +goose StatementBegin
-- Fail-closed exact-version scan results and unverified cited AI proposals.
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE SCHEMA IF NOT EXISTS ai;

ALTER TABLE documents.document_versions
 ADD COLUMN scan_engine text,
 ADD COLUMN scan_result_code text,
 ADD COLUMN scan_completed_at timestamptz,
 ADD CONSTRAINT document_scan_result_shape CHECK (
   (scan_state='QUARANTINED' AND scan_engine IS NULL AND scan_result_code IS NULL AND scan_completed_at IS NULL) OR
   (scan_state IN ('CLEAN','REJECTED','FAILED') AND scan_engine IS NOT NULL AND length(scan_engine) BETWEEN 1 AND 100 AND scan_result_code IS NOT NULL AND length(scan_result_code) BETWEEN 1 AND 100 AND scan_completed_at IS NOT NULL)
 );

CREATE TABLE ai.extraction_runs (
 id uuid PRIMARY KEY,
 job_id uuid NOT NULL UNIQUE REFERENCES jobs.jobs(id) ON DELETE RESTRICT,
 document_version_id uuid NOT NULL UNIQUE REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 provider text NOT NULL CHECK(provider='OPENAI'),
 model text NOT NULL CHECK(length(model) BETWEEN 1 AND 200),
 schema_version integer NOT NULL CHECK(schema_version=1),
 provider_response_id text CHECK(provider_response_id IS NULL OR length(provider_response_id) BETWEEN 1 AND 255),
 state text NOT NULL CHECK(state IN ('RUNNING','SUCCEEDED','FAILED')),
 started_at timestamptz NOT NULL,
 completed_at timestamptz,
 failure_code text CHECK(failure_code IS NULL OR length(failure_code) BETWEEN 1 AND 100),
 CHECK((state='RUNNING' AND provider_response_id IS NULL AND completed_at IS NULL AND failure_code IS NULL) OR (state='SUCCEEDED' AND provider_response_id IS NOT NULL AND completed_at IS NOT NULL AND failure_code IS NULL) OR (state='FAILED' AND completed_at IS NOT NULL AND failure_code IS NOT NULL))
);

CREATE TABLE ai.requirement_suggestions (
 id uuid PRIMARY KEY,
 extraction_run_id uuid NOT NULL REFERENCES ai.extraction_runs(id) ON DELETE RESTRICT,
 document_version_id uuid NOT NULL REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 ordinal integer NOT NULL CHECK(ordinal BETWEEN 1 AND 500),
 requirement_text text NOT NULL CHECK(length(requirement_text) BETWEEN 1 AND 10000 AND requirement_text ~ '\S'),
 category text NOT NULL CHECK(length(category) BETWEEN 1 AND 100 AND category ~ '\S'),
 mandatory boolean NOT NULL,
 citation_page integer CHECK(citation_page IS NULL OR citation_page BETWEEN 1 AND 10000),
 citation_location text NOT NULL CHECK(length(citation_location) BETWEEN 1 AND 500 AND citation_location ~ '\S'),
 citation_quote text NOT NULL CHECK(length(citation_quote) BETWEEN 1 AND 2000 AND citation_quote ~ '\S'),
 confidence numeric(5,4) NOT NULL CHECK(confidence BETWEEN 0 AND 1),
 review_state text NOT NULL CHECK(review_state IN ('PROPOSED_UNVERIFIED','ACCEPTED','REJECTED')),
 created_at timestamptz NOT NULL,
 reviewed_by uuid,
 reviewed_at timestamptz,
 UNIQUE(extraction_run_id,ordinal),
 CONSTRAINT requirement_suggestion_initial_review_shape CHECK((review_state='PROPOSED_UNVERIFIED' AND reviewed_by IS NULL AND reviewed_at IS NULL) OR (review_state IN ('ACCEPTED','REJECTED') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL))
);
CREATE INDEX requirement_suggestions_review_idx ON ai.requirement_suggestions(document_version_id,review_state,ordinal);

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=7,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=6;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 6'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=6,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=7;
DROP TABLE ai.requirement_suggestions;
DROP TABLE ai.extraction_runs;
DROP SCHEMA ai;
ALTER TABLE documents.document_versions DROP CONSTRAINT document_scan_result_shape, DROP COLUMN scan_engine, DROP COLUMN scan_result_code, DROP COLUMN scan_completed_at;
-- +goose StatementEnd
