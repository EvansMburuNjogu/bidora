-- name: LockBidForRequirementCreation :one
SELECT id,version,source_version_id,state
FROM bid.workspaces
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND archived_at IS NULL AND state='BID'
  AND source_version_id=sqlc.arg(source_version_id)
FOR UPDATE;

-- name: InsertBidRequirement :exec
INSERT INTO bid.requirements
(id,bid_id,source_version_id,source_location,text,mandatory,category,status,owner_id,due_at,version,created_by,created_at,updated_at,current_content_revision_id)
VALUES (sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(source_version_id),sqlc.arg(source_location),sqlc.arg(text),sqlc.arg(mandatory),sqlc.arg(category),'OPEN',sqlc.narg(owner_id),sqlc.narg(due_at),1,sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(created_at),sqlc.arg(id));

-- name: LockRequirementForAssignment :one
SELECT r.id,r.bid_id,r.version AS requirement_version,w.version AS workspace_version,w.state
FROM bid.requirements r JOIN bid.workspaces w ON w.id=r.bid_id
WHERE r.id=sqlc.arg(requirement_id) AND r.bid_id=sqlc.arg(bid_id)
  AND r.version=sqlc.arg(expected_requirement_version)
  AND w.version=sqlc.arg(expected_workspace_version)
  AND w.archived_at IS NULL AND w.state='BID'
FOR UPDATE OF r,w;

-- name: AssignBidRequirement :execrows
UPDATE bid.requirements SET owner_id=sqlc.arg(owner_id),due_at=sqlc.narg(due_at),version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version);

-- name: AdvanceWorkspaceForRequirementChange :execrows
UPDATE bid.workspaces SET version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND archived_at IS NULL;

-- name: GetBidRequirement :one
SELECT id,bid_id,source_version_id,source_location,text,mandatory,category,status,owner_id,due_at,version,created_at,updated_at
FROM bid.requirements WHERE id=sqlc.arg(id);

-- name: ListBidRequirements :many
SELECT id,bid_id,source_version_id,source_location,text,mandatory,category,status,owner_id,due_at,version,created_at,updated_at
FROM bid.requirements WHERE bid_id=sqlc.arg(bid_id)
ORDER BY mandatory DESC,category,id;

-- name: GetBidReadiness :one
SELECT w.id,w.state,w.version,
       count(r.id) FILTER (WHERE r.mandatory AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=r.id AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at)))))::bigint AS open_mandatory_count,
       count(r.id) FILTER (WHERE r.mandatory AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=r.id AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at)))) AND r.owner_id IS NULL)::bigint AS unassigned_mandatory_count,
       count(r.id) FILTER (WHERE (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=r.id AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at)))) AND r.due_at IS NOT NULL AND r.due_at<sqlc.arg(observed_at))::bigint AS overdue_count,
       count(r.id) FILTER (WHERE r.status='SATISFIED' AND EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=r.id AND v.expires_at IS NOT NULL AND v.expires_at<=sqlc.arg(observed_at)) AND NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=r.id AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at))))::bigint AS expired_evidence_count
FROM bid.workspaces w LEFT JOIN bid.requirements r ON r.bid_id=w.id
WHERE w.id=sqlc.arg(id) AND w.archived_at IS NULL
GROUP BY w.id,w.state,w.version;

-- name: InsertRequirementCreatedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'RequirementCreated','bid_requirement',sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'source_version_id',sqlc.arg(source_version_id)::uuid),1,sqlc.arg(occurred_at));

-- name: InsertRequirementCreatedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'bid_workspace',sqlc.arg(bid_id),sqlc.arg(workspace_version),'bid.requirement.created.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid,'source_version_id',sqlc.arg(source_version_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));

-- name: InsertRequirementAssignedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'RequirementAssigned','bid_requirement',sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'owner_id',sqlc.arg(owner_id)::uuid,'requirement_version',sqlc.arg(requirement_version)::bigint),1,sqlc.arg(occurred_at));

-- name: InsertRequirementAssignedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'bid_workspace',sqlc.arg(bid_id),sqlc.arg(workspace_version),'bid.requirement.assigned.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid,'owner_id',sqlc.arg(owner_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
