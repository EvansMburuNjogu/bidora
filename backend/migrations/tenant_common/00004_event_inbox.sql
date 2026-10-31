-- +goose Up
-- +goose StatementBegin
-- Durable tenant-local inbox receipt. Insert the receipt in the same transaction
-- as database effects; rollback removes both. External effects need their own
-- operation state/reconciliation and are not made exactly-once by this table.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE TABLE bidora_eventing.inbox_events (
    id uuid PRIMARY KEY,
    consumer text NOT NULL CHECK (length(consumer) BETWEEN 1 AND 200 AND consumer ~ '\S'),
    event_id uuid NOT NULL,
    event_type text NOT NULL CHECK (length(event_type) BETWEEN 1 AND 200 AND event_type ~ '\S'),
    schema_version integer NOT NULL CHECK (schema_version > 0),
    aggregate_type text NOT NULL CHECK (length(aggregate_type) BETWEEN 1 AND 100 AND aggregate_type ~ '\S'),
    aggregate_id uuid NOT NULL,
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    occurred_at timestamptz NOT NULL,
    processed_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (consumer, event_id)
);
CREATE INDEX inbox_events_aggregate_idx
    ON bidora_eventing.inbox_events
       (consumer, aggregate_type, aggregate_id, aggregate_version);
CREATE INDEX inbox_events_processed_idx
    ON bidora_eventing.inbox_events (processed_at, id);

CREATE FUNCTION bidora_eventing.reject_inbox_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'inbox receipts are append-only' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER inbox_events_append_only
BEFORE UPDATE OR DELETE ON bidora_eventing.inbox_events
FOR EACH ROW EXECUTE FUNCTION bidora_eventing.reject_inbox_mutation();
COMMENT ON TABLE bidora_eventing.inbox_events IS
'No automatic expiry; approved replay/retention policy is required before cleanup.';

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_meta.schema_compatibility
    SET schema_version = 4, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'tenant_common' AND schema_version = 3;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'tenant_common compatibility row is not at version 3' USING ERRCODE = '55000';
    END IF;
END;
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
UPDATE bidora_meta.schema_compatibility
SET schema_version = 3, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'tenant_common' AND schema_version = 4;
DROP TABLE bidora_eventing.inbox_events;
DROP FUNCTION bidora_eventing.reject_inbox_mutation();
-- +goose StatementEnd
