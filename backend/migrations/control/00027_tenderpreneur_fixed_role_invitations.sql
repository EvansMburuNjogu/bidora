-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE tenancy.invitations SET invalidated_at=CURRENT_TIMESTAMP WHERE invalidated_at IS NULL AND consumed_at IS NULL;
ALTER TABLE tenancy.invitations DROP CONSTRAINT invitations_role_key_check;
ALTER TABLE tenancy.invitations DROP CONSTRAINT invitations_capability_check;
ALTER TABLE tenancy.invitations ADD CONSTRAINT invitations_role_key_check CHECK(role_key IN('TENDERPRENEUR_CAPTURE_CONTRIBUTOR','ORGANIZATION_ADMIN','BID_MANAGER','CONTRIBUTOR','APPROVER'));
ALTER TABLE tenancy.invitations ADD CONSTRAINT invitations_capability_check CHECK(capability IN('tenderpreneur.opportunity.capture_external','ROLE_BUNDLE_V1'));
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_control_meta.schema_compatibility SET schema_version=27,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=26; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 26'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
DO $$ BEGIN IF EXISTS(SELECT 1 FROM tenancy.invitations WHERE role_key<>'TENDERPRENEUR_CAPTURE_CONTRIBUTOR') THEN RAISE EXCEPTION 'fixed-role invitation history prevents downgrade'; END IF; END $$;
ALTER TABLE tenancy.invitations DROP CONSTRAINT invitations_role_key_check;
ALTER TABLE tenancy.invitations DROP CONSTRAINT invitations_capability_check;
ALTER TABLE tenancy.invitations ADD CONSTRAINT invitations_role_key_check CHECK(role_key='TENDERPRENEUR_CAPTURE_CONTRIBUTOR');
ALTER TABLE tenancy.invitations ADD CONSTRAINT invitations_capability_check CHECK(capability='tenderpreneur.opportunity.capture_external');
UPDATE bidora_control_meta.schema_compatibility SET schema_version=26,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=27;
-- +goose StatementEnd
