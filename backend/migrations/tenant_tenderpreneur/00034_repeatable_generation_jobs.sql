-- +goose Up
-- +goose StatementBegin
-- Malware scanning and tender extraction are singleton effects per immutable
-- version. Human-requested advisory drafts are repeatable immutable runs.
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE jobs.jobs DROP CONSTRAINT jobs_kind_document_version_id_key;
CREATE UNIQUE INDEX jobs_singleton_document_kind_uidx ON jobs.jobs(kind,document_version_id)
 WHERE kind IN ('MALWARE_SCAN','TENDER_ANALYSIS');
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=34,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=33;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 33'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM jobs.jobs WHERE kind IN ('RESPONSE_DRAFT','PROPOSAL_DRAFT') GROUP BY kind,document_version_id HAVING count(*)>1) THEN
  RAISE EXCEPTION 'cannot roll back retained repeated generation jobs';
 END IF;
END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=33,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=34;
DROP INDEX jobs.jobs_singleton_document_kind_uidx;
ALTER TABLE jobs.jobs ADD CONSTRAINT jobs_kind_document_version_id_key UNIQUE(kind,document_version_id);
-- +goose StatementEnd
