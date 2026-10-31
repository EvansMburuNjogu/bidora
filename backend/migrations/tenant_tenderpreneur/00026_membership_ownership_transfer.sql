-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE TABLE bid.assignment_guards(user_id uuid PRIMARY KEY,blocked_by_operation_id uuid UNIQUE,version bigint NOT NULL DEFAULT 1 CHECK(version>0),updated_at timestamptz NOT NULL);
CREATE TABLE bid.membership_ownership_transfer_events(id uuid PRIMARY KEY,operation_id uuid NOT NULL,sequence integer NOT NULL CHECK(sequence>0),source_user_id uuid NOT NULL,target_user_id uuid NOT NULL CHECK(source_user_id<>target_user_id),workspace_count integer NOT NULL CHECK(workspace_count>=0),requirement_count integer NOT NULL CHECK(requirement_count>=0),review_count integer NOT NULL CHECK(review_count>=0),actor_id uuid NOT NULL,reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000 AND reason~'\S'),occurred_at timestamptz NOT NULL,UNIQUE(operation_id,sequence));
CREATE FUNCTION bid.reject_ownership_transfer_event_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'ownership transfer events are immutable'; END $$;
CREATE TRIGGER ownership_transfer_events_immutable BEFORE UPDATE OR DELETE ON bid.membership_ownership_transfer_events FOR EACH ROW EXECUTE FUNCTION bid.reject_ownership_transfer_event_change();
CREATE FUNCTION bid.assert_assignment_target_allowed(target uuid) RETURNS void LANGUAGE plpgsql AS $$ DECLARE blocked uuid; BEGIN IF target IS NULL THEN RETURN; END IF; INSERT INTO bid.assignment_guards(user_id,updated_at) VALUES(target,CURRENT_TIMESTAMP) ON CONFLICT(user_id) DO NOTHING; SELECT blocked_by_operation_id INTO blocked FROM bid.assignment_guards WHERE user_id=target FOR KEY SHARE; IF blocked IS NOT NULL THEN RAISE EXCEPTION 'assignment target is offboarding'; END IF; END $$;
CREATE FUNCTION bid.guard_workspace_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF TG_OP='INSERT' OR NEW.owner_id IS DISTINCT FROM OLD.owner_id THEN PERFORM bid.assert_assignment_target_allowed(NEW.owner_id); END IF; RETURN NEW; END $$;
CREATE FUNCTION bid.guard_requirement_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF TG_OP='INSERT' OR NEW.owner_id IS DISTINCT FROM OLD.owner_id THEN PERFORM bid.assert_assignment_target_allowed(NEW.owner_id); END IF; RETURN NEW; END $$;
CREATE FUNCTION bid.guard_review_assignee() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF TG_OP='INSERT' OR NEW.assigned_to IS DISTINCT FROM OLD.assigned_to THEN PERFORM bid.assert_assignment_target_allowed(NEW.assigned_to); END IF; RETURN NEW; END $$;
CREATE TRIGGER workspace_owner_assignment_guard BEFORE INSERT OR UPDATE OF owner_id ON bid.workspaces FOR EACH ROW EXECUTE FUNCTION bid.guard_workspace_owner();
CREATE TRIGGER requirement_owner_assignment_guard BEFORE INSERT OR UPDATE OF owner_id ON bid.requirements FOR EACH ROW EXECUTE FUNCTION bid.guard_requirement_owner();
CREATE TRIGGER review_assignee_assignment_guard BEFORE INSERT OR UPDATE OF assigned_to ON bid.review_threads FOR EACH ROW EXECUTE FUNCTION bid.guard_review_assignee();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_meta.schema_compatibility SET schema_version=26,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=25; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 25'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=25,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=26;
DROP TRIGGER review_assignee_assignment_guard ON bid.review_threads; DROP TRIGGER requirement_owner_assignment_guard ON bid.requirements; DROP TRIGGER workspace_owner_assignment_guard ON bid.workspaces;
DROP FUNCTION bid.guard_review_assignee(); DROP FUNCTION bid.guard_requirement_owner(); DROP FUNCTION bid.guard_workspace_owner(); DROP FUNCTION bid.assert_assignment_target_allowed(uuid);
DROP TRIGGER ownership_transfer_events_immutable ON bid.membership_ownership_transfer_events; DROP FUNCTION bid.reject_ownership_transfer_event_change();
DROP TABLE bid.membership_ownership_transfer_events; DROP TABLE bid.assignment_guards;
-- +goose StatementEnd
