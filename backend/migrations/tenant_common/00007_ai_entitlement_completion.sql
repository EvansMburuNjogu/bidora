-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

CREATE TABLE bidora_entitlement.ai_usage_completion_requests(
 id uuid PRIMARY KEY,
 checkpoint_id uuid NOT NULL UNIQUE REFERENCES bidora_entitlement.ai_usage_checkpoints(id) ON DELETE RESTRICT,
 reservation_id uuid NOT NULL,
 execution_id uuid NOT NULL UNIQUE,
 operation text NOT NULL CHECK(operation IN('SETTLE','RELEASE')),
 actual_credits bigint CHECK(actual_credits>0),
 correlation_id uuid NOT NULL,
 requested_at timestamptz NOT NULL,
 CHECK((operation='SETTLE' AND actual_credits IS NOT NULL) OR (operation='RELEASE' AND actual_credits IS NULL))
);
CREATE TABLE bidora_entitlement.ai_usage_completion_acknowledgements(
 id uuid PRIMARY KEY,
 request_id uuid NOT NULL UNIQUE REFERENCES bidora_entitlement.ai_usage_completion_requests(id) ON DELETE RESTRICT,
 checkpoint_id uuid NOT NULL UNIQUE REFERENCES bidora_entitlement.ai_usage_checkpoints(id) ON DELETE RESTRICT,
 reservation_id uuid NOT NULL,
 execution_id uuid NOT NULL UNIQUE,
 operation text NOT NULL CHECK(operation IN('SETTLE','RELEASE')),
 final_state text NOT NULL CHECK(final_state IN('SETTLED','RELEASED')),
 actual_credits bigint CHECK(actual_credits>0),
 acknowledged_at timestamptz NOT NULL,
 CHECK((operation='SETTLE' AND final_state='SETTLED' AND actual_credits IS NOT NULL) OR (operation='RELEASE' AND final_state='RELEASED' AND actual_credits IS NULL))
);
CREATE TRIGGER ai_usage_completion_requests_immutable BEFORE UPDATE OR DELETE ON bidora_entitlement.ai_usage_completion_requests FOR EACH ROW EXECUTE FUNCTION bidora_entitlement.reject_checkpoint_transition_change();
CREATE TRIGGER ai_usage_completion_acknowledgements_immutable BEFORE UPDATE OR DELETE ON bidora_entitlement.ai_usage_completion_acknowledgements FOR EACH ROW EXECUTE FUNCTION bidora_entitlement.reject_checkpoint_transition_change();

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=7,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_common' AND schema_version=6;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_common compatibility row missing or not at version 6'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_completion_requests) OR EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_completion_acknowledgements) OR EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_checkpoints WHERE state IN('SETTLE_PENDING','RELEASE_PENDING','SETTLED','RELEASED')) THEN
  RAISE EXCEPTION 'cannot roll back retained AI entitlement completion evidence';
 END IF;
END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=6,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_common' AND schema_version=7;
DROP TABLE bidora_entitlement.ai_usage_completion_acknowledgements;
DROP TABLE bidora_entitlement.ai_usage_completion_requests;
-- +goose StatementEnd
