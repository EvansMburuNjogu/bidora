-- name: PrepareProposalForAIDraft :one
SELECT p.id AS proposal_id,p.bid_id,p.version,p.current_revision_id,w.version AS workspace_version
FROM bid.proposals p JOIN bid.workspaces w ON w.id=p.bid_id AND w.archived_at IS NULL AND w.state='BID'
WHERE p.bid_id=sqlc.arg(bid_id) AND p.current_revision_id=sqlc.arg(proposal_version_id) AND p.version=sqlc.arg(expected_proposal_version) AND w.version=sqlc.arg(expected_workspace_version);

-- name: LockProposalForAIDraft :one
SELECT p.id AS proposal_id,p.bid_id,p.version,p.current_revision_id,w.version AS workspace_version
FROM bid.proposals p JOIN bid.workspaces w ON w.id=p.bid_id AND w.archived_at IS NULL AND w.state='BID'
WHERE p.bid_id=sqlc.arg(bid_id) AND p.current_revision_id=sqlc.arg(proposal_version_id) AND p.version=sqlc.arg(expected_proposal_version) AND w.version=sqlc.arg(expected_workspace_version)
FOR UPDATE OF p,w;

-- name: ListCleanProposalDraftSources :many
SELECT DISTINCT x.source_document_version_id AS document_version_id
FROM bid.proposal_response_versions x JOIN documents.document_versions v ON v.id=x.source_document_version_id AND v.scan_state='CLEAN'
WHERE x.proposal_version_id=sqlc.arg(proposal_version_id) AND x.source_document_version_id IS NOT NULL AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at))
ORDER BY x.source_document_version_id LIMIT 5;

-- name: GetProposalDraftInputObject :one
SELECT v.id AS document_version_id,sqlc.arg(ordinal)::integer AS ordinal,v.object_key,v.object_version,v.media_type,v.size_bytes,v.sha256
FROM documents.document_versions v WHERE v.id=sqlc.arg(document_version_id) AND v.scan_state='CLEAN' AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at));

-- name: InsertProposalDraftJob :exec
INSERT INTO jobs.jobs(id,kind,document_version_id,state,requested_by,created_at) VALUES(sqlc.arg(id),'PROPOSAL_DRAFT',sqlc.arg(document_version_id),'PENDING_RESERVATION',sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: InsertProposalDraftRun :exec
INSERT INTO ai.proposal_draft_runs(id,job_id,bid_id,proposal_id,proposal_version_id,requested_workspace_version,requested_proposal_version,provider,model,schema_version,state,requested_by,correlation_id,created_at,input_characters,estimated_credits,checkpoint_id)
VALUES(sqlc.arg(id),sqlc.arg(job_id),sqlc.arg(bid_id),sqlc.arg(proposal_id),sqlc.arg(proposal_version_id),sqlc.arg(workspace_version),sqlc.arg(proposal_version),'OPENAI',sqlc.arg(model),1,'PENDING_RESERVATION',sqlc.arg(actor_id),sqlc.arg(correlation_id),sqlc.arg(created_at),sqlc.arg(input_characters),sqlc.arg(estimated_credits),sqlc.arg(checkpoint_id));

-- name: InsertProposalDraftInput :exec
INSERT INTO ai.proposal_draft_inputs(run_id,document_version_id,ordinal) VALUES(sqlc.arg(run_id),sqlc.arg(document_version_id),sqlc.arg(ordinal));

-- name: GetProposalDraftRun :one
SELECT id,job_id,state,input_characters,estimated_credits,checkpoint_id FROM ai.proposal_draft_runs WHERE id=sqlc.arg(id);

-- name: CancelStaleProposalDrafts :exec
WITH stale AS (
 SELECT r.id FROM ai.proposal_draft_runs r JOIN jobs.jobs j ON j.id=r.job_id JOIN bid.workspaces w ON w.id=r.bid_id JOIN bid.proposals p ON p.id=r.proposal_id
 WHERE r.state='QUEUED' AND j.state='QUEUED' AND (w.version<>r.requested_workspace_version OR w.state<>'BID' OR w.archived_at IS NOT NULL OR p.version<>r.requested_proposal_version OR p.current_revision_id<>r.proposal_version_id)
 FOR UPDATE OF r,j
), cancelled AS (
 UPDATE ai.proposal_draft_runs r SET state='CANCELLED',completed_at=sqlc.arg(completed_at) FROM stale WHERE r.id=stale.id RETURNING r.job_id
)
UPDATE jobs.jobs j SET state='CANCELLED',completed_at=sqlc.arg(completed_at) FROM cancelled WHERE j.id=cancelled.job_id;

-- name: LockStaleRunningProposalDraft :one
SELECT r.id,r.job_id,r.correlation_id,r.input_characters,r.estimated_credits,r.checkpoint_id,c.reservation_id
FROM ai.proposal_draft_runs r JOIN jobs.jobs j ON j.id=r.job_id AND j.state='RUNNING' JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=r.checkpoint_id AND c.state='RESERVED'
WHERE r.state='RUNNING' AND r.started_at<sqlc.arg(stale_before) ORDER BY r.started_at,r.id FOR UPDATE OF r,j SKIP LOCKED LIMIT 1;

-- name: ClaimNextProposalDraft :one
WITH candidate AS (
 SELECT r.id FROM ai.proposal_draft_runs r JOIN jobs.jobs j ON j.id=r.job_id JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=r.checkpoint_id AND c.target_id=j.id AND c.state='RESERVED'
 WHERE r.state='READY_FOR_PROVIDER' AND j.state='READY_FOR_PROVIDER'
 ORDER BY r.created_at,r.id FOR UPDATE OF r,j SKIP LOCKED LIMIT 1
)
UPDATE ai.proposal_draft_runs r SET state='RUNNING',started_at=sqlc.arg(started_at)
FROM candidate c,jobs.jobs j,bid.workspaces w,bid.proposals p,bidora_entitlement.ai_usage_checkpoints cp
WHERE r.id=c.id AND j.id=r.job_id AND w.id=r.bid_id AND p.id=r.proposal_id AND cp.id=r.checkpoint_id AND cp.state='RESERVED'
RETURNING r.id,r.job_id,r.bid_id,r.proposal_id,r.proposal_version_id,r.requested_workspace_version,r.requested_proposal_version,r.model,r.requested_by,r.correlation_id,r.input_characters,r.estimated_credits,r.checkpoint_id,cp.reservation_id,cp.usage_kind,
 COALESCE((w.version=r.requested_workspace_version AND w.state='BID' AND w.archived_at IS NULL AND p.version=r.requested_proposal_version AND p.current_revision_id=r.proposal_version_id),false)::boolean AS snapshot_current;

-- name: StartProposalDraftJob :execrows
UPDATE jobs.jobs SET state='RUNNING',attempts=attempts+1,started_at=sqlc.arg(started_at) WHERE id=sqlc.arg(id) AND state='READY_FOR_PROVIDER' AND kind='PROPOSAL_DRAFT';

-- name: CountProposalDraftInputs :one
SELECT count(*)::bigint FROM ai.proposal_draft_inputs WHERE run_id=sqlc.arg(run_id);

-- name: ListProposalDraftInputObjects :many
SELECT i.document_version_id,i.ordinal,v.object_key,v.object_version,v.media_type,v.size_bytes,v.sha256
FROM ai.proposal_draft_inputs i JOIN documents.document_versions v ON v.id=i.document_version_id AND v.scan_state='CLEAN'
WHERE i.run_id=sqlc.arg(run_id) AND (v.expires_at IS NULL OR v.expires_at>sqlc.arg(observed_at)) ORDER BY i.ordinal;

-- name: InsertProposalDraftSuggestion :exec
INSERT INTO ai.proposal_draft_suggestions(id,run_id,introduction,closing,rationale,review_state,version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(run_id),sqlc.arg(introduction),sqlc.arg(closing),sqlc.arg(rationale),'PROPOSED_UNVERIFIED',1,sqlc.arg(created_at));

-- name: InsertProposalDraftCitation :exec
INSERT INTO ai.proposal_draft_citations(suggestion_id,ordinal,document_version_id,citation_location,citation_quote)
VALUES(sqlc.arg(suggestion_id),sqlc.arg(ordinal),sqlc.arg(document_version_id),sqlc.arg(location),sqlc.arg(quote));

-- name: CompleteProposalDraftRun :execrows
UPDATE ai.proposal_draft_runs SET state='SUCCEEDED',provider_response_id=sqlc.arg(provider_response_id),completed_at=sqlc.arg(completed_at) WHERE id=sqlc.arg(id) AND state='RUNNING';
-- name: FailProposalDraftRun :execrows
UPDATE ai.proposal_draft_runs SET state=sqlc.arg(state),failure_code=sqlc.narg(failure_code),completed_at=sqlc.arg(completed_at) WHERE id=sqlc.arg(id) AND state='RUNNING';
-- name: CompleteProposalDraftJob :execrows
UPDATE jobs.jobs SET state=sqlc.arg(state),completed_at=sqlc.arg(completed_at) WHERE id=sqlc.arg(id) AND state='RUNNING' AND kind='PROPOSAL_DRAFT';

-- name: ListProposalDraftSuggestions :many
SELECT s.id AS suggestion_id,r.id AS run_id,s.introduction,s.closing,s.rationale,s.review_state,s.version AS suggestion_version,s.applied_proposal_version_id,s.reviewed_by,s.reviewed_at,s.review_note,COALESCE(s.created_at,r.created_at)::timestamptz AS created_at,r.state AS run_state,r.proposal_version_id,r.requested_workspace_version,r.requested_proposal_version
FROM ai.proposal_draft_runs r LEFT JOIN ai.proposal_draft_suggestions s ON s.run_id=r.id
WHERE r.bid_id=sqlc.arg(bid_id) AND r.proposal_version_id=sqlc.arg(proposal_version_id) ORDER BY r.created_at DESC LIMIT 20;
-- name: ListProposalDraftCitations :many
SELECT ordinal,document_version_id,citation_location,citation_quote FROM ai.proposal_draft_citations WHERE suggestion_id=sqlc.arg(suggestion_id) ORDER BY ordinal;

-- name: InsertProposalDraftAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'ai_proposal_draft',sqlc.arg(run_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'job_id',sqlc.arg(job_id)::uuid),1,sqlc.arg(occurred_at));
-- name: InsertProposalDraftOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'ai_proposal_draft',sqlc.arg(run_id),1,sqlc.arg(event_type),1,jsonb_build_object('run_id',sqlc.arg(run_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
