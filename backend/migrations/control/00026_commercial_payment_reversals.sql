-- +goose Up
-- +goose StatementBegin
-- BID-API-011C5 authoritative payment reversal and fail-closed access handling.
-- Provider querying is performed only by the existing separately configured boundary.
SET lock_timeout='1s'; SET statement_timeout='5s';

DO $$ DECLARE constraint_name text; BEGIN
 SELECT c.conname INTO constraint_name FROM pg_constraint c WHERE c.conrelid='commercial.payment_operations'::regclass AND c.contype='c' AND pg_get_constraintdef(c.oid) LIKE '%CREATE_AMBIGUOUS%' AND pg_get_constraintdef(c.oid) LIKE '%DELIVERY_EXHAUSTED%';
 IF constraint_name IS NULL THEN RAISE EXCEPTION 'payment reconciliation reason constraint not found'; END IF;
 EXECUTE format('ALTER TABLE commercial.payment_operations DROP CONSTRAINT %I',constraint_name);
END $$;
ALTER TABLE commercial.payment_operations ADD CONSTRAINT commercial_payment_reconciliation_reason_v2 CHECK(reconciliation_reason IS NULL OR reconciliation_reason IN('CREATE_AMBIGUOUS','PROVIDER_PENDING','POST_SETTLEMENT_RECHECK','ACCOUNTING_COMPLETION_REQUIRED','INTEGRITY_CONFLICT','DELIVERY_EXHAUSTED'));
DO $$ DECLARE constraint_name text; BEGIN
 SELECT c.conname INTO constraint_name FROM pg_constraint c WHERE c.conrelid='commercial.payment_reconciliation_outcomes'::regclass AND c.contype='c' AND pg_get_constraintdef(c.oid) LIKE '%ORDER_ABSENT%' AND pg_get_constraintdef(c.oid) LIKE '%DELIVERY_EXHAUSTED%';
 IF constraint_name IS NULL THEN RAISE EXCEPTION 'payment reconciliation outcome constraint not found'; END IF;
 EXECUTE format('ALTER TABLE commercial.payment_reconciliation_outcomes DROP CONSTRAINT %I',constraint_name);
END $$;
ALTER TABLE commercial.payment_reconciliation_outcomes ADD CONSTRAINT commercial_payment_reconciliation_outcome_v2 CHECK(outcome IN('ORDER_ABSENT','PROVIDER_PENDING','TERMINAL_FAILED','TERMINAL_CANCELLED','ACCOUNTING_REQUIRED','SETTLEMENT_CONFIRMED','RETRYABLE_OR_AMBIGUOUS','INTEGRITY_CONFLICT','DELIVERY_EXHAUSTED'));

CREATE TABLE commercial.payment_reconciliation_requests(
 id uuid PRIMARY KEY,payment_id uuid NOT NULL REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,
 actor_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,expected_payment_version bigint NOT NULL CHECK(expected_payment_version>0),
 reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000 AND reason=btrim(reason)),authority_hash bytea NOT NULL CHECK(octet_length(authority_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),requested_at timestamptz NOT NULL
);
CREATE TRIGGER payment_reconciliation_requests_immutable BEFORE UPDATE OR DELETE ON commercial.payment_reconciliation_requests FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

CREATE UNIQUE INDEX commercial_settlement_effect_payment_observation ON commercial.payment_settlement_effects(payment_id,observation_id);
CREATE UNIQUE INDEX commercial_payment_observation_id_payment ON commercial.payment_observations(id,payment_id);

CREATE TABLE commercial.subscription_access_holds(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 source_payment_id uuid NOT NULL UNIQUE REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,
 reason text NOT NULL CHECK(reason='PAYMENT_REVERSAL'),access_mode text NOT NULL CHECK(access_mode='RETAINED_READ_ONLY'),
 state text NOT NULL CHECK(state='ACTIVE'),activated_at timestamptz NOT NULL,effect_hash bytea NOT NULL UNIQUE CHECK(octet_length(effect_hash)=32),
 UNIQUE(id,source_payment_id,subscription_id),FOREIGN KEY(subscription_id,tenant_id) REFERENCES commercial.subscriptions(id,tenant_id) ON DELETE RESTRICT
);
CREATE TRIGGER subscription_access_holds_immutable BEFORE UPDATE OR DELETE ON commercial.subscription_access_holds FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

CREATE TABLE commercial.payment_reversal_effects(
 payment_id uuid PRIMARY KEY REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,
 invoice_id uuid NOT NULL UNIQUE REFERENCES commercial.invoices(id) ON DELETE RESTRICT,
 subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 settlement_observation_id uuid NOT NULL REFERENCES commercial.payment_observations(id) ON DELETE RESTRICT,
 reversal_observation_id uuid NOT NULL UNIQUE REFERENCES commercial.payment_observations(id) ON DELETE RESTRICT,
 access_hold_id uuid NOT NULL UNIQUE REFERENCES commercial.subscription_access_holds(id) ON DELETE RESTRICT,
 prior_payment_version bigint NOT NULL CHECK(prior_payment_version>0),resulting_payment_version bigint NOT NULL CHECK(resulting_payment_version=prior_payment_version+1),
 prior_invoice_version bigint NOT NULL CHECK(prior_invoice_version>0),resulting_invoice_version bigint NOT NULL CHECK(resulting_invoice_version=prior_invoice_version+1),
 prior_subscription_version bigint NOT NULL CHECK(prior_subscription_version>0),resulting_subscription_version bigint NOT NULL CHECK(resulting_subscription_version=prior_subscription_version+1),
 amount_minor_units bigint NOT NULL CHECK(amount_minor_units>0),currency char(3) NOT NULL CHECK(currency='KES'),
 provider_observed_at timestamptz,recorded_at timestamptz NOT NULL,applied_at timestamptz NOT NULL,
 settlement_effect_hash bytea NOT NULL CHECK(octet_length(settlement_effect_hash)=32),effect_hash bytea NOT NULL UNIQUE CHECK(octet_length(effect_hash)=32),
 FOREIGN KEY(payment_id,settlement_observation_id) REFERENCES commercial.payment_settlement_effects(payment_id,observation_id) ON DELETE RESTRICT,
 FOREIGN KEY(reversal_observation_id,payment_id) REFERENCES commercial.payment_observations(id,payment_id) ON DELETE RESTRICT,
 FOREIGN KEY(access_hold_id,payment_id,subscription_id) REFERENCES commercial.subscription_access_holds(id,source_payment_id,subscription_id) ON DELETE RESTRICT
);
CREATE TRIGGER payment_reversal_effects_immutable BEFORE UPDATE OR DELETE ON commercial.payment_reversal_effects FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=26,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=25; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 25'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.payment_reversal_effects) OR EXISTS(SELECT 1 FROM commercial.subscription_access_holds) OR EXISTS(SELECT 1 FROM commercial.payment_reconciliation_requests) OR EXISTS(SELECT 1 FROM commercial.payment_operations WHERE reconciliation_reason='POST_SETTLEMENT_RECHECK') THEN RAISE EXCEPTION 'cannot roll back payment reversal migration with retained evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=25,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=26;
DROP TRIGGER payment_reversal_effects_immutable ON commercial.payment_reversal_effects; DROP TABLE commercial.payment_reversal_effects;
DROP TRIGGER subscription_access_holds_immutable ON commercial.subscription_access_holds; DROP TABLE commercial.subscription_access_holds;
DROP TRIGGER payment_reconciliation_requests_immutable ON commercial.payment_reconciliation_requests; DROP TABLE commercial.payment_reconciliation_requests;
DROP INDEX commercial.commercial_payment_observation_id_payment; DROP INDEX commercial.commercial_settlement_effect_payment_observation;
ALTER TABLE commercial.payment_reconciliation_outcomes DROP CONSTRAINT commercial_payment_reconciliation_outcome_v2;
ALTER TABLE commercial.payment_reconciliation_outcomes ADD CONSTRAINT payment_reconciliation_outcomes_outcome_check CHECK(outcome IN('ORDER_ABSENT','PROVIDER_PENDING','TERMINAL_FAILED','TERMINAL_CANCELLED','ACCOUNTING_REQUIRED','RETRYABLE_OR_AMBIGUOUS','INTEGRITY_CONFLICT','DELIVERY_EXHAUSTED'));
ALTER TABLE commercial.payment_operations DROP CONSTRAINT commercial_payment_reconciliation_reason_v2;
ALTER TABLE commercial.payment_operations ADD CONSTRAINT payment_operations_reconciliation_reason_check CHECK(reconciliation_reason IS NULL OR reconciliation_reason IN('CREATE_AMBIGUOUS','PROVIDER_PENDING','ACCOUNTING_COMPLETION_REQUIRED','INTEGRITY_CONFLICT','DELIVERY_EXHAUSTED'));
-- +goose StatementEnd
