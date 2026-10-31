-- +goose Up
-- +goose StatementBegin
-- BID-API-011C provider-neutral financial foundation. No tax rate is seeded,
-- so invoice/payment runtime remains fail closed until a reviewed profile exists.
SET lock_timeout='1s'; SET statement_timeout='5s';

CREATE TABLE commercial.tax_profiles(
 id uuid PRIMARY KEY,sequence bigint NOT NULL UNIQUE CHECK(sequence>0),state text NOT NULL CHECK(state IN('DRAFT','VALIDATED','SCHEDULED','ACTIVE','SUPERSEDED','WITHDRAWN')),
 version bigint NOT NULL CHECK(version>0),country char(2) NOT NULL CHECK(country='KE'),currency char(3) NOT NULL CHECK(currency='KES'),
 tax_code text NOT NULL CHECK(length(tax_code) BETWEEN 1 AND 40 AND tax_code~'^[A-Z0-9_]+$'),display_label text NOT NULL CHECK(length(display_label) BETWEEN 1 AND 200 AND display_label=btrim(display_label)),
 rate_numerator bigint NOT NULL CHECK(rate_numerator>=0),rate_denominator bigint NOT NULL CHECK(rate_denominator BETWEEN 1 AND 1000000),
 rounding_profile text NOT NULL CHECK(rounding_profile='KES_COMMERCIAL_V1_HALF_UP_PER_TAX_CODE'),
 effective_from timestamptz NOT NULL,effective_until timestamptz,legal_review_reference_hash bytea NOT NULL CHECK(octet_length(legal_review_reference_hash)=32),
 configuration_hash bytea NOT NULL UNIQUE CHECK(octet_length(configuration_hash)=32),created_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,activated_at timestamptz,superseded_at timestamptz,
 CHECK(effective_until IS NULL OR effective_until>effective_from),CHECK(updated_at>=created_at),
 CHECK((state='ACTIVE' AND activated_at IS NOT NULL) OR state<>'ACTIVE')
);
CREATE UNIQUE INDEX commercial_one_active_tax_profile ON commercial.tax_profiles(country,currency) WHERE state='ACTIVE';

CREATE TABLE commercial.financial_number_heads(
 document_kind text NOT NULL CHECK(document_kind IN('INVOICE','CREDIT_NOTE','RECEIPT')),calendar_year integer NOT NULL CHECK(calendar_year BETWEEN 2020 AND 9999),
 next_sequence bigint NOT NULL DEFAULT 1 CHECK(next_sequence BETWEEN 1 AND 99999999),version bigint NOT NULL DEFAULT 1 CHECK(version>0),updated_at timestamptz NOT NULL,
 PRIMARY KEY(document_kind,calendar_year)
);

CREATE UNIQUE INDEX commercial_subscriptions_id_tenant_financial ON commercial.subscriptions(id,tenant_id);
CREATE TABLE commercial.invoices(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 purchase_operation_id uuid NOT NULL UNIQUE,invoice_number text NOT NULL UNIQUE CHECK(invoice_number~'^BDR-INV-[0-9]{4}-[0-9]{8}$'),
 state text NOT NULL CHECK(state IN('ISSUED','PAYMENT_PENDING','PAID','PARTIALLY_CREDITED','CREDITED','VOID','REVIEW_REQUIRED')),version bigint NOT NULL CHECK(version>0),
 purchase_kind text NOT NULL CHECK(purchase_kind IN('PLAN_BASE','ADDITIONAL_LICENCE')),quantity bigint NOT NULL CHECK(quantity>0),
 plan_key text NOT NULL CHECK(plan_key IN('BASIC','GROWTH','PROFESSIONAL','ENTERPRISE')),catalogue_version text NOT NULL CHECK(length(catalogue_version) BETWEEN 1 AND 100),
 cadence text NOT NULL CHECK(cadence IN('MONTHLY','ANNUAL','CONTRACT')),term_start timestamptz NOT NULL,term_end timestamptz NOT NULL,
 tax_profile_id uuid NOT NULL REFERENCES commercial.tax_profiles(id) ON DELETE RESTRICT,tax_profile_hash bytea NOT NULL CHECK(octet_length(tax_profile_hash)=32),arithmetic_profile text NOT NULL CHECK(arithmetic_profile='KES_COMMERCIAL_V1'),
 currency char(3) NOT NULL CHECK(currency='KES'),subtotal_minor_units bigint NOT NULL CHECK(subtotal_minor_units>=0),credit_minor_units bigint NOT NULL DEFAULT 0 CHECK(credit_minor_units>=0),
 taxable_minor_units bigint NOT NULL CHECK(taxable_minor_units>=0),tax_minor_units bigint NOT NULL CHECK(tax_minor_units>=0),total_minor_units bigint NOT NULL CHECK(total_minor_units>=0),
 outstanding_minor_units bigint NOT NULL CHECK(outstanding_minor_units>=0 AND outstanding_minor_units<=total_minor_units),refunded_minor_units bigint NOT NULL DEFAULT 0 CHECK(refunded_minor_units>=0 AND refunded_minor_units<=total_minor_units),
 seller_legal_name text NOT NULL CHECK(length(seller_legal_name) BETWEEN 1 AND 300 AND seller_legal_name=btrim(seller_legal_name)),seller_tax_identifier text NOT NULL CHECK(length(seller_tax_identifier) BETWEEN 1 AND 200 AND seller_tax_identifier=btrim(seller_tax_identifier)),
 seller_billing_address text NOT NULL CHECK(length(seller_billing_address) BETWEEN 1 AND 1000 AND seller_billing_address=btrim(seller_billing_address)),
 customer_legal_name text NOT NULL CHECK(length(customer_legal_name) BETWEEN 1 AND 300 AND customer_legal_name=btrim(customer_legal_name)),customer_tax_identifier text CHECK(customer_tax_identifier IS NULL OR (length(customer_tax_identifier) BETWEEN 1 AND 200 AND customer_tax_identifier=btrim(customer_tax_identifier))),
 customer_billing_address text NOT NULL CHECK(length(customer_billing_address) BETWEEN 1 AND 1000 AND customer_billing_address=btrim(customer_billing_address)),
 billing_snapshot_version bigint NOT NULL CHECK(billing_snapshot_version>0),billing_snapshot_hash bytea NOT NULL CHECK(octet_length(billing_snapshot_hash)=32),quote_hash bytea NOT NULL CHECK(octet_length(quote_hash)=32),
 issued_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,issued_at timestamptz NOT NULL,due_at timestamptz NOT NULL,voided_at timestamptz,void_reason text,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,
 CHECK(term_end>term_start),CHECK(due_at>=issued_at),CHECK(updated_at>=created_at),CHECK(subtotal_minor_units>=credit_minor_units),
 CHECK(taxable_minor_units=subtotal_minor_units-credit_minor_units),CHECK(total_minor_units=taxable_minor_units+tax_minor_units),
 CHECK((state='VOID' AND voided_at IS NOT NULL AND void_reason IS NOT NULL AND length(void_reason) BETWEEN 1 AND 2000) OR (state<>'VOID' AND voided_at IS NULL AND void_reason IS NULL)),
 FOREIGN KEY(subscription_id,tenant_id) REFERENCES commercial.subscriptions(id,tenant_id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX commercial_invoices_id_tenant ON commercial.invoices(id,tenant_id);
CREATE INDEX commercial_invoices_tenant_issued ON commercial.invoices(tenant_id,issued_at DESC,id DESC);
CREATE UNIQUE INDEX commercial_one_open_invoice_purchase ON commercial.invoices(tenant_id,purchase_operation_id) WHERE state NOT IN('VOID','CREDITED');

CREATE TABLE commercial.invoice_lines(
 id uuid PRIMARY KEY,invoice_id uuid NOT NULL REFERENCES commercial.invoices(id) ON DELETE RESTRICT,line_number integer NOT NULL CHECK(line_number BETWEEN 1 AND 100),
 line_kind text NOT NULL CHECK(line_kind IN('PLAN_BASE','ADDITIONAL_LICENCE','APPROVED_CREDIT','VAT')),description_code text NOT NULL CHECK(length(description_code) BETWEEN 1 AND 100 AND description_code~'^[A-Z0-9_]+$'),
 quantity bigint NOT NULL CHECK(quantity>0),unit_minor_units bigint NOT NULL CHECK(unit_minor_units>=0),proration_numerator bigint CHECK(proration_numerator IS NULL OR proration_numerator>=0),proration_denominator bigint CHECK(proration_denominator IS NULL OR proration_denominator>0),
 net_minor_units bigint NOT NULL CHECK(net_minor_units>=0),tax_code text CHECK(tax_code IS NULL OR (length(tax_code) BETWEEN 1 AND 40 AND tax_code~'^[A-Z0-9_]+$')),tax_minor_units bigint NOT NULL CHECK(tax_minor_units>=0),gross_minor_units bigint NOT NULL CHECK(gross_minor_units=net_minor_units+tax_minor_units),
 calculation_hash bytea NOT NULL CHECK(octet_length(calculation_hash)=32),UNIQUE(invoice_id,line_number)
);

CREATE TABLE commercial.payment_operations(
 id uuid PRIMARY KEY,invoice_id uuid NOT NULL REFERENCES commercial.invoices(id) ON DELETE RESTRICT,tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 provider text NOT NULL CHECK(provider='PESAPAL'),merchant_operation_id uuid NOT NULL UNIQUE,provider_order_reference text UNIQUE CHECK(provider_order_reference IS NULL OR length(provider_order_reference) BETWEEN 1 AND 300),
 state text NOT NULL CHECK(state IN('CREATED','PROVIDER_CREATE_PENDING','AWAITING_CUSTOMER','PROVIDER_PENDING','SETTLED','FAILED','CANCELLED','REVERSED','REFUND_PENDING','PARTIALLY_REFUNDED','REFUNDED','REVIEW_REQUIRED')),
 version bigint NOT NULL CHECK(version>0),currency char(3) NOT NULL CHECK(currency='KES'),amount_minor_units bigint NOT NULL CHECK(amount_minor_units>0),
 provider_configuration_version_id uuid NOT NULL,created_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,reconciled_at timestamptz,
 CHECK(updated_at>=created_at),FOREIGN KEY(invoice_id,tenant_id) REFERENCES commercial.invoices(id,tenant_id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX commercial_one_nonterminal_payment_invoice ON commercial.payment_operations(invoice_id) WHERE state IN('CREATED','PROVIDER_CREATE_PENDING','AWAITING_CUSTOMER','PROVIDER_PENDING','REVIEW_REQUIRED');

CREATE TABLE commercial.payment_observations(
 id uuid PRIMARY KEY,payment_id uuid NOT NULL REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,provider_event_key_hash bytea CHECK(provider_event_key_hash IS NULL OR octet_length(provider_event_key_hash)=32),
 source text NOT NULL CHECK(source IN('CREATE_RESPONSE','IPN_TRIGGER','STATUS_QUERY','REFUND_QUERY')),provider_state_code text NOT NULL CHECK(length(provider_state_code) BETWEEN 1 AND 100),
 normalized_state text NOT NULL CHECK(normalized_state IN('PENDING','SETTLED','FAILED','CANCELLED','REVERSED','PARTIALLY_REFUNDED','REFUNDED','UNKNOWN')),
 amount_minor_units bigint CHECK(amount_minor_units IS NULL OR amount_minor_units>=0),currency char(3) CHECK(currency IS NULL OR currency='KES'),raw_evidence_hash bytea NOT NULL CHECK(octet_length(raw_evidence_hash)=32),
 provider_observed_at timestamptz,recorded_at timestamptz NOT NULL,UNIQUE(payment_id,provider_event_key_hash)
);

CREATE TABLE commercial.receipts(
 id uuid PRIMARY KEY,payment_id uuid NOT NULL UNIQUE REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,invoice_id uuid NOT NULL REFERENCES commercial.invoices(id) ON DELETE RESTRICT,
 receipt_number text NOT NULL UNIQUE CHECK(receipt_number~'^BDR-RCT-[0-9]{4}-[0-9]{8}$'),currency char(3) NOT NULL CHECK(currency='KES'),amount_minor_units bigint NOT NULL CHECK(amount_minor_units>0),
 issued_at timestamptz NOT NULL,content_hash bytea NOT NULL UNIQUE CHECK(octet_length(content_hash)=32)
);

CREATE TABLE commercial.credit_notes(
 id uuid PRIMARY KEY,invoice_id uuid NOT NULL REFERENCES commercial.invoices(id) ON DELETE RESTRICT,credit_number text NOT NULL UNIQUE CHECK(credit_number~'^BDR-CRN-[0-9]{4}-[0-9]{8}$'),
 state text NOT NULL CHECK(state IN('ISSUED','REFUND_PENDING','SETTLED','REVIEW_REQUIRED','VOID')),version bigint NOT NULL CHECK(version>0),reason_code text NOT NULL CHECK(reason_code IN('SERVICE_CREDIT','DUPLICATE_PAYMENT_REFUND','SERVICE_FAILURE_REFUND','PAYMENT_REVERSAL_CORRECTION','OTHER_CONTRACTUAL_REFUND')),
 currency char(3) NOT NULL CHECK(currency='KES'),net_minor_units bigint NOT NULL CHECK(net_minor_units>=0),tax_minor_units bigint NOT NULL CHECK(tax_minor_units>=0),total_minor_units bigint NOT NULL CHECK(total_minor_units=net_minor_units+tax_minor_units AND total_minor_units>0),
 original_tax_profile_id uuid NOT NULL REFERENCES commercial.tax_profiles(id) ON DELETE RESTRICT,issued_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,issued_at timestamptz NOT NULL,content_hash bytea NOT NULL UNIQUE CHECK(octet_length(content_hash)=32)
);

CREATE TABLE commercial.financial_changes(
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,invoice_id uuid NOT NULL REFERENCES commercial.invoices(id) ON DELETE RESTRICT,payment_id uuid REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,
 action text NOT NULL CHECK(action IN('ISSUE_CREDIT','REQUEST_REFUND')),state text NOT NULL CHECK(state IN('PENDING_APPROVAL','APPROVED','REJECTED','EXPIRED','REVIEW_REQUIRED')),version bigint NOT NULL CHECK(version>0),
 reason_code text NOT NULL CHECK(reason_code IN('SERVICE_CREDIT','DUPLICATE_PAYMENT_REFUND','SERVICE_FAILURE_REFUND','PAYMENT_REVERSAL_CORRECTION','OTHER_CONTRACTUAL_REFUND')),
 amount_minor_units bigint NOT NULL CHECK(amount_minor_units>0),entitlement_disposition text NOT NULL CHECK(entitlement_disposition IN('NO_ENTITLEMENT_CHANGE','REDUCE_ADDITIONAL_LICENCES_AT_RENEWAL','END_PAID_ACCESS_ON_REFUND_SETTLEMENT','REVIEW_REQUIRED')),
 maker_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,checker_user_id uuid REFERENCES identity.users(id) ON DELETE RESTRICT,proposal_reason text NOT NULL CHECK(length(proposal_reason) BETWEEN 1 AND 2000 AND proposal_reason=btrim(proposal_reason)),decision_reason text,
 created_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,decided_at timestamptz,
 CHECK(expires_at=created_at+interval '30 minutes'),CHECK(checker_user_id IS NULL OR checker_user_id<>maker_user_id),
 CHECK((state='PENDING_APPROVAL' AND checker_user_id IS NULL AND decided_at IS NULL AND decision_reason IS NULL) OR (state IN('APPROVED','REJECTED') AND checker_user_id IS NOT NULL AND decided_at IS NOT NULL AND decision_reason IS NOT NULL) OR (state='EXPIRED' AND checker_user_id IS NULL AND decided_at IS NOT NULL))
);
CREATE UNIQUE INDEX commercial_one_pending_financial_change_invoice ON commercial.financial_changes(invoice_id) WHERE state='PENDING_APPROVAL';

CREATE TABLE commercial.refund_operations(
 id uuid PRIMARY KEY,change_id uuid NOT NULL UNIQUE REFERENCES commercial.financial_changes(id) ON DELETE RESTRICT,payment_id uuid NOT NULL REFERENCES commercial.payment_operations(id) ON DELETE RESTRICT,credit_note_id uuid REFERENCES commercial.credit_notes(id) ON DELETE RESTRICT,
 provider text NOT NULL CHECK(provider='PESAPAL'),provider_refund_reference text UNIQUE CHECK(provider_refund_reference IS NULL OR length(provider_refund_reference) BETWEEN 1 AND 300),
 state text NOT NULL CHECK(state IN('APPROVED','PROVIDER_PENDING','SETTLED','CANCELLED','REVIEW_REQUIRED')),version bigint NOT NULL CHECK(version>0),amount_minor_units bigint NOT NULL CHECK(amount_minor_units>0),created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,CHECK(updated_at>=created_at)
);

CREATE TABLE commercial.financial_commands(
 id uuid PRIMARY KEY,actor_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,tenant_id uuid REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 operation text NOT NULL CHECK(operation IN('ISSUE_INVOICE','INITIATE_PAYMENT','VOID_INVOICE','PROPOSE_ADJUSTMENT','APPROVE_ADJUSTMENT','REJECT_ADJUSTMENT','REQUEST_RECONCILIATION')),
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),target_id uuid NOT NULL,response_state text NOT NULL CHECK(length(response_state) BETWEEN 1 AND 40),response_version bigint NOT NULL CHECK(response_version>0),response_status integer NOT NULL CHECK(response_status BETWEEN 200 AND 299),created_at timestamptz NOT NULL,completed_at timestamptz NOT NULL CHECK(completed_at>=created_at),
 UNIQUE(actor_user_id,operation,key_hash)
);

CREATE FUNCTION commercial.reject_financial_fact_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'commercial financial fact is immutable' USING ERRCODE='55000'; END $$;
CREATE TRIGGER tax_profile_delete_immutable BEFORE DELETE ON commercial.tax_profiles FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();
CREATE FUNCTION commercial.protect_tax_profile_facts() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id IS DISTINCT FROM OLD.id OR NEW.sequence IS DISTINCT FROM OLD.sequence OR NEW.country IS DISTINCT FROM OLD.country OR NEW.currency IS DISTINCT FROM OLD.currency OR NEW.tax_code IS DISTINCT FROM OLD.tax_code OR NEW.display_label IS DISTINCT FROM OLD.display_label OR NEW.rate_numerator IS DISTINCT FROM OLD.rate_numerator OR NEW.rate_denominator IS DISTINCT FROM OLD.rate_denominator OR NEW.rounding_profile IS DISTINCT FROM OLD.rounding_profile OR NEW.effective_from IS DISTINCT FROM OLD.effective_from OR NEW.effective_until IS DISTINCT FROM OLD.effective_until OR NEW.legal_review_reference_hash IS DISTINCT FROM OLD.legal_review_reference_hash OR NEW.configuration_hash IS DISTINCT FROM OLD.configuration_hash OR NEW.created_by IS DISTINCT FROM OLD.created_by OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'commercial tax profile immutable facts changed' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER tax_profiles_protect_facts BEFORE UPDATE ON commercial.tax_profiles FOR EACH ROW EXECUTE FUNCTION commercial.protect_tax_profile_facts();
CREATE TRIGGER invoice_lines_immutable BEFORE UPDATE OR DELETE ON commercial.invoice_lines FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();
CREATE TRIGGER payment_observations_immutable BEFORE UPDATE OR DELETE ON commercial.payment_observations FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();
CREATE TRIGGER receipts_immutable BEFORE UPDATE OR DELETE ON commercial.receipts FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();
CREATE TRIGGER credit_notes_delete_immutable BEFORE DELETE ON commercial.credit_notes FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();
CREATE TRIGGER financial_commands_immutable BEFORE UPDATE OR DELETE ON commercial.financial_commands FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();
CREATE FUNCTION commercial.protect_invoice_facts() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id IS DISTINCT FROM OLD.id OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.subscription_id IS DISTINCT FROM OLD.subscription_id OR NEW.purchase_operation_id IS DISTINCT FROM OLD.purchase_operation_id OR NEW.invoice_number IS DISTINCT FROM OLD.invoice_number OR NEW.purchase_kind IS DISTINCT FROM OLD.purchase_kind OR NEW.quantity IS DISTINCT FROM OLD.quantity OR NEW.plan_key IS DISTINCT FROM OLD.plan_key OR NEW.catalogue_version IS DISTINCT FROM OLD.catalogue_version OR NEW.cadence IS DISTINCT FROM OLD.cadence OR NEW.term_start IS DISTINCT FROM OLD.term_start OR NEW.term_end IS DISTINCT FROM OLD.term_end OR NEW.tax_profile_id IS DISTINCT FROM OLD.tax_profile_id OR NEW.tax_profile_hash IS DISTINCT FROM OLD.tax_profile_hash OR NEW.currency IS DISTINCT FROM OLD.currency OR NEW.subtotal_minor_units IS DISTINCT FROM OLD.subtotal_minor_units OR NEW.credit_minor_units IS DISTINCT FROM OLD.credit_minor_units OR NEW.taxable_minor_units IS DISTINCT FROM OLD.taxable_minor_units OR NEW.tax_minor_units IS DISTINCT FROM OLD.tax_minor_units OR NEW.total_minor_units IS DISTINCT FROM OLD.total_minor_units OR NEW.seller_legal_name IS DISTINCT FROM OLD.seller_legal_name OR NEW.seller_tax_identifier IS DISTINCT FROM OLD.seller_tax_identifier OR NEW.seller_billing_address IS DISTINCT FROM OLD.seller_billing_address OR NEW.customer_legal_name IS DISTINCT FROM OLD.customer_legal_name OR NEW.customer_tax_identifier IS DISTINCT FROM OLD.customer_tax_identifier OR NEW.customer_billing_address IS DISTINCT FROM OLD.customer_billing_address OR NEW.billing_snapshot_version IS DISTINCT FROM OLD.billing_snapshot_version OR NEW.billing_snapshot_hash IS DISTINCT FROM OLD.billing_snapshot_hash OR NEW.quote_hash IS DISTINCT FROM OLD.quote_hash OR NEW.issued_by IS DISTINCT FROM OLD.issued_by OR NEW.issued_at IS DISTINCT FROM OLD.issued_at OR NEW.due_at IS DISTINCT FROM OLD.due_at OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'commercial invoice immutable facts changed' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER invoices_protect_facts BEFORE UPDATE ON commercial.invoices FOR EACH ROW EXECUTE FUNCTION commercial.protect_invoice_facts();

DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=22,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=21; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 21'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.invoices) OR EXISTS(SELECT 1 FROM commercial.tax_profiles) OR EXISTS(SELECT 1 FROM commercial.payment_operations) OR EXISTS(SELECT 1 FROM commercial.financial_changes) THEN RAISE EXCEPTION 'cannot roll back commercial financial migration with retained evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=21,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=22;
DROP TRIGGER invoices_protect_facts ON commercial.invoices; DROP FUNCTION commercial.protect_invoice_facts();
DROP TRIGGER financial_commands_immutable ON commercial.financial_commands; DROP TRIGGER credit_notes_delete_immutable ON commercial.credit_notes; DROP TRIGGER receipts_immutable ON commercial.receipts; DROP TRIGGER payment_observations_immutable ON commercial.payment_observations; DROP TRIGGER invoice_lines_immutable ON commercial.invoice_lines; DROP TRIGGER tax_profiles_protect_facts ON commercial.tax_profiles; DROP FUNCTION commercial.protect_tax_profile_facts(); DROP TRIGGER tax_profile_delete_immutable ON commercial.tax_profiles; DROP FUNCTION commercial.reject_financial_fact_change();
DROP TABLE commercial.financial_commands; DROP TABLE commercial.refund_operations; DROP TABLE commercial.financial_changes; DROP TABLE commercial.credit_notes; DROP TABLE commercial.receipts; DROP TABLE commercial.payment_observations; DROP TABLE commercial.payment_operations; DROP TABLE commercial.invoice_lines; DROP TABLE commercial.invoices; DROP INDEX commercial.commercial_subscriptions_id_tenant_financial; DROP TABLE commercial.financial_number_heads; DROP TABLE commercial.tax_profiles;
-- +goose StatementEnd
