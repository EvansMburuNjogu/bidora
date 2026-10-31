-- +goose Up
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
CREATE SCHEMA organization;
CREATE TABLE organization.company_profiles(
 singleton_key boolean PRIMARY KEY DEFAULT true CHECK(singleton_key),
 id uuid NOT NULL UNIQUE,
 registered_name text NOT NULL CHECK(length(registered_name) BETWEEN 1 AND 300 AND registered_name=btrim(registered_name)),
 trading_name text CHECK(trading_name IS NULL OR (length(trading_name) BETWEEN 1 AND 300 AND trading_name=btrim(trading_name))),
 business_type text NOT NULL CHECK(length(business_type) BETWEEN 1 AND 100 AND business_type=btrim(business_type)),
 primary_industry text NOT NULL CHECK(length(primary_industry) BETWEEN 1 AND 200 AND primary_industry=btrim(primary_industry)),
 description text NOT NULL CHECK(length(description) BETWEEN 1 AND 4000 AND description=btrim(description)),
 services_capabilities text NOT NULL CHECK(length(services_capabilities) BETWEEN 1 AND 10000 AND services_capabilities=btrim(services_capabilities)),
 work_email text NOT NULL CHECK(length(work_email) BETWEEN 3 AND 254 AND work_email=btrim(work_email)),
 work_phone text NOT NULL CHECK(length(work_phone) BETWEEN 3 AND 40 AND work_phone=btrim(work_phone)),
 preferred_language text NOT NULL CHECK(length(preferred_language) BETWEEN 2 AND 35 AND preferred_language=btrim(preferred_language)),
 locale text NOT NULL CHECK(length(locale) BETWEEN 2 AND 35 AND locale=btrim(locale)),
 website text CHECK(website IS NULL OR (length(website) BETWEEN 1 AND 2048 AND website=btrim(website))),
 postal_address text CHECK(postal_address IS NULL OR (length(postal_address) BETWEEN 1 AND 1000 AND postal_address=btrim(postal_address))),
 physical_address text CHECK(physical_address IS NULL OR (length(physical_address) BETWEEN 1 AND 1000 AND physical_address=btrim(physical_address))),
 registration_identifier text CHECK(registration_identifier IS NULL OR (length(registration_identifier) BETWEEN 1 AND 200 AND registration_identifier=btrim(registration_identifier))),
 tax_identifier text CHECK(tax_identifier IS NULL OR (length(tax_identifier) BETWEEN 1 AND 200 AND tax_identifier=btrim(tax_identifier))),
 version bigint NOT NULL CHECK(version>0), created_by uuid NOT NULL, updated_by uuid NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL CHECK(updated_at>=created_at)
);
CREATE TABLE organization.letterhead_versions(
 id uuid PRIMARY KEY, version_number bigint NOT NULL UNIQUE CHECK(version_number>0),
 registered_name text NOT NULL CHECK(length(registered_name) BETWEEN 1 AND 300 AND registered_name=btrim(registered_name)),
 trading_name text CHECK(trading_name IS NULL OR (length(trading_name) BETWEEN 1 AND 300 AND trading_name=btrim(trading_name))),
 registration_identifier text CHECK(registration_identifier IS NULL OR (length(registration_identifier) BETWEEN 1 AND 200 AND registration_identifier=btrim(registration_identifier))),
 tax_identifier text CHECK(tax_identifier IS NULL OR (length(tax_identifier) BETWEEN 1 AND 200 AND tax_identifier=btrim(tax_identifier))),
 postal_address text CHECK(postal_address IS NULL OR (length(postal_address) BETWEEN 1 AND 1000 AND postal_address=btrim(postal_address))),
 physical_address text CHECK(physical_address IS NULL OR (length(physical_address) BETWEEN 1 AND 1000 AND physical_address=btrim(physical_address))),
 phone text NOT NULL CHECK(length(phone) BETWEEN 3 AND 40 AND phone=btrim(phone)),
 email text NOT NULL CHECK(length(email) BETWEEN 3 AND 254 AND email=btrim(email)),
 website text CHECK(website IS NULL OR (length(website) BETWEEN 1 AND 2048 AND website=btrim(website))),
 slogan text CHECK(slogan IS NULL OR (length(slogan) BETWEEN 1 AND 300 AND slogan=btrim(slogan))),
 header_layout text NOT NULL CHECK(header_layout IN('COMPACT','STANDARD','FORMAL')),
 footer_layout text NOT NULL CHECK(footer_layout IN('MINIMAL','CONTACT','LEGAL')),
 page_number_style text NOT NULL CHECK(page_number_style IN('NONE','PAGE_X','PAGE_X_OF_Y')),
 primary_color char(7) NOT NULL CHECK(primary_color~'^#[0-9A-F]{6}$'),
 secondary_color char(7) NOT NULL CHECK(secondary_color~'^#[0-9A-F]{6}$'),
 disclaimer text CHECK(disclaimer IS NULL OR (length(disclaimer) BETWEEN 1 AND 2000 AND disclaimer=btrim(disclaimer))),
 template_key text NOT NULL CHECK(template_key IN('STANDARD_PROPOSAL')),
 content_hash bytea NOT NULL UNIQUE CHECK(octet_length(content_hash)=32), created_by uuid NOT NULL, created_at timestamptz NOT NULL
);
CREATE FUNCTION organization.reject_letterhead_version_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'letterhead versions are immutable'; END $$;
CREATE TRIGGER letterhead_versions_immutable BEFORE UPDATE OR DELETE ON organization.letterhead_versions FOR EACH ROW EXECUTE FUNCTION organization.reject_letterhead_version_change();
CREATE TABLE organization.letterhead_configuration(singleton_key boolean PRIMARY KEY DEFAULT true CHECK(singleton_key),active_version_id uuid REFERENCES organization.letterhead_versions(id) ON DELETE RESTRICT,next_version_number bigint NOT NULL DEFAULT 1 CHECK(next_version_number>0),version bigint NOT NULL DEFAULT 1 CHECK(version>0),updated_by uuid,updated_at timestamptz NOT NULL);
INSERT INTO organization.letterhead_configuration(singleton_key,updated_at) VALUES(true,CURRENT_TIMESTAMP);
CREATE TABLE organization.letterhead_activation_events(id uuid PRIMARY KEY,letterhead_version_id uuid NOT NULL REFERENCES organization.letterhead_versions(id) ON DELETE RESTRICT,previous_version_id uuid REFERENCES organization.letterhead_versions(id) ON DELETE RESTRICT,configuration_version bigint NOT NULL CHECK(configuration_version>1),actor_id uuid NOT NULL,reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 2000 AND reason=btrim(reason)),occurred_at timestamptz NOT NULL);
CREATE FUNCTION organization.reject_letterhead_activation_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'letterhead activation events are immutable'; END $$;
CREATE TRIGGER letterhead_activation_events_immutable BEFORE UPDATE OR DELETE ON organization.letterhead_activation_events FOR EACH ROW EXECUTE FUNCTION organization.reject_letterhead_activation_change();
DO $$ DECLARE affected integer; BEGIN UPDATE bidora_meta.schema_compatibility SET schema_version=27,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=26; GET DIAGNOSTICS affected=ROW_COUNT; IF affected<>1 THEN RAISE EXCEPTION 'tenant_tenderpreneur compatibility row missing or not at version 26'; END IF; END $$;
-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SET lock_timeout='1s'; SET statement_timeout='5s';
UPDATE bidora_meta.schema_compatibility SET schema_version=26,updated_at=CURRENT_TIMESTAMP WHERE migration_set='tenant_tenderpreneur' AND schema_version=27;
DROP TRIGGER letterhead_activation_events_immutable ON organization.letterhead_activation_events; DROP FUNCTION organization.reject_letterhead_activation_change();
DROP TABLE organization.letterhead_activation_events; DROP TABLE organization.letterhead_configuration;
DROP TRIGGER letterhead_versions_immutable ON organization.letterhead_versions; DROP FUNCTION organization.reject_letterhead_version_change();
DROP TABLE organization.letterhead_versions; DROP TABLE organization.company_profiles; DROP SCHEMA organization;
-- +goose StatementEnd
