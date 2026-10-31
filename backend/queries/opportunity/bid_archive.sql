-- name: GetBidArchiveIdempotency :one
SELECT request_hash,state,response_status,response_ref,response_version FROM bidora_commands.idempotency_records WHERE actor_id=sqlc.arg(actor_id) AND operation='ARCHIVE_BID' AND key_hash=sqlc.arg(key_hash);
-- name: LockBidForArchive :one
SELECT w.id,w.state,w.version,w.archived_at,
 (SELECT count(*)::bigint FROM bid.review_threads t WHERE t.bid_id=w.id AND t.status='OPEN') AS open_reviews,
 (SELECT count(*)::bigint FROM documents.upload_sessions u WHERE (u.bid_id=w.id OR u.opportunity_id=w.opportunity_id) AND u.state='INITIATED' AND u.expires_at>sqlc.arg(observed_at)) AS pending_uploads,
 (SELECT count(*)::bigint FROM jobs.jobs j JOIN documents.document_versions v ON v.id=j.document_version_id JOIN documents.documents d ON d.id=v.document_id WHERE (d.bid_id=w.id OR d.opportunity_id=w.opportunity_id) AND (j.state IN ('QUEUED','WAITING_FOR_CLEAN','MEASURING','PENDING_RESERVATION','READY_FOR_PROVIDER','RUNNING') OR EXISTS(SELECT 1 FROM ai.document_measurements m JOIN ai.document_measurement_entitlements e ON e.measurement_id=m.id JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=e.checkpoint_id WHERE m.job_id=j.id AND c.state IN('RESERVED','SETTLE_PENDING','RELEASE_PENDING')))) AS pending_jobs,
 EXISTS(SELECT 1 FROM bid.submission_confirmations c WHERE c.bid_id=w.id) AS has_submission,
 COALESCE((SELECT outcome FROM bid.reported_outcomes o WHERE o.bid_id=w.id AND NOT EXISTS(SELECT 1 FROM bid.reported_outcomes x WHERE x.supersedes_outcome_id=o.id) ORDER BY sequence DESC LIMIT 1),'') AS current_outcome
FROM bid.workspaces w WHERE w.id=sqlc.arg(bid_id) FOR UPDATE;
-- name: ArchiveBidWorkspace :execrows
UPDATE bid.workspaces SET archived_at=sqlc.arg(archived_at),archived_by=sqlc.arg(actor_id),archive_reason=sqlc.arg(reason),archive_policy_version=1,version=version+1,updated_at=sqlc.arg(archived_at) WHERE id=sqlc.arg(bid_id) AND version=sqlc.arg(expected_version) AND archived_at IS NULL AND state IN ('NO_BID','SUBMITTED');
-- name: InsertBidArchiveEvent :exec
INSERT INTO bid.archive_events(id,bid_id,workspace_version,archived_by,reason,policy_version,archived_at) VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(workspace_version),sqlc.arg(actor_id),sqlc.arg(reason),1,sqlc.arg(archived_at));
-- name: GetBidArchiveEvent :one
SELECT id,bid_id,workspace_version,archived_by,reason,policy_version,archived_at FROM bid.archive_events WHERE id=sqlc.arg(id);
-- name: GetBidArchiveEventByBid :one
SELECT id,bid_id,workspace_version,archived_by,reason,policy_version,archived_at FROM bid.archive_events WHERE bid_id=sqlc.arg(bid_id);
-- name: InsertBidArchiveAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),sqlc.arg(actor_id),'BidArchived','bid_workspace',sqlc.arg(bid_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('workspace_version',sqlc.arg(workspace_version)::bigint,'read_only',true,'deleted',false,'retention_disposition_started',false,'legal_hold_changed',false),1,sqlc.arg(occurred_at));
-- name: InsertBidArchiveOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id) VALUES(sqlc.arg(id),'bid',sqlc.arg(bid_id),sqlc.arg(workspace_version),'bid.archived.v1',1,jsonb_build_object('archive_event_id',sqlc.arg(archive_id)::uuid,'workspace_version',sqlc.arg(workspace_version)::bigint,'read_only',true,'deleted',false),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
