-- +goose Up
-- +goose StatementBegin
-- Approved MVP upload policy: PDF/DOCX, 25 MiB, immutable MinIO versions,
-- fail-closed quarantine, and analysis blocked until CLEAN.
SET lock_timeout='1s';
SET statement_timeout='5s';
CREATE SCHEMA IF NOT EXISTS documents;
CREATE SCHEMA IF NOT EXISTS jobs;

CREATE TABLE documents.documents (
 id uuid PRIMARY KEY,
 opportunity_id uuid NOT NULL REFERENCES bidora_opportunity.opportunities(id) ON DELETE RESTRICT,
 title text NOT NULL CHECK(length(title) BETWEEN 1 AND 255 AND title ~ '\S'),
 state text NOT NULL CHECK(state IN ('UPLOADING','ACTIVE','ARCHIVED')),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 version bigint NOT NULL DEFAULT 1 CHECK(version>0)
);
CREATE INDEX documents_opportunity_created_idx ON documents.documents(opportunity_id,created_at,id);

CREATE TABLE documents.upload_sessions (
 id uuid PRIMARY KEY,
 document_id uuid NOT NULL REFERENCES documents.documents(id) ON DELETE RESTRICT,
 opportunity_id uuid NOT NULL REFERENCES bidora_opportunity.opportunities(id) ON DELETE RESTRICT,
 object_key text NOT NULL UNIQUE CHECK(length(object_key) BETWEEN 1 AND 1000),
 expected_media_type text NOT NULL CHECK(expected_media_type IN ('application/pdf','application/vnd.openxmlformats-officedocument.wordprocessingml.document')),
 expected_size_bytes bigint NOT NULL CHECK(expected_size_bytes BETWEEN 1 AND 26214400),
 expected_sha256 bytea NOT NULL CHECK(octet_length(expected_sha256)=32),
 state text NOT NULL CHECK(state IN ('INITIATED','COMPLETED','EXPIRED')),
 expires_at timestamptz NOT NULL,
 completed_version_id uuid,
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 completed_at timestamptz,
 CHECK(expires_at>created_at),
 CHECK((state='COMPLETED')=(completed_version_id IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE INDEX upload_sessions_expiry_idx ON documents.upload_sessions(state,expires_at,id);

CREATE TABLE documents.document_versions (
 id uuid PRIMARY KEY,
 document_id uuid NOT NULL REFERENCES documents.documents(id) ON DELETE RESTRICT,
 revision integer NOT NULL CHECK(revision>0),
 media_type text NOT NULL CHECK(media_type IN ('application/pdf','application/vnd.openxmlformats-officedocument.wordprocessingml.document')),
 size_bytes bigint NOT NULL CHECK(size_bytes BETWEEN 1 AND 26214400),
 sha256 bytea NOT NULL CHECK(octet_length(sha256)=32),
 object_key text NOT NULL,
 object_version text NOT NULL CHECK(length(object_version) BETWEEN 1 AND 1024),
 scan_state text NOT NULL CHECK(scan_state IN ('QUARANTINED','CLEAN','REJECTED','FAILED')),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 UNIQUE(document_id,revision), UNIQUE(object_key,object_version)
);
ALTER TABLE documents.upload_sessions ADD CONSTRAINT upload_completed_version_fk FOREIGN KEY(completed_version_id) REFERENCES documents.document_versions(id) ON DELETE RESTRICT;

CREATE TABLE jobs.jobs (
 id uuid PRIMARY KEY,
 kind text NOT NULL CONSTRAINT jobs_kind_check CHECK(kind IN ('MALWARE_SCAN','TENDER_ANALYSIS')),
 document_version_id uuid NOT NULL REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 state text NOT NULL CHECK(state IN ('QUEUED','WAITING_FOR_CLEAN','RUNNING','SUCCEEDED','FAILED','CANCELLED')),
 blocked_reason text,
 requested_by uuid NOT NULL,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 created_at timestamptz NOT NULL,
 started_at timestamptz,
 completed_at timestamptz,
 CHECK((state='WAITING_FOR_CLEAN' AND blocked_reason='DOCUMENT_NOT_CLEAN') OR (state<>'WAITING_FOR_CLEAN' AND blocked_reason IS NULL)),
 UNIQUE(kind,document_version_id)
);
CREATE INDEX jobs_claim_idx ON jobs.jobs(kind,state,created_at,id);

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=6,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='tenant_tenderpreneur' AND schema_version=5;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 5'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=5,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=6;
DROP TABLE jobs.jobs;
DROP TABLE documents.upload_sessions;
DROP TABLE documents.document_versions;
DROP TABLE documents.documents;
DROP SCHEMA jobs;
DROP SCHEMA documents;
-- +goose StatementEnd
