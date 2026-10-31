-- +goose Up
-- +goose StatementBegin
-- BID-BE-003 human-owned requirements and assignments. AI suggestions must be
-- accepted through the same human command before becoming requirement facts.
SET lock_timeout='1s';
SET statement_timeout='5s';

CREATE TABLE bid.requirements (
    id uuid PRIMARY KEY,
    bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
    source_version_id uuid NOT NULL REFERENCES bidora_opportunity.source_versions(id) ON DELETE RESTRICT,
    source_location text NOT NULL CHECK (length(source_location) BETWEEN 1 AND 500 AND source_location ~ '\S'),
    text text NOT NULL CHECK (length(text) BETWEEN 1 AND 10000 AND text ~ '\S'),
    mandatory boolean NOT NULL,
    category text NOT NULL CHECK (length(category) BETWEEN 1 AND 100 AND category ~ '\S'),
    status text NOT NULL CHECK (status IN ('OPEN','SATISFIED')),
    owner_id uuid,
    due_at timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version>0),
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (updated_at>=created_at)
);
CREATE INDEX requirements_bid_status_idx ON bid.requirements(bid_id,status,id);
CREATE INDEX requirements_owner_due_idx ON bid.requirements(owner_id,due_at,id) WHERE owner_id IS NOT NULL;

DO $$
DECLARE affected integer;
BEGIN
    UPDATE bidora_meta.schema_compatibility SET schema_version=5,updated_at=CURRENT_TIMESTAMP
    WHERE migration_set='tenant_tenderpreneur' AND schema_version=4;
    GET DIAGNOSTICS affected=ROW_COUNT;
    IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 4'; END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s';
SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=4,updated_at=CURRENT_TIMESTAMP
WHERE migration_set='tenant_tenderpreneur' AND schema_version=5;
DROP TABLE bid.requirements;
-- +goose StatementEnd
