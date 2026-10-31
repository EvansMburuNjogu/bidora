-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE bid.proposal_package_manifests ADD CONSTRAINT proposal_package_manifests_id_bid_unique UNIQUE(id,bid_id);
CREATE TABLE bid.external_action_evidence (
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 package_manifest_id uuid NOT NULL,
 report_type text NOT NULL CHECK(report_type='EXTERNAL_ACTION_REPORTED'),
 verification_status text NOT NULL CHECK(verification_status='REPORTED_UNVERIFIED'),
 external_system_name text NOT NULL CHECK(length(external_system_name) BETWEEN 1 AND 200 AND external_system_name ~ '\S'),
 external_reference text CHECK(external_reference IS NULL OR (length(external_reference) BETWEEN 1 AND 500 AND external_reference ~ '\S')),
 reported_action_time_text text NOT NULL CHECK(length(reported_action_time_text) BETWEEN 1 AND 100 AND reported_action_time_text ~ '\S'),
 reported_action_at timestamptz NOT NULL,
 report_note text NOT NULL CHECK(length(report_note) BETWEEN 1 AND 5000 AND report_note ~ '\S'),
 supersedes_evidence_id uuid UNIQUE REFERENCES bid.external_action_evidence(id) ON DELETE RESTRICT,
 recorded_by uuid NOT NULL,
 recorded_at timestamptz NOT NULL,
 FOREIGN KEY(package_manifest_id,bid_id) REFERENCES bid.proposal_package_manifests(id,bid_id) ON DELETE RESTRICT,
 CHECK(supersedes_evidence_id IS NULL OR supersedes_evidence_id<>id)
);
CREATE INDEX external_action_evidence_bid_recorded_idx ON bid.external_action_evidence(bid_id,recorded_at DESC,id DESC);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=21,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=20;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 20'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=20,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=21;
DROP TABLE bid.external_action_evidence;
ALTER TABLE bid.proposal_package_manifests DROP CONSTRAINT proposal_package_manifests_id_bid_unique;
-- +goose StatementEnd
