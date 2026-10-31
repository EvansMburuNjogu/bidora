-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bid.proposal_approvals (
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL,
 proposal_id uuid NOT NULL,
 proposal_version_id uuid NOT NULL,
 package_manifest_id uuid NOT NULL UNIQUE REFERENCES bid.proposal_package_manifests(id) ON DELETE RESTRICT,
 review_decision_id uuid NOT NULL REFERENCES bid.internal_review_decisions(id) ON DELETE RESTRICT,
 approver_id uuid NOT NULL,
 policy_version integer NOT NULL CHECK(policy_version=1),
 rationale text CHECK(rationale IS NULL OR (length(rationale) BETWEEN 1 AND 5000 AND rationale ~ '\S')),
 approved_at timestamptz NOT NULL,
 FOREIGN KEY(proposal_version_id,proposal_id) REFERENCES bid.proposal_versions(id,proposal_id) ON DELETE RESTRICT,
 FOREIGN KEY(proposal_id,bid_id) REFERENCES bid.proposals(id,bid_id) ON DELETE RESTRICT
);
CREATE INDEX proposal_approvals_bid_approved_idx ON bid.proposal_approvals(bid_id,approved_at DESC,id DESC);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=22,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=21;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 21'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=21,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=22;
DROP TABLE bid.proposal_approvals;
-- +goose StatementEnd
