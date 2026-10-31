-- +goose Up
-- +goose StatementBegin
-- AI response drafts are advisory snapshots grounded in exact CLEAN versions.
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE jobs.jobs DROP CONSTRAINT jobs_kind_check, ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('MALWARE_SCAN','TENDER_ANALYSIS','RESPONSE_DRAFT'));

CREATE TABLE ai.response_draft_runs (
 id uuid PRIMARY KEY,
 job_id uuid NOT NULL UNIQUE REFERENCES jobs.jobs(id) ON DELETE RESTRICT,
 bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 requirement_id uuid NOT NULL REFERENCES bid.requirements(id) ON DELETE RESTRICT,
 source_document_version_id uuid NOT NULL REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 requested_workspace_version bigint NOT NULL CHECK(requested_workspace_version>0),
 requested_section_version bigint NOT NULL CHECK(requested_section_version>=0),
 current_response_version_id uuid REFERENCES bid.response_versions(id) ON DELETE RESTRICT,
 provider text NOT NULL CHECK(provider='OPENAI'),
 model text NOT NULL CHECK(length(model) BETWEEN 1 AND 200),
 schema_version integer NOT NULL CHECK(schema_version=1),
 state text NOT NULL CHECK(state IN ('QUEUED','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
 provider_response_id text CHECK(provider_response_id IS NULL OR length(provider_response_id) BETWEEN 1 AND 255),
 failure_code text CHECK(failure_code IS NULL OR length(failure_code) BETWEEN 1 AND 100),
 requested_by uuid NOT NULL,
 correlation_id uuid NOT NULL,
 created_at timestamptz NOT NULL,
 started_at timestamptz,
 completed_at timestamptz,
 CHECK((state='QUEUED' AND started_at IS NULL AND completed_at IS NULL) OR (state='RUNNING' AND started_at IS NOT NULL AND completed_at IS NULL) OR (state='SUCCEEDED' AND provider_response_id IS NOT NULL AND completed_at IS NOT NULL) OR (state='FAILED' AND failure_code IS NOT NULL AND completed_at IS NOT NULL) OR (state='CANCELLED' AND completed_at IS NOT NULL))
);
CREATE TABLE ai.response_draft_inputs (
 run_id uuid NOT NULL REFERENCES ai.response_draft_runs(id) ON DELETE RESTRICT,
 document_version_id uuid NOT NULL REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 input_kind text NOT NULL CHECK(input_kind IN ('TENDER_SOURCE','REQUIREMENT_EVIDENCE')),
 ordinal integer NOT NULL CHECK(ordinal BETWEEN 1 AND 5),
 PRIMARY KEY(run_id,document_version_id), UNIQUE(run_id,ordinal)
);
CREATE TABLE ai.response_draft_suggestions (
 id uuid PRIMARY KEY,
 run_id uuid NOT NULL UNIQUE REFERENCES ai.response_draft_runs(id) ON DELETE RESTRICT,
 suggested_content text NOT NULL CHECK(length(suggested_content) BETWEEN 1 AND 20000 AND suggested_content ~ '\S'),
 rationale text NOT NULL CHECK(length(rationale) BETWEEN 1 AND 2000 AND rationale ~ '\S'),
 review_state text NOT NULL CHECK(review_state='PROPOSED_UNVERIFIED'),
 created_at timestamptz NOT NULL
);
CREATE TABLE ai.response_draft_citations (
 suggestion_id uuid NOT NULL REFERENCES ai.response_draft_suggestions(id) ON DELETE RESTRICT,
 ordinal integer NOT NULL CHECK(ordinal BETWEEN 1 AND 100),
 document_version_id uuid NOT NULL REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 citation_location text NOT NULL CHECK(length(citation_location) BETWEEN 1 AND 500 AND citation_location ~ '\S'),
 citation_quote text NOT NULL CHECK(length(citation_quote) BETWEEN 1 AND 2000 AND citation_quote ~ '\S'),
 PRIMARY KEY(suggestion_id,ordinal)
);
CREATE TABLE ai.response_gap_suggestions (
 suggestion_id uuid NOT NULL REFERENCES ai.response_draft_suggestions(id) ON DELETE RESTRICT,
 ordinal integer NOT NULL CHECK(ordinal BETWEEN 1 AND 100),
 gap_code text NOT NULL CHECK(gap_code IN ('MISSING_EVIDENCE','UNSUPPORTED_CLAIM','MISSING_FACT','AMBIGUOUS_REQUIREMENT','REVIEW_NEEDED')),
 description text NOT NULL CHECK(length(description) BETWEEN 1 AND 2000 AND description ~ '\S'),
 document_version_id uuid REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 citation_location text CHECK(citation_location IS NULL OR (length(citation_location) BETWEEN 1 AND 500 AND citation_location ~ '\S')),
 citation_quote text CHECK(citation_quote IS NULL OR (length(citation_quote) BETWEEN 1 AND 2000 AND citation_quote ~ '\S')),
 PRIMARY KEY(suggestion_id,ordinal),
 CHECK((document_version_id IS NULL AND citation_location IS NULL AND citation_quote IS NULL) OR (document_version_id IS NOT NULL AND citation_location IS NOT NULL AND citation_quote IS NOT NULL))
);

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=11,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=10;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 10'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=10,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=11;
DROP TABLE ai.response_gap_suggestions;
DROP TABLE ai.response_draft_citations;
DROP TABLE ai.response_draft_suggestions;
DROP TABLE ai.response_draft_inputs;
DROP TABLE ai.response_draft_runs;
ALTER TABLE jobs.jobs DROP CONSTRAINT jobs_kind_check, ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('MALWARE_SCAN','TENDER_ANALYSIS'));
-- +goose StatementEnd
