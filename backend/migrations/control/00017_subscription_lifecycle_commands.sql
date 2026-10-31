-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE commercial.subscription_commands DROP CONSTRAINT subscription_commands_operation_check;
ALTER TABLE commercial.subscription_commands ADD CONSTRAINT subscription_commands_operation_check CHECK(operation IN('START_TRIAL','SCHEDULE_CANCELLATION','WITHDRAW_CANCELLATION'));
CREATE TABLE commercial.subscription_transitions(
 id uuid PRIMARY KEY,subscription_id uuid NOT NULL REFERENCES commercial.subscriptions(id) ON DELETE RESTRICT,
 from_state text NOT NULL,to_state text NOT NULL,action text NOT NULL CHECK(action IN('SCHEDULE_CANCELLATION','WITHDRAW_CANCELLATION')),
 subscription_version bigint NOT NULL CHECK(subscription_version>1),actor_user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 occurred_at timestamptz NOT NULL,UNIQUE(subscription_id,subscription_version)
);
CREATE TRIGGER subscription_transitions_immutable BEFORE UPDATE OR DELETE ON commercial.subscription_transitions FOR EACH ROW EXECUTE FUNCTION commercial.reject_immutable_fact_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=17,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=16; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 16'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM commercial.subscription_commands WHERE operation IN('SCHEDULE_CANCELLATION','WITHDRAW_CANCELLATION')) OR EXISTS(SELECT 1 FROM commercial.subscription_transitions) THEN RAISE EXCEPTION 'cannot roll back commercial lifecycle migration with retained evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=16,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=17;
DROP TABLE commercial.subscription_transitions;
ALTER TABLE commercial.subscription_commands DROP CONSTRAINT subscription_commands_operation_check;
ALTER TABLE commercial.subscription_commands ADD CONSTRAINT subscription_commands_operation_check CHECK(operation='START_TRIAL');
-- +goose StatementEnd
