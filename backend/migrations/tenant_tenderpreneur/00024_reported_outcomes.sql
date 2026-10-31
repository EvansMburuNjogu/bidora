-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE documents.documents DROP CONSTRAINT documents_document_kind_check;
ALTER TABLE documents.documents ADD CONSTRAINT documents_document_kind_check CHECK(document_kind IN ('TENDER_SOURCE','BID_EVIDENCE','SUBMISSION_PROOF','OUTCOME_PROOF'));
ALTER TABLE documents.documents DROP CONSTRAINT document_owner_shape;
ALTER TABLE documents.documents ADD CONSTRAINT document_owner_shape CHECK(
 (document_kind='TENDER_SOURCE' AND opportunity_id IS NOT NULL AND bid_id IS NULL AND external_action_evidence_id IS NULL) OR
 (document_kind='BID_EVIDENCE' AND opportunity_id IS NULL AND bid_id IS NOT NULL AND external_action_evidence_id IS NULL) OR
 (document_kind='SUBMISSION_PROOF' AND opportunity_id IS NULL AND bid_id IS NOT NULL AND external_action_evidence_id IS NOT NULL) OR
 (document_kind='OUTCOME_PROOF' AND opportunity_id IS NULL AND bid_id IS NOT NULL AND external_action_evidence_id IS NULL));
CREATE TABLE bid.reported_outcomes(
 id uuid PRIMARY KEY,bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 sequence integer NOT NULL CHECK(sequence BETWEEN 1 AND 100),
 outcome text NOT NULL CHECK(outcome IN ('PENDING','UNKNOWN','WON','LOST','DISQUALIFIED','CANCELLED','WITHDRAWN')),
 verification_status text NOT NULL CHECK(verification_status='REPORTED_UNVERIFIED'),
 provenance text NOT NULL CHECK(provenance='HUMAN_REPORTED_EXTERNAL_OUTCOME'),
 source_class text NOT NULL CHECK(source_class IN ('BUYER_NOTICE','EXTERNAL_PORTAL','EMAIL','INTERNAL_OBSERVATION','OTHER')),
 reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 5000 AND reason ~ '\S'),
 lessons text CHECK(lessons IS NULL OR (length(lessons) BETWEEN 1 AND 5000 AND lessons ~ '\S')),
 reported_outcome_time_text text NOT NULL CHECK(length(reported_outcome_time_text) BETWEEN 1 AND 100),
 reported_outcome_at timestamptz NOT NULL,recorded_at timestamptz NOT NULL,reported_by uuid NOT NULL,
 proof_document_version_id uuid REFERENCES documents.document_versions(id) ON DELETE RESTRICT,
 supersedes_outcome_id uuid UNIQUE REFERENCES bid.reported_outcomes(id) ON DELETE RESTRICT,
 correction_reason text CHECK(correction_reason IS NULL OR (length(correction_reason) BETWEEN 1 AND 5000 AND correction_reason ~ '\S')),
 UNIQUE(bid_id,sequence),CHECK((sequence=1 AND supersedes_outcome_id IS NULL AND correction_reason IS NULL) OR (sequence>1 AND supersedes_outcome_id IS NOT NULL AND correction_reason IS NOT NULL)),
 CHECK(outcome IN ('PENDING','UNKNOWN') OR proof_document_version_id IS NOT NULL)
);
CREATE INDEX reported_outcomes_bid_recorded_idx ON bid.reported_outcomes(bid_id,recorded_at DESC,id DESC);
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_meta.schema_compatibility SET schema_version=24,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=23; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 23'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=23,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=24;
DROP TABLE bid.reported_outcomes;
ALTER TABLE documents.documents DROP CONSTRAINT document_owner_shape, DROP CONSTRAINT documents_document_kind_check;
ALTER TABLE documents.documents ADD CONSTRAINT documents_document_kind_check CHECK(document_kind IN ('TENDER_SOURCE','BID_EVIDENCE','SUBMISSION_PROOF'));
ALTER TABLE documents.documents ADD CONSTRAINT document_owner_shape CHECK((document_kind='TENDER_SOURCE' AND opportunity_id IS NOT NULL AND bid_id IS NULL AND external_action_evidence_id IS NULL) OR (document_kind='BID_EVIDENCE' AND opportunity_id IS NULL AND bid_id IS NOT NULL AND external_action_evidence_id IS NULL) OR (document_kind='SUBMISSION_PROOF' AND opportunity_id IS NULL AND bid_id IS NOT NULL AND external_action_evidence_id IS NOT NULL));
-- +goose StatementEnd
