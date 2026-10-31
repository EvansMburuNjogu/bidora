-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE bid.workspaces ADD COLUMN archived_by uuid, ADD COLUMN archive_reason text, ADD COLUMN archive_policy_version integer,
 ADD CONSTRAINT workspace_archive_shape CHECK((archived_at IS NULL AND archived_by IS NULL AND archive_reason IS NULL AND archive_policy_version IS NULL) OR (archived_at IS NOT NULL AND archived_by IS NOT NULL AND archive_reason IS NOT NULL AND length(archive_reason) BETWEEN 1 AND 5000 AND archive_reason ~ '\S' AND archive_policy_version=1));
CREATE TABLE bid.archive_events(id uuid PRIMARY KEY,bid_id uuid NOT NULL UNIQUE REFERENCES bid.workspaces(id) ON DELETE RESTRICT,workspace_version bigint NOT NULL CHECK(workspace_version>0),archived_by uuid NOT NULL,reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 5000 AND reason ~ '\S'),policy_version integer NOT NULL CHECK(policy_version=1),archived_at timestamptz NOT NULL);
CREATE FUNCTION bid.reject_archived_workspace_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.archived_at IS NOT NULL THEN RAISE EXCEPTION 'archived workspace is immutable'; END IF; IF TG_OP='DELETE' THEN RETURN OLD; END IF; RETURN NEW; END $$;
CREATE TRIGGER workspaces_archived_immutable BEFORE UPDATE OR DELETE ON bid.workspaces FOR EACH ROW EXECUTE FUNCTION bid.reject_archived_workspace_change();
CREATE FUNCTION bid.reject_archive_event_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'archive event is immutable'; END $$;
CREATE TRIGGER archive_events_immutable BEFORE UPDATE OR DELETE ON bid.archive_events FOR EACH ROW EXECUTE FUNCTION bid.reject_archive_event_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_meta.schema_compatibility SET schema_version=25,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=24; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 24'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=24,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=25;
DROP TRIGGER archive_events_immutable ON bid.archive_events; DROP FUNCTION bid.reject_archive_event_change();
DROP TABLE bid.archive_events;
DROP TRIGGER workspaces_archived_immutable ON bid.workspaces; DROP FUNCTION bid.reject_archived_workspace_change();
ALTER TABLE bid.workspaces DROP CONSTRAINT workspace_archive_shape,DROP COLUMN archived_by,DROP COLUMN archive_reason,DROP COLUMN archive_policy_version;
-- +goose StatementEnd
