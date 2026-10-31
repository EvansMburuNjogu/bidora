-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
LOCK TABLE bidora_control_eventing.outbox_events IN SHARE ROW EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM bidora_control_eventing.outbox_events WHERE published_at IS NULL AND event_type IN('commercial.ai_usage.reserved.v1','commercial.ai_usage.denied.v1') AND attempts>=9) THEN
  RAISE EXCEPTION 'cannot introduce finite acknowledgement dispatch claims while over-budget events remain unresolved';
 END IF;
END $$;

CREATE TABLE bidora_control_eventing.ai_acknowledgement_dispatch_holds(
 event_id uuid PRIMARY KEY REFERENCES bidora_control_eventing.outbox_events(id) ON DELETE RESTRICT,
 tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 envelope_hash bytea NOT NULL CHECK(octet_length(envelope_hash)=32),
 category text NOT NULL CHECK(category IN('STALE_ASSIGNMENT','INTEGRITY_CONFLICT','POISON_EVENT','DELIVERY_EXHAUSTED')),
 reason_code text NOT NULL CHECK(reason_code IN(
  'STALE_ASSIGNMENT','GENERATION_AHEAD_OF_REGISTRY','INVALID_ENVELOPE','REGISTRY_UNAVAILABLE',
  'CURRENT_ASSIGNMENT_TEMPORARILY_UNAVAILABLE','ENDPOINT_UNAVAILABLE','TENANT_APPLY_UNAVAILABLE'
 )),
 event_assignment_generation bigint CHECK(event_assignment_generation IS NULL OR event_assignment_generation>0),
 observed_assignment_generation bigint CHECK(observed_assignment_generation IS NULL OR observed_assignment_generation>0),
 dispatch_attempt integer NOT NULL CHECK(dispatch_attempt BETWEEN 1 AND 9),
 held_at timestamptz NOT NULL,
 CHECK((category='STALE_ASSIGNMENT' AND reason_code='STALE_ASSIGNMENT' AND event_assignment_generation IS NOT NULL AND observed_assignment_generation IS NOT NULL AND observed_assignment_generation>event_assignment_generation) OR
       (category='INTEGRITY_CONFLICT' AND reason_code='GENERATION_AHEAD_OF_REGISTRY' AND event_assignment_generation IS NOT NULL AND observed_assignment_generation IS NOT NULL AND event_assignment_generation>observed_assignment_generation) OR
       (category='POISON_EVENT' AND reason_code='INVALID_ENVELOPE') OR
       (category='DELIVERY_EXHAUSTED' AND reason_code IN('REGISTRY_UNAVAILABLE','CURRENT_ASSIGNMENT_TEMPORARILY_UNAVAILABLE','ENDPOINT_UNAVAILABLE','TENANT_APPLY_UNAVAILABLE') AND dispatch_attempt=9))
);
CREATE INDEX ai_acknowledgement_dispatch_holds_category_idx ON bidora_control_eventing.ai_acknowledgement_dispatch_holds(category,held_at,event_id);
CREATE FUNCTION bidora_control_eventing.reject_ai_acknowledgement_dispatch_hold_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'AI acknowledgement dispatch hold is immutable' USING ERRCODE='55000'; END $$;
CREATE TRIGGER ai_acknowledgement_dispatch_holds_immutable BEFORE UPDATE OR DELETE ON bidora_control_eventing.ai_acknowledgement_dispatch_holds FOR EACH ROW EXECUTE FUNCTION bidora_control_eventing.reject_ai_acknowledgement_dispatch_hold_change();

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=21,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=20;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 20'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM bidora_control_eventing.ai_acknowledgement_dispatch_holds) THEN RAISE EXCEPTION 'cannot roll back retained AI acknowledgement dispatch holds'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=20,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=21;
DROP TRIGGER ai_acknowledgement_dispatch_holds_immutable ON bidora_control_eventing.ai_acknowledgement_dispatch_holds;
DROP FUNCTION bidora_control_eventing.reject_ai_acknowledgement_dispatch_hold_change();
DROP TABLE bidora_control_eventing.ai_acknowledgement_dispatch_holds;
-- +goose StatementEnd
