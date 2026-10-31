-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

ALTER TABLE jobs.jobs
 DROP CONSTRAINT jobs_state_check,
 DROP CONSTRAINT jobs_check,
 ADD CONSTRAINT jobs_state_check CHECK(state IN ('QUEUED','WAITING_FOR_CLEAN','MEASURING','PENDING_RESERVATION','READY_FOR_PROVIDER','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
 ADD CONSTRAINT jobs_check CHECK(
   (state='WAITING_FOR_CLEAN' AND blocked_reason='DOCUMENT_NOT_CLEAN') OR
   (state='PENDING_RESERVATION' AND blocked_reason='AI_ENTITLEMENT_PENDING') OR
   (state NOT IN ('WAITING_FOR_CLEAN','PENDING_RESERVATION') AND blocked_reason IS NULL)
 );

CREATE TABLE ai.document_measurements(
 id uuid PRIMARY KEY,
 job_id uuid NOT NULL UNIQUE REFERENCES jobs.jobs(id) ON DELETE RESTRICT,
 document_version_id uuid NOT NULL UNIQUE REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 requester_user_id uuid NOT NULL,
 assignment_generation bigint NOT NULL CHECK(assignment_generation>0),
 correlation_id uuid NOT NULL,
 media_type text NOT NULL CHECK(media_type IN ('application/pdf','application/vnd.openxmlformats-officedocument.wordprocessingml.document')),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 26214400),
 sha256 bytea NOT NULL CHECK(octet_length(sha256)=32),
 parser_key text NOT NULL CHECK(length(parser_key) BETWEEN 1 AND 100),
 parser_version text NOT NULL CHECK(length(parser_version) BETWEEN 1 AND 100),
 semantics_version text NOT NULL CHECK(length(semantics_version) BETWEEN 1 AND 100),
 profile_version text NOT NULL CHECK(length(profile_version) BETWEEN 1 AND 100),
 unit text NOT NULL CHECK(unit IN ('PDF_PAGES','DOCX_EXTRACTED_CHARACTERS')),
 measured_units bigint NOT NULL CHECK(measured_units>0),
 applicability_hash bytea NOT NULL UNIQUE CHECK(octet_length(applicability_hash)=32),
 usage_kind text NOT NULL CHECK(usage_kind IN ('DOCUMENT_PDF','DOCUMENT_DOCX')),
 estimated_credits bigint NOT NULL CHECK(estimated_credits>0),
 created_at timestamptz NOT NULL,
 CHECK((media_type='application/pdf' AND unit='PDF_PAGES' AND usage_kind='DOCUMENT_PDF') OR
       (media_type='application/vnd.openxmlformats-officedocument.wordprocessingml.document' AND unit='DOCX_EXTRACTED_CHARACTERS' AND usage_kind='DOCUMENT_DOCX'))
);

CREATE TABLE ai.document_measurement_entitlements(
 measurement_id uuid PRIMARY KEY REFERENCES ai.document_measurements(id) ON DELETE RESTRICT,
 checkpoint_id uuid NOT NULL UNIQUE REFERENCES bidora_entitlement.ai_usage_checkpoints(id) ON DELETE RESTRICT,
 linked_at timestamptz NOT NULL
);

CREATE FUNCTION ai.reject_document_measurement_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'document measurement evidence is immutable' USING ERRCODE='55000'; END $$;
CREATE TRIGGER document_measurements_immutable BEFORE UPDATE OR DELETE ON ai.document_measurements FOR EACH ROW EXECUTE FUNCTION ai.reject_document_measurement_change();
CREATE TRIGGER document_measurement_entitlements_immutable BEFORE UPDATE OR DELETE ON ai.document_measurement_entitlements FOR EACH ROW EXECUTE FUNCTION ai.reject_document_measurement_change();

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=29,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=28;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 28'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM ai.document_measurements) OR EXISTS(SELECT 1 FROM ai.document_measurement_entitlements) OR EXISTS(SELECT 1 FROM jobs.jobs WHERE state IN ('MEASURING','PENDING_RESERVATION','READY_FOR_PROVIDER')) THEN
  RAISE EXCEPTION 'cannot roll back retained document measurement evidence or active measurement jobs';
 END IF;
END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=28,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=29;
DROP TRIGGER document_measurement_entitlements_immutable ON ai.document_measurement_entitlements;
DROP TRIGGER document_measurements_immutable ON ai.document_measurements;
DROP FUNCTION ai.reject_document_measurement_change();
DROP TABLE ai.document_measurement_entitlements;
DROP TABLE ai.document_measurements;
ALTER TABLE jobs.jobs
 DROP CONSTRAINT jobs_state_check,
 DROP CONSTRAINT jobs_check,
 ADD CONSTRAINT jobs_state_check CHECK(state IN ('QUEUED','WAITING_FOR_CLEAN','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
 ADD CONSTRAINT jobs_check CHECK((state='WAITING_FOR_CLEAN' AND blocked_reason='DOCUMENT_NOT_CLEAN') OR (state<>'WAITING_FOR_CLEAN' AND blocked_reason IS NULL));
-- +goose StatementEnd
