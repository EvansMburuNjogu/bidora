-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM jobs.jobs WHERE state='MEASURING') THEN RAISE EXCEPTION 'cannot add fenced measurement leases while unfenced measurement jobs are active'; END IF; END $$;

ALTER TABLE jobs.jobs
 ADD COLUMN measurement_attempts integer NOT NULL DEFAULT 0 CHECK(measurement_attempts BETWEEN 0 AND 3),
 ADD COLUMN measurement_lease_id uuid,
 ADD COLUMN measurement_lease_expires_at timestamptz,
 ADD COLUMN measurement_next_attempt_at timestamptz,
 ADD COLUMN measurement_failure_code text CHECK(measurement_failure_code IS NULL OR measurement_failure_code IN(
  'AUTHORIZATION_DENIED','INPUT_HASH_MISMATCH','INPUT_SIZE_MISMATCH','UNSUPPORTED_OR_MISMATCHED_FORMAT','ENCRYPTED_DOCUMENT','CORRUPT_DOCUMENT',
  'PAGE_LIMIT_EXCEEDED','CHARACTER_LIMIT_EXCEEDED','ARCHIVE_LIMIT_EXCEEDED','XML_LIMIT_EXCEEDED','RESOURCE_LIMIT_EXCEEDED','PARSER_TIMEOUT',
  'MEASUREMENT_INPUT_INVALID','MEASUREMENT_RESULT_MISMATCH','MEASUREMENT_UNKNOWN_FAILURE','MEASUREMENT_RETRY_EXHAUSTED'
 )),
 ADD CONSTRAINT jobs_measurement_lease_shape CHECK(
  (kind='TENDER_ANALYSIS' AND state='MEASURING' AND measurement_lease_id IS NOT NULL AND measurement_lease_expires_at IS NOT NULL AND started_at IS NOT NULL AND measurement_next_attempt_at IS NULL AND measurement_failure_code IS NULL)
  OR (state<>'MEASURING' AND measurement_lease_id IS NULL AND measurement_lease_expires_at IS NULL)
 ),
 ADD CONSTRAINT jobs_measurement_retry_shape CHECK(measurement_next_attempt_at IS NULL OR (kind='TENDER_ANALYSIS' AND state='QUEUED' AND measurement_failure_code IS NULL)),
 ADD CONSTRAINT jobs_measurement_failure_shape CHECK(measurement_failure_code IS NULL OR (kind='TENDER_ANALYSIS' AND state IN('FAILED','CANCELLED') AND completed_at IS NOT NULL));

CREATE INDEX jobs_measurement_retry_idx ON jobs.jobs(measurement_next_attempt_at,created_at,id)
 WHERE kind='TENDER_ANALYSIS' AND state='QUEUED';
CREATE INDEX jobs_measurement_expiry_idx ON jobs.jobs(measurement_lease_expires_at,id)
 WHERE kind='TENDER_ANALYSIS' AND state='MEASURING';

CREATE TABLE ai.document_measurement_attempts(
 lease_id uuid PRIMARY KEY,
 job_id uuid NOT NULL REFERENCES jobs.jobs(id) ON DELETE RESTRICT,
 document_version_id uuid NOT NULL REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 attempt_number integer NOT NULL CHECK(attempt_number BETWEEN 1 AND 3),
 parser_key text NOT NULL CHECK(length(parser_key) BETWEEN 1 AND 100),
 parser_version text NOT NULL CHECK(length(parser_version) BETWEEN 1 AND 100),
 semantics_version text NOT NULL CHECK(length(semantics_version) BETWEEN 1 AND 100),
 profile_version text NOT NULL CHECK(length(profile_version) BETWEEN 1 AND 100),
 claimed_at timestamptz NOT NULL,
 lease_expires_at timestamptz NOT NULL CHECK(lease_expires_at>claimed_at),
 UNIQUE(job_id,attempt_number)
);
CREATE TABLE ai.document_measurement_attempt_outcomes(
 id uuid PRIMARY KEY,
 lease_id uuid NOT NULL UNIQUE REFERENCES ai.document_measurement_attempts(lease_id) ON DELETE RESTRICT,
 outcome text NOT NULL CHECK(outcome IN('SUCCEEDED','TRANSIENT_FAILURE','TERMINAL_FAILURE','LEASE_EXPIRED')),
 failure_code text CHECK(failure_code IS NULL OR length(failure_code) BETWEEN 1 AND 100),
 occurred_at timestamptz NOT NULL,
 next_attempt_at timestamptz,
 CHECK((outcome='SUCCEEDED' AND failure_code IS NULL AND next_attempt_at IS NULL) OR
       (outcome='TRANSIENT_FAILURE' AND failure_code IS NOT NULL AND next_attempt_at IS NOT NULL) OR
       (outcome='TERMINAL_FAILURE' AND failure_code IS NOT NULL AND next_attempt_at IS NULL) OR
       (outcome='LEASE_EXPIRED' AND failure_code='MEASUREMENT_WORKER_INTERRUPTED')),
 CHECK(next_attempt_at IS NULL OR next_attempt_at>occurred_at)
);
CREATE FUNCTION ai.reject_document_measurement_attempt_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'document measurement attempt evidence is immutable' USING ERRCODE='55000'; END $$;
CREATE TRIGGER document_measurement_attempts_immutable BEFORE UPDATE OR DELETE ON ai.document_measurement_attempts FOR EACH ROW EXECUTE FUNCTION ai.reject_document_measurement_attempt_change();
CREATE TRIGGER document_measurement_attempt_outcomes_immutable BEFORE UPDATE OR DELETE ON ai.document_measurement_attempt_outcomes FOR EACH ROW EXECUTE FUNCTION ai.reject_document_measurement_attempt_change();

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=30,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=29;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 29'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM ai.document_measurement_attempts) OR EXISTS(SELECT 1 FROM ai.document_measurement_attempt_outcomes) OR EXISTS(SELECT 1 FROM jobs.jobs WHERE measurement_lease_id IS NOT NULL OR measurement_next_attempt_at IS NOT NULL OR measurement_failure_code IS NOT NULL OR measurement_attempts<>0) THEN
  RAISE EXCEPTION 'cannot roll back retained document measurement lease or attempt evidence';
 END IF;
END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=29,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=30;
DROP TRIGGER document_measurement_attempt_outcomes_immutable ON ai.document_measurement_attempt_outcomes;
DROP TRIGGER document_measurement_attempts_immutable ON ai.document_measurement_attempts;
DROP FUNCTION ai.reject_document_measurement_attempt_change();
DROP TABLE ai.document_measurement_attempt_outcomes;
DROP TABLE ai.document_measurement_attempts;
DROP INDEX jobs.jobs_measurement_expiry_idx;
DROP INDEX jobs.jobs_measurement_retry_idx;
ALTER TABLE jobs.jobs DROP CONSTRAINT jobs_measurement_failure_shape,DROP CONSTRAINT jobs_measurement_retry_shape,DROP CONSTRAINT jobs_measurement_lease_shape,
 DROP COLUMN measurement_failure_code,DROP COLUMN measurement_next_attempt_at,DROP COLUMN measurement_lease_expires_at,DROP COLUMN measurement_lease_id,DROP COLUMN measurement_attempts;
-- +goose StatementEnd
