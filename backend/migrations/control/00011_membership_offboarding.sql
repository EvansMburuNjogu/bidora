-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE tenancy.memberships DROP CONSTRAINT memberships_state_check;
ALTER TABLE tenancy.memberships ADD CONSTRAINT memberships_state_check CHECK(state IN ('INVITED','ACTIVE','OFFBOARDING','SUSPENDED','REVOKED'));
CREATE TABLE tenancy.membership_offboarding_operations(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 target_membership_id uuid NOT NULL REFERENCES tenancy.memberships(id) ON DELETE RESTRICT,
 successor_membership_id uuid NOT NULL REFERENCES tenancy.memberships(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 state text NOT NULL CHECK(state IN ('STARTED','TENANT_TRANSFERRED','COMPLETED','CANCELLED')),
 version bigint NOT NULL CHECK(version>0),reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000 AND reason~'\S'),
 assignment_generation bigint NOT NULL CHECK(assignment_generation>0),key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 workspace_count integer,requirement_count integer,review_count integer,
 started_at timestamptz NOT NULL,tenant_transferred_at timestamptz,completed_at timestamptz,cancelled_at timestamptz,
 CHECK(target_membership_id<>successor_membership_id),UNIQUE(actor_id,key_hash),
 CHECK((state='STARTED' AND tenant_transferred_at IS NULL AND completed_at IS NULL AND cancelled_at IS NULL) OR (state='TENANT_TRANSFERRED' AND tenant_transferred_at IS NOT NULL AND completed_at IS NULL AND cancelled_at IS NULL) OR (state='COMPLETED' AND tenant_transferred_at IS NOT NULL AND completed_at IS NOT NULL AND cancelled_at IS NULL) OR (state='CANCELLED' AND completed_at IS NULL AND cancelled_at IS NOT NULL)),
 CHECK(workspace_count IS NULL OR workspace_count>=0),CHECK(requirement_count IS NULL OR requirement_count>=0),CHECK(review_count IS NULL OR review_count>=0)
);
CREATE UNIQUE INDEX membership_offboarding_one_open_target ON tenancy.membership_offboarding_operations(target_membership_id) WHERE state IN ('STARTED','TENANT_TRANSFERRED');
CREATE INDEX membership_offboarding_successor_open ON tenancy.membership_offboarding_operations(successor_membership_id) WHERE state IN ('STARTED','TENANT_TRANSFERRED');
CREATE TABLE tenancy.membership_offboarding_events(id uuid PRIMARY KEY,operation_id uuid NOT NULL REFERENCES tenancy.membership_offboarding_operations(id) ON DELETE RESTRICT,sequence integer NOT NULL CHECK(sequence>0),event_type text NOT NULL CHECK(event_type IN ('STARTED','TENANT_TRANSFERRED','RETARGETED','CANCELLED','COMPLETED')),actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,successor_membership_id uuid NOT NULL REFERENCES tenancy.memberships(id) ON DELETE RESTRICT,reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000 AND reason~'\S'),key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),occurred_at timestamptz NOT NULL,UNIQUE(operation_id,sequence),UNIQUE(actor_id,key_hash));
CREATE FUNCTION tenancy.reject_membership_offboarding_event_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'membership offboarding events are immutable'; END $$;
CREATE TRIGGER membership_offboarding_events_immutable BEFORE UPDATE OR DELETE ON tenancy.membership_offboarding_events FOR EACH ROW EXECUTE FUNCTION tenancy.reject_membership_offboarding_event_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=11,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=10; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 10'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_control_meta.schema_compatibility SET schema_version=10,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=11;
DROP TRIGGER membership_offboarding_events_immutable ON tenancy.membership_offboarding_events; DROP FUNCTION tenancy.reject_membership_offboarding_event_change();
DROP TABLE tenancy.membership_offboarding_events; DROP TABLE tenancy.membership_offboarding_operations;
ALTER TABLE tenancy.memberships DROP CONSTRAINT memberships_state_check;
ALTER TABLE tenancy.memberships ADD CONSTRAINT memberships_state_check CHECK(state IN ('INVITED','ACTIVE','SUSPENDED','REVOKED'));
-- +goose StatementEnd
