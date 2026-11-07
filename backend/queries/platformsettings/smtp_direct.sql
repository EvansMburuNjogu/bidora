-- name: ActivateSMTPDirect :execrows
UPDATE platform_settings.integration_versions SET state='ACTIVE',version=version+1,activated_at=sqlc.arg(at)::timestamptz,superseded_at=NULL,updated_at=sqlc.arg(at)::timestamptz WHERE id=sqlc.arg(id)::uuid AND setting_kind='SMTP' AND state='DRAFT' AND version=sqlc.arg(expected_version)::bigint;
-- name: SupersedePendingSMTPVersions :exec
UPDATE platform_settings.integration_versions SET state='SUPERSEDED',version=version+1,updated_at=sqlc.arg(at)::timestamptz,superseded_at=sqlc.arg(at)::timestamptz WHERE setting_kind='SMTP' AND state='PENDING_APPROVAL';
-- name: RetireSMTPApprovalRequests :exec
UPDATE platform_settings.integration_changes SET state='EXPIRED',version=version+1,decision_reason='SMTP is managed directly by the Owner',decided_at=sqlc.arg(at)::timestamptz WHERE setting_kind='SMTP' AND state='PENDING_APPROVAL';
-- name: InsertDirectSMTPHistory :exec
INSERT INTO platform_settings.integration_activation_history(id,setting_kind,version_id,previous_version_id,change_id,action,generation,activated_by,activated_at,activation_method) VALUES(sqlc.arg(id)::uuid,'SMTP',sqlc.arg(version_id)::uuid,sqlc.narg(previous_version_id)::uuid,NULL,'ACTIVATE',sqlc.arg(generation)::bigint,sqlc.arg(actor_id)::uuid,sqlc.arg(at)::timestamptz,'DIRECT');
