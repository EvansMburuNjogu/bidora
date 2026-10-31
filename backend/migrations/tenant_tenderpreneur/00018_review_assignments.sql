-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE bid.review_threads ADD COLUMN assigned_to uuid, ADD COLUMN due_at timestamptz;
CREATE INDEX review_threads_assignee_due_idx ON bid.review_threads(assigned_to,due_at,id) WHERE status='OPEN';
CREATE TABLE bid.review_assignments (
 id uuid PRIMARY KEY,
 thread_id uuid NOT NULL REFERENCES bid.review_threads(id) ON DELETE RESTRICT,
 thread_version bigint NOT NULL CHECK(thread_version BETWEEN 2 AND 200),
 assigned_to uuid NOT NULL,
 due_at timestamptz,
 assignment_reason text NOT NULL CHECK(length(assignment_reason) BETWEEN 1 AND 2000 AND assignment_reason ~ '\S'),
 assigned_by uuid NOT NULL,
 assigned_at timestamptz NOT NULL,
 UNIQUE(thread_id,thread_version)
);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=18,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=17;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 17'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=17,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=18;
DROP TABLE bid.review_assignments;
DROP INDEX bid.review_threads_assignee_due_idx;
ALTER TABLE bid.review_threads DROP COLUMN due_at, DROP COLUMN assigned_to;
-- +goose StatementEnd
