-- +goose Up
-- +goose StatementBegin
-- BID-DB-003 bounded foundation for tenant-owned command evidence.
-- Preconditions: PostgreSQL 15+, UTF-8 database, migrator owns these schemas.
-- Runtime role grants are deliberately deployment-bound and MUST be applied and
-- tested before any business route is enabled. This migration creates no roles.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE SCHEMA bidora_audit;
CREATE SCHEMA bidora_eventing;
CREATE SCHEMA bidora_commands;

CREATE TABLE bidora_audit.audit_events (
    id uuid PRIMARY KEY,
    actor_id uuid NOT NULL,
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
CREATE INDEX audit_events_target_time_idx
    ON bidora_audit.audit_events (target_type, target_id, occurred_at DESC, id DESC);
CREATE INDEX audit_events_correlation_idx
    ON bidora_audit.audit_events (correlation_id);

CREATE FUNCTION bidora_audit.reject_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit events are append-only' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER audit_events_append_only
BEFORE UPDATE OR DELETE ON bidora_audit.audit_events
FOR EACH ROW EXECUTE FUNCTION bidora_audit.reject_mutation();

CREATE TABLE bidora_eventing.outbox_events (
    id uuid PRIMARY KEY,
    aggregate_type text NOT NULL CHECK (length(aggregate_type) BETWEEN 1 AND 100),
    aggregate_id uuid NOT NULL,
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    event_type text NOT NULL CHECK (length(event_type) BETWEEN 1 AND 200),
    schema_version integer NOT NULL CHECK (schema_version > 0),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    occurred_at timestamptz NOT NULL,
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    CHECK (published_at IS NULL OR published_at >= occurred_at)
);
CREATE INDEX outbox_events_pending_idx
    ON bidora_eventing.outbox_events (next_attempt_at, id)
    WHERE published_at IS NULL;
CREATE INDEX outbox_events_aggregate_idx
    ON bidora_eventing.outbox_events (aggregate_type, aggregate_id, aggregate_version);

CREATE FUNCTION bidora_eventing.protect_outbox_facts() RETURNS trigger
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
CREATE TRIGGER outbox_event_facts_immutable
BEFORE UPDATE ON bidora_eventing.outbox_events
FOR EACH ROW EXECUTE FUNCTION bidora_eventing.protect_outbox_facts();

CREATE TABLE bidora_commands.idempotency_records (
    id uuid PRIMARY KEY,
    actor_id uuid NOT NULL,
    operation text NOT NULL CHECK (length(operation) BETWEEN 1 AND 100),
    key_hash bytea NOT NULL CHECK (octet_length(key_hash) = 32),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    state text NOT NULL CHECK (state IN ('PENDING', 'COMPLETED')),
    response_status integer CHECK (response_status BETWEEN 200 AND 599),
    response_ref uuid,
    response_version bigint CHECK (response_version > 0),
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    UNIQUE (actor_id, operation, key_hash),
    CHECK (
      (state = 'PENDING' AND response_status IS NULL AND response_ref IS NULL
       AND response_version IS NULL AND completed_at IS NULL)
      OR
      (state = 'COMPLETED' AND response_status IS NOT NULL AND response_ref IS NOT NULL
       AND response_version IS NOT NULL AND completed_at IS NOT NULL
       AND completed_at >= created_at)
    )
);
-- Approved request policy: no automatic TTL. Retention/erasure is separately governed.
COMMENT ON TABLE bidora_commands.idempotency_records IS
'No automatic expiry; retain while target history exists. Reauthorize every replay.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Destructive rollback is local/empty-database development convenience only.
-- Never auto-run Down against data-bearing environments; forward-fix or restore.
DROP SCHEMA bidora_commands CASCADE;
-- +goose StatementEnd
DROP SCHEMA bidora_eventing CASCADE;
DROP SCHEMA bidora_audit CASCADE;
