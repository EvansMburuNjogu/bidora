-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Historical IDs are provenance, never login/permission authority. Keeping only
-- this non-personal key lets the actual user row and credentials be removed
-- without deleting immutable role, invitation, financial or audit records.
CREATE TABLE identity.user_references (
 id uuid PRIMARY KEY,
 created_at timestamptz NOT NULL,
 deleted_at timestamptz,
 CHECK(deleted_at IS NULL OR deleted_at>=created_at)
);
INSERT INTO identity.user_references(id,created_at) SELECT id,created_at FROM identity.users;

DO $$ DECLARE fk record; definition text; replacement text; BEGIN
 FOR fk IN SELECT oid,conrelid,conname FROM pg_constraint
 WHERE contype='f' AND confrelid='identity.users'::regclass
 AND conrelid NOT IN ('identity.authentication_identities'::regclass,'identity.challenges'::regclass,
 'identity.external_identities'::regclass,'identity.sessions'::regclass,'identity.password_recovery_challenges'::regclass,
 'identity.mfa_factors'::regclass,'identity.mfa_recovery_codes'::regclass,'identity.mfa_transactions'::regclass)
 LOOP
  definition:=pg_get_constraintdef(fk.oid);
  replacement:=replace(definition,'REFERENCES identity.users(','REFERENCES identity.user_references(');
  IF definition=replacement THEN RAISE EXCEPTION 'unsupported historical user reference'; END IF;
  EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I, ADD CONSTRAINT %I %s',fk.conrelid::regclass,fk.conname,fk.conname,replacement);
 END LOOP;
END $$;
ALTER TABLE identity.users ADD CONSTRAINT users_reference_id_fkey FOREIGN KEY(id) REFERENCES identity.user_references(id) ON DELETE RESTRICT;

CREATE FUNCTION identity.register_user_reference() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM identity.user_references WHERE id=NEW.id AND deleted_at IS NOT NULL) THEN
  RAISE EXCEPTION 'deleted identity IDs cannot be reused';
 END IF;
 INSERT INTO identity.user_references(id,created_at) VALUES(NEW.id,NEW.created_at) ON CONFLICT(id) DO NOTHING;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION identity.register_user_reference() FROM PUBLIC;
CREATE TRIGGER users_register_reference BEFORE INSERT ON identity.users FOR EACH ROW EXECUTE FUNCTION identity.register_user_reference();

CREATE FUNCTION identity.delete_disabled_user_secrets() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF OLD.disabled_at IS NULL THEN RAISE EXCEPTION 'disable the user before deletion'; END IF;
 -- Live-account FKs remain in place for authentication data. Purge these only
 -- inside the account DELETE transaction; any later failure rolls everything back.
 DELETE FROM identity.mfa_recovery_codes WHERE user_id=OLD.id;
 DELETE FROM identity.mfa_transactions WHERE user_id=OLD.id;
 DELETE FROM identity.mfa_factors WHERE user_id=OLD.id;
 DELETE FROM identity.password_recovery_challenges WHERE user_id=OLD.id;
 DELETE FROM identity.challenges WHERE user_id=OLD.id;
 DELETE FROM identity.external_identities WHERE user_id=OLD.id;
 DELETE FROM identity.sessions WHERE user_id=OLD.id;
 DELETE FROM identity.authentication_identities WHERE user_id=OLD.id;
 UPDATE identity.user_references SET deleted_at=clock_timestamp() WHERE id=OLD.id AND deleted_at IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'user reference missing or already deleted'; END IF;
 RETURN OLD;
END $$;
REVOKE ALL ON FUNCTION identity.delete_disabled_user_secrets() FROM PUBLIC;
CREATE TRIGGER users_delete_disabled_secrets BEFORE DELETE ON identity.users FOR EACH ROW EXECUTE FUNCTION identity.delete_disabled_user_secrets();

-- Preserve every existing validation, adding only the new executed lifecycle action.
DO $$ DECLARE c record; definition text; replacement text; BEGIN
 FOR c IN SELECT oid,conname FROM pg_constraint WHERE conrelid='platform_administration.administrator_changes'::regclass AND contype='c' LOOP
  definition:=pg_get_constraintdef(c.oid);
  IF position('REACTIVATE_IDENTITY' IN definition)>0 THEN
   replacement:=replace(definition,'''REACTIVATE_IDENTITY''::text','''REACTIVATE_IDENTITY''::text, ''DELETE_IDENTITY''::text');
   IF replacement=definition THEN RAISE EXCEPTION 'unsupported administrator action constraint'; END IF;
   EXECUTE format('ALTER TABLE platform_administration.administrator_changes DROP CONSTRAINT %I, ADD CONSTRAINT %I %s',c.conname,c.conname,replacement);
  END IF;
 END LOOP;
END $$;
DO $$ DECLARE definition text; replacement text; BEGIN
 definition:=pg_get_viewdef('bidora_control_audit.platform_action_times_v1'::regclass,true);
 replacement:=replace(definition,'''PlatformAdministratorReactivated''::text','''PlatformAdministratorReactivated''::text, ''PlatformAdministratorDeleted''::text');
 IF replacement=definition THEN RAISE EXCEPTION 'administrator deletion audit projection not found'; END IF;
 EXECUTE 'CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH(security_barrier=true) AS '||replacement;
END $$;
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=60,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=59;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 59'; END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'User deletion is forward-only; restore to a separate target for recovery'; END $$;
-- +goose StatementEnd
