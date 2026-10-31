-- +goose Up
-- +goose StatementBegin
-- Preconditions: tenant_common is at schema version 4. This additive migration
-- preserves pre-v5 rows but they must be reconciled before dispatch/validation.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE bidora_eventing.outbox_events ADD COLUMN actor_id uuid;
ALTER TABLE bidora_eventing.outbox_events
    ADD CONSTRAINT outbox_events_actor_required CHECK (actor_id IS NOT NULL) NOT VALID;
COMMENT ON COLUMN bidora_eventing.outbox_events.actor_id IS
'Actor provenance only; never authority. Pre-v5 rows require reconciliation before dispatch.';

CREATE OR REPLACE FUNCTION bidora_eventing.protect_outbox_facts() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.actor_id IS DISTINCT FROM OLD.actor_id
       OR NEW.aggregate_type IS DISTINCT FROM OLD.aggregate_type
       OR NEW.aggregate_id IS DISTINCT FROM OLD.aggregate_id
       OR NEW.aggregate_version IS DISTINCT FROM OLD.aggregate_version
       OR NEW.event_type IS DISTINCT FROM OLD.event_type
       OR NEW.schema_version IS DISTINCT FROM OLD.schema_version
       OR NEW.payload IS DISTINCT FROM OLD.payload
       OR NEW.occurred_at IS DISTINCT FROM OLD.occurred_at
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'outbox event facts are immutable' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_meta.schema_compatibility
    SET schema_version = 5, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'tenant_common' AND schema_version = 4;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'tenant_common compatibility row missing or not at version 4';
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
-- +goose StatementBegin
SET lock_timeout = '1s';
SET statement_timeout = '5s';
UPDATE bidora_meta.schema_compatibility
SET schema_version = 4, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'tenant_common' AND schema_version = 5;
ALTER TABLE bidora_eventing.outbox_events DROP CONSTRAINT IF EXISTS outbox_events_actor_required;
ALTER TABLE bidora_eventing.outbox_events DROP COLUMN IF EXISTS actor_id;
CREATE OR REPLACE FUNCTION bidora_eventing.protect_outbox_facts() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.aggregate_type IS DISTINCT FROM OLD.aggregate_type
       OR NEW.aggregate_id IS DISTINCT FROM OLD.aggregate_id
       OR NEW.aggregate_version IS DISTINCT FROM OLD.aggregate_version
       OR NEW.event_type IS DISTINCT FROM OLD.event_type
       OR NEW.schema_version IS DISTINCT FROM OLD.schema_version
       OR NEW.payload IS DISTINCT FROM OLD.payload
       OR NEW.occurred_at IS DISTINCT FROM OLD.occurred_at
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'outbox event facts are immutable' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
