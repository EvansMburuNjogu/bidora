-- +goose Up
-- +goose StatementBegin
-- Human review never mutates the original AI text/citation. Acceptance creates
-- a separate authoritative bid requirement in the same transaction.
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE bid.requirements ADD COLUMN source_document_version_id uuid REFERENCES documents.document_versions(id) ON DELETE RESTRICT;

ALTER TABLE ai.requirement_suggestions
 ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 ADD COLUMN accepted_requirement_id uuid REFERENCES bid.requirements(id) ON DELETE RESTRICT,
 ADD COLUMN review_note text CHECK(review_note IS NULL OR (length(review_note) BETWEEN 1 AND 2000 AND review_note ~ '\S')),
 DROP CONSTRAINT requirement_suggestion_initial_review_shape,
 ADD CONSTRAINT requirement_suggestion_review_shape CHECK(
   (review_state='PROPOSED_UNVERIFIED' AND reviewed_by IS NULL AND reviewed_at IS NULL AND accepted_requirement_id IS NULL AND review_note IS NULL) OR
   (review_state='ACCEPTED' AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL AND accepted_requirement_id IS NOT NULL) OR
   (review_state='REJECTED' AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL AND accepted_requirement_id IS NULL AND review_note IS NOT NULL)
 );
CREATE UNIQUE INDEX requirement_suggestion_accepted_requirement_uidx ON ai.requirement_suggestions(accepted_requirement_id) WHERE accepted_requirement_id IS NOT NULL;

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=8,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=7;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 7'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=7,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=8;
DROP INDEX ai.requirement_suggestion_accepted_requirement_uidx;
ALTER TABLE ai.requirement_suggestions DROP CONSTRAINT requirement_suggestion_review_shape, ADD CONSTRAINT requirement_suggestion_initial_review_shape CHECK((review_state='PROPOSED_UNVERIFIED' AND reviewed_by IS NULL AND reviewed_at IS NULL) OR (review_state IN ('ACCEPTED','REJECTED') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)), DROP COLUMN review_note, DROP COLUMN accepted_requirement_id, DROP COLUMN version;
ALTER TABLE bid.requirements DROP COLUMN source_document_version_id;
-- +goose StatementEnd
