-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
-- Existing schema-57 sandbox snapshots already contain these immutable columns.
ALTER TABLE platform_administration.administrator_invitations
 ADD COLUMN IF NOT EXISTS first_name text NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS last_name text NOT NULL DEFAULT '';
ALTER TABLE platform_administration.administrator_invitations
 ADD CONSTRAINT administrator_invitation_person_names_v59_check CHECK(
 octet_length(first_name)<=100 AND first_name=btrim(first_name) AND first_name !~ '[[:cntrl:]]'
 AND octet_length(last_name)<=100 AND last_name=btrim(last_name) AND last_name !~ '[[:cntrl:]]'
 AND ((first_name='' AND last_name='') OR (first_name<>'' AND last_name<>'')));
CREATE OR REPLACE FUNCTION platform_administration.guard_invitation_person_names() RETURNS trigger LANGUAGE plpgsql SET search_path TO pg_catalog AS $$ BEGIN
 IF NEW.first_name IS DISTINCT FROM OLD.first_name OR NEW.last_name IS DISTINCT FROM OLD.last_name THEN
  RAISE EXCEPTION 'Invitation names are immutable' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='platform_administration.administrator_invitations'::regclass AND tgname='guard_invitation_person_names') THEN
  CREATE TRIGGER guard_invitation_person_names BEFORE UPDATE ON platform_administration.administrator_invitations FOR EACH ROW EXECUTE FUNCTION platform_administration.guard_invitation_person_names();
 END IF;
END $$;
DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=59,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=58;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 58'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DO $$ BEGIN RAISE EXCEPTION 'Forward-only migration: retain invitation names'; END $$;
