-- name: LockDocumentEntitlementTarget :one
SELECT m.id AS measurement_id,m.job_id,m.document_version_id,m.requester_user_id,m.assignment_generation,j.state
FROM ai.document_measurement_entitlements e
JOIN ai.document_measurements m ON m.id=e.measurement_id
JOIN jobs.jobs j ON j.id=m.job_id AND j.document_version_id=m.document_version_id
WHERE e.checkpoint_id=sqlc.arg(checkpoint_id) AND j.kind='TENDER_ANALYSIS'
FOR UPDATE OF j;

-- name: LockResponseDraftEntitlementTarget :one
SELECT r.id AS run_id,r.job_id,r.requested_by,c.assignment_generation,r.state AS run_state,j.state AS job_state
FROM ai.response_draft_runs r
JOIN jobs.jobs j ON j.id=r.job_id AND j.kind='RESPONSE_DRAFT'
JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=r.checkpoint_id AND c.target_id=j.id AND c.target_type='RESPONSE_DRAFT'
WHERE r.checkpoint_id=sqlc.arg(checkpoint_id)
FOR UPDATE OF r,j;

-- name: LockProposalDraftEntitlementTarget :one
SELECT r.id AS run_id,r.job_id,r.requested_by,c.assignment_generation,r.state AS run_state,j.state AS job_state
FROM ai.proposal_draft_runs r JOIN jobs.jobs j ON j.id=r.job_id AND j.kind='PROPOSAL_DRAFT'
JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=r.checkpoint_id AND c.target_id=j.id AND c.target_type='PROPOSAL_DRAFT'
WHERE r.checkpoint_id=sqlc.arg(checkpoint_id) FOR UPDATE OF r,j;

-- name: MarkProposalDraftReadyForProvider :execrows
WITH updated AS (UPDATE ai.proposal_draft_runs r SET state='READY_FOR_PROVIDER' WHERE r.id=sqlc.arg(run_id) AND r.state='PENDING_RESERVATION' RETURNING r.job_id)
UPDATE jobs.jobs j SET state='READY_FOR_PROVIDER' FROM updated u WHERE j.id=u.job_id AND j.id=sqlc.arg(job_id) AND j.kind='PROPOSAL_DRAFT' AND j.state='PENDING_RESERVATION';
-- name: MarkProposalDraftEntitlementDenied :execrows
WITH updated AS (UPDATE ai.proposal_draft_runs r SET state='FAILED',failure_code='ENTITLEMENT_DENIED',completed_at=sqlc.arg(completed_at) WHERE r.id=sqlc.arg(run_id) AND r.state='PENDING_RESERVATION' RETURNING r.job_id)
UPDATE jobs.jobs j SET state='FAILED',completed_at=sqlc.arg(completed_at) FROM updated u WHERE j.id=u.job_id AND j.id=sqlc.arg(job_id) AND j.kind='PROPOSAL_DRAFT' AND j.state='PENDING_RESERVATION';
-- name: InsertProposalDraftEntitlementDispositionAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'ai_proposal_draft',sqlc.arg(run_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('checkpoint_id',sqlc.arg(checkpoint_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'job_state',sqlc.arg(job_state)::text,'disposition',sqlc.arg(disposition)::text,'denial_code',NULLIF(sqlc.arg(denial_code)::text,'')),1,sqlc.arg(occurred_at));
-- name: InsertProposalDraftEntitlementDispositionOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ai_proposal_draft',sqlc.arg(run_id),2,sqlc.arg(event_type),1,jsonb_build_object('run_id',sqlc.arg(run_id)::uuid,'checkpoint_id',sqlc.arg(checkpoint_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'job_state',sqlc.arg(job_state)::text,'disposition',sqlc.arg(disposition)::text,'denial_code',NULLIF(sqlc.arg(denial_code)::text,'')),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));

-- name: MarkResponseDraftReadyForProvider :execrows
WITH updated AS (
 UPDATE ai.response_draft_runs r SET state='READY_FOR_PROVIDER' WHERE r.id=sqlc.arg(run_id) AND state='PENDING_RESERVATION' RETURNING job_id
)
UPDATE jobs.jobs j SET state='READY_FOR_PROVIDER' FROM updated u WHERE j.id=u.job_id AND j.id=sqlc.arg(job_id) AND j.kind='RESPONSE_DRAFT' AND j.state='PENDING_RESERVATION';

-- name: MarkResponseDraftEntitlementDenied :execrows
WITH updated AS (
 UPDATE ai.response_draft_runs r SET state='FAILED',failure_code='ENTITLEMENT_DENIED',completed_at=sqlc.arg(completed_at) WHERE r.id=sqlc.arg(run_id) AND state='PENDING_RESERVATION' RETURNING job_id
)
UPDATE jobs.jobs j SET state='FAILED',completed_at=sqlc.arg(completed_at) FROM updated u WHERE j.id=u.job_id AND j.id=sqlc.arg(job_id) AND j.kind='RESPONSE_DRAFT' AND j.state='PENDING_RESERVATION';

-- name: InsertResponseDraftEntitlementDispositionAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'ai_response_draft',sqlc.arg(run_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('checkpoint_id',sqlc.arg(checkpoint_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'job_state',sqlc.arg(job_state)::text,'disposition',sqlc.arg(disposition)::text,'denial_code',NULLIF(sqlc.arg(denial_code)::text,'')),1,sqlc.arg(occurred_at));

-- name: InsertResponseDraftEntitlementDispositionOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ai_response_draft',sqlc.arg(run_id),2,sqlc.arg(event_type),1,jsonb_build_object('run_id',sqlc.arg(run_id)::uuid,'checkpoint_id',sqlc.arg(checkpoint_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'job_state',sqlc.arg(job_state)::text,'disposition',sqlc.arg(disposition)::text,'denial_code',NULLIF(sqlc.arg(denial_code)::text,'')),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));

-- name: MarkDocumentAnalysisReadyForProvider :execrows
UPDATE jobs.jobs SET state='READY_FOR_PROVIDER',blocked_reason=NULL
WHERE id=sqlc.arg(job_id) AND kind='TENDER_ANALYSIS' AND state='PENDING_RESERVATION' AND blocked_reason='AI_ENTITLEMENT_PENDING';

-- name: MarkDocumentAnalysisEntitlementDenied :execrows
UPDATE jobs.jobs SET state='FAILED',blocked_reason=NULL,completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(job_id) AND kind='TENDER_ANALYSIS' AND state='PENDING_RESERVATION' AND blocked_reason='AI_ENTITLEMENT_PENDING';

-- name: InsertDocumentEntitlementDispositionAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'document_measurement',sqlc.arg(measurement_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('checkpoint_id',sqlc.arg(checkpoint_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'job_state',sqlc.arg(job_state)::text,'disposition',sqlc.arg(disposition)::text,'denial_code',NULLIF(sqlc.arg(denial_code)::text,'')),1,sqlc.arg(occurred_at));

-- name: InsertDocumentEntitlementDispositionOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'document_measurement',sqlc.arg(measurement_id),2,sqlc.arg(event_type),1,jsonb_build_object('measurement_id',sqlc.arg(measurement_id)::uuid,'document_version_id',sqlc.arg(document_version_id)::uuid,'checkpoint_id',sqlc.arg(checkpoint_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'job_state',sqlc.arg(job_state)::text,'disposition',sqlc.arg(disposition)::text,'denial_code',NULLIF(sqlc.arg(denial_code)::text,'')),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));
