-- +goose Up
-- +goose StatementBegin
-- Editable proposal metadata and immutable response snapshots. Export/approval are separate operations.
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bid.proposals (
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL UNIQUE REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 current_revision_id uuid,
 version bigint NOT NULL CHECK(version>0),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 updated_by uuid NOT NULL,
 updated_at timestamptz NOT NULL,
 CHECK(updated_at>=created_at)
);
CREATE TABLE bid.proposal_versions (
 id uuid PRIMARY KEY,
 proposal_id uuid NOT NULL REFERENCES bid.proposals(id) ON DELETE RESTRICT,
 revision bigint NOT NULL CHECK(revision>0),
 workspace_version_at_creation bigint NOT NULL CHECK(workspace_version_at_creation>0),
 organization_legal_name text NOT NULL CHECK(length(organization_legal_name) BETWEEN 1 AND 300 AND organization_legal_name ~ '\S'),
 title text NOT NULL CHECK(length(title) BETWEEN 1 AND 300 AND title ~ '\S'),
 recipient text NOT NULL CHECK(length(recipient) BETWEEN 1 AND 1000 AND recipient ~ '\S'),
 letterhead_lines jsonb NOT NULL CHECK(jsonb_typeof(letterhead_lines)='array' AND jsonb_array_length(letterhead_lines) BETWEEN 0 AND 10 AND length(letterhead_lines::text)<=5000),
 introduction text NOT NULL CHECK(length(introduction) BETWEEN 1 AND 10000 AND introduction ~ '\S'),
 closing text NOT NULL CHECK(length(closing) BETWEEN 1 AND 10000 AND closing ~ '\S'),
 change_note text NOT NULL CHECK(length(change_note) BETWEEN 1 AND 2000 AND change_note ~ '\S'),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 UNIQUE(proposal_id,revision), UNIQUE(id,proposal_id)
);
ALTER TABLE bid.proposals ADD CONSTRAINT proposals_current_revision_fk FOREIGN KEY(current_revision_id,id) REFERENCES bid.proposal_versions(id,proposal_id) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE bid.proposal_response_versions (
 proposal_version_id uuid NOT NULL REFERENCES bid.proposal_versions(id) ON DELETE RESTRICT,
 requirement_id uuid NOT NULL REFERENCES bid.requirements(id) ON DELETE RESTRICT,
 response_version_id uuid NOT NULL REFERENCES bid.response_versions(id) ON DELETE RESTRICT,
 source_document_version_id uuid REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 requirement_text text NOT NULL CHECK(length(requirement_text) BETWEEN 1 AND 10000 AND requirement_text ~ '\S'),
 requirement_category text NOT NULL CHECK(length(requirement_category) BETWEEN 1 AND 100 AND requirement_category ~ '\S'),
 requirement_mandatory boolean NOT NULL,
 ordinal integer NOT NULL CHECK(ordinal BETWEEN 1 AND 1000),
 PRIMARY KEY(proposal_version_id,requirement_id), UNIQUE(proposal_version_id,ordinal)
);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=12,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=11;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 11'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=11,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=12;
ALTER TABLE bid.proposals DROP CONSTRAINT proposals_current_revision_fk;
DROP TABLE bid.proposal_response_versions;
DROP TABLE bid.proposal_versions;
DROP TABLE bid.proposals;
-- +goose StatementEnd
