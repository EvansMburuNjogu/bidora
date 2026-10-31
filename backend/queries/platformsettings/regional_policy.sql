-- name: ListConfiguredCountries :many
SELECT * FROM platform_settings.countries ORDER BY country_code LIMIT 249;
-- name: ReadCountryConfiguration :one
SELECT * FROM platform_settings.countries WHERE country_code=sqlc.arg(country_code);
-- name: FenceCountryConfiguration :one
SELECT * FROM platform_settings.countries WHERE country_code=sqlc.arg(country_code) AND enabled FOR SHARE;
-- name: ClearDefaultCountry :exec
UPDATE platform_settings.countries SET is_default=false,version=version+1,updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(at) WHERE is_default AND country_code<>sqlc.arg(country_code);
-- name: SaveCountryConfiguration :execrows
INSERT INTO platform_settings.countries(country_code,currency,language,timezone,locale,enabled,is_default,version,updated_by,updated_at)
SELECT sqlc.arg(country_code),sqlc.arg(currency),sqlc.arg(language),sqlc.arg(timezone),sqlc.arg(locale),sqlc.arg(enabled),sqlc.arg(is_default),1,sqlc.arg(actor_id),sqlc.arg(at) WHERE sqlc.arg(expected_version)::bigint=0
ON CONFLICT(country_code) DO NOTHING;
-- name: UpdateCountryConfiguration :execrows
UPDATE platform_settings.countries SET currency=sqlc.arg(currency),language=sqlc.arg(language),timezone=sqlc.arg(timezone),locale=sqlc.arg(locale),enabled=sqlc.arg(enabled),is_default=sqlc.arg(is_default),version=version+1,updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(at) WHERE country_code=sqlc.arg(country_code) AND version=sqlc.arg(expected_version);
-- name: CountDefaultCountries :one
SELECT count(*)::bigint FROM platform_settings.countries WHERE is_default AND enabled;
-- name: ReadCountryVersions :many
SELECT * FROM platform_settings.country_versions WHERE country_code=sqlc.arg(country_code) ORDER BY version DESC LIMIT 25;
-- name: ReadOperationalPolicy :one
SELECT * FROM platform_settings.operational_policy WHERE singleton;
-- name: FenceOperationalPolicy :one
SELECT * FROM platform_settings.operational_policy WHERE singleton FOR SHARE;
-- name: SaveOperationalPolicy :execrows
UPDATE platform_settings.operational_policy SET version=version+1,organization_registration_enabled=sqlc.arg(organization_registration_enabled),ai_features_enabled=sqlc.arg(ai_features_enabled),document_uploads_enabled=sqlc.arg(document_uploads_enabled),max_document_bytes=sqlc.arg(max_document_bytes),maintenance_enabled=sqlc.arg(maintenance_enabled),maintenance_message=sqlc.arg(maintenance_message),maintenance_starts_at=sqlc.narg(maintenance_starts_at),maintenance_ends_at=sqlc.narg(maintenance_ends_at),restricted_metadata_retention_days=sqlc.arg(restricted_metadata_retention_days),updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(at) WHERE singleton AND version=sqlc.arg(expected_version);
-- name: AppendOperationalPolicyVersion :exec
INSERT INTO platform_settings.operational_policy_versions(version,value,updated_by,updated_at) VALUES(sqlc.arg(version),sqlc.arg(value)::jsonb,sqlc.arg(actor_id),sqlc.arg(at));
-- name: ReadOperationalPolicyVersions :many
SELECT value FROM platform_settings.operational_policy_versions ORDER BY version DESC LIMIT 25;
-- name: ReadRegionalPolicyCommand :one
SELECT * FROM platform_settings.regional_policy_commands WHERE actor_id=sqlc.arg(actor_id) AND kind=sqlc.arg(kind) AND key_hash=sqlc.arg(key_hash);
-- name: AppendRegionalPolicyCommand :exec
INSERT INTO platform_settings.regional_policy_commands(actor_id,kind,key_hash,request_hash,result_version,target,created_at) VALUES(sqlc.arg(actor_id),sqlc.arg(kind),sqlc.arg(key_hash),sqlc.arg(request_hash),sqlc.arg(result_version),sqlc.arg(target),sqlc.arg(at));
-- name: AppendRegionalPolicyAudit :exec
INSERT INTO bidora_control_audit.audit_events(id,tenant_id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),sqlc.arg(action),'platform_regional_policy',sqlc.arg(target_id),sqlc.arg(at),'SUCCEEDED',sqlc.arg(correlation_id),sqlc.arg(summary)::jsonb,1,sqlc.arg(at));
-- name: AppendRegionalPolicyOutbox :exec
INSERT INTO bidora_control_eventing.outbox_events(id,tenant_id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,classification,payload,occurred_at,next_attempt_at,created_at) VALUES(sqlc.arg(id),NULL,sqlc.arg(actor_id),'platform_regional_policy',sqlc.arg(target_id),sqlc.arg(version),sqlc.arg(action),1,'INTERNAL',sqlc.arg(summary)::jsonb,sqlc.arg(at),sqlc.arg(at),sqlc.arg(at));
-- name: InsertRestrictedRequestMetadata :exec
INSERT INTO bidora_control_audit.request_metadata(correlation_id,actor_id,ip_address,browser,observed_at) VALUES(sqlc.arg(correlation_id),sqlc.arg(actor_id),sqlc.arg(ip_address),sqlc.arg(browser),sqlc.arg(at)) ON CONFLICT DO NOTHING;
-- name: ReadRestrictedRequestMetadata :one
SELECT ip_address,browser,observed_at FROM bidora_control_audit.platform_request_metadata_v1 WHERE audit_id=sqlc.arg(audit_id) AND observed_at>=sqlc.arg(cutoff);
