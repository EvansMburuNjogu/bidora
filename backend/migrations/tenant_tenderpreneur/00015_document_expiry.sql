-- +goose Up
-- +goose StatementBegin
-- Expiry is explicit human-supplied evidence metadata; it is never inferred by AI.
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE documents.upload_sessions ADD COLUMN evidence_expires_at timestamptz CHECK(evidence_expires_at IS NULL OR evidence_expires_at>created_at);
ALTER TABLE documents.document_versions ADD COLUMN expires_at timestamptz CHECK(expires_at IS NULL OR expires_at>created_at);
CREATE INDEX document_versions_expiry_idx ON documents.document_versions(expires_at,id) WHERE expires_at IS NOT NULL;
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=15,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=14;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 14'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=14,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=15;
DROP INDEX documents.document_versions_expiry_idx;
ALTER TABLE documents.document_versions DROP COLUMN expires_at;
ALTER TABLE documents.upload_sessions DROP COLUMN evidence_expires_at;
-- +goose StatementEnd
