-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bidora_control_eventing.inbox_events(
 id uuid PRIMARY KEY,consumer text NOT NULL CHECK(length(consumer) BETWEEN 1 AND 200 AND consumer ~ '\S'),event_id uuid NOT NULL,event_type text NOT NULL CHECK(length(event_type) BETWEEN 1 AND 200 AND event_type ~ '\S'),schema_version integer NOT NULL CHECK(schema_version>0),
 aggregate_type text NOT NULL CHECK(length(aggregate_type) BETWEEN 1 AND 100 AND aggregate_type ~ '\S'),aggregate_id uuid NOT NULL,aggregate_version bigint NOT NULL CHECK(aggregate_version>0),tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 assignment_generation bigint NOT NULL CHECK(assignment_generation>0),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),occurred_at timestamptz NOT NULL,processed_at timestamptz NOT NULL,created_at timestamptz NOT NULL,
 UNIQUE(consumer,event_id),UNIQUE(event_id),CHECK(processed_at>=occurred_at)
);
CREATE INDEX control_inbox_aggregate_idx ON bidora_control_eventing.inbox_events(consumer,aggregate_type,aggregate_id,aggregate_version);
CREATE INDEX control_inbox_processed_idx ON bidora_control_eventing.inbox_events(processed_at,id);
CREATE TRIGGER control_inbox_events_immutable BEFORE UPDATE OR DELETE ON bidora_control_eventing.inbox_events FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
CREATE TABLE commercial.ai_reservation_request_outcomes(
 id uuid PRIMARY KEY,event_id uuid NOT NULL UNIQUE,checkpoint_id uuid NOT NULL UNIQUE,tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 source_operation_id uuid NOT NULL,request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),assignment_generation bigint NOT NULL CHECK(assignment_generation>0),requester_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 disposition text NOT NULL CHECK(disposition IN('RESERVED','DENIED')),reservation_id uuid REFERENCES commercial.ai_usage_reservations(id) ON DELETE RESTRICT,denial_code text CHECK(denial_code IS NULL OR denial_code IN('LIMIT_EXCEEDED','ACCESS_UNAVAILABLE','SUBSCRIPTION_UNAVAILABLE')),
 processed_at timestamptz NOT NULL,
 FOREIGN KEY(event_id) REFERENCES bidora_control_eventing.inbox_events(event_id) DEFERRABLE INITIALLY DEFERRED,
 CHECK((disposition='RESERVED' AND reservation_id IS NOT NULL AND denial_code IS NULL) OR (disposition='DENIED' AND reservation_id IS NULL AND denial_code IS NOT NULL))
);
CREATE TRIGGER ai_reservation_request_outcomes_immutable BEFORE UPDATE OR DELETE ON commercial.ai_reservation_request_outcomes FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=20,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=19; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 19'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.ai_reservation_request_outcomes) OR EXISTS(SELECT 1 FROM bidora_control_eventing.inbox_events) THEN RAISE EXCEPTION 'cannot roll back AI reservation consumption with retained inbox/outcome evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=19,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=20;
DROP TABLE commercial.ai_reservation_request_outcomes; DROP TABLE bidora_control_eventing.inbox_events;
-- +goose StatementEnd
