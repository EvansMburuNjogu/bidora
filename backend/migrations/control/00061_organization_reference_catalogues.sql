-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';

-- Stable codes remain available when a label changes or an entry is disabled.
-- Existing organization-creation free text is intentionally not rewritten.
CREATE TABLE platform_settings.organization_reference_catalogue (
 kind text NOT NULL CHECK(kind IN ('BUSINESS_TYPE','INDUSTRY')),
 code text NOT NULL CHECK(length(code) BETWEEN 1 AND 64 AND code ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
 label text NOT NULL CHECK(length(label) BETWEEN 1 AND 100 AND label=btrim(label)),
 enabled boolean NOT NULL,
 version bigint NOT NULL CHECK(version>0),
 updated_by uuid REFERENCES identity.users(id) ON DELETE RESTRICT,
 updated_at timestamptz NOT NULL,
 PRIMARY KEY(kind,code)
);
CREATE UNIQUE INDEX organization_reference_catalogue_label_idx ON platform_settings.organization_reference_catalogue(kind,lower(label));

INSERT INTO platform_settings.organization_reference_catalogue(kind,code,label,enabled,version,updated_at) VALUES
 ('BUSINESS_TYPE','sole-proprietorship','Sole proprietorship',true,1,CURRENT_TIMESTAMP),
 ('BUSINESS_TYPE','partnership','Partnership',true,1,CURRENT_TIMESTAMP),
 ('BUSINESS_TYPE','limited-company','Limited company',true,1,CURRENT_TIMESTAMP),
 ('BUSINESS_TYPE','nonprofit','Nonprofit',true,1,CURRENT_TIMESTAMP),
 ('BUSINESS_TYPE','cooperative','Cooperative',true,1,CURRENT_TIMESTAMP),
 ('BUSINESS_TYPE','other','Other',true,1,CURRENT_TIMESTAMP),
 ('INDUSTRY','construction','Construction',true,1,CURRENT_TIMESTAMP),
 ('INDUSTRY','information-technology','Information technology',true,1,CURRENT_TIMESTAMP),
 ('INDUSTRY','professional-services','Professional services',true,1,CURRENT_TIMESTAMP),
 ('INDUSTRY','healthcare','Healthcare',true,1,CURRENT_TIMESTAMP),
 ('INDUSTRY','manufacturing','Manufacturing',true,1,CURRENT_TIMESTAMP),
 ('INDUSTRY','agriculture','Agriculture',true,1,CURRENT_TIMESTAMP),
 ('INDUSTRY','other','Other',true,1,CURRENT_TIMESTAMP);

CREATE TABLE platform_settings.organization_reference_commands (
 actor_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),
 request_hash bytea NOT NULL CHECK(octet_length(request_hash)=32),
 kind text NOT NULL CHECK(kind IN ('BUSINESS_TYPE','INDUSTRY')),
 code text NOT NULL,
 result_version bigint NOT NULL CHECK(result_version>0),
 created_at timestamptz NOT NULL,
 PRIMARY KEY(actor_id,key_hash),
 FOREIGN KEY(kind,code) REFERENCES platform_settings.organization_reference_catalogue(kind,code) ON DELETE RESTRICT
);

-- Extend the existing restricted projection by exactly the two catalogue actions.
DO $$ DECLARE definition text; replacement text; BEGIN
 definition:=pg_get_viewdef('bidora_control_audit.platform_action_times_v1'::regclass,true);
 replacement:=replace(definition,'''CountryConfigurationUpdated''::text','''CountryConfigurationUpdated''::text, ''OrganizationBusinessTypeUpdated''::text, ''OrganizationIndustryUpdated''::text');
 IF replacement=definition THEN RAISE EXCEPTION 'regional audit projection not found'; END IF;
 EXECUTE 'CREATE OR REPLACE VIEW bidora_control_audit.platform_action_times_v1 WITH(security_barrier=true) AS '||replacement;
END $$;

DO $$ DECLARE affected integer; BEGIN
 UPDATE bidora_control_meta.schema_compatibility SET schema_version=61,updated_at=CURRENT_TIMESTAMP WHERE migration_set='control_registry' AND schema_version=60;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>1 THEN RAISE EXCEPTION 'control compatibility row missing or not at version 60'; END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'organization reference catalogue is forward-only'; END $$;
-- +goose StatementEnd
