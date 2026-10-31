-- +goose Up
-- +goose StatementBegin
-- Human application preserves AI output and creates a separate authoritative proposal revision.
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE ai.proposal_draft_suggestions DROP CONSTRAINT proposal_draft_suggestions_review_state_check;
ALTER TABLE ai.proposal_draft_suggestions
 ADD COLUMN reviewed_by uuid,
 ADD COLUMN reviewed_at timestamptz,
 ADD COLUMN applied_proposal_version_id uuid REFERENCES bid.proposal_versions(id) ON DELETE RESTRICT,
 ADD COLUMN review_note text CHECK(review_note IS NULL OR (length(review_note) BETWEEN 1 AND 2000 AND review_note ~ '\S'));
ALTER TABLE ai.proposal_draft_suggestions
 ADD CONSTRAINT proposal_draft_suggestions_review_state_check CHECK(review_state IN ('PROPOSED_UNVERIFIED','APPLIED')),
 ADD CONSTRAINT proposal_draft_review_consistency CHECK((review_state='PROPOSED_UNVERIFIED' AND reviewed_by IS NULL AND reviewed_at IS NULL AND applied_proposal_version_id IS NULL AND review_note IS NULL) OR (review_state='APPLIED' AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL AND applied_proposal_version_id IS NOT NULL AND review_note IS NOT NULL));
ALTER TABLE bid.proposal_versions ADD COLUMN source_ai_suggestion_id uuid REFERENCES ai.proposal_draft_suggestions(id) ON DELETE RESTRICT;
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=14,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=13;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 13'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=13,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=14;
ALTER TABLE bid.proposal_versions DROP COLUMN source_ai_suggestion_id;
ALTER TABLE ai.proposal_draft_suggestions DROP CONSTRAINT proposal_draft_review_consistency, DROP COLUMN review_note, DROP COLUMN applied_proposal_version_id, DROP COLUMN reviewed_at, DROP COLUMN reviewed_by, DROP CONSTRAINT proposal_draft_suggestions_review_state_check;
ALTER TABLE ai.proposal_draft_suggestions ADD CONSTRAINT proposal_draft_suggestions_review_state_check CHECK(review_state='PROPOSED_UNVERIFIED');
-- +goose StatementEnd
