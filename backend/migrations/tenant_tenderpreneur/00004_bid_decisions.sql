-- +goose Up
-- +goose StatementBegin
-- BID-BE-002 simple human-only Bid/No-Bid history.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE bid.workspaces
    DROP CONSTRAINT workspaces_state_check,
    ADD CONSTRAINT workspaces_state_check CHECK (state IN ('DECISION_PENDING','BID','NO_BID'));

CREATE TABLE bid.decisions (
    id uuid PRIMARY KEY,
    bid_id uuid NOT NULL REFERENCES bid.workspaces(id) ON DELETE RESTRICT,
    sequence integer NOT NULL CHECK (sequence > 0),
    workspace_version bigint NOT NULL CHECK (workspace_version > 1),
    decision text NOT NULL CHECK (decision IN ('BID','NO_BID')),
    rationale text NOT NULL CHECK (length(rationale) BETWEEN 1 AND 2000 AND rationale ~ '\S'),
    actor_id uuid NOT NULL,
    decided_at timestamptz NOT NULL,
    UNIQUE (bid_id, sequence),
    UNIQUE (bid_id, workspace_version)
);
CREATE INDEX decisions_bid_time_idx ON bid.decisions (bid_id, decided_at DESC, id DESC);

CREATE FUNCTION bid.reject_decision_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'bid decisions are immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER decisions_append_only
BEFORE UPDATE OR DELETE ON bid.decisions
FOR EACH ROW EXECUTE FUNCTION bid.reject_decision_mutation();

DO $$
DECLARE affected integer;
BEGIN
    UPDATE bidora_meta.schema_compatibility
    SET schema_version = 4, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'tenant_tenderpreneur' AND schema_version = 3;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 3';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=3,updated_at=CURRENT_TIMESTAMP
WHERE migration_set='tenant_tenderpreneur' AND schema_version=4;
DROP TABLE bid.decisions;
DROP FUNCTION bid.reject_decision_mutation();
ALTER TABLE bid.workspaces
    DROP CONSTRAINT workspaces_state_check,
    ADD CONSTRAINT workspaces_state_check CHECK (state = 'DECISION_PENDING');
-- +goose StatementEnd
