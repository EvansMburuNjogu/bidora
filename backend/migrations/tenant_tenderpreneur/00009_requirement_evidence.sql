-- +goose Up
-- +goose StatementBegin
-- Bid evidence uses the approved private PDF/DOCX upload policy. A requirement
-- can become SATISFIED only with an exact CLEAN immutable evidence version.
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE documents.documents
 ADD COLUMN document_kind text NOT NULL DEFAULT 'TENDER_SOURCE' CHECK(document_kind IN ('TENDER_SOURCE','BID_EVIDENCE')),
 ADD COLUMN bid_id uuid REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 ALTER COLUMN opportunity_id DROP NOT NULL,
 ADD CONSTRAINT document_owner_shape CHECK((document_kind='TENDER_SOURCE' AND opportunity_id IS NOT NULL AND bid_id IS NULL) OR (document_kind='BID_EVIDENCE' AND opportunity_id IS NULL AND bid_id IS NOT NULL));
CREATE INDEX documents_bid_created_idx ON documents.documents(bid_id,created_at,id) WHERE bid_id IS NOT NULL;

ALTER TABLE documents.upload_sessions
 ADD COLUMN bid_id uuid REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 ALTER COLUMN opportunity_id DROP NOT NULL,
 ADD CONSTRAINT upload_owner_shape CHECK((opportunity_id IS NOT NULL AND bid_id IS NULL) OR (opportunity_id IS NULL AND bid_id IS NOT NULL));

CREATE TABLE bid.requirement_evidence (
 id uuid PRIMARY KEY,
 requirement_id uuid NOT NULL REFERENCES bid.requirements(id) ON DELETE RESTRICT,
 bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 document_version_id uuid NOT NULL REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 evidence_location text NOT NULL CHECK(length(evidence_location) BETWEEN 1 AND 500 AND evidence_location ~ '\S'),
 justification text NOT NULL CHECK(length(justification) BETWEEN 1 AND 2000 AND justification ~ '\S'),
 added_by uuid NOT NULL,
 added_at timestamptz NOT NULL,
 UNIQUE(requirement_id,document_version_id)
);
CREATE INDEX requirement_evidence_requirement_idx ON bid.requirement_evidence(requirement_id,added_at,id);

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=9,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=8;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 8'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=8,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=9;
DROP TABLE bid.requirement_evidence;
ALTER TABLE documents.upload_sessions DROP CONSTRAINT upload_owner_shape, ALTER COLUMN opportunity_id SET NOT NULL, DROP COLUMN bid_id;
DROP INDEX documents.documents_bid_created_idx;
ALTER TABLE documents.documents DROP CONSTRAINT document_owner_shape, ALTER COLUMN opportunity_id SET NOT NULL, DROP COLUMN bid_id, DROP COLUMN document_kind;
-- +goose StatementEnd
