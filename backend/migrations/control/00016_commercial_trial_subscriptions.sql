-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE SCHEMA commercial;
CREATE TABLE commercial.subscriptions(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL UNIQUE REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 state text NOT NULL CHECK(state IN('TRIAL_ACTIVE','PAYMENT_PENDING','ACTIVE','PAST_DUE','SUSPENDED','ENDED','REVIEW_REQUIRED')),
 current_term_id uuid,version bigint NOT NULL CHECK(version>0),trial_started_at timestamptz,trial_ends_at timestamptz,
 grace_ends_at timestamptz,cancel_at timestamptz,pending_plan_key text,pending_cadence text,
 last_reconciled_state text CHECK(last_reconciled_state IS NULL OR last_reconciled_state IN('TRIAL_ACTIVE','ACTIVE','PAST_DUE')),
 last_reconciled_at timestamptz,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL CHECK(updated_at>=created_at),
 CHECK((trial_started_at IS NULL)=(trial_ends_at IS NULL)),CHECK(state<>'TRIAL_ACTIVE' OR trial_started_at IS NOT NULL),
 CHECK((pending_plan_key IS NULL)=(pending_cadence IS NULL)),CHECK(state<>'PAYMENT_PENDING' OR pending_plan_key IS NOT NULL),CHECK(state<>'PAST_DUE' OR grace_ends_at IS NOT NULL),
 CHECK((last_reconciled_state IS NULL)=(last_reconciled_at IS NULL))
);
CREATE TABLE commercial.subscription_terms(
 id uuid PRIMARY KEY,subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,term_version bigint NOT NULL CHECK(term_version>0),
 plan_key text NOT NULL CHECK(plan_key IN('TRIAL','BASIC','GROWTH','PROFESSIONAL','ENTERPRISE')),catalogue_version text NOT NULL CHECK(length(catalogue_version) BETWEEN 1 AND 100),
 cadence text NOT NULL CHECK(cadence IN('TRIAL','MONTHLY','ANNUAL','CONTRACT')),currency char(3) NOT NULL CHECK(currency='KES'),price_minor_units bigint NOT NULL CHECK(price_minor_units>=0),
 price_kind text NOT NULL CHECK(price_kind='FIXED'),additional_licence_monthly_minor_units bigint CHECK(additional_licence_monthly_minor_units>0),
 bid_workspace_limit bigint NOT NULL CHECK(bid_workspace_limit>0),included_licence_limit bigint NOT NULL CHECK(included_licence_limit>0),storage_limit_bytes bigint NOT NULL CHECK(storage_limit_bytes>0),
 document_version_limit bigint NOT NULL CHECK(document_version_limit>0),ai_credit_limit bigint NOT NULL CHECK(ai_credit_limit>0),concurrent_ai_job_limit integer NOT NULL CHECK(concurrent_ai_job_limit>0),
 commercial_period_start timestamptz NOT NULL,commercial_period_end timestamptz NOT NULL,usage_period_start timestamptz NOT NULL,usage_period_end timestamptz NOT NULL,
 created_at timestamptz NOT NULL,UNIQUE(subscription_id,term_version),UNIQUE(id,subscription_id),
 CHECK(commercial_period_end>commercial_period_start),CHECK(usage_period_end>usage_period_start AND usage_period_end<=commercial_period_end)
);
ALTER TABLE commercial.subscriptions ADD CONSTRAINT subscriptions_current_term_fk FOREIGN KEY(current_term_id,id) REFERENCES commercial.subscription_terms(id,subscription_id) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE commercial.usage_periods(
 id uuid PRIMARY KEY,subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,term_id uuid NOT NULL REFERENCES commercial.subscription_terms(id) ON DELETE RESTRICT,
 period_start timestamptz NOT NULL,period_end timestamptz NOT NULL,bid_workspaces_consumed bigint NOT NULL DEFAULT 0 CHECK(bid_workspaces_consumed>=0),
 document_versions_consumed bigint NOT NULL DEFAULT 0 CHECK(document_versions_consumed>=0),ai_credits_consumed bigint NOT NULL DEFAULT 0 CHECK(ai_credits_consumed>=0),
 storage_bytes_used bigint NOT NULL DEFAULT 0 CHECK(storage_bytes_used>=0),concurrent_ai_jobs integer NOT NULL DEFAULT 0 CHECK(concurrent_ai_jobs>=0),version bigint NOT NULL CHECK(version>0),created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,
 UNIQUE(subscription_id,period_start),CHECK(period_end>period_start),CHECK(updated_at>=created_at)
);
CREATE TABLE commercial.trial_lifetime_markers(
 tenant_id uuid PRIMARY KEY REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,subscription_id uuid NOT NULL UNIQUE REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 reason text NOT NULL CHECK(reason IN('TRIAL_STARTED','PAID_ACTIVATED_WITHOUT_TRIAL')),initiating_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 execution_id uuid NOT NULL,marked_at timestamptz NOT NULL
);
CREATE TABLE commercial.subscription_commands(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,operation text NOT NULL CHECK(operation='START_TRIAL'),
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 response_version bigint NOT NULL CHECK(response_version>0),created_at timestamptz NOT NULL,completed_at timestamptz NOT NULL CHECK(completed_at>=created_at),UNIQUE(tenant_id,operation,key_hash)
);
CREATE FUNCTION commercial.reject_immutable_fact_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'commercial fact is immutable' USING ERRCODE='55000'; END $$;
CREATE TRIGGER subscription_terms_immutable BEFORE UPDATE OR DELETE ON commercial.subscription_terms FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
CREATE TRIGGER trial_lifetime_markers_immutable BEFORE UPDATE OR DELETE ON commercial.trial_lifetime_markers FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
CREATE TRIGGER subscription_commands_immutable BEFORE UPDATE OR DELETE ON commercial.subscription_commands FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=16,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=15; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 15'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_control_meta.schema_compatibility SET schema_version=15,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=16;
DROP TABLE commercial.subscription_commands; DROP TABLE commercial.trial_lifetime_markers; DROP TABLE commercial.usage_periods;
ALTER TABLE commercial.subscriptions DROP CONSTRAINT subscriptions_current_term_fk; DROP TABLE commercial.subscription_terms; DROP TABLE commercial.subscriptions;
DROP FUNCTION commercial.reject_immutable_fact_change(); DROP SCHEMA commercial;
-- +goose StatementEnd
