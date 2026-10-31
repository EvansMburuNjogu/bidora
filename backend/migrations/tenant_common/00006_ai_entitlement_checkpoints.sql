-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE SCHEMA bidora_entitlement;
CREATE TABLE bidora_entitlement.ai_usage_checkpoints(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL,target_type text NOT NULL CHECK(target_type IN('TENDER_ANALYSIS','REQUIREMENT_REVIEW','RESPONSE_DRAFT','PROPOSAL_DRAFT')),
 target_id uuid NOT NULL,source_operation_id uuid NOT NULL,request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),requester_user_id uuid NOT NULL,assignment_generation bigint NOT NULL CHECK(assignment_generation>0),
 usage_kind text NOT NULL CHECK(usage_kind IN('DOCUMENT_PDF','DOCUMENT_DOCX','GENERATION_TEXT')),estimated_credits bigint NOT NULL CHECK(estimated_credits>0),
 state text NOT NULL CHECK(state IN('PENDING_RESERVATION','RESERVED','DENIED','SETTLE_PENDING','RELEASE_PENDING','SETTLED','RELEASED')),
 reservation_id uuid,denial_code text CHECK(denial_code IS NULL OR denial_code IN('LIMIT_EXCEEDED','ACCESS_UNAVAILABLE','SUBSCRIPTION_UNAVAILABLE')),
 version bigint NOT NULL CHECK(version>0),created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,
 UNIQUE(target_type,target_id),UNIQUE(source_operation_id),
 CHECK(updated_at>=created_at),
 CHECK((state='PENDING_RESERVATION' AND reservation_id IS NULL AND denial_code IS NULL) OR
       (state='DENIED' AND reservation_id IS NULL AND denial_code IS NOT NULL) OR
       (state NOT IN('PENDING_RESERVATION','DENIED') AND reservation_id IS NOT NULL AND denial_code IS NULL))
);
CREATE INDEX ai_usage_checkpoints_state_idx ON bidora_entitlement.ai_usage_checkpoints(state,created_at,id);
CREATE TABLE bidora_entitlement.ai_usage_checkpoint_transitions(
 id uuid PRIMARY KEY,checkpoint_id uuid NOT NULL REFERENCES bidora_entitlement.ai_usage_checkpoints(id) ON DELETE RESTRICT,
 from_state text NOT NULL CHECK(from_state='PENDING_RESERVATION'),to_state text NOT NULL CHECK(to_state IN('RESERVED','DENIED')),checkpoint_version bigint NOT NULL CHECK(checkpoint_version=2),
 source_event_id uuid NOT NULL,source_event_type text NOT NULL,source_aggregate_version bigint NOT NULL CHECK(source_aggregate_version>0),
 reservation_id uuid,denial_code text,occurred_at timestamptz NOT NULL,
 UNIQUE(checkpoint_id,checkpoint_version),UNIQUE(source_event_id),
 CHECK((to_state='RESERVED' AND reservation_id IS NOT NULL AND denial_code IS NULL) OR (to_state='DENIED' AND reservation_id IS NULL AND denial_code IS NOT NULL))
);
CREATE FUNCTION bidora_entitlement.reject_checkpoint_transition_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'AI entitlement transition is immutable' USING ERRCODE='55000'; END $$;
CREATE TRIGGER ai_usage_checkpoint_transitions_immutable BEFORE UPDATE OR DELETE ON bidora_entitlement.ai_usage_checkpoint_transitions FOR EACH ROW EXECUTE FUNCTION bidora_entitlement.reject_checkpoint_transition_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_meta.schema_compatibility SET schema_version=6,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_common' AND schema_version=5; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_common compatibility row missing or not at version 5'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_checkpoints) OR EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_checkpoint_transitions) THEN RAISE EXCEPTION 'cannot roll back AI entitlement checkpoints with retained evidence'; END IF; END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=5,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_common' AND schema_version=6;
DROP TABLE bidora_entitlement.ai_usage_checkpoint_transitions; DROP TABLE bidora_entitlement.ai_usage_checkpoints; DROP FUNCTION bidora_entitlement.reject_checkpoint_transition_change(); DROP SCHEMA bidora_entitlement;
-- +goose StatementEnd
