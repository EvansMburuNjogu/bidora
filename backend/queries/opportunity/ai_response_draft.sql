-- name: PrepareRequirementForAIDraft :one
SELECT r.id,r.bid_id,r.text,r.source_document_version_id,r.version AS requirement_version,w.version AS workspace_version,w.state,
       s.id AS section_id,s.version AS section_version,s.current_revision_id
FROM bid.requirements r JOIN bid.workspaces w ON w.id=r.bid_id AND w.archived_at IS NULL
LEFT JOIN bid.response_sections s ON s.bid_id=w.id AND s.requirement_id=r.id
JOIN documents.document_versions source ON source.id=r.source_document_version_id AND source.scan_state='CLEAN'
WHERE r.id=sqlc.arg(requirement_id) AND r.bid_id=sqlc.arg(bid_id) AND w.version=sqlc.arg(expected_workspace_version) AND w.state='BID'
 AND ((sqlc.arg(expected_section_version)::bigint=0 AND s.id IS NULL) OR s.version=sqlc.arg(expected_section_version));

-- name: LockRequirementForAIDraft :one
SELECT r.id,r.bid_id,r.text,r.source_document_version_id,r.version AS requirement_version,w.version AS workspace_version,w.state,
       s.id AS section_id,s.version AS section_version,s.current_revision_id
FROM bid.requirements r JOIN bid.workspaces w ON w.id=r.bid_id AND w.archived_at IS NULL
LEFT JOIN bid.response_sections s ON s.bid_id=w.id AND s.requirement_id=r.id
JOIN documents.document_versions source ON source.id=r.source_document_version_id AND source.scan_state='CLEAN'
WHERE r.id=sqlc.arg(requirement_id) AND r.bid_id=sqlc.arg(bid_id) AND w.version=sqlc.arg(expected_workspace_version) AND w.state='BID'
 AND ((sqlc.arg(expected_section_version)::bigint=0 AND s.id IS NULL) OR s.version=sqlc.arg(expected_section_version))
FOR UPDATE OF r,w,source;

-- name: ListCleanRequirementEvidenceForDraft :many
SELECT e.document_version_id FROM bid.requirement_evidence e
JOIN documents.document_versions v ON v.id=e.document_version_id AND v.scan_state='CLEAN'
WHERE e.requirement_id=sqlc.arg(requirement_id) AND e.bid_id=sqlc.arg(bid_id) AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at))
ORDER BY e.added_at,e.id LIMIT 4;

-- name: GetResponseDraftInputObject :one
SELECT v.id AS document_version_id,sqlc.arg(input_kind)::text AS input_kind,sqlc.arg(ordinal)::integer AS ordinal,v.object_key,v.object_version,v.media_type,v.size_bytes,v.sha256
FROM documents.document_versions v WHERE v.id=sqlc.arg(document_version_id) AND v.scan_state='CLEAN' AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at));

-- name: InsertResponseDraftJob :exec
INSERT INTO jobs.jobs(id,kind,document_version_id,state,requested_by,created_at)
VALUES(sqlc.arg(id),'RESPONSE_DRAFT',sqlc.arg(document_version_id),'PENDING_RESERVATION',sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: InsertResponseDraftRun :exec
INSERT INTO ai.response_draft_runs(id,job_id,bid_id,requirement_id,source_document_version_id,requested_workspace_version,requested_section_version,current_response_version_id,provider,model,schema_version,state,requested_by,correlation_id,created_at,input_characters,estimated_credits,checkpoint_id)
VALUES(sqlc.arg(id),sqlc.arg(job_id),sqlc.arg(bid_id),sqlc.arg(requirement_id),sqlc.arg(source_document_version_id),sqlc.arg(workspace_version),sqlc.arg(section_version),sqlc.narg(current_response_version_id),'OPENAI',sqlc.arg(model),1,'PENDING_RESERVATION',sqlc.arg(actor_id),sqlc.arg(correlation_id),sqlc.arg(created_at),sqlc.arg(input_characters),sqlc.arg(estimated_credits),sqlc.arg(checkpoint_id));

-- name: InsertResponseDraftInput :exec
INSERT INTO ai.response_draft_inputs(run_id,document_version_id,input_kind,ordinal)
VALUES(sqlc.arg(run_id),sqlc.arg(document_version_id),sqlc.arg(input_kind),sqlc.arg(ordinal));

-- name: GetResponseDraftRun :one
SELECT id,job_id,bid_id,requirement_id,source_document_version_id,requested_workspace_version,requested_section_version,current_response_version_id,model,state,provider_response_id,failure_code,requested_by,correlation_id,created_at,started_at,completed_at,input_characters,estimated_credits,checkpoint_id
FROM ai.response_draft_runs WHERE id=sqlc.arg(id);

-- name: CancelStaleResponseDrafts :exec
WITH stale AS (
 SELECT r.id FROM ai.response_draft_runs r JOIN jobs.jobs j ON j.id=r.job_id JOIN bid.workspaces w ON w.id=r.bid_id
 WHERE r.state='READY_FOR_PROVIDER' AND j.state='READY_FOR_PROVIDER' AND (w.version<>r.requested_workspace_version OR w.state<>'BID' OR w.archived_at IS NOT NULL OR NOT ((r.requested_section_version=0 AND r.current_response_version_id IS NULL AND NOT EXISTS(SELECT 1 FROM bid.response_sections s WHERE s.bid_id=r.bid_id AND s.requirement_id=r.requirement_id)) OR EXISTS(SELECT 1 FROM bid.response_sections s WHERE s.bid_id=r.bid_id AND s.requirement_id=r.requirement_id AND s.version=r.requested_section_version AND s.current_revision_id=r.current_response_version_id)))
 FOR UPDATE OF r,j
), cancelled AS (
 UPDATE ai.response_draft_runs r SET state='CANCELLED',completed_at=sqlc.arg(completed_at) FROM stale WHERE r.id=stale.id RETURNING r.job_id
)
UPDATE jobs.jobs j SET state='CANCELLED',completed_at=sqlc.arg(completed_at) FROM cancelled WHERE j.id=cancelled.job_id;

-- name: LockStaleRunningResponseDraft :one
SELECT r.id,r.job_id,r.bid_id,r.requirement_id,r.requested_by,r.correlation_id,r.input_characters,r.estimated_credits,r.checkpoint_id,c.reservation_id,c.usage_kind
FROM ai.response_draft_runs r
JOIN jobs.jobs j ON j.id=r.job_id AND j.state='RUNNING'
JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=r.checkpoint_id AND c.state='RESERVED'
WHERE r.state='RUNNING' AND r.started_at<sqlc.arg(stale_before)
ORDER BY r.started_at,r.id FOR UPDATE OF r,j SKIP LOCKED LIMIT 1;

-- name: ClaimNextResponseDraft :one
WITH candidate AS (
 SELECT r.id FROM ai.response_draft_runs r JOIN jobs.jobs j ON j.id=r.job_id
 JOIN bidora_entitlement.ai_usage_checkpoints cp ON cp.id=r.checkpoint_id AND cp.target_id=j.id AND cp.state='RESERVED'
 WHERE r.state='READY_FOR_PROVIDER' AND j.state='READY_FOR_PROVIDER' AND j.kind='RESPONSE_DRAFT'
 ORDER BY r.created_at,r.id FOR UPDATE OF r,j SKIP LOCKED LIMIT 1
)
UPDATE ai.response_draft_runs r SET state='RUNNING',started_at=sqlc.arg(started_at)
FROM candidate c,jobs.jobs j,bid.workspaces w,bid.requirements req,bidora_entitlement.ai_usage_checkpoints cp
WHERE r.id=c.id AND j.id=r.job_id AND cp.id=r.checkpoint_id AND cp.target_id=j.id AND cp.state='RESERVED' AND w.id=r.bid_id AND req.id=r.requirement_id
RETURNING r.id,r.job_id,r.bid_id,r.requirement_id,r.source_document_version_id,r.requested_workspace_version,r.requested_section_version,r.current_response_version_id,r.model,r.requested_by,r.correlation_id,req.text,r.input_characters,r.estimated_credits,r.checkpoint_id,cp.reservation_id,cp.usage_kind,
 COALESCE((w.version=r.requested_workspace_version AND w.state='BID' AND w.archived_at IS NULL AND ((r.requested_section_version=0 AND r.current_response_version_id IS NULL AND NOT EXISTS(SELECT 1 FROM bid.response_sections s WHERE s.bid_id=r.bid_id AND s.requirement_id=r.requirement_id)) OR EXISTS(SELECT 1 FROM bid.response_sections s WHERE s.bid_id=r.bid_id AND s.requirement_id=r.requirement_id AND s.version=r.requested_section_version AND s.current_revision_id=r.current_response_version_id))),false)::boolean AS snapshot_current;

-- name: StartResponseDraftJob :execrows
UPDATE jobs.jobs SET state='RUNNING',attempts=attempts+1,started_at=sqlc.arg(started_at) WHERE id=sqlc.arg(id) AND state='READY_FOR_PROVIDER' AND kind='RESPONSE_DRAFT';

-- name: CountResponseDraftInputs :one
SELECT count(*)::bigint FROM ai.response_draft_inputs WHERE run_id=sqlc.arg(run_id);

-- name: ListResponseDraftInputObjects :many
SELECT i.document_version_id,i.input_kind,i.ordinal,v.object_key,v.object_version,v.media_type,v.size_bytes,v.sha256
FROM ai.response_draft_inputs i JOIN documents.document_versions v ON v.id=i.document_version_id AND v.scan_state='CLEAN'
WHERE i.run_id=sqlc.arg(run_id) AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at)) ORDER BY i.ordinal;

-- name: InsertResponseDraftSuggestion :exec
INSERT INTO ai.response_draft_suggestions(id,run_id,suggested_content,rationale,review_state,created_at)
VALUES(sqlc.arg(id),sqlc.arg(run_id),sqlc.arg(content),sqlc.arg(rationale),'PROPOSED_UNVERIFIED',sqlc.arg(created_at));

-- name: InsertResponseDraftCitation :exec
INSERT INTO ai.response_draft_citations(suggestion_id,ordinal,document_version_id,citation_location,citation_quote)
VALUES(sqlc.arg(suggestion_id),sqlc.arg(ordinal),sqlc.arg(document_version_id),sqlc.arg(location),sqlc.arg(quote));

-- name: InsertResponseGapSuggestion :exec
INSERT INTO ai.response_gap_suggestions(suggestion_id,ordinal,gap_code,description,document_version_id,citation_location,citation_quote)
VALUES(sqlc.arg(suggestion_id),sqlc.arg(ordinal),sqlc.arg(gap_code),sqlc.arg(description),sqlc.narg(document_version_id),sqlc.narg(location),sqlc.narg(quote));

-- name: CompleteResponseDraftRun :execrows
UPDATE ai.response_draft_runs SET state='SUCCEEDED',provider_response_id=sqlc.arg(provider_response_id),completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(id) AND state='RUNNING';

-- name: FailResponseDraftRun :execrows
UPDATE ai.response_draft_runs SET state=sqlc.arg(state),failure_code=sqlc.narg(failure_code),completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(id) AND state='RUNNING';

-- name: CompleteResponseDraftJob :execrows
UPDATE jobs.jobs SET state=sqlc.arg(state),completed_at=sqlc.arg(completed_at) WHERE id=sqlc.arg(id) AND state='RUNNING' AND kind='RESPONSE_DRAFT';

-- name: ListResponseDraftSuggestions :many
SELECT s.id AS suggestion_id,r.id AS run_id,s.suggested_content,s.rationale,s.review_state,COALESCE(s.created_at,r.created_at)::timestamptz AS created_at,r.state AS run_state,r.bid_id,r.requirement_id,r.requested_workspace_version,r.requested_section_version
FROM ai.response_draft_runs r LEFT JOIN ai.response_draft_suggestions s ON s.run_id=r.id
WHERE r.bid_id=sqlc.arg(bid_id) AND r.requirement_id=sqlc.arg(requirement_id)
ORDER BY r.created_at DESC LIMIT 20;

-- name: ListResponseDraftCitations :many
SELECT ordinal,document_version_id,citation_location,citation_quote FROM ai.response_draft_citations WHERE suggestion_id=sqlc.arg(suggestion_id) ORDER BY ordinal;

-- name: ListResponseGapSuggestions :many
SELECT ordinal,gap_code,description,document_version_id,citation_location,citation_quote FROM ai.response_gap_suggestions WHERE suggestion_id=sqlc.arg(suggestion_id) ORDER BY ordinal;

-- name: InsertResponseDraftAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'ai_response_draft',sqlc.arg(run_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid,'job_id',sqlc.arg(job_id)::uuid),1,sqlc.arg(occurred_at));

-- name: InsertResponseDraftOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'ai_response_draft',sqlc.arg(run_id),1,sqlc.arg(event_type),1,jsonb_build_object('run_id',sqlc.arg(run_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'bid_id',sqlc.arg(bid_id)::uuid,'requirement_id',sqlc.arg(requirement_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
