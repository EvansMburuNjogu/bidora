-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
ALTER TABLE tenancy.invitations DROP CONSTRAINT invitations_capability_check;
ALTER TABLE tenancy.invitations ADD CONSTRAINT invitations_capability_check CHECK(capability IN('tenderpreneur.opportunity.capture_external','ROLE_BUNDLE_V1','ROLE_BUNDLE_V2'));
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=28,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=27; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 27'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM tenancy.invitations WHERE capability='ROLE_BUNDLE_V2') THEN RAISE EXCEPTION 'role bundle v2 invitation history prevents downgrade'; END IF; END $$;
ALTER TABLE tenancy.invitations DROP CONSTRAINT invitations_capability_check;
ALTER TABLE tenancy.invitations ADD CONSTRAINT invitations_capability_check CHECK(capability IN('tenderpreneur.opportunity.capture_external','ROLE_BUNDLE_V1'));
UPDATE bidora_control_meta.schema_compatibility SET schema_version=27,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=28;
-- +goose StatementEnd
