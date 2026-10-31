-- +goose Up
-- +goose StatementBegin
-- Technical responses are plain UTF-8 text with immutable revisions. Financial
-- content and approval candidates are deliberately outside this migration.
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bid.response_sections (
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 requirement_id uuid NOT NULL REFERENCES bid.requirements(id) ON DELETE RESTRICT,
 current_revision_id uuid,
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 UNIQUE(bid_id,requirement_id),
 CHECK(updated_at>=created_at)
);
CREATE TABLE bid.response_versions (
 id uuid PRIMARY KEY,
 section_id uuid NOT NULL REFERENCES bid.response_sections(id) ON DELETE RESTRICT,
 revision integer NOT NULL CHECK(revision>0),
 requirement_version_at_creation bigint NOT NULL CHECK(requirement_version_at_creation>0),
 content text NOT NULL CHECK(length(content) BETWEEN 1 AND 20000 AND content ~ '\S'),
 change_note text NOT NULL CHECK(length(change_note) BETWEEN 1 AND 2000 AND change_note ~ '\S'),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 UNIQUE(section_id,revision)
);
ALTER TABLE bid.response_sections ADD CONSTRAINT response_current_revision_fk FOREIGN KEY(current_revision_id) REFERENCES bid.response_versions(id) ON DELETE RESTRICT;
CREATE TABLE bid.response_evidence (
 response_version_id uuid NOT NULL REFERENCES bid.response_versions(id) ON DELETE RESTRICT,
 requirement_evidence_id uuid NOT NULL REFERENCES bid.requirement_evidence(id) ON DELETE RESTRICT,
 PRIMARY KEY(response_version_id,requirement_evidence_id)
);
CREATE INDEX response_versions_section_created_idx ON bid.response_versions(section_id,revision DESC);

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=10,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=9;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 9'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=9,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=10;
DROP TABLE bid.response_evidence;
ALTER TABLE bid.response_sections DROP CONSTRAINT response_current_revision_fk;
DROP TABLE bid.response_versions;
DROP TABLE bid.response_sections;
-- +goose StatementEnd
