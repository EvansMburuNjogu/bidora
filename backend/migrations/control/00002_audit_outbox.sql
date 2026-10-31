-- +goose Up
-- +goose StatementBegin
-- Plane-local immutable control audit/event facts. Payloads must remain minimal;
-- tenant/actor IDs are provenance, never authorization.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA bidora_control_audit;
CREATE SCHEMA bidora_control_eventing;

CREATE TABLE bidora_control_audit.audit_events (
    id uuid PRIMARY KEY,
    tenant_id uuid REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
    actor_id uuid,
    action text NOT NULL CHECK (length(action) BETWEEN 1 AND 100),
    target_type text NOT NULL CHECK (length(target_type) BETWEEN 1 AND 100),
    target_id uuid NOT NULL,
    occurred_at timestamptz NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('SUCCEEDED', 'FAILED')),
    correlation_id uuid NOT NULL,
    change_summary jsonb NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(change_summary) = 'object'),
    policy_version bigint CHECK (policy_version > 0),
    created_at timestamptz NOT NULL
);
CREATE INDEX control_audit_target_time_idx
    ON bidora_control_audit.audit_events (target_type, target_id, occurred_at DESC, id DESC);
CREATE INDEX control_audit_tenant_time_idx
    ON bidora_control_audit.audit_events (tenant_id, occurred_at DESC, id DESC)
    WHERE tenant_id IS NOT NULL;
CREATE INDEX control_audit_correlation_idx
    ON bidora_control_audit.audit_events (correlation_id);

CREATE FUNCTION bidora_control_audit.reject_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'control audit events are append-only' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER control_audit_events_append_only
BEFORE UPDATE OR DELETE ON bidora_control_audit.audit_events
FOR EACH ROW EXECUTE FUNCTION bidora_control_audit.reject_mutation();

CREATE TABLE bidora_control_eventing.outbox_events (
    id uuid PRIMARY KEY,
    tenant_id uuid REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
    actor_id uuid,
    aggregate_type text NOT NULL CHECK (length(aggregate_type) BETWEEN 1 AND 100),
    aggregate_id uuid NOT NULL,
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    event_type text NOT NULL CHECK (length(event_type) BETWEEN 1 AND 200),
    schema_version integer NOT NULL CHECK (schema_version > 0),
    classification text NOT NULL CHECK (length(classification) BETWEEN 1 AND 50 AND classification ~ '\S'),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    occurred_at timestamptz NOT NULL,
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    CHECK (published_at IS NULL OR published_at >= occurred_at)
);
CREATE INDEX control_outbox_pending_idx
    ON bidora_control_eventing.outbox_events (next_attempt_at, id)
    WHERE published_at IS NULL;
CREATE INDEX control_outbox_aggregate_idx
    ON bidora_control_eventing.outbox_events (aggregate_type, aggregate_id, aggregate_version);
CREATE INDEX control_outbox_tenant_idx
    ON bidora_control_eventing.outbox_events (tenant_id, occurred_at, id)
    WHERE tenant_id IS NOT NULL;

CREATE FUNCTION bidora_control_eventing.protect_outbox_facts() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.actor_id IS DISTINCT FROM OLD.actor_id
       OR NEW.aggregate_type IS DISTINCT FROM OLD.aggregate_type
       OR NEW.aggregate_id IS DISTINCT FROM OLD.aggregate_id
       OR NEW.aggregate_version IS DISTINCT FROM OLD.aggregate_version
       OR NEW.event_type IS DISTINCT FROM OLD.event_type
       OR NEW.schema_version IS DISTINCT FROM OLD.schema_version
       OR NEW.classification IS DISTINCT FROM OLD.classification
       OR NEW.payload IS DISTINCT FROM OLD.payload
       OR NEW.occurred_at IS DISTINCT FROM OLD.occurred_at
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'control outbox event facts are immutable' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER control_outbox_event_facts_immutable
BEFORE UPDATE ON bidora_control_eventing.outbox_events
FOR EACH ROW EXECUTE FUNCTION bidora_control_eventing.protect_outbox_facts();

DO $$
DECLARE
    affected integer;
BEGIN
    UPDATE bidora_control_meta.schema_compatibility
    SET schema_version = 2, updated_at = CURRENT_TIMESTAMP
    WHERE migration_set = 'control_registry' AND schema_version = 1;
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected <> 1 THEN
        RAISE EXCEPTION 'control compatibility row is not at version 1' USING ERRCODE = '55000';
    END IF;
END;
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
UPDATE bidora_control_meta.schema_compatibility
SET schema_version = 1, updated_at = CURRENT_TIMESTAMP
WHERE migration_set = 'control_registry' AND schema_version = 2;
DROP SCHEMA bidora_control_eventing CASCADE;
DROP SCHEMA bidora_control_audit CASCADE;
-- +goose StatementEnd
