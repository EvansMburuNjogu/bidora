-- +goose Up
-- +goose StatementBegin
-- Review discussion is append-only and separate from authoritative proposal/requirement content.
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bid.review_threads (
 id uuid PRIMARY KEY,
 bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
 target_type text NOT NULL CHECK(target_type IN ('BID','REQUIREMENT','PROPOSAL_VERSION')),
 target_id uuid NOT NULL,
 subject text NOT NULL CHECK(length(subject) BETWEEN 1 AND 300 AND subject ~ '\S'),
 status text NOT NULL CHECK(status IN ('OPEN','RESOLVED')),
 version bigint NOT NULL CHECK(version BETWEEN 1 AND 200),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 resolved_by uuid,
 resolved_at timestamptz,
 CHECK((status='OPEN' AND resolved_by IS NULL AND resolved_at IS NULL) OR (status='RESOLVED' AND resolved_by IS NOT NULL AND resolved_at IS NOT NULL))
);
CREATE INDEX review_threads_bid_status_idx ON bid.review_threads(bid_id,status,created_at,id);
CREATE TABLE bid.review_comments (
 id uuid PRIMARY KEY,
 thread_id uuid NOT NULL REFERENCES bid.review_threads(id) ON DELETE RESTRICT,
 thread_version bigint NOT NULL CHECK(thread_version BETWEEN 1 AND 200),
 comment_kind text NOT NULL CHECK(comment_kind IN ('COMMENT','RESOLUTION_NOTE')),
 body text NOT NULL CHECK(length(body) BETWEEN 1 AND 10000 AND body ~ '\S'),
 created_by uuid NOT NULL,
 created_at timestamptz NOT NULL,
 UNIQUE(thread_id,thread_version)
);
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=17,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=16;
 GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 16'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=16,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=17;
DROP TABLE bid.review_comments;
DROP TABLE bid.review_threads;
-- +goose StatementEnd
