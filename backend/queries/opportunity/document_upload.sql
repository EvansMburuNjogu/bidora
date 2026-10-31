-- name: LockOpportunityForDocumentUpload :one
SELECT id FROM bidora_opportunity.opportunities WHERE id=sqlc.arg(id) FOR KEY SHARE;

-- name: InsertTenderDocument :exec
INSERT INTO documents.documents(id,opportunity_id,title,state,created_by,created_at,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(opportunity_id),sqlc.arg(title),'UPLOADING',sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(created_at));

-- name: InsertDocumentUploadSession :exec
INSERT INTO documents.upload_sessions(id,document_id,opportunity_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(document_id),sqlc.arg(opportunity_id),sqlc.arg(object_key),sqlc.arg(media_type),sqlc.arg(size_bytes),sqlc.arg(sha256),'INITIATED',sqlc.arg(expires_at),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: GetDocumentUploadSession :one
SELECT id,document_id,opportunity_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at,completed_version_id,created_by,created_at,completed_at
FROM documents.upload_sessions WHERE id=sqlc.arg(id) AND opportunity_id=sqlc.arg(opportunity_id) AND created_by=sqlc.arg(actor_id);

-- name: GetCompletedDocumentUploadResult :one
SELECT u.id AS upload_id,u.document_id,u.completed_version_id AS document_version_id,
       s.id AS scan_job_id,a.id AS analysis_job_id,v.scan_state,a.state AS analysis_state
FROM documents.upload_sessions u
JOIN documents.document_versions v ON v.id=u.completed_version_id
JOIN jobs.jobs s ON s.document_version_id=v.id AND s.kind='MALWARE_SCAN'
JOIN jobs.jobs a ON a.document_version_id=v.id AND a.kind='TENDER_ANALYSIS'
WHERE u.id=sqlc.arg(id) AND u.opportunity_id=sqlc.arg(opportunity_id) AND u.created_by=sqlc.arg(actor_id) AND u.state='COMPLETED';

-- name: LockDocumentUploadSession :one
SELECT id,document_id,opportunity_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at,completed_version_id,created_by,created_at,completed_at
FROM documents.upload_sessions WHERE id=sqlc.arg(id) AND opportunity_id=sqlc.arg(opportunity_id) AND created_by=sqlc.arg(actor_id) FOR UPDATE;

-- name: InsertTenderDocumentVersion :exec
INSERT INTO documents.document_versions(id,document_id,revision,media_type,size_bytes,sha256,object_key,object_version,scan_state,expires_at,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(document_id),1,sqlc.arg(media_type),sqlc.arg(size_bytes),sqlc.arg(sha256),sqlc.arg(object_key),sqlc.arg(object_version),'QUARANTINED',sqlc.narg(expires_at),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: CompleteDocumentUploadSession :execrows
UPDATE documents.upload_sessions SET state='COMPLETED',completed_version_id=sqlc.arg(version_id),completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(id) AND state='INITIATED';

-- name: ActivateTenderDocument :execrows
UPDATE documents.documents SET state='ACTIVE',version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND state='UPLOADING';

-- name: GetOwnedDocumentJob :one
SELECT j.id,j.kind,j.document_version_id,j.state,j.blocked_reason,j.attempts,j.created_at,j.started_at,j.completed_at,
       v.document_id,d.opportunity_id,d.bid_id,d.document_kind
FROM jobs.jobs j
JOIN documents.document_versions v ON v.id=j.document_version_id
JOIN documents.documents d ON d.id=v.document_id
WHERE j.id=sqlc.arg(id) AND j.requested_by=sqlc.arg(actor_id);

-- name: InsertDocumentJob :exec
INSERT INTO jobs.jobs(id,kind,document_version_id,state,blocked_reason,requested_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(kind),sqlc.arg(document_version_id),sqlc.arg(state),sqlc.narg(blocked_reason),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: InsertDocumentUploadInitiatedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'TenderDocumentUploadInitiated','document',sqlc.arg(document_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('opportunity_id',sqlc.arg(opportunity_id)::uuid,'upload_id',sqlc.arg(upload_id)::uuid),1,sqlc.arg(occurred_at));

-- name: InsertDocumentUploadCompletedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'TenderDocumentUploadCompleted','document_version',sqlc.arg(version_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('document_id',sqlc.arg(document_id)::uuid,'scan_state','QUARANTINED','analysis_state','WAITING_FOR_CLEAN'),1,sqlc.arg(occurred_at));

-- name: InsertDocumentVersionQuarantinedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'document',sqlc.arg(document_id),2,'document.version.quarantined.v1',1,jsonb_build_object('document_id',sqlc.arg(document_id)::uuid,'document_version_id',sqlc.arg(version_id)::uuid,'scan_job_id',sqlc.arg(scan_job_id)::uuid,'analysis_job_id',sqlc.arg(analysis_job_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
