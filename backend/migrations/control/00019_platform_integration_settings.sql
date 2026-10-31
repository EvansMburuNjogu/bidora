-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE SCHEMA platform_settings;
CREATE TABLE platform_settings.integration_heads(
 setting_kind text PRIMARY KEY CHECK(setting_kind IN('SMTP','OPENAI')),next_sequence bigint NOT NULL DEFAULT 1 CHECK(next_sequence>0),active_version_id uuid,generation bigint NOT NULL DEFAULT 0 CHECK(generation>=0),updated_at timestamptz NOT NULL
);
CREATE TABLE platform_settings.integration_versions(
 id uuid PRIMARY KEY,setting_kind text NOT NULL REFERENCES platform_settings.integration_heads(setting_kind) ON DELETE RESTRICT,sequence bigint NOT NULL CHECK(sequence>0),
 state text NOT NULL CHECK(state IN('DRAFT','TEST_PASSED','TEST_FAILED','PENDING_APPROVAL','ACTIVE','SUPERSEDED','REJECTED')),
 version bigint NOT NULL CHECK(version>0),configuration_hash bytea NOT NULL CHECK(octet_length(configuration_hash)=32),latest_test_id uuid,
 created_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,creation_reason text NOT NULL CHECK(length(creation_reason) BETWEEN 1 AND 1000 AND creation_reason ~ '\S'),
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,activated_at timestamptz,superseded_at timestamptz,UNIQUE(setting_kind,sequence),UNIQUE(id,setting_kind),CHECK(updated_at>=created_at)
);
CREATE UNIQUE INDEX integration_versions_one_active_kind ON platform_settings.integration_versions(setting_kind) WHERE state='ACTIVE';
ALTER TABLE platform_settings.integration_heads ADD CONSTRAINT integration_heads_active_fk FOREIGN KEY(active_version_id,setting_kind) REFERENCES platform_settings.integration_versions(id,setting_kind) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE platform_settings.smtp_versions(
 version_id uuid PRIMARY KEY REFERENCES platform_settings.integration_versions(id) ON DELETE RESTRICT,host text NOT NULL CHECK(length(host) BETWEEN 1 AND 253),port integer NOT NULL CHECK(port BETWEEN 1 AND 65535),
 credential_secret_ref text NOT NULL CHECK(length(credential_secret_ref) BETWEEN 1 AND 500),trust_roots_secret_ref text NOT NULL CHECK(length(trust_roots_secret_ref) BETWEEN 1 AND 500),
 sender_name text NOT NULL CHECK(length(sender_name) BETWEEN 1 AND 200),sender_address text NOT NULL CHECK(length(sender_address) BETWEEN 3 AND 254),reply_to_address text CHECK(reply_to_address IS NULL OR length(reply_to_address) BETWEEN 3 AND 254),
 connect_timeout_seconds integer NOT NULL CHECK(connect_timeout_seconds BETWEEN 1 AND 30),command_timeout_seconds integer NOT NULL CHECK(command_timeout_seconds BETWEEN 1 AND 60),delivery_timeout_seconds integer NOT NULL CHECK(delivery_timeout_seconds BETWEEN 1 AND 120),max_concurrency integer NOT NULL CHECK(max_concurrency BETWEEN 1 AND 100),starttls_required boolean NOT NULL CHECK(starttls_required)
);
CREATE TABLE platform_settings.openai_versions(
 version_id uuid PRIMARY KEY REFERENCES platform_settings.integration_versions(id) ON DELETE RESTRICT,endpoint text NOT NULL CHECK(endpoint='https://api.openai.com/v1/responses'),model text NOT NULL CHECK(length(model) BETWEEN 1 AND 200),
 api_key_secret_ref text NOT NULL CHECK(length(api_key_secret_ref) BETWEEN 1 AND 500),trust_roots_secret_ref text NOT NULL CHECK(length(trust_roots_secret_ref) BETWEEN 1 AND 500),
 store_responses boolean NOT NULL CHECK(NOT store_responses),timeout_seconds integer NOT NULL CHECK(timeout_seconds BETWEEN 10 AND 900),max_concurrency integer NOT NULL CHECK(max_concurrency BETWEEN 1 AND 100)
);
CREATE TABLE platform_settings.integration_test_attempts(
 id uuid PRIMARY KEY,version_id uuid NOT NULL REFERENCES platform_settings.integration_versions(id) ON DELETE RESTRICT,attempt bigint NOT NULL CHECK(attempt>0),outcome text NOT NULL CHECK(outcome IN('PASSED','FAILED')),
 result_code text NOT NULL CHECK(result_code IN('SMTP_STARTTLS_VERIFIED','OPENAI_STRUCTURED_RESPONSE_VERIFIED','CONNECTIVITY_FAILED','AUTHENTICATION_FAILED','TLS_FAILED','PROVIDER_REJECTED','INVALID_RESPONSE')),
 applicability_hash bytea NOT NULL CHECK(octet_length(applicability_hash)=32),tested_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 1000 AND reason ~ '\S'),occurred_at timestamptz NOT NULL,UNIQUE(version_id,attempt),UNIQUE(id,version_id)
);
ALTER TABLE platform_settings.integration_versions ADD CONSTRAINT integration_versions_latest_test_fk FOREIGN KEY(latest_test_id,id) REFERENCES platform_settings.integration_test_attempts(id,version_id) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE platform_settings.integration_changes(
 id uuid PRIMARY KEY,setting_kind text NOT NULL,target_version_id uuid NOT NULL,action text NOT NULL CHECK(action IN('ACTIVATE','ROLLBACK')),
 state text NOT NULL CHECK(state IN('PENDING_APPROVAL','APPROVED_EXECUTED','REJECTED','EXPIRED')),version bigint NOT NULL CHECK(version>0),target_prior_state text NOT NULL CHECK(target_prior_state IN('TEST_PASSED','SUPERSEDED')),
 target_test_id uuid NOT NULL,expected_active_version_id uuid,maker_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,checker_user_id uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
 proposal_reason text NOT NULL CHECK(length(proposal_reason) BETWEEN 1 AND 1000 AND proposal_reason ~ '\S'),decision_reason text CHECK(decision_reason IS NULL OR (length(decision_reason) BETWEEN 1 AND 1000 AND decision_reason ~ '\S')),
 created_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,decided_at timestamptz,
 FOREIGN KEY(target_version_id,setting_kind) REFERENCES platform_settings.integration_versions(id,setting_kind) ON DELETE RESTRICT,
 FOREIGN KEY(expected_active_version_id,setting_kind) REFERENCES platform_settings.integration_versions(id,setting_kind) ON DELETE RESTRICT,
 FOREIGN KEY(target_test_id,target_version_id) REFERENCES platform_settings.integration_test_attempts(id,version_id) ON DELETE RESTRICT,
 CHECK(expires_at>created_at),CHECK((state='PENDING_APPROVAL' AND checker_user_id IS NULL AND decision_reason IS NULL AND decided_at IS NULL) OR (state='EXPIRED' AND checker_user_id IS NULL AND decision_reason IS NOT NULL AND decided_at IS NOT NULL) OR (state IN('APPROVED_EXECUTED','REJECTED') AND checker_user_id IS NOT NULL AND decision_reason IS NOT NULL AND decided_at IS NOT NULL)),CHECK(checker_user_id IS NULL OR checker_user_id<>maker_user_id)
);
CREATE UNIQUE INDEX integration_changes_one_pending_kind ON platform_settings.integration_changes(setting_kind) WHERE state='PENDING_APPROVAL';
CREATE TABLE platform_settings.integration_activation_history(
 id uuid PRIMARY KEY,setting_kind text NOT NULL,version_id uuid NOT NULL,previous_version_id uuid,change_id uuid NOT NULL UNIQUE REFERENCES platform_settings.integration_changes(id) ON DELETE RESTRICT,
 action text NOT NULL CHECK(action IN('ACTIVATE','ROLLBACK')),generation bigint NOT NULL CHECK(generation>0),activated_by uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,activated_at timestamptz NOT NULL,
 FOREIGN KEY(version_id,setting_kind) REFERENCES platform_settings.integration_versions(id,setting_kind) ON DELETE RESTRICT,
 FOREIGN KEY(previous_version_id,setting_kind) REFERENCES platform_settings.integration_versions(id,setting_kind) ON DELETE RESTRICT
);
CREATE TABLE platform_settings.integration_commands(
 id uuid PRIMARY KEY,actor_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,operation text NOT NULL CHECK(operation IN('CREATE_VERSION','TEST_VERSION','PROPOSE_ACTIVATION','APPROVE_CHANGE','REJECT_CHANGE')),
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),state text NOT NULL CHECK(state IN('PENDING','COMPLETED')),target_id uuid,response_state text,response_version bigint CHECK(response_version>0),response_status integer CHECK(response_status BETWEEN 200 AND 299),created_at timestamptz NOT NULL,completed_at timestamptz,UNIQUE(actor_user_id,operation,key_hash),
 CHECK((state='PENDING' AND target_id IS NOT NULL AND response_state IS NULL AND response_version IS NULL AND response_status IS NULL AND completed_at IS NULL) OR (state='COMPLETED' AND target_id IS NOT NULL AND response_state IS NOT NULL AND length(response_state) BETWEEN 1 AND 40 AND response_version IS NOT NULL AND response_status IS NOT NULL AND completed_at IS NOT NULL AND completed_at>=created_at))
);
CREATE FUNCTION platform_settings.reject_immutable_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'platform integration setting fact is immutable' USING ERRCODE='55000'; END $$;
CREATE TRIGGER smtp_versions_immutable BEFORE UPDATE OR DELETE ON platform_settings.smtp_versions FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
CREATE TRIGGER openai_versions_immutable BEFORE UPDATE OR DELETE ON platform_settings.openai_versions FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
CREATE TRIGGER integration_tests_immutable BEFORE UPDATE OR DELETE ON platform_settings.integration_test_attempts FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
CREATE TRIGGER integration_activation_history_immutable BEFORE UPDATE OR DELETE ON platform_settings.integration_activation_history FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
CREATE TRIGGER integration_commands_delete_immutable BEFORE DELETE ON platform_settings.integration_commands FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
CREATE FUNCTION platform_settings.protect_command_transition() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.state<>'PENDING' OR NEW.state<>'COMPLETED' OR NEW.id IS DISTINCT FROM OLD.id OR NEW.actor_user_id IS DISTINCT FROM OLD.actor_user_id OR NEW.operation IS DISTINCT FROM OLD.operation OR NEW.key_hash IS DISTINCT FROM OLD.key_hash OR NEW.request_hash IS DISTINCT FROM OLD.request_hash OR NEW.target_id IS DISTINCT FROM OLD.target_id OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN RAISE EXCEPTION 'invalid platform integration command transition' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER integration_commands_transition BEFORE UPDATE ON platform_settings.integration_commands FOR EACH ROW EXECUTE FUNCTION platform_settings.protect_command_transition();
CREATE FUNCTION platform_settings.protect_change_facts() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id IS DISTINCT FROM OLD.id OR NEW.setting_kind IS DISTINCT FROM OLD.setting_kind OR NEW.target_version_id IS DISTINCT FROM OLD.target_version_id OR NEW.action IS DISTINCT FROM OLD.action OR NEW.target_prior_state IS DISTINCT FROM OLD.target_prior_state OR NEW.target_test_id IS DISTINCT FROM OLD.target_test_id OR NEW.expected_active_version_id IS DISTINCT FROM OLD.expected_active_version_id OR NEW.maker_user_id IS DISTINCT FROM OLD.maker_user_id OR NEW.proposal_reason IS DISTINCT FROM OLD.proposal_reason OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.expires_at IS DISTINCT FROM OLD.expires_at THEN RAISE EXCEPTION 'platform integration change facts are immutable' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER integration_change_facts_immutable BEFORE UPDATE ON platform_settings.integration_changes FOR EACH ROW EXECUTE FUNCTION platform_settings.protect_change_facts();
CREATE TRIGGER integration_changes_delete_immutable BEFORE DELETE ON platform_settings.integration_changes FOR EACH ROW EXECUTE FUNCTION platform_settings.reject_immutable_change();
CREATE FUNCTION platform_settings.protect_version_facts() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id IS DISTINCT FROM OLD.id OR NEW.setting_kind IS DISTINCT FROM OLD.setting_kind OR NEW.sequence IS DISTINCT FROM OLD.sequence OR NEW.configuration_hash IS DISTINCT FROM OLD.configuration_hash OR NEW.created_by IS DISTINCT FROM OLD.created_by OR NEW.creation_reason IS DISTINCT FROM OLD.creation_reason OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN RAISE EXCEPTION 'platform integration version facts are immutable' USING ERRCODE='55000'; END IF; RETURN NEW; END $$;
CREATE TRIGGER integration_version_facts_immutable BEFORE UPDATE ON platform_settings.integration_versions FOR EACH ROW EXECUTE FUNCTION platform_settings.protect_version_facts();
INSERT INTO platform_settings.integration_heads(setting_kind,updated_at) VALUES('SMTP',CURRENT_TIMESTAMP),('OPENAI',CURRENT_TIMESTAMP);
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=19,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=18; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 18'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM platform_settings.integration_versions) OR EXISTS(SELECT 1 FROM platform_settings.integration_changes) THEN RAISE EXCEPTION 'cannot roll back platform integration settings with retained evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=18,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=19;
ALTER TABLE platform_settings.integration_versions DROP CONSTRAINT integration_versions_latest_test_fk; ALTER TABLE platform_settings.integration_heads DROP CONSTRAINT integration_heads_active_fk;
DROP TABLE platform_settings.integration_commands; DROP TABLE platform_settings.integration_activation_history; DROP TABLE platform_settings.integration_changes; DROP TABLE platform_settings.integration_test_attempts; DROP TABLE platform_settings.smtp_versions; DROP TABLE platform_settings.openai_versions; DROP TABLE platform_settings.integration_versions; DROP TABLE platform_settings.integration_heads;
DROP FUNCTION platform_settings.protect_version_facts(); DROP FUNCTION platform_settings.protect_change_facts(); DROP FUNCTION platform_settings.protect_command_transition(); DROP FUNCTION platform_settings.reject_immutable_change(); DROP SCHEMA platform_settings;
-- +goose StatementEnd
