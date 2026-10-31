-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE bid.workspaces DROP CONSTRAINT workspaces_state_check;
ALTER TABLE bid.workspaces ADD CONSTRAINT workspaces_state_check CHECK(state IN ('DECISION_PENDING','BID','NO_BID','SUBMITTED'));
ALTER TABLE documents.documents DROP CONSTRAINT document_owner_shape;
ALTER TABLE documents.documents ADD COLUMN external_action_evidence_id uuid REFERENCES bid.external_action_evidence(id) ON DELETE RESTRICT;
ALTER TABLE documents.documents DROP CONSTRAINT documents_document_kind_check;
ALTER TABLE documents.documents ADD CONSTRAINT documents_document_kind_check CHECK(document_kind IN ('TENDER_SOURCE','BID_EVIDENCE','SUBMISSION_PROOF'));
CREATE UNIQUE INDEX documents_submission_proof_evidence_unique ON documents.documents(external_action_evidence_id) WHERE document_kind='SUBMISSION_PROOF';
ALTER TABLE documents.documents ADD CONSTRAINT document_owner_shape CHECK(
 (document_kind='TENDER_SOURCE' AND opportunity_id IS NOT NULL AND bid_id IS NULL AND external_action_evidence_id IS NULL) OR
 (document_kind='BID_EVIDENCE' AND opportunity_id IS NULL AND bid_id IS NOT NULL AND external_action_evidence_id IS NULL) OR
 (document_kind='SUBMISSION_PROOF' AND opportunity_id IS NULL AND bid_id IS NOT NULL AND external_action_evidence_id IS NOT NULL));
ALTER TABLE documents.upload_sessions DROP CONSTRAINT upload_owner_shape;
ALTER TABLE documents.upload_sessions ADD COLUMN external_action_evidence_id uuid REFERENCES bid.external_action_evidence(id) ON DELETE RESTRICT;
ALTER TABLE documents.upload_sessions ADD CONSTRAINT upload_owner_shape CHECK(
 (opportunity_id IS NOT NULL AND bid_id IS NULL AND external_action_evidence_id IS NULL) OR
 (opportunity_id IS NULL AND bid_id IS NOT NULL));
CREATE TABLE bid.submission_confirmations(
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL UNIQUE REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 external_action_evidence_id uuid NOT NULL UNIQUE REFERENCES bid.external_action_evidence(id) ON DELETE RESTRICT,
 package_manifest_id uuid NOT NULL REFERENCES bid.proposal_package_manifests(id) ON DELETE RESTRICT,
 proposal_approval_id uuid NOT NULL REFERENCES bid.proposal_approvals(id) ON DELETE RESTRICT,
 proof_document_version_id uuid NOT NULL UNIQUE REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 reporter_id uuid NOT NULL,
 verifier_id uuid NOT NULL,
 reported_action_at timestamptz NOT NULL,
 report_recorded_at timestamptz NOT NULL,
 confirmed_at timestamptz NOT NULL,
 rationale text NOT NULL CHECK(length(rationale) BETWEEN 1 AND 5000 AND rationale ~ '\S'),
 submission_basis text NOT NULL CHECK(submission_basis='HUMAN_CONFIRMED_EXTERNAL_ACTION'),
 policy_version integer NOT NULL CHECK(policy_version=1),
 workspace_version bigint NOT NULL CHECK(workspace_version>0),
 CHECK(reporter_id<>verifier_id)
);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=23,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=22;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 22'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=22,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=23;
DROP TABLE bid.submission_confirmations;
ALTER TABLE documents.upload_sessions DROP CONSTRAINT upload_owner_shape, DROP COLUMN external_action_evidence_id;
ALTER TABLE documents.upload_sessions ADD CONSTRAINT upload_owner_shape CHECK((opportunity_id IS NOT NULL AND bid_id IS NULL) OR (opportunity_id IS NULL AND bid_id IS NOT NULL));
DROP INDEX documents.documents_submission_proof_evidence_unique;
ALTER TABLE documents.documents DROP CONSTRAINT document_owner_shape, DROP CONSTRAINT documents_document_kind_check, DROP COLUMN external_action_evidence_id;
ALTER TABLE documents.documents ADD CONSTRAINT documents_document_kind_check CHECK(document_kind IN ('TENDER_SOURCE','BID_EVIDENCE'));
ALTER TABLE documents.documents ADD CONSTRAINT document_owner_shape CHECK((document_kind='TENDER_SOURCE' AND opportunity_id IS NOT NULL AND bid_id IS NULL) OR (document_kind='BID_EVIDENCE' AND opportunity_id IS NULL AND bid_id IS NOT NULL));
ALTER TABLE bid.workspaces DROP CONSTRAINT workspaces_state_check;
ALTER TABLE bid.workspaces ADD CONSTRAINT workspaces_state_check CHECK(state IN ('DECISION_PENDING','BID','NO_BID'));
-- +goose StatementEnd
