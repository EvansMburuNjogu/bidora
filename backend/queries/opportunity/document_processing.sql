-- name: ClaimNextScanJob :one
WITH candidate AS (
 SELECT j.id FROM jobs.jobs j JOIN documents.document_versions v ON v.id=j.document_version_id
 WHERE j.kind='MALWARE_SCAN' AND j.state='QUEUED' AND v.scan_state='QUARANTINED'
 ORDER BY j.created_at,j.id FOR UPDATE OF j SKIP LOCKED LIMIT 1
)
UPDATE jobs.jobs j SET state='RUNNING',attempts=attempts+1,started_at=sqlc.arg(started_at)
FROM candidate c,documents.document_versions v,documents.documents d
WHERE j.id=c.id AND v.id=j.document_version_id AND d.id=v.document_id
RETURNING j.id,j.document_version_id,j.requested_by,j.attempts,v.object_key,v.object_version,v.size_bytes,v.media_type,d.opportunity_id,d.document_kind;

-- name: ReleaseScanJob :execrows
UPDATE jobs.jobs SET state='QUEUED',started_at=NULL WHERE id=sqlc.arg(id) AND kind='MALWARE_SCAN' AND state='RUNNING';

-- name: CancelRunningJob :execrows
UPDATE jobs.jobs SET state='CANCELLED',completed_at=sqlc.arg(completed_at) WHERE id=sqlc.arg(id) AND state='RUNNING';

-- name: LockRunningScanJob :one
SELECT j.id,j.document_version_id,j.requested_by,v.document_id,v.scan_state
FROM jobs.jobs j JOIN documents.document_versions v ON v.id=j.document_version_id
WHERE j.id=sqlc.arg(id) AND j.kind='MALWARE_SCAN' AND j.state='RUNNING' FOR UPDATE OF j,v;

-- name: MarkDocumentVersionScanned :execrows
UPDATE documents.document_versions SET scan_state=sqlc.arg(scan_state),scan_engine=sqlc.arg(scan_engine),scan_result_code=sqlc.arg(result_code),scan_completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(id) AND scan_state='QUARANTINED';

-- name: CompleteScanJob :execrows
UPDATE jobs.jobs SET state='SUCCEEDED',completed_at=sqlc.arg(completed_at) WHERE id=sqlc.arg(id) AND state='RUNNING';

-- name: ReleaseAnalysisAfterClean :execrows
UPDATE jobs.jobs SET state='QUEUED',blocked_reason=NULL WHERE document_version_id=sqlc.arg(document_version_id) AND kind='TENDER_ANALYSIS' AND state='WAITING_FOR_CLEAN';

-- name: CancelAnalysisAfterRejection :execrows
UPDATE jobs.jobs SET state='CANCELLED',blocked_reason=NULL,completed_at=sqlc.arg(completed_at) WHERE document_version_id=sqlc.arg(document_version_id) AND kind='TENDER_ANALYSIS' AND state='WAITING_FOR_CLEAN';

-- name: InsertDocumentScanAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'TenderDocumentScanned','document_version',sqlc.arg(version_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('scan_state',sqlc.arg(scan_state)::text,'result_code',sqlc.arg(result_code)::text),1,sqlc.arg(occurred_at));

-- name: InsertDocumentScanOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'document',sqlc.arg(document_id),3,sqlc.arg(event_type),1,jsonb_build_object('document_id',sqlc.arg(document_id)::uuid,'document_version_id',sqlc.arg(version_id)::uuid,'scan_state',sqlc.arg(scan_state)::text),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));

-- name: ClaimNextAnalysisJob :one
WITH candidate AS (
 SELECT j.id
 FROM jobs.jobs j
 JOIN documents.document_versions v ON v.id=j.document_version_id
 JOIN ai.document_measurements m ON m.job_id=j.id AND m.document_version_id=v.id
 JOIN ai.document_measurement_entitlements e ON e.measurement_id=m.id
 JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=e.checkpoint_id AND c.target_id=j.id AND c.state='RESERVED'
 WHERE j.kind='TENDER_ANALYSIS' AND j.state='READY_FOR_PROVIDER' AND v.scan_state='CLEAN'
 ORDER BY j.created_at,j.id FOR UPDATE OF j SKIP LOCKED LIMIT 1
)
UPDATE jobs.jobs j SET state='RUNNING',attempts=attempts+1,started_at=sqlc.arg(started_at)
FROM candidate c,documents.document_versions v,documents.documents d,
     ai.document_measurements m,ai.document_measurement_entitlements e,bidora_entitlement.ai_usage_checkpoints cp
WHERE j.id=c.id AND v.id=j.document_version_id AND d.id=v.document_id
  AND m.job_id=j.id AND m.document_version_id=v.id AND e.measurement_id=m.id
  AND cp.id=e.checkpoint_id AND cp.target_id=j.id AND cp.state='RESERVED'
RETURNING j.id,j.document_version_id,j.requested_by,j.attempts,v.object_key,v.object_version,v.size_bytes,v.media_type,d.opportunity_id,d.document_kind,
          m.id AS measurement_id,m.correlation_id,m.estimated_credits,m.usage_kind,e.checkpoint_id,cp.reservation_id;

-- name: ReleaseAnalysisJob :execrows
UPDATE jobs.jobs SET state='READY_FOR_PROVIDER',started_at=NULL WHERE id=sqlc.arg(id) AND kind='TENDER_ANALYSIS' AND state='RUNNING';

-- name: InsertExtractionRun :exec
INSERT INTO ai.extraction_runs(id,job_id,document_version_id,provider,model,schema_version,state,started_at)
VALUES(sqlc.arg(id),sqlc.arg(job_id),sqlc.arg(document_version_id),'OPENAI',sqlc.arg(model),1,'RUNNING',sqlc.arg(started_at));

-- name: InsertRequirementSuggestion :exec
INSERT INTO ai.requirement_suggestions(id,extraction_run_id,document_version_id,ordinal,requirement_text,category,mandatory,citation_page,citation_location,citation_quote,confidence,review_state,created_at)
VALUES(sqlc.arg(id),sqlc.arg(run_id),sqlc.arg(document_version_id),sqlc.arg(ordinal),sqlc.arg(requirement_text),sqlc.arg(category),sqlc.arg(mandatory),sqlc.narg(citation_page),sqlc.arg(citation_location),sqlc.arg(citation_quote),sqlc.arg(confidence),'PROPOSED_UNVERIFIED',sqlc.arg(created_at));

-- name: CompleteExtractionRun :execrows
UPDATE ai.extraction_runs SET state='SUCCEEDED',provider_response_id=sqlc.arg(provider_response_id),completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(id) AND state='RUNNING';

-- name: FailExtractionRun :execrows
UPDATE ai.extraction_runs SET state='FAILED',failure_code=sqlc.arg(failure_code),completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(id) AND state='RUNNING';

-- name: CompleteAnalysisJob :execrows
UPDATE jobs.jobs SET state=sqlc.arg(state),completed_at=sqlc.arg(completed_at) WHERE id=sqlc.arg(id) AND kind='TENDER_ANALYSIS' AND state='RUNNING';

-- name: InsertExtractionAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'TenderRequirementsExtracted','document_version',sqlc.arg(version_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('extraction_run_id',sqlc.arg(run_id)::uuid,'suggestion_count',sqlc.arg(suggestion_count)::integer,'review_state','PROPOSED_UNVERIFIED'),1,sqlc.arg(occurred_at));

-- name: InsertExtractionOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'document_version',sqlc.arg(version_id),1,'ai.tender_requirements.proposed.v1',1,jsonb_build_object('document_version_id',sqlc.arg(version_id)::uuid,'extraction_run_id',sqlc.arg(run_id)::uuid,'suggestion_count',sqlc.arg(suggestion_count)::integer),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
