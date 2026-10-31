-- name: LockBidForEvidenceUpload :one
SELECT id,version,state FROM bid.workspaces WHERE id=sqlc.arg(id) AND archived_at IS NULL AND state='BID' FOR KEY SHARE;

-- name: InsertBidEvidenceDocument :exec
INSERT INTO documents.documents(id,opportunity_id,bid_id,document_kind,title,state,created_by,created_at,updated_at)
VALUES(sqlc.arg(id),NULL,sqlc.arg(bid_id),'BID_EVIDENCE',sqlc.arg(title),'UPLOADING',sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(created_at));

-- name: InsertEvidenceUploadSession :exec
INSERT INTO documents.upload_sessions(id,document_id,opportunity_id,bid_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at,evidence_expires_at,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(document_id),NULL,sqlc.arg(bid_id),sqlc.arg(object_key),sqlc.arg(media_type),sqlc.arg(size_bytes),sqlc.arg(sha256),'INITIATED',sqlc.arg(expires_at),sqlc.narg(evidence_expires_at),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: GetEvidenceUploadSession :one
SELECT id,document_id,bid_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at,evidence_expires_at,completed_version_id,created_by,created_at,completed_at
FROM documents.upload_sessions WHERE id=sqlc.arg(id) AND bid_id=sqlc.arg(bid_id) AND created_by=sqlc.arg(actor_id);

-- name: LockEvidenceUploadSession :one
SELECT u.id,u.document_id,u.bid_id,u.object_key,u.expected_media_type,u.expected_size_bytes,u.expected_sha256,u.state,u.expires_at,u.evidence_expires_at,u.completed_version_id,u.created_by,u.created_at,u.completed_at
FROM documents.upload_sessions u JOIN bid.workspaces w ON w.id=u.bid_id AND w.archived_at IS NULL AND w.state='BID'
WHERE u.id=sqlc.arg(id) AND u.bid_id=sqlc.arg(bid_id) AND u.created_by=sqlc.arg(actor_id) FOR UPDATE OF u;

-- name: GetCompletedEvidenceUploadResult :one
SELECT u.id AS upload_id,u.document_id,u.completed_version_id AS document_version_id,s.id AS scan_job_id,v.scan_state,v.expires_at
FROM documents.upload_sessions u
JOIN documents.document_versions v ON v.id=u.completed_version_id
JOIN jobs.jobs s ON s.document_version_id=v.id AND s.kind='MALWARE_SCAN'
WHERE u.id=sqlc.arg(id) AND u.bid_id=sqlc.arg(bid_id) AND u.created_by=sqlc.arg(actor_id) AND u.state='COMPLETED';

-- name: InsertEvidenceUploadAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),sqlc.arg(target_type),sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'upload_id',sqlc.arg(upload_id)::uuid),1,sqlc.arg(occurred_at));

-- name: InsertEvidenceVersionQuarantinedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'document',sqlc.arg(document_id),2,'bid.evidence.quarantined.v1',1,jsonb_build_object('document_id',sqlc.arg(document_id)::uuid,'document_version_id',sqlc.arg(version_id)::uuid,'scan_job_id',sqlc.arg(scan_job_id)::uuid,'bid_id',sqlc.arg(bid_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));

-- name: LockRequirementForEvidenceSatisfaction :one
SELECT r.id,r.bid_id,r.version AS requirement_version,r.status,w.version AS workspace_version,w.state,
       v.id AS document_version_id,v.scan_state,v.expires_at,d.bid_id AS evidence_bid_id,d.document_kind
FROM bid.requirements r
JOIN bid.workspaces w ON w.id=r.bid_id AND w.archived_at IS NULL
JOIN documents.document_versions v ON v.id=sqlc.arg(document_version_id)
JOIN documents.documents d ON d.id=v.document_id
WHERE r.id=sqlc.arg(requirement_id) AND r.bid_id=sqlc.arg(bid_id)
 AND r.version=sqlc.arg(expected_requirement_version) AND r.status='OPEN'
 AND w.version=sqlc.arg(expected_workspace_version) AND w.state='BID'
 AND v.scan_state='CLEAN' AND d.document_kind='BID_EVIDENCE' AND d.bid_id=w.id
FOR UPDATE OF r,w;

-- name: InsertRequirementEvidence :exec
INSERT INTO bid.requirement_evidence(id,requirement_id,bid_id,document_version_id,evidence_location,justification,added_by,added_at)
VALUES(sqlc.arg(id),sqlc.arg(requirement_id),sqlc.arg(bid_id),sqlc.arg(document_version_id),sqlc.arg(evidence_location),sqlc.arg(justification),sqlc.arg(actor_id),sqlc.arg(added_at));

-- name: SatisfyRequirement :execrows
UPDATE bid.requirements SET status='SATISFIED',version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND status='OPEN';

-- name: GetRequirementEvidence :one
SELECT e.id,e.requirement_id,e.bid_id,e.document_version_id,e.evidence_location,e.justification,e.added_by,e.added_at,v.expires_at
FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.id=sqlc.arg(id);

-- name: ListRequirementEvidence :many
SELECT e.id,e.requirement_id,e.bid_id,e.document_version_id,e.evidence_location,e.justification,e.added_by,e.added_at,v.expires_at
FROM bid.requirement_evidence e JOIN documents.document_versions v ON v.id=e.document_version_id WHERE e.requirement_id=sqlc.arg(requirement_id) AND e.bid_id=sqlc.arg(bid_id)
ORDER BY e.added_at,e.id LIMIT 100;

-- name: InsertRequirementSatisfiedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'RequirementSatisfied','bid_requirement',sqlc.arg(requirement_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'evidence_id',sqlc.arg(evidence_id)::uuid,'document_version_id',sqlc.arg(document_version_id)::uuid,'requirement_version',sqlc.arg(requirement_version)::bigint,'workspace_version',sqlc.arg(workspace_version)::bigint),1,sqlc.arg(occurred_at));

-- name: InsertRequirementSatisfiedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'bid_workspace',sqlc.arg(bid_id),sqlc.arg(workspace_version),'bid.requirement.satisfied.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid,'evidence_id',sqlc.arg(evidence_id)::uuid,'document_version_id',sqlc.arg(document_version_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
