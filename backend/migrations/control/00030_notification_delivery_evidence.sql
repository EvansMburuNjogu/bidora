-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

CREATE SCHEMA notification;

CREATE TABLE notification.notifications(
 id uuid PRIMARY KEY,
 tenant_id uuid REFERENCES bidora_tenancy.tenants(id) ON DELETE RESTRICT,
 actor_id uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
 recipient_user_id uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
 category text NOT NULL CHECK(length(category) BETWEEN 1 AND 80 AND category ~ '^[A-Z][A-Z0-9_]*$'),
 template_key text NOT NULL CHECK(length(template_key) BETWEEN 1 AND 100 AND template_key ~ '^[a-z][a-z0-9_.-]*$'),
 template_version bigint NOT NULL CHECK(template_version>0),
 source_type text NOT NULL CHECK(length(source_type) BETWEEN 1 AND 80 AND source_type ~ '^[A-Z][A-Z0-9_]*$'),
 source_id uuid NOT NULL,
 recipient_fingerprint bytea NOT NULL CHECK(octet_length(recipient_fingerprint)=32),
 content_fingerprint bytea NOT NULL CHECK(octet_length(content_fingerprint)=32),
 fingerprint_key_version integer NOT NULL CHECK(fingerprint_key_version>0),
 idempotency_key_hash bytea NOT NULL UNIQUE CHECK(octet_length(idempotency_key_hash)=32),
 correlation_id uuid NOT NULL,
 created_at timestamptz NOT NULL
);
CREATE INDEX notification_source_idx ON notification.notifications(source_type,source_id,created_at DESC,id DESC);
CREATE INDEX notification_recipient_user_idx ON notification.notifications(recipient_user_id,created_at DESC,id DESC) WHERE recipient_user_id IS NOT NULL;
CREATE INDEX notification_tenant_idx ON notification.notifications(tenant_id,created_at DESC,id DESC) WHERE tenant_id IS NOT NULL;

CREATE TABLE notification.delivery_attempts(
 id uuid PRIMARY KEY,
 notification_id uuid NOT NULL REFERENCES notification.notifications(id) ON DELETE RESTRICT,
 attempt_number integer NOT NULL CHECK(attempt_number BETWEEN 1 AND 100),
 smtp_setting_kind text NOT NULL DEFAULT 'SMTP' CHECK(smtp_setting_kind='SMTP'),
 smtp_version_id uuid NOT NULL,
 correlation_id uuid NOT NULL,
 started_at timestamptz NOT NULL,
 UNIQUE(notification_id,attempt_number),
 FOREIGN KEY(smtp_version_id,smtp_setting_kind) REFERENCES platform_settings.integration_versions(id,setting_kind) ON DELETE RESTRICT
);
CREATE INDEX notification_attempt_started_idx ON notification.delivery_attempts(started_at,id);

CREATE TABLE notification.delivery_attempt_results(
 id uuid PRIMARY KEY,
 attempt_id uuid NOT NULL UNIQUE REFERENCES notification.delivery_attempts(id) ON DELETE RESTRICT,
 disposition text NOT NULL CHECK(disposition IN('SMTP_ACCEPTED','FAILED','AMBIGUOUS')),
 failure_code text CHECK(failure_code IS NULL OR failure_code IN('CONNECTIVITY_FAILED','AUTHENTICATION_FAILED','TLS_FAILED','PROVIDER_REJECTED','TIMEOUT','PROCESS_INTERRUPTED','PROVIDER_OUTCOME_UNKNOWN','INTERNAL_ERROR')),
 correlation_id uuid NOT NULL,
 finished_at timestamptz NOT NULL,
 CHECK((disposition='SMTP_ACCEPTED' AND failure_code IS NULL) OR (disposition<>'SMTP_ACCEPTED' AND failure_code IS NOT NULL))
);
CREATE INDEX notification_result_finished_idx ON notification.delivery_attempt_results(finished_at,id);

CREATE FUNCTION notification.reject_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'notification delivery evidence is append-only' USING ERRCODE='55000'; END $$;
CREATE TRIGGER notifications_append_only BEFORE UPDATE OR DELETE ON notification.notifications FOR EACH ROW EXECUTE FUNCTION notification.reject_mutation();
CREATE TRIGGER notification_attempts_append_only BEFORE UPDATE OR DELETE ON notification.delivery_attempts FOR EACH ROW EXECUTE FUNCTION notification.reject_mutation();
CREATE TRIGGER notification_results_append_only BEFORE UPDATE OR DELETE ON notification.delivery_attempt_results FOR EACH ROW EXECUTE FUNCTION notification.reject_mutation();

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=30,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=29;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 29'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM notification.notifications) THEN RAISE EXCEPTION 'cannot roll back retained notification evidence'; END IF; END $$;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=29,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=30;
DROP TABLE notification.delivery_attempt_results;
DROP TABLE notification.delivery_attempts;
DROP TABLE notification.notifications;
DROP FUNCTION notification.reject_mutation();
DROP SCHEMA notification;
-- +goose StatementEnd
