-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE commercial.usage_periods ADD COLUMN ai_credits_reserved bigint NOT NULL DEFAULT 0 CHECK(ai_credits_reserved>=0);
CREATE TABLE commercial.ai_usage_reservations(
 id uuid PRIMARY KEY,checkpoint_id uuid NOT NULL UNIQUE,assignment_generation bigint NOT NULL CHECK(assignment_generation>0),tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 usage_period_id uuid NOT NULL REFERENCES commercial.usage_periods(id) ON DELETE RESTRICT,
 source_operation_id uuid NOT NULL,operation_hash bytea NOT NULL CHECK(octet_length(operation_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 requester_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,kind text NOT NULL CHECK(kind IN('DOCUMENT_PDF','DOCUMENT_DOCX','GENERATION_TEXT')),
 estimated_credits bigint NOT NULL CHECK(estimated_credits>0),actual_credits bigint CHECK(actual_credits>0),
 status text NOT NULL CHECK(status IN('RESERVED','SETTLED','RELEASED')),version bigint NOT NULL CHECK(version>0),
 reserved_at timestamptz NOT NULL,settled_at timestamptz,released_at timestamptz,updated_at timestamptz NOT NULL,
 UNIQUE(tenant_id,source_operation_id),UNIQUE(tenant_id,operation_hash),
 CHECK((status='RESERVED' AND actual_credits IS NULL AND settled_at IS NULL AND released_at IS NULL) OR
       (status='SETTLED' AND actual_credits IS NOT NULL AND settled_at IS NOT NULL AND released_at IS NULL) OR
       (status='RELEASED' AND actual_credits IS NULL AND settled_at IS NULL AND released_at IS NOT NULL)),
 CHECK(updated_at>=reserved_at)
);
CREATE TABLE commercial.ai_usage_reservation_transitions(
 id uuid PRIMARY KEY,reservation_id uuid NOT NULL REFERENCES commercial.ai_usage_reservations(id) ON DELETE RESTRICT,
 from_status text NOT NULL CHECK(from_status='RESERVED'),to_status text NOT NULL CHECK(to_status IN('SETTLED','RELEASED')),
 reservation_version bigint NOT NULL CHECK(reservation_version=2),actual_credits bigint CHECK(actual_credits>0),execution_id uuid NOT NULL,occurred_at timestamptz NOT NULL,
 UNIQUE(reservation_id,reservation_version),CHECK((to_status='SETTLED')=(actual_credits IS NOT NULL))
);
CREATE TRIGGER ai_usage_reservation_transitions_immutable BEFORE UPDATE OR DELETE ON commercial.ai_usage_reservation_transitions FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=18,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=17; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 17'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.ai_usage_reservations) OR EXISTS(SELECT 1 FROM commercial.ai_usage_reservation_transitions) THEN RAISE EXCEPTION 'cannot roll back AI usage migration with retained accounting evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=17,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=18;
DROP TABLE commercial.ai_usage_reservation_transitions; DROP TABLE commercial.ai_usage_reservations;
ALTER TABLE commercial.usage_periods DROP COLUMN ai_credits_reserved;
-- +goose StatementEnd
