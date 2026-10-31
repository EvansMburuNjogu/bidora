-- name: LockRequirementForResponse :one
SELECT r.id,r.bid_id,r.version AS requirement_version,r.status,w.version AS workspace_version,w.state
FROM bid.requirements r JOIN bid.workspaces w ON w.id=r.bid_id
WHERE r.id=sqlc.arg(requirement_id) AND r.bid_id=sqlc.arg(bid_id)
 AND w.version=sqlc.arg(expected_workspace_version) AND w.archived_at IS NULL AND w.state='BID'
FOR UPDATE OF r,w;

-- name: GetResponseSectionForUpdate :one
SELECT id,bid_id,requirement_id,current_revision_id,version,created_by,created_at,updated_at
FROM bid.response_sections WHERE bid_id=sqlc.arg(bid_id) AND requirement_id=sqlc.arg(requirement_id) FOR UPDATE;

-- name: InsertResponseSection :exec
INSERT INTO bid.response_sections(id,bid_id,requirement_id,version,created_by,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(requirement_id),1,sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(created_at));

-- name: InsertResponseVersion :exec
INSERT INTO bid.response_versions(id,section_id,revision,requirement_version_at_creation,content,change_note,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(section_id),sqlc.arg(revision),sqlc.arg(requirement_version),sqlc.arg(content),sqlc.arg(change_note),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: AdvanceResponseSection :execrows
UPDATE bid.response_sections SET current_revision_id=sqlc.arg(revision_id),version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version);

-- name: SetInitialResponseRevision :execrows
UPDATE bid.response_sections SET current_revision_id=sqlc.arg(revision_id),updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=1 AND current_revision_id IS NULL;

-- name: InsertResponseEvidence :execrows
INSERT INTO bid.response_evidence(response_version_id,requirement_evidence_id)
SELECT sqlc.arg(response_version_id),e.id FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id
WHERE e.id=sqlc.arg(evidence_id) AND e.requirement_id=sqlc.arg(requirement_id) AND e.bid_id=sqlc.arg(bid_id) AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at));

-- name: GetTechnicalResponseByVersion :one
SELECT s.id AS section_id,s.bid_id,s.requirement_id,v.revision::bigint AS section_version,v.id AS response_version_id,v.revision,v.content,v.change_note,v.created_by,v.created_at
FROM bid.response_sections s JOIN bid.response_versions v ON v.section_id=s.id
WHERE v.id=sqlc.arg(id);

-- name: GetCurrentTechnicalResponse :one
SELECT s.id AS section_id,s.bid_id,s.requirement_id,s.version AS section_version,v.id AS response_version_id,v.revision,v.content,v.change_note,v.created_by,v.created_at
FROM bid.response_sections s JOIN bid.response_versions v ON v.id=s.current_revision_id
WHERE s.bid_id=sqlc.arg(bid_id) AND s.requirement_id=sqlc.arg(requirement_id);

-- name: ListTechnicalResponseHistory :many
SELECT s.id AS section_id,s.bid_id,s.requirement_id,v.revision::bigint AS section_version,v.id AS response_version_id,v.revision,v.content,v.change_note,v.created_by,v.created_at
FROM bid.response_sections s JOIN bid.response_versions v ON v.section_id=s.id
WHERE s.bid_id=sqlc.arg(bid_id) AND s.requirement_id=sqlc.arg(requirement_id)
ORDER BY v.revision DESC LIMIT 100;

-- name: ListResponseVersionEvidence :many
SELECT requirement_evidence_id FROM bid.response_evidence WHERE response_version_id=sqlc.arg(response_version_id) ORDER BY requirement_evidence_id;

-- name: GetBidForGapReview :one
SELECT id,state,version FROM bid.workspaces WHERE id=sqlc.arg(id) AND archived_at IS NULL;

-- name: CountBidRequirementsForGap :one
SELECT count(*)::bigint FROM bid.requirements WHERE bid_id=sqlc.arg(bid_id);

-- name: GetBidGapInputs :many
SELECT r.id AS requirement_id,r.text,r.mandatory,r.status,r.owner_id,r.due_at,r.version AS requirement_version,
       s.id AS section_id,s.version AS section_version,s.current_revision_id,
       (rv.id IS NOT NULL AND rv.requirement_version_at_creation=r.version) AS response_current,
       EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=r.id AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at))) AS evidence_current,
       EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=r.id AND v.expires_at IS NOT NULL AND v.expires_at<=sqlc.arg(observed_at)) AS evidence_expired
FROM bid.workspaces w JOIN bid.requirements r ON r.bid_id=w.id
LEFT JOIN bid.response_sections s ON s.bid_id=w.id AND s.requirement_id=r.id
LEFT JOIN bid.response_versions rv ON rv.id=s.current_revision_id
WHERE w.id=sqlc.arg(bid_id) AND w.archived_at IS NULL
ORDER BY r.mandatory DESC,r.category,r.id LIMIT 1000;

-- name: InsertTechnicalResponseAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'TechnicalResponseRevised','response_section',sqlc.arg(section_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid,'response_version_id',sqlc.arg(response_version_id)::uuid,'revision',sqlc.arg(revision)::integer,'section_version',sqlc.arg(section_version)::bigint,'workspace_version',sqlc.arg(workspace_version)::bigint),1,sqlc.arg(occurred_at));

-- name: InsertTechnicalResponseOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'bid_workspace',sqlc.arg(bid_id),sqlc.arg(workspace_version),'bid.technical_response.revised.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid,'section_id',sqlc.arg(section_id)::uuid,'response_version_id',sqlc.arg(response_version_id)::uuid,'revision',sqlc.arg(revision)::integer),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
