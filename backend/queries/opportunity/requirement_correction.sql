-- name: LockRequirementForCorrection :one
SELECT r.id,r.bid_id,r.source_version_id,r.source_document_version_id,r.status,r.version AS requirement_version,w.version AS workspace_version,v.revision AS content_revision
FROM bid.requirements r JOIN bid.workspaces w ON w.id=r.bid_id AND w.archived_at IS NULL AND w.state='BID'
JOIN bid.requirement_versions v ON v.id=r.current_content_revision_id
WHERE r.id=sqlc.arg(requirement_id) AND r.bid_id=sqlc.arg(bid_id) AND r.status='OPEN'
 AND r.version=sqlc.arg(expected_requirement_version) AND w.version=sqlc.arg(expected_workspace_version)
FOR UPDATE OF r,w;

-- name: InsertRequirementCorrection :exec
INSERT INTO bid.requirement_versions(id,requirement_id,revision,source_version_id,source_document_version_id,source_location,text,mandatory,category,correction_reason,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(requirement_id),sqlc.arg(revision),sqlc.arg(source_version_id),sqlc.narg(source_document_version_id),sqlc.arg(source_location),sqlc.arg(text),sqlc.arg(mandatory),sqlc.arg(category),sqlc.arg(correction_reason),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: ApplyRequirementCorrection :execrows
UPDATE bid.requirements SET source_location=sqlc.arg(source_location),text=sqlc.arg(text),mandatory=sqlc.arg(mandatory),category=sqlc.arg(category),current_content_revision_id=sqlc.arg(content_revision_id),version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND status='OPEN';

-- name: GetRequirementCorrectionVersion :one
SELECT v.id,v.requirement_id,r.bid_id,v.revision,v.source_version_id,v.source_document_version_id,v.source_location,v.text,v.mandatory,v.category,v.correction_reason,v.created_by,v.created_at
FROM bid.requirement_versions v JOIN bid.requirements r ON r.id=v.requirement_id
WHERE v.id=sqlc.arg(id);

-- name: ListRequirementCorrectionHistory :many
SELECT v.id,v.requirement_id,r.bid_id,v.revision,v.source_version_id,v.source_document_version_id,v.source_location,v.text,v.mandatory,v.category,v.correction_reason,v.created_by,v.created_at
FROM bid.requirement_versions v JOIN bid.requirements r ON r.id=v.requirement_id
WHERE v.requirement_id=sqlc.arg(requirement_id) AND r.bid_id=sqlc.arg(bid_id)
ORDER BY v.revision DESC LIMIT 100;

-- name: InsertRequirementCorrectedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'RequirementCorrected','bid_requirement',sqlc.arg(requirement_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'content_revision_id',sqlc.arg(content_revision_id)::uuid,'content_revision',sqlc.arg(content_revision)::bigint,'requirement_version',sqlc.arg(requirement_version)::bigint,'workspace_version',sqlc.arg(workspace_version)::bigint),1,sqlc.arg(occurred_at));

-- name: InsertRequirementCorrectedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'bid_workspace',sqlc.arg(bid_id),sqlc.arg(workspace_version),'bid.requirement.corrected.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid,'content_revision_id',sqlc.arg(content_revision_id)::uuid,'content_revision',sqlc.arg(content_revision)::bigint),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
