-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- HTTP metadata is installed transaction-locally before owner-module work.
-- Audit INSERT and metadata INSERT succeed or roll back together, including
-- session creation and email-code completion before a browser session exists.
CREATE FUNCTION bidora_control_audit.capture_request_metadata() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $$
DECLARE client_ip text; client_browser text;
BEGIN
 IF NEW.tenant_id IS NOT NULL OR NEW.actor_id IS NULL
 OR current_setting('bidora.audit_required',true) IS DISTINCT FROM '1' THEN RETURN NEW; END IF;
 -- Capture only control-plane Platform identities. No tenant-private or other
 -- account types' metadata is added to the Platform audit collection.
 IF NOT EXISTS (SELECT 1 FROM identity.users WHERE id=NEW.actor_id AND account_type='PLATFORM_ADMIN') THEN RETURN NEW; END IF;
 client_ip:=nullif(current_setting('bidora.audit_client_ip',true),'');
 client_browser:=nullif(current_setting('bidora.audit_browser',true),'');
 IF client_ip IS NULL OR client_browser IS NULL OR client_browser NOT IN
 ('Chrome','Firefox','Safari','Edge','Opera','Internet Explorer','Unknown browser') THEN
  RAISE EXCEPTION 'audit request metadata unavailable' USING ERRCODE='22023';
 END IF;
 INSERT INTO bidora_control_audit.request_metadata(correlation_id,actor_id,ip_address,browser,observed_at)
 VALUES(NEW.correlation_id,NEW.actor_id,client_ip::inet,client_browser,NEW.occurred_at)
 ON CONFLICT(correlation_id) DO NOTHING;
 IF NOT EXISTS (SELECT 1 FROM bidora_control_audit.request_metadata
 WHERE correlation_id=NEW.correlation_id AND actor_id=NEW.actor_id
 AND ip_address=client_ip::inet AND browser=client_browser) THEN
  RAISE EXCEPTION 'audit request metadata conflict' USING ERRCODE='22023';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION bidora_control_audit.capture_request_metadata() FROM PUBLIC;
CREATE TRIGGER control_audit_capture_request_metadata AFTER INSERT ON bidora_control_audit.audit_events
FOR EACH ROW EXECUTE FUNCTION bidora_control_audit.capture_request_metadata();
DO $$ BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=51,updated_at=CURRENT_TIMESTAMP
 WHERE migration_set='control_registry' AND schema_version=50;
 IF NOT FOUND THEN RAISE EXCEPTION 'control compatibility must be at 50'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'atomic request audit metadata requires forward recovery'; END $$;
-- +goose StatementEnd
