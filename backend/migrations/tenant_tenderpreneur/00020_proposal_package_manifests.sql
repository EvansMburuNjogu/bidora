-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bid.proposal_package_manifests (
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL,
 proposal_id uuid NOT NULL,
 proposal_version_id uuid NOT NULL,
 proposal_revision bigint NOT NULL CHECK(proposal_revision>0),
 package_status text NOT NULL CHECK(package_status='UNAPPROVED_PREPARATION_ONLY'),
 pdf_render_profile text NOT NULL CHECK(pdf_render_profile='BIDORA_PDF_V1'),
 pdf_sha256 bytea NOT NULL CHECK(octet_length(pdf_sha256)=32),
 pdf_size bigint NOT NULL CHECK(pdf_size BETWEEN 1 AND 8388608),
 docx_render_profile text NOT NULL CHECK(docx_render_profile='BIDORA_DOCX_V1'),
 docx_sha256 bytea NOT NULL CHECK(octet_length(docx_sha256)=32),
 docx_size bigint NOT NULL CHECK(docx_size BETWEEN 1 AND 8388608),
 readiness_observed_at timestamptz NOT NULL,
 readiness_ready boolean NOT NULL,
 readiness_blockers jsonb NOT NULL CHECK(jsonb_typeof(readiness_blockers)='array' AND jsonb_array_length(readiness_blockers)<=100 AND length(readiness_blockers::text)<=10000),
 manifest_sha256 bytea NOT NULL CHECK(octet_length(manifest_sha256)=32),
 manifest_version integer NOT NULL CHECK(manifest_version=1),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 FOREIGN KEY(proposal_version_id,proposal_id) REFERENCES bid.proposal_versions(id,proposal_id) ON DELETE RESTRICT,
 FOREIGN KEY(proposal_id,bid_id) REFERENCES bid.proposals(id,bid_id) ON DELETE RESTRICT
);
CREATE INDEX proposal_package_manifests_bid_created_idx ON bid.proposal_package_manifests(bid_id,created_at DESC,id DESC);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=20,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=19;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 19'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=19,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=20;
DROP TABLE bid.proposal_package_manifests;
-- +goose StatementEnd
