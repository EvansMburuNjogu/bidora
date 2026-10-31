-- name: ClaimWorkflowIdempotency :one
INSERT INTO bidora_commands.idempotency_records
(id,actor_id,operation,key_hash,request_hash,state,created_at)
VALUES (sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(operation),sqlc.arg(key_hash),sqlc.arg(request_hash),'PENDING',sqlc.arg(created_at))
ON CONFLICT (actor_id,operation,key_hash) DO NOTHING
RETURNING id;

-- name: GetWorkflowIdempotencyForUpdate :one
SELECT id,request_hash,state,response_status,response_ref,response_version
FROM bidora_commands.idempotency_records
WHERE actor_id=sqlc.arg(actor_id) AND operation=sqlc.arg(operation) AND key_hash=sqlc.arg(key_hash)
FOR UPDATE;

-- name: CompleteWorkflowIdempotency :execrows
UPDATE bidora_commands.idempotency_records
SET state='COMPLETED',response_status=sqlc.arg(response_status),response_ref=sqlc.arg(response_ref),response_version=sqlc.arg(response_version),completed_at=sqlc.arg(completed_at)
WHERE id=sqlc.arg(id) AND state='PENDING';

-- name: LockOpportunityForVerification :one
SELECT o.id,o.version,o.current_source_version_id,s.revision,s.title,s.buyer_name,s.source_uri,
       s.source_document_version_id,s.closes_at,s.source_timezone,s.source_date_text,s.request_hash
FROM bidora_opportunity.opportunities o
JOIN bidora_opportunity.source_versions s ON s.opportunity_id=o.id AND s.id=o.current_source_version_id
WHERE o.id=sqlc.arg(opportunity_id) AND o.version=sqlc.arg(expected_version)
  AND s.id=sqlc.arg(source_version_id) AND s.verification_state='UNVERIFIED'
FOR UPDATE OF o;

-- name: InsertVerifiedSourceVersion :exec
INSERT INTO bidora_opportunity.source_versions
(id,opportunity_id,revision,title,buyer_name,source_uri,source_document_version_id,closes_at,source_timezone,source_date_text,verification_state,request_hash,created_at,created_by,verified_by,verified_at)
VALUES (sqlc.arg(id),sqlc.arg(opportunity_id),sqlc.arg(revision),sqlc.arg(title),sqlc.narg(buyer_name),sqlc.narg(source_uri),sqlc.narg(source_document_version_id),sqlc.narg(closes_at),sqlc.narg(source_timezone),sqlc.narg(source_date_text),'VERIFIED',sqlc.arg(request_hash),sqlc.arg(created_at),sqlc.arg(actor_id),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: AdvanceVerifiedOpportunity :execrows
UPDATE bidora_opportunity.opportunities
SET current_source_version_id=sqlc.arg(source_version_id),version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version);

-- name: LockVerifiedOpportunityForWorkspace :one
SELECT o.id,o.version,s.id AS source_version_id
FROM bidora_opportunity.opportunities o
JOIN bidora_opportunity.source_versions s ON s.opportunity_id=o.id
WHERE o.id=sqlc.arg(opportunity_id) AND o.version=sqlc.arg(expected_version)
  AND o.current_source_version_id=sqlc.arg(source_version_id)
  AND s.id=sqlc.arg(source_version_id) AND s.verification_state='VERIFIED'
FOR UPDATE OF o;

-- name: InsertBidWorkspace :exec
INSERT INTO bid.workspaces
(id,opportunity_id,source_version_id,owner_id,state,version,created_by,created_at,updated_at)
VALUES (sqlc.arg(id),sqlc.arg(opportunity_id),sqlc.arg(source_version_id),sqlc.arg(owner_id),'DECISION_PENDING',1,sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(created_at));

-- name: GetActiveWorkspaceByOpportunity :one
SELECT id,version FROM bid.workspaces WHERE opportunity_id=sqlc.arg(opportunity_id) AND archived_at IS NULL;

-- name: LockBidWorkspaceForDecision :one
SELECT id,version,state
FROM bid.workspaces
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND archived_at IS NULL
FOR UPDATE;

-- name: InsertBidDecision :exec
INSERT INTO bid.decisions
(id,bid_id,sequence,workspace_version,decision,rationale,actor_id,decided_at)
VALUES (sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(sequence),sqlc.arg(workspace_version),sqlc.arg(decision),sqlc.arg(rationale),sqlc.arg(actor_id),sqlc.arg(decided_at));

-- name: AdvanceBidWorkspaceDecision :execrows
UPDATE bid.workspaces
SET state=sqlc.arg(state),version=version+1,updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND archived_at IS NULL;

-- name: InsertBidDecisionAudit :exec
INSERT INTO bidora_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES (sqlc.arg(id),sqlc.arg(actor_id),'BidDecisionRecorded','bid_workspace',sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('decision_id',sqlc.arg(decision_id)::uuid,'decision',sqlc.arg(decision)::text,'workspace_version',sqlc.arg(workspace_version)::bigint),1,sqlc.arg(occurred_at));

-- name: InsertBidDecisionOutbox :exec
INSERT INTO bidora_eventing.outbox_events
(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES (sqlc.arg(id),sqlc.arg(actor_id),'bid_workspace',sqlc.arg(aggregate_id),sqlc.arg(aggregate_version),'bid.decision.recorded.v1',1,jsonb_build_object('bid_id',sqlc.arg(aggregate_id)::uuid,'decision_id',sqlc.arg(decision_id)::uuid,'decision',sqlc.arg(decision)::text),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));

-- name: InsertOpportunityVerifiedAudit :exec
INSERT INTO bidora_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES (sqlc.arg(id),sqlc.arg(actor_id),'OpportunityVerified','opportunity',sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('source_version_id',sqlc.arg(source_version_id)::uuid),1,sqlc.arg(occurred_at));

-- name: InsertOpportunityVerifiedOutbox :exec
INSERT INTO bidora_eventing.outbox_events
(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES (sqlc.arg(id),sqlc.arg(actor_id),'opportunity',sqlc.arg(aggregate_id),sqlc.arg(aggregate_version),'opportunity.verified.v1',1,jsonb_build_object('opportunity_id',sqlc.arg(aggregate_id)::uuid,'source_version_id',sqlc.arg(source_version_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));

-- name: InsertBidWorkspaceCreatedAudit :exec
INSERT INTO bidora_audit.audit_events
(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES (sqlc.arg(id),sqlc.arg(actor_id),'BidWorkspaceCreated','bid_workspace',sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('opportunity_id',sqlc.arg(opportunity_id)::uuid,'source_version_id',sqlc.arg(source_version_id)::uuid,'owner_id',sqlc.arg(owner_id)::uuid),1,sqlc.arg(occurred_at));

-- name: InsertBidWorkspaceCreatedOutbox :exec
INSERT INTO bidora_eventing.outbox_events
(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES (sqlc.arg(id),sqlc.arg(actor_id),'bid_workspace',sqlc.arg(aggregate_id),1,'bid.workspace.created.v1',1,jsonb_build_object('bid_id',sqlc.arg(aggregate_id)::uuid,'opportunity_id',sqlc.arg(opportunity_id)::uuid,'source_version_id',sqlc.arg(source_version_id)::uuid,'owner_id',sqlc.arg(owner_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));
