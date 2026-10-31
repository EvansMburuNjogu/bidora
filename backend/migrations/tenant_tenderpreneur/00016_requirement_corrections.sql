-- +goose Up
-- +goose StatementBegin
-- Requirement corrections append immutable content history; source identities and satisfaction evidence are not rewritten.
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bid.requirement_versions (
 id uuid PRIMARY KEY,
 requirement_id uuid NOT NULL REFERENCES bid.requirements(id) ON DELETE RESTRICT,
 revision bigint NOT NULL CHECK(revision>0),
 source_version_id uuid NOT NULL REFERENCES bidora_opportunity.source_versions(id) ON DELETE RESTRICT,
 source_document_version_id uuid REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 source_location text NOT NULL CHECK(length(source_location) BETWEEN 1 AND 500 AND source_location ~ '\S'),
 text text NOT NULL CHECK(length(text) BETWEEN 1 AND 10000 AND text ~ '\S'),
 mandatory boolean NOT NULL,
 category text NOT NULL CHECK(length(category) BETWEEN 1 AND 100 AND category ~ '\S'),
 correction_reason text NOT NULL CHECK(length(correction_reason) BETWEEN 1 AND 2000 AND correction_reason ~ '\S'),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 UNIQUE(requirement_id,revision), UNIQUE(id,requirement_id)
);
ALTER TABLE bid.requirements ADD COLUMN current_content_revision_id uuid;
INSERT INTO bid.requirement_versions(id,requirement_id,revision,source_version_id,source_document_version_id,source_location,text,mandatory,category,correction_reason,created_by,created_at)
SELECT r.id,r.id,1,r.source_version_id,r.source_document_version_id,r.source_location,r.text,r.mandatory,r.category,'Initial requirement capture',r.created_by,r.created_at FROM bid.requirements r;
UPDATE bid.requirements SET current_content_revision_id=id;
ALTER TABLE bid.requirements ALTER COLUMN current_content_revision_id SET NOT NULL;
ALTER TABLE bid.requirements ADD CONSTRAINT requirements_current_content_revision_fk FOREIGN KEY(current_content_revision_id,id) REFERENCES bid.requirement_versions(id,requirement_id) DEFERRABLE INITIALLY DEFERRED;
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=16,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=15;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 15'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=15,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=16;
ALTER TABLE bid.requirements DROP CONSTRAINT requirements_current_content_revision_fk;
ALTER TABLE bid.requirements DROP COLUMN current_content_revision_id;
DROP TABLE bid.requirement_versions;
-- +goose StatementEnd
