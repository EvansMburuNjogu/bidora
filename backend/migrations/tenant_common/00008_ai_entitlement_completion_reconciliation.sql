-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

CREATE TABLE bidora_entitlement.ai_usage_completion_reconciliations(
 request_id uuid PRIMARY KEY REFERENCES bidora_entitlement.ai_usage_completion_requests(id) ON DELETE RESTRICT,
 state text NOT NULL CHECK(state IN('PENDING','LEASED','RETRY_WAIT','HELD','ACKNOWLEDGED')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 9),
 lease_id uuid,
 lease_expires_at timestamptz,
 next_attempt_at timestamptz,
 hold_code text CHECK(hold_code IS NULL OR hold_code IN('INVALID_REQUEST','ACCOUNTING_CONFLICT','STALE_ASSIGNMENT','GENERATION_INTEGRITY_CONFLICT','RECONCILIATION_EXHAUSTED')),
 updated_at timestamptz NOT NULL,
 CHECK((state='LEASED' AND lease_id IS NOT NULL AND lease_expires_at IS NOT NULL AND next_attempt_at IS NULL AND hold_code IS NULL) OR
       (state='RETRY_WAIT' AND lease_id IS NULL AND lease_expires_at IS NULL AND next_attempt_at IS NOT NULL AND hold_code IS NULL) OR
       (state='HELD' AND lease_id IS NULL AND lease_expires_at IS NULL AND next_attempt_at IS NULL AND hold_code IS NOT NULL) OR
       (state IN('PENDING','ACKNOWLEDGED') AND lease_id IS NULL AND lease_expires_at IS NULL AND next_attempt_at IS NULL AND hold_code IS NULL))
);
CREATE INDEX ai_usage_completion_reconciliation_due_idx ON bidora_entitlement.ai_usage_completion_reconciliations(next_attempt_at,request_id) WHERE state IN('PENDING','RETRY_WAIT');
CREATE INDEX ai_usage_completion_reconciliation_lease_idx ON bidora_entitlement.ai_usage_completion_reconciliations(lease_expires_at,request_id) WHERE state='LEASED';
CREATE INDEX ai_usage_completion_reconciliation_hold_idx ON bidora_entitlement.ai_usage_completion_reconciliations(hold_code,updated_at,request_id) WHERE state='HELD';

CREATE TABLE bidora_entitlement.ai_usage_completion_reconciliation_attempts(
 lease_id uuid PRIMARY KEY,
 request_id uuid NOT NULL REFERENCES bidora_entitlement.ai_usage_completion_requests(id) ON DELETE RESTRICT,
 attempt_number integer NOT NULL CHECK(attempt_number BETWEEN 1 AND 9),
 operation text NOT NULL CHECK(operation IN('SETTLE','RELEASE')),
 assignment_generation bigint NOT NULL CHECK(assignment_generation>0),
 claimed_at timestamptz NOT NULL,
 lease_expires_at timestamptz NOT NULL CHECK(lease_expires_at>claimed_at),
 UNIQUE(request_id,attempt_number)
);
CREATE TABLE bidora_entitlement.ai_usage_completion_reconciliation_outcomes(
 id uuid PRIMARY KEY,
 lease_id uuid NOT NULL UNIQUE REFERENCES bidora_entitlement.ai_usage_completion_reconciliation_attempts(lease_id) ON DELETE RESTRICT,
 outcome text NOT NULL CHECK(outcome IN('ACKNOWLEDGED','TRANSIENT_FAILURE','LEASE_EXPIRED','HELD')),
 result_code text NOT NULL CHECK(result_code IN('COMPLETED','TRANSIENT_UNAVAILABLE','AMBIGUOUS_INTERNAL_EFFECT','LOCAL_ACK_UNAVAILABLE','LEASE_EXPIRED','INVALID_REQUEST','ACCOUNTING_CONFLICT','STALE_ASSIGNMENT','GENERATION_INTEGRITY_CONFLICT','RECONCILIATION_EXHAUSTED')),
 occurred_at timestamptz NOT NULL,
 next_attempt_at timestamptz,
 CHECK((outcome='TRANSIENT_FAILURE' AND next_attempt_at IS NOT NULL) OR (outcome IN('ACKNOWLEDGED','HELD') AND next_attempt_at IS NULL) OR outcome='LEASE_EXPIRED'),
 CHECK(next_attempt_at IS NULL OR next_attempt_at>occurred_at)
);
CREATE TRIGGER ai_usage_completion_reconciliation_attempts_immutable BEFORE UPDATE OR DELETE ON bidora_entitlement.ai_usage_completion_reconciliation_attempts FOR EACH ROW EXECUTE FUNCTION bidora_entitlement.reject_checkpoint_transition_change();
CREATE TRIGGER ai_usage_completion_reconciliation_outcomes_immutable BEFORE UPDATE OR DELETE ON bidora_entitlement.ai_usage_completion_reconciliation_outcomes FOR EACH ROW EXECUTE FUNCTION bidora_entitlement.reject_checkpoint_transition_change();

INSERT INTO bidora_entitlement.ai_usage_completion_reconciliations(request_id,state,attempts,updated_at)
SELECT r.id,CASE WHEN a.id IS NULL THEN 'PENDING' ELSE 'ACKNOWLEDGED' END,0,COALESCE(a.acknowledged_at,r.requested_at)
FROM bidora_entitlement.ai_usage_completion_requests r
LEFT JOIN bidora_entitlement.ai_usage_completion_acknowledgements a ON a.request_id=r.id;

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_meta.schema_compatibility SET schema_version=8,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_common' AND schema_version=7;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'tenant_common compatibility row missing or not at version 7'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_completion_reconciliation_attempts) OR EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_completion_reconciliation_outcomes) OR EXISTS(SELECT 1 FROM bidora_entitlement.ai_usage_completion_reconciliations WHERE state NOT IN('PENDING','ACKNOWLEDGED') OR attempts<>0) THEN
  RAISE EXCEPTION 'cannot roll back retained AI completion reconciliation evidence';
 END IF;
END $$;
UPDATE bidora_meta.schema_compatibility SET schema_version=7,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_common' AND schema_version=8;
DROP TRIGGER ai_usage_completion_reconciliation_outcomes_immutable ON bidora_entitlement.ai_usage_completion_reconciliation_outcomes;
DROP TRIGGER ai_usage_completion_reconciliation_attempts_immutable ON bidora_entitlement.ai_usage_completion_reconciliation_attempts;
DROP TABLE bidora_entitlement.ai_usage_completion_reconciliation_outcomes;
DROP TABLE bidora_entitlement.ai_usage_completion_reconciliation_attempts;
DROP TABLE bidora_entitlement.ai_usage_completion_reconciliations;
-- +goose StatementEnd
