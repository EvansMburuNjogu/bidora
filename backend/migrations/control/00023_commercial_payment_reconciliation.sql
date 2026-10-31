-- +goose Up
-- +goose StatementBegin
-- BID-API-011C2 provider-neutral payment creation intent and authoritative
-- query-before-retry reconciliation. No PesaPal protocol or entitlement effect.
SET lock_timeout='1s'; SET statement_timeout='5s';

ALTER TABLE commercial.payment_operations
 ADD COLUMN create_attempt_id uuid,
 ADD COLUMN create_dispatched_at timestamptz,
 ADD COLUMN reconciliation_state text NOT NULL DEFAULT 'NOT_REQUIRED' CHECK(reconciliation_state IN('NOT_REQUIRED','DUE','LEASED','RETRY_WAIT','HELD','COMPLETE')),
 ADD COLUMN reconciliation_reason text CHECK(reconciliation_reason IS NULL OR reconciliation_reason IN('CREATE_AMBIGUOUS','PROVIDER_PENDING','ACCOUNTING_COMPLETION_REQUIRED','INTEGRITY_CONFLICT','DELIVERY_EXHAUSTED')),
 ADD COLUMN reconciliation_attempts integer NOT NULL DEFAULT 0 CHECK(reconciliation_attempts BETWEEN 0 AND 9),
 ADD COLUMN reconciliation_due_at timestamptz,
 ADD COLUMN reconciliation_lease_id uuid,
 ADD COLUMN reconciliation_lease_expires_at timestamptz,
 ADD COLUMN last_queried_at timestamptz,
 ADD CONSTRAINT commercial_payment_create_intent CHECK((create_attempt_id IS NULL AND create_dispatched_at IS NULL) OR (create_attempt_id IS NOT NULL AND create_dispatched_at IS NOT NULL)),
 ADD CONSTRAINT commercial_payment_reconciliation_shape CHECK(
  (reconciliation_state IN('NOT_REQUIRED','COMPLETE') AND reconciliation_due_at IS NULL AND reconciliation_lease_id IS NULL AND reconciliation_lease_expires_at IS NULL)
  OR (reconciliation_state IN('DUE','RETRY_WAIT') AND reconciliation_due_at IS NOT NULL AND reconciliation_lease_id IS NULL AND reconciliation_lease_expires_at IS NULL)
  OR (reconciliation_state='LEASED' AND reconciliation_due_at IS NOT NULL AND reconciliation_lease_id IS NOT NULL AND reconciliation_lease_expires_at IS NOT NULL)
  OR (reconciliation_state='HELD' AND reconciliation_reason IS NOT NULL AND reconciliation_due_at IS NULL AND reconciliation_lease_id IS NULL AND reconciliation_lease_expires_at IS NULL)
 );
CREATE INDEX commercial_payment_reconciliation_due ON commercial.payment_operations(reconciliation_due_at,id) WHERE reconciliation_state IN('DUE','RETRY_WAIT');

CREATE TABLE commercial.payment_reconciliation_claims(
 id uuid PRIMARY KEY,payment_id uuid NOT NULL REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,
 attempt_number integer NOT NULL CHECK(attempt_number BETWEEN 1 AND 9),lease_id uuid NOT NULL,
 started_at timestamptz NOT NULL,lease_expires_at timestamptz NOT NULL CHECK(lease_expires_at=started_at+interval '2 minutes'),
 UNIQUE(payment_id,attempt_number),UNIQUE(payment_id,lease_id)
);
CREATE TABLE commercial.payment_reconciliation_outcomes(
 claim_id uuid PRIMARY KEY REFERENCES commercial.payment_reconciliation_claims(id) ON DELETE RESTRICT,
 outcome text NOT NULL CHECK(outcome IN('ORDER_ABSENT','PROVIDER_PENDING','TERMINAL_FAILED','TERMINAL_CANCELLED','ACCOUNTING_REQUIRED','RETRYABLE_OR_AMBIGUOUS','INTEGRITY_CONFLICT','DELIVERY_EXHAUSTED')),
 normalized_state text CHECK(normalized_state IS NULL OR normalized_state IN('PENDING','SETTLED','FAILED','CANCELLED','REVERSED','UNKNOWN')),
 provider_state_code text CHECK(provider_state_code IS NULL OR length(provider_state_code) BETWEEN 1 AND 100),
 raw_evidence_hash bytea CHECK(raw_evidence_hash IS NULL OR octet_length(raw_evidence_hash)=32),completed_at timestamptz NOT NULL
);
CREATE TRIGGER payment_reconciliation_claims_immutable BEFORE UPDATE OR DELETE ON commercial.payment_reconciliation_claims FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();
CREATE TRIGGER payment_reconciliation_outcomes_immutable BEFORE UPDATE OR DELETE ON commercial.payment_reconciliation_outcomes FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

CREATE FUNCTION commercial.protect_payment_facts() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.invoice_id IS DISTINCT FROM OLD.invoice_id OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.provider IS DISTINCT FROM OLD.provider OR NEW.merchant_operation_id IS DISTINCT FROM OLD.merchant_operation_id OR NEW.currency IS DISTINCT FROM OLD.currency OR NEW.amount_minor_units IS DISTINCT FROM OLD.amount_minor_units OR NEW.provider_configuration_version_id IS DISTINCT FROM OLD.provider_configuration_version_id OR NEW.created_by IS DISTINCT FROM OLD.created_by OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.create_attempt_id IS DISTINCT FROM OLD.create_attempt_id OR NEW.create_dispatched_at IS DISTINCT FROM OLD.create_dispatched_at OR NEW.version<>OLD.version+1 THEN
  RAISE EXCEPTION 'commercial payment immutable facts changed' USING ERRCODE='55000';
 END IF; RETURN NEW; END $$;
CREATE TRIGGER payment_operations_protect_facts BEFORE UPDATE ON commercial.payment_operations FOR EACH ROW EXECUTE FUNCTION commercial.protect_payment_facts();

DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=23,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=22; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 22'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.payment_reconciliation_claims) OR EXISTS(SELECT 1 FROM commercial.payment_operations WHERE create_attempt_id IS NOT NULL) THEN RAISE EXCEPTION 'cannot roll back payment reconciliation migration with retained evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=22,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=23;
DROP TRIGGER payment_operations_protect_facts ON commercial.payment_operations; DROP FUNCTION commercial.protect_payment_facts();
DROP TRIGGER payment_reconciliation_outcomes_immutable ON commercial.payment_reconciliation_outcomes; DROP TRIGGER payment_reconciliation_claims_immutable ON commercial.payment_reconciliation_claims;
DROP TABLE commercial.payment_reconciliation_outcomes; DROP TABLE commercial.payment_reconciliation_claims;
DROP INDEX commercial.commercial_payment_reconciliation_due;
ALTER TABLE commercial.payment_operations DROP CONSTRAINT commercial_payment_reconciliation_shape,DROP CONSTRAINT commercial_payment_create_intent,DROP COLUMN last_queried_at,DROP COLUMN reconciliation_lease_expires_at,DROP COLUMN reconciliation_lease_id,DROP COLUMN reconciliation_due_at,DROP COLUMN reconciliation_attempts,DROP COLUMN reconciliation_reason,DROP COLUMN reconciliation_state,DROP COLUMN create_dispatched_at,DROP COLUMN create_attempt_id;
-- +goose StatementEnd
