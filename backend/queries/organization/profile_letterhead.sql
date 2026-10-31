-- name: GetCompanyProfile :one
SELECT * FROM organization.company_profiles WHERE singleton_key=true;
-- name: LockCompanyProfile :one
SELECT * FROM organization.company_profiles WHERE singleton_key=true FOR UPDATE;
-- name: InsertCompanyProfile :exec
INSERT INTO organization.company_profiles(singleton_key,id,registered_name,trading_name,business_type,primary_industry,description,services_capabilities,work_email,work_phone,preferred_language,locale,website,postal_address,physical_address,registration_identifier,tax_identifier,version,created_by,updated_by,created_at,updated_at)
VALUES(true,sqlc.arg(id),sqlc.arg(registered_name),sqlc.narg(trading_name),sqlc.arg(business_type),sqlc.arg(primary_industry),sqlc.arg(description),sqlc.arg(services_capabilities),sqlc.arg(work_email),sqlc.arg(work_phone),sqlc.arg(preferred_language),sqlc.arg(locale),sqlc.narg(website),sqlc.narg(postal_address),sqlc.narg(physical_address),sqlc.narg(registration_identifier),sqlc.narg(tax_identifier),1,sqlc.arg(actor_id),sqlc.arg(actor_id),sqlc.arg(at),sqlc.arg(at));
-- name: UpdateCompanyProfile :execrows
UPDATE organization.company_profiles SET registered_name=sqlc.arg(registered_name),trading_name=sqlc.narg(trading_name),business_type=sqlc.arg(business_type),primary_industry=sqlc.arg(primary_industry),description=sqlc.arg(description),services_capabilities=sqlc.arg(services_capabilities),work_email=sqlc.arg(work_email),work_phone=sqlc.arg(work_phone),preferred_language=sqlc.arg(preferred_language),locale=sqlc.arg(locale),website=sqlc.narg(website),postal_address=sqlc.narg(postal_address),physical_address=sqlc.narg(physical_address),registration_identifier=sqlc.narg(registration_identifier),tax_identifier=sqlc.narg(tax_identifier),version=version+1,updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(at) WHERE singleton_key=true AND version=sqlc.arg(expected_version);
-- name: LockLetterheadConfiguration :one
SELECT * FROM organization.letterhead_configuration WHERE singleton_key=true FOR UPDATE;
-- name: InsertLetterheadVersion :exec
INSERT INTO organization.letterhead_versions(id,version_number,registered_name,trading_name,registration_identifier,tax_identifier,postal_address,physical_address,phone,email,website,slogan,header_layout,footer_layout,page_number_style,primary_color,secondary_color,disclaimer,template_key,content_hash,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(version_number),sqlc.arg(registered_name),sqlc.narg(trading_name),sqlc.narg(registration_identifier),sqlc.narg(tax_identifier),sqlc.narg(postal_address),sqlc.narg(physical_address),sqlc.arg(phone),sqlc.arg(email),sqlc.narg(website),sqlc.narg(slogan),sqlc.arg(header_layout),sqlc.arg(footer_layout),sqlc.arg(page_number_style),sqlc.arg(primary_color),sqlc.arg(secondary_color),sqlc.narg(disclaimer),sqlc.arg(template_key),sqlc.arg(content_hash),sqlc.arg(actor_id),sqlc.arg(at));
-- name: AdvanceLetterheadSequence :execrows
UPDATE organization.letterhead_configuration SET next_version_number=next_version_number+1,version=version+1,updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(at) WHERE singleton_key=true AND version=sqlc.arg(expected_version) AND next_version_number=sqlc.arg(expected_next_version);
-- name: GetLetterheadVersion :one
SELECT * FROM organization.letterhead_versions WHERE id=sqlc.arg(id);
-- name: GetActiveLetterhead :one
SELECT v.*,c.version AS configuration_version,c.next_version_number,c.updated_by AS configuration_updated_by,c.updated_at AS configuration_updated_at FROM organization.letterhead_configuration c JOIN organization.letterhead_versions v ON v.id=c.active_version_id WHERE c.singleton_key=true;
-- name: ActivateLetterheadVersion :execrows
UPDATE organization.letterhead_configuration SET active_version_id=sqlc.arg(version_id),version=version+1,updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(at) WHERE singleton_key=true AND version=sqlc.arg(expected_version) AND EXISTS(SELECT 1 FROM organization.letterhead_versions WHERE id=sqlc.arg(version_id));
-- name: InsertLetterheadActivationEvent :exec
INSERT INTO organization.letterhead_activation_events(id,letterhead_version_id,previous_version_id,configuration_version,actor_id,reason,occurred_at) VALUES(sqlc.arg(id),sqlc.arg(version_id),sqlc.narg(previous_version_id),sqlc.arg(configuration_version),sqlc.arg(actor_id),sqlc.arg(reason),sqlc.arg(at));
