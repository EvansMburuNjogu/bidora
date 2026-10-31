-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE bid.proposals ADD CONSTRAINT proposals_id_bid_unique UNIQUE(id,bid_id);
CREATE TABLE bid.internal_review_decisions (
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL,
 proposal_id uuid NOT NULL,
 proposal_version_id uuid NOT NULL,
 FOREIGN KEY(proposal_version_id,proposal_id) REFERENCES bid.proposal_versions(id,proposal_id) ON DELETE RESTRICT,
 FOREIGN KEY(proposal_id,bid_id) REFERENCES bid.proposals(id,bid_id) ON DELETE RESTRICT,
 reviewer_id uuid NOT NULL,
 reviewer_sequence integer NOT NULL CHECK(reviewer_sequence BETWEEN 1 AND 100),
 decision text NOT NULL CHECK(decision IN ('REVIEW_COMPLETE','CHANGES_REQUESTED')),
 rationale text NOT NULL CHECK(length(rationale) BETWEEN 1 AND 5000 AND rationale ~ '\S'),
 decided_at timestamptz NOT NULL,
 UNIQUE(proposal_version_id,reviewer_id,reviewer_sequence)
);
CREATE INDEX internal_review_decisions_version_idx ON bid.internal_review_decisions(proposal_version_id,reviewer_id,reviewer_sequence DESC);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=19,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=18;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 18'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=18,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=19;
DROP TABLE bid.internal_review_decisions;
ALTER TABLE bid.proposals DROP CONSTRAINT proposals_id_bid_unique;
-- +goose StatementEnd
