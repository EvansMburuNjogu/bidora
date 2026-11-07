-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE notification.message_snapshots (
 notification_id uuid PRIMARY KEY REFERENCES notification.notifications(id) ON DELETE RESTRICT,
 key_version integer NOT NULL CHECK(key_version>0),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 16 AND 65536),
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 expires_at timestamptz NOT NULL CHECK(expires_at>created_at)
);
CREATE INDEX notification_message_expiry_idx ON notification.message_snapshots(expires_at);
CREATE TRIGGER notification_message_no_update BEFORE UPDATE ON notification.message_snapshots
 FOR EACH ROW EXECUTE FUNCTION notification.reject_mutation();
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=54,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=53;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility must be at version 53'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM notification.message_snapshots) THEN RAISE EXCEPTION 'cannot drop retained message snapshots'; END IF; END $$;
DROP TABLE notification.message_snapshots;
UPDATE bidora_control_meta.schema_compatibility SET schema_version=53,updated_at=CURRENT_TIMESTAMP WHERE schema_version=54 AND migration_set='control_registry';
-- +goose StatementEnd
