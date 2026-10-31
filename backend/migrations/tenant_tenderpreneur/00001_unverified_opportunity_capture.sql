-- +goose Up
-- +goose StatementBegin
-- BID-DB-004 bounded schema for approved UNVERIFIED manual capture only.
-- Apply tenant_common/00001 first. No document FK is added until BID-DB-005.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA bidora_opportunity;

CREATE TABLE bidora_opportunity.opportunities (
    id uuid PRIMARY KEY,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    current_source_version_id uuid,
    created_at timestamptz NOT NULL,
    created_by uuid NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at >= created_at)
);

CREATE TABLE bidora_opportunity.source_versions (
    id uuid PRIMARY KEY,
    opportunity_id uuid NOT NULL REFERENCES bidora_opportunity.opportunities(id) ON DELETE RESTRICT,
    revision integer NOT NULL CHECK (revision > 0),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 300 AND title ~ '\S'),
    buyer_name text CHECK (length(buyer_name) <= 300),
    source_uri text CHECK (length(source_uri) <= 2048),
    -- Logical exact-version reference until the separately reviewed document
    -- schema adds a local FK. A route accepting this value must remain disabled
    -- until owner-port access/eligibility checks and that constraint are ready.
    source_document_version_id uuid,
    closes_at timestamptz,
    source_timezone text CHECK (length(source_timezone) <= 128),
    source_date_text text CHECK (length(source_date_text) <= 4096),
    verification_state text NOT NULL CHECK (verification_state = 'UNVERIFIED'),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    created_at timestamptz NOT NULL,
    created_by uuid NOT NULL,
    UNIQUE (opportunity_id, revision),
    UNIQUE (opportunity_id, id),
    CHECK (closes_at IS NULL OR (
        source_timezone IS NOT NULL AND source_timezone ~ '\S'
        AND source_date_text IS NOT NULL AND source_date_text ~ '\S'
    ))
);

ALTER TABLE bidora_opportunity.opportunities
    ADD CONSTRAINT opportunities_current_source_fk
    FOREIGN KEY (id, current_source_version_id)
    REFERENCES bidora_opportunity.source_versions(opportunity_id, id) ON DELETE RESTRICT;

CREATE INDEX opportunities_created_idx
    ON bidora_opportunity.opportunities (created_at DESC, id DESC);
CREATE INDEX source_versions_opportunity_idx
    ON bidora_opportunity.source_versions (opportunity_id, revision DESC);
CREATE INDEX source_versions_closes_idx
    ON bidora_opportunity.source_versions (closes_at, opportunity_id)
    WHERE closes_at IS NOT NULL;

CREATE FUNCTION bidora_opportunity.reject_source_version_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'opportunity source versions are immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER source_versions_immutable
BEFORE UPDATE OR DELETE ON bidora_opportunity.source_versions
FOR EACH ROW EXECUTE FUNCTION bidora_opportunity.reject_source_version_mutation();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
DROP SCHEMA bidora_opportunity CASCADE;
-- +goose StatementEnd
