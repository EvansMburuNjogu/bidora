-- +goose Up
-- +goose StatementBegin
-- BID-API-011C4 provider-neutral credit/refund maker-checker governance.
-- Provider refund dispatch and settlement remain excluded.
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.financial_changes) THEN RAISE EXCEPTION 'pre-governance financial changes require reviewed reconciliation before migration'; END IF; END $$;

ALTER TABLE commercial.financial_changes
 ADD COLUMN expected_invoice_version bigint NOT NULL CHECK(expected_invoice_version>0),
 ADD COLUMN expected_payment_version bigint NOT NULL CHECK(expected_payment_version>0),
 ADD COLUMN net_minor_units bigint NOT NULL CHECK(net_minor_units>=0),
 ADD COLUMN tax_minor_units bigint NOT NULL CHECK(tax_minor_units>=0),
 ADD COLUMN tax_profile_id uuid NOT NULL REFERENCES commercial.tax_profiles(id) ON DELETE RESTRICT,
 ADD COLUMN tax_profile_hash bytea NOT NULL CHECK(octet_length(tax_profile_hash)=32),
 ADD COLUMN maker_authority_hash bytea NOT NULL CHECK(octet_length(maker_authority_hash)=32),
 ADD COLUMN checker_authority_hash bytea CHECK(checker_authority_hash IS NULL OR octet_length(checker_authority_hash)=32),
 ADD COLUMN request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 ADD CONSTRAINT commercial_financial_change_amount_parts CHECK(amount_minor_units=net_minor_units+tax_minor_units);
DO $$ DECLARE constraint_name text; BEGIN
 SELECT c.conname INTO constraint_name FROM pg_constraint c WHERE c.conrelid='commercial.financial_changes'::regclass AND c.contype='c' AND pg_get_constraintdef(c.oid) LIKE '%PENDING_APPROVAL%' AND pg_get_constraintdef(c.oid) LIKE '%checker_user_id%' AND pg_get_constraintdef(c.oid) LIKE '%decision_reason%';
 IF constraint_name IS NULL THEN RAISE EXCEPTION 'financial change state-shape constraint not found'; END IF;
 EXECUTE format('ALTER TABLE commercial.financial_changes DROP CONSTRAINT %I',constraint_name);
END $$;
ALTER TABLE commercial.financial_changes ADD CONSTRAINT commercial_financial_change_state_shape CHECK(
 (state='PENDING_APPROVAL' AND checker_user_id IS NULL AND checker_authority_hash IS NULL AND decided_at IS NULL AND decision_reason IS NULL)
 OR (state IN('APPROVED','REJECTED') AND checker_user_id IS NOT NULL AND checker_authority_hash IS NOT NULL AND decided_at IS NOT NULL AND decision_reason IS NOT NULL)
 OR (state='EXPIRED' AND checker_user_id IS NULL AND checker_authority_hash IS NULL AND decided_at IS NOT NULL AND decision_reason IS NULL)
 OR (state='REVIEW_REQUIRED' AND checker_user_id IS NOT NULL AND checker_authority_hash IS NOT NULL AND decided_at IS NOT NULL AND decision_reason IS NOT NULL));

ALTER TABLE commercial.credit_notes ADD COLUMN change_id uuid UNIQUE REFERENCES commercial.financial_changes(id) ON DELETE RESTRICT;

CREATE TABLE commercial.financial_change_decisions(
 id uuid PRIMARY KEY,change_id uuid NOT NULL UNIQUE REFERENCES commercial.financial_changes(id) ON DELETE RESTRICT,
 decision text NOT NULL CHECK(decision IN('APPROVED','REJECTED')),checker_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 expected_change_version bigint NOT NULL CHECK(expected_change_version>0),reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000 AND reason=btrim(reason)),
 authority_hash bytea NOT NULL CHECK(octet_length(authority_hash)=32),decided_at timestamptz NOT NULL
);
CREATE TRIGGER financial_change_decisions_immutable BEFORE UPDATE OR DELETE ON commercial.financial_change_decisions FOR EACH ROW EXECUTE FUNCTION commercial.reject_financial_fact_change();

CREATE FUNCTION commercial.protect_financial_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='DELETE' OR OLD.state<>'PENDING_APPROVAL' OR NEW.state NOT IN('APPROVED','REJECTED','EXPIRED','REVIEW_REQUIRED')
 OR NEW.id IS DISTINCT FROM OLD.id OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.invoice_id IS DISTINCT FROM OLD.invoice_id OR NEW.payment_id IS DISTINCT FROM OLD.payment_id OR NEW.action IS DISTINCT FROM OLD.action OR NEW.reason_code IS DISTINCT FROM OLD.reason_code OR NEW.amount_minor_units IS DISTINCT FROM OLD.amount_minor_units OR NEW.net_minor_units IS DISTINCT FROM OLD.net_minor_units OR NEW.tax_minor_units IS DISTINCT FROM OLD.tax_minor_units OR NEW.tax_profile_id IS DISTINCT FROM OLD.tax_profile_id OR NEW.tax_profile_hash IS DISTINCT FROM OLD.tax_profile_hash OR NEW.entitlement_disposition IS DISTINCT FROM OLD.entitlement_disposition OR NEW.maker_user_id IS DISTINCT FROM OLD.maker_user_id OR NEW.maker_authority_hash IS DISTINCT FROM OLD.maker_authority_hash OR NEW.proposal_reason IS DISTINCT FROM OLD.proposal_reason OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.expires_at IS DISTINCT FROM OLD.expires_at OR NEW.expected_invoice_version IS DISTINCT FROM OLD.expected_invoice_version OR NEW.expected_payment_version IS DISTINCT FROM OLD.expected_payment_version OR NEW.request_hash IS DISTINCT FROM OLD.request_hash OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'commercial financial change transition invalid' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER financial_changes_protect BEFORE UPDATE OR DELETE ON commercial.financial_changes FOR EACH ROW EXECUTE FUNCTION commercial.protect_financial_change();

CREATE FUNCTION commercial.protect_credit_note_facts() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='DELETE' OR NEW.id IS DISTINCT FROM OLD.id OR NEW.invoice_id IS DISTINCT FROM OLD.invoice_id OR NEW.change_id IS DISTINCT FROM OLD.change_id OR NEW.credit_number IS DISTINCT FROM OLD.credit_number OR NEW.reason_code IS DISTINCT FROM OLD.reason_code OR NEW.currency IS DISTINCT FROM OLD.currency OR NEW.net_minor_units IS DISTINCT FROM OLD.net_minor_units OR NEW.tax_minor_units IS DISTINCT FROM OLD.tax_minor_units OR NEW.total_minor_units IS DISTINCT FROM OLD.total_minor_units OR NEW.original_tax_profile_id IS DISTINCT FROM OLD.original_tax_profile_id OR NEW.issued_by IS DISTINCT FROM OLD.issued_by OR NEW.issued_at IS DISTINCT FROM OLD.issued_at OR NEW.content_hash IS DISTINCT FROM OLD.content_hash OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'commercial credit note immutable facts changed' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER credit_notes_protect_facts BEFORE UPDATE ON commercial.credit_notes FOR EACH ROW EXECUTE FUNCTION commercial.protect_credit_note_facts();

CREATE FUNCTION commercial.protect_refund_facts() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='DELETE' OR NEW.id IS DISTINCT FROM OLD.id OR NEW.change_id IS DISTINCT FROM OLD.change_id OR NEW.payment_id IS DISTINCT FROM OLD.payment_id OR NEW.credit_note_id IS DISTINCT FROM OLD.credit_note_id OR NEW.provider IS DISTINCT FROM OLD.provider OR NEW.amount_minor_units IS DISTINCT FROM OLD.amount_minor_units OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'commercial refund immutable facts changed' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER refund_operations_protect_facts BEFORE UPDATE OR DELETE ON commercial.refund_operations FOR EACH ROW EXECUTE FUNCTION commercial.protect_refund_facts();

DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=25,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=24; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 24'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.financial_changes) OR EXISTS(SELECT 1 FROM commercial.credit_notes) OR EXISTS(SELECT 1 FROM commercial.refund_operations) THEN RAISE EXCEPTION 'cannot roll back adjustment governance migration with retained evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=24,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=25;
DROP TRIGGER refund_operations_protect_facts ON commercial.refund_operations; DROP FUNCTION commercial.protect_refund_facts();
DROP TRIGGER credit_notes_protect_facts ON commercial.credit_notes; DROP FUNCTION commercial.protect_credit_note_facts();
DROP TRIGGER financial_changes_protect ON commercial.financial_changes; DROP FUNCTION commercial.protect_financial_change();
DROP TRIGGER financial_change_decisions_immutable ON commercial.financial_change_decisions; DROP TABLE commercial.financial_change_decisions;
ALTER TABLE commercial.credit_notes DROP COLUMN change_id;
ALTER TABLE commercial.financial_changes DROP CONSTRAINT commercial_financial_change_state_shape,DROP CONSTRAINT commercial_financial_change_amount_parts,DROP COLUMN request_hash,DROP COLUMN checker_authority_hash,DROP COLUMN maker_authority_hash,DROP COLUMN tax_profile_hash,DROP COLUMN tax_profile_id,DROP COLUMN tax_minor_units,DROP COLUMN net_minor_units,DROP COLUMN expected_payment_version,DROP COLUMN expected_invoice_version;
ALTER TABLE commercial.financial_changes ADD CONSTRAINT financial_changes_state_shape CHECK((state='PENDING_APPROVAL' AND checker_user_id IS NULL AND decided_at IS NULL AND decision_reason IS NULL) OR (state IN('APPROVED','REJECTED') AND checker_user_id IS NOT NULL AND decided_at IS NOT NULL AND decision_reason IS NOT NULL) OR (state='EXPIRED' AND checker_user_id IS NULL AND decided_at IS NOT NULL));
-- +goose StatementEnd
