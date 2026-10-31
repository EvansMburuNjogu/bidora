-- +goose Up
-- +goose StatementBegin
-- BID-API-011C3 exact invoice entitlement snapshots and atomic settlement facts.
-- No provider protocol, callback, refund or runtime activation is introduced.
SET lock_timeout='1s'; SET statement_timeout='5s';

ALTER TABLE commercial.payment_reconciliation_outcomes ADD COLUMN observation_id uuid REFERENCES commercial.payment_observations(id) ON DELETE RESTRICT;
CREATE UNIQUE INDEX commercial_payment_accounting_observation ON commercial.payment_reconciliation_outcomes(observation_id) WHERE observation_id IS NOT NULL;

CREATE UNIQUE INDEX commercial_invoices_id_subscription ON commercial.invoices(id,subscription_id);
CREATE TABLE commercial.invoice_entitlement_snapshots(
 invoice_id uuid PRIMARY KEY REFERENCES commercial.invoices(id) ON DELETE RESTRICT,
 subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 effect_kind text NOT NULL CHECK(effect_kind IN('PLAN_ACTIVATION','ADDITIONAL_LICENCES')),
 expected_subscription_version bigint NOT NULL CHECK(expected_subscription_version>0),target_term_id uuid REFERENCES commercial.subscription_terms(id) ON DELETE RESTRICT,
 plan_key text NOT NULL CHECK(plan_key IN('BASIC','GROWTH','PROFESSIONAL','ENTERPRISE')),catalogue_version text NOT NULL CHECK(length(catalogue_version) BETWEEN 1 AND 100),cadence text NOT NULL CHECK(cadence IN('MONTHLY','ANNUAL','CONTRACT')),
 price_minor_units bigint,additional_licence_monthly_minor_units bigint,bid_workspace_limit bigint,included_licence_limit bigint,storage_limit_bytes bigint,document_version_limit bigint,ai_credit_limit bigint,concurrent_ai_job_limit integer,
 commercial_period_start timestamptz NOT NULL,commercial_period_end timestamptz NOT NULL,usage_period_start timestamptz,usage_period_end timestamptz,additional_licence_quantity bigint,
 snapshot_hash bytea NOT NULL UNIQUE CHECK(octet_length(snapshot_hash)=32),created_at timestamptz NOT NULL,
 FOREIGN KEY(invoice_id,subscription_id) REFERENCES commercial.invoices(id,subscription_id) ON DELETE RESTRICT,
 FOREIGN KEY(target_term_id,subscription_id) REFERENCES commercial.subscription_terms(id,subscription_id) ON DELETE RESTRICT,
 CHECK(commercial_period_end>commercial_period_start),
 CHECK((effect_kind='PLAN_ACTIVATION' AND target_term_id IS NULL AND price_minor_units IS NOT NULL AND price_minor_units>0 AND bid_workspace_limit>0 AND included_licence_limit>0 AND storage_limit_bytes>0 AND document_version_limit>0 AND ai_credit_limit>0 AND concurrent_ai_job_limit>0 AND usage_period_start IS NOT NULL AND usage_period_end IS NOT NULL AND usage_period_end>usage_period_start AND usage_period_end<=commercial_period_end AND additional_licence_quantity IS NULL)
 OR (effect_kind='ADDITIONAL_LICENCES' AND target_term_id IS NOT NULL AND price_minor_units IS NULL AND additional_licence_monthly_minor_units IS NULL AND bid_workspace_limit IS NULL AND included_licence_limit IS NULL AND storage_limit_bytes IS NULL AND document_version_limit IS NULL AND ai_credit_limit IS NULL AND concurrent_ai_job_limit IS NULL AND usage_period_start IS NULL AND usage_period_end IS NULL AND additional_licence_quantity>0))
);
CREATE TRIGGER invoice_entitlement_snapshots_immutable BEFORE UPDATE OR DELETE ON commercial.invoice_entitlement_snapshots FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

ALTER TABLE commercial.subscription_terms ADD COLUMN settlement_payment_id uuid UNIQUE REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT;

CREATE TABLE commercial.additional_licence_grants(
 id uuid PRIMARY KEY,subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,term_id uuid NOT NULL REFERENCES commercial.subscription_terms(id) ON DELETE RESTRICT,
 invoice_id uuid NOT NULL UNIQUE REFERENCES commercial.invoices(id) ON DELETE RESTRICT,payment_id uuid NOT NULL UNIQUE REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,
 quantity bigint NOT NULL CHECK(quantity>0),effective_from timestamptz NOT NULL,effective_until timestamptz NOT NULL CHECK(effective_until>effective_from),created_at timestamptz NOT NULL,
 UNIQUE(id,subscription_id),FOREIGN KEY(term_id,subscription_id) REFERENCES commercial.subscription_terms(id,subscription_id) ON DELETE RESTRICT
);
CREATE TRIGGER additional_licence_grants_immutable BEFORE UPDATE OR DELETE ON commercial.additional_licence_grants FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

CREATE TABLE commercial.payment_settlement_effects(
 payment_id uuid PRIMARY KEY REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,invoice_id uuid NOT NULL UNIQUE REFERENCES commercial.invoices(id) ON DELETE RESTRICT,
 observation_id uuid NOT NULL UNIQUE REFERENCES commercial.payment_observations(id) ON DELETE RESTRICT,receipt_id uuid NOT NULL UNIQUE REFERENCES commercial.receipts(id) ON DELETE RESTRICT,
 subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,effect_kind text NOT NULL CHECK(effect_kind IN('PLAN_ACTIVATION','ADDITIONAL_LICENCES')),
 resulting_term_id uuid REFERENCES commercial.subscription_terms(id) ON DELETE RESTRICT,additional_licence_grant_id uuid REFERENCES commercial.additional_licence_grants(id) ON DELETE RESTRICT,
 prior_subscription_version bigint NOT NULL CHECK(prior_subscription_version>0),resulting_subscription_version bigint NOT NULL CHECK(resulting_subscription_version=prior_subscription_version+1),
 amount_minor_units bigint NOT NULL CHECK(amount_minor_units>0),currency char(3) NOT NULL CHECK(currency='KES'),settled_at timestamptz NOT NULL,effect_hash bytea NOT NULL UNIQUE CHECK(octet_length(effect_hash)=32),
 CHECK((effect_kind='PLAN_ACTIVATION' AND resulting_term_id IS NOT NULL AND additional_licence_grant_id IS NULL) OR (effect_kind='ADDITIONAL_LICENCES' AND resulting_term_id IS NULL AND additional_licence_grant_id IS NOT NULL))
);
CREATE TRIGGER payment_settlement_effects_immutable BEFORE UPDATE OR DELETE ON commercial.payment_settlement_effects FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=24,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=23; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 23'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.invoice_entitlement_snapshots) OR EXISTS(SELECT 1 FROM commercial.payment_settlement_effects) OR EXISTS(SELECT 1 FROM commercial.additional_licence_grants) THEN RAISE EXCEPTION 'cannot roll back settlement entitlement migration with retained evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=23,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=24;
DROP TRIGGER payment_settlement_effects_immutable ON commercial.payment_settlement_effects; DROP TABLE commercial.payment_settlement_effects;
DROP TRIGGER additional_licence_grants_immutable ON commercial.additional_licence_grants; DROP TABLE commercial.additional_licence_grants;
ALTER TABLE commercial.subscription_terms DROP COLUMN settlement_payment_id;
DROP TRIGGER invoice_entitlement_snapshots_immutable ON commercial.invoice_entitlement_snapshots; DROP TABLE commercial.invoice_entitlement_snapshots; DROP INDEX commercial.commercial_invoices_id_subscription;
DROP INDEX commercial.commercial_payment_accounting_observation; ALTER TABLE commercial.payment_reconciliation_outcomes DROP COLUMN observation_id;
-- +goose StatementEnd
