-- name: LockReviewTarget :one
SELECT w.id FROM bid.workspaces w
WHERE w.id=sqlc.arg(bid_id) AND w.archived_at IS NULL AND w.state='BID' AND (
 (sqlc.arg(target_type)::text='BID' AND sqlc.arg(target_id)::uuid=w.id) OR
 (sqlc.arg(target_type)::text='REQUIREMENT' AND EXISTS(SELECT 1 FROM bid.requirements r WHERE r.id=sqlc.arg(target_id) AND r.bid_id=w.id)) OR
 (sqlc.arg(target_type)::text='PROPOSAL_VERSION' AND EXISTS(SELECT 1 FROM bid.proposal_versions v JOIN bid.proposals p ON p.id=v.proposal_id WHERE v.id=sqlc.arg(target_id) AND p.bid_id=w.id))
) FOR KEY SHARE;

-- name: InsertReviewThread :exec
INSERT INTO bid.review_threads(id,bid_id,target_type,target_id,subject,status,version,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(target_type),sqlc.arg(target_id),sqlc.arg(subject),'OPEN',1,sqlc.arg(actor_id),sqlc.arg(created_at));
-- name: InsertReviewComment :exec
INSERT INTO bid.review_comments(id,thread_id,thread_version,comment_kind,body,created_by,created_at)
VALUES(sqlc.arg(id),sqlc.arg(thread_id),sqlc.arg(thread_version),sqlc.arg(comment_kind),sqlc.arg(body),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: LockReviewThread :one
SELECT t.id,t.bid_id,t.target_type,t.target_id,t.subject,t.status,t.version,t.created_by,t.created_at,t.resolved_by,t.resolved_at,t.assigned_to,t.due_at
FROM bid.review_threads t JOIN bid.workspaces w ON w.id=t.bid_id AND w.archived_at IS NULL AND w.state='BID'
WHERE t.id=sqlc.arg(id) AND t.bid_id=sqlc.arg(bid_id) AND t.version=sqlc.arg(expected_version) FOR UPDATE OF t;

-- name: AdvanceReviewThread :execrows
UPDATE bid.review_threads SET version=version+1 WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND status='OPEN';
-- name: ResolveReviewThread :execrows
UPDATE bid.review_threads SET status='RESOLVED',version=version+1,resolved_by=sqlc.arg(actor_id),resolved_at=sqlc.arg(resolved_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND status='OPEN';

-- name: GetReviewThread :one
SELECT id,bid_id,target_type,target_id,subject,status,version,created_by,created_at,resolved_by,resolved_at,assigned_to,due_at FROM bid.review_threads WHERE id=sqlc.arg(id) AND bid_id=sqlc.arg(bid_id);
-- name: ValidateReviewDashboardBid :one
SELECT id FROM bid.workspaces WHERE id=sqlc.arg(bid_id);
-- name: GetBidReviewDashboardCounts :one
SELECT count(*)::bigint AS total_count,
 count(*) FILTER(WHERE status='OPEN')::bigint AS open_count,
 count(*) FILTER(WHERE status='RESOLVED')::bigint AS resolved_count,
 count(*) FILTER(WHERE status='OPEN' AND assigned_to IS NULL)::bigint AS unassigned_count,
 count(*) FILTER(WHERE status='OPEN' AND due_at IS NOT NULL AND due_at<=sqlc.arg(observed_at))::bigint AS overdue_count
FROM bid.review_threads WHERE bid_id=sqlc.arg(bid_id);
-- name: ListBidReviewerWorkloads :many
SELECT assigned_to,count(*)::bigint AS open_count,
 count(*) FILTER(WHERE due_at IS NOT NULL AND due_at<=sqlc.arg(observed_at))::bigint AS overdue_count,
 min(due_at)::timestamptz AS earliest_due_at
FROM bid.review_threads WHERE bid_id=sqlc.arg(bid_id) AND status='OPEN' AND assigned_to IS NOT NULL
GROUP BY assigned_to ORDER BY assigned_to LIMIT 101;
-- name: GetTenantReviewWorkloadCounts :one
SELECT count(*) FILTER(WHERE t.status='OPEN')::bigint AS open_count,
 count(*) FILTER(WHERE t.status='OPEN' AND t.assigned_to IS NULL)::bigint AS unassigned_count,
 count(*) FILTER(WHERE t.status='OPEN' AND t.due_at IS NOT NULL AND t.due_at<=sqlc.arg(observed_at))::bigint AS overdue_count,
 count(DISTINCT t.bid_id) FILTER(WHERE t.status='OPEN')::bigint AS bid_count
FROM bid.review_threads t JOIN bid.workspaces w ON w.id=t.bid_id AND w.archived_at IS NULL;
-- name: ListTenantReviewerWorkloads :many
SELECT t.assigned_to,count(*)::bigint AS open_count,count(DISTINCT t.bid_id)::bigint AS bid_count,
 count(*) FILTER(WHERE t.due_at IS NOT NULL AND t.due_at<=sqlc.arg(observed_at))::bigint AS overdue_count,
 min(t.due_at)::timestamptz AS earliest_due_at
FROM bid.review_threads t JOIN bid.workspaces w ON w.id=t.bid_id AND w.archived_at IS NULL
WHERE t.status='OPEN' AND t.assigned_to IS NOT NULL
GROUP BY t.assigned_to ORDER BY t.assigned_to LIMIT 101;
-- name: ListReviewerOpenWork :many
SELECT t.id,t.bid_id,t.target_type,t.target_id,t.subject,t.version,t.due_at,t.created_at
FROM bid.review_threads t JOIN bid.workspaces w ON w.id=t.bid_id AND w.archived_at IS NULL
WHERE t.status='OPEN' AND t.assigned_to=sqlc.arg(assigned_to)
ORDER BY t.due_at ASC NULLS LAST,t.created_at ASC,t.id ASC LIMIT 201;

-- name: CountBidReviewThreads :one
SELECT count(*)::bigint FROM bid.review_threads WHERE bid_id=sqlc.arg(bid_id);
-- name: ListReviewThreads :many
SELECT id,bid_id,target_type,target_id,subject,status,version,created_by,created_at,resolved_by,resolved_at,assigned_to,due_at FROM bid.review_threads WHERE bid_id=sqlc.arg(bid_id) ORDER BY created_at DESC,id DESC LIMIT 100;
-- name: UpdateReviewAssignment :execrows
UPDATE bid.review_threads SET assigned_to=sqlc.arg(assigned_to),due_at=sqlc.narg(due_at),version=version+1
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND status='OPEN';
-- name: InsertReviewAssignment :exec
INSERT INTO bid.review_assignments(id,thread_id,thread_version,assigned_to,due_at,assignment_reason,assigned_by,assigned_at)
VALUES(sqlc.arg(id),sqlc.arg(thread_id),sqlc.arg(thread_version),sqlc.arg(assigned_to),sqlc.narg(due_at),sqlc.arg(reason),sqlc.arg(actor_id),sqlc.arg(assigned_at));
-- name: ListReviewAssignmentHistory :many
SELECT id,thread_id,thread_version,assigned_to,due_at,assignment_reason,assigned_by,assigned_at FROM bid.review_assignments WHERE thread_id=sqlc.arg(thread_id) ORDER BY thread_version DESC LIMIT 100;

-- name: CountReviewComments :one
SELECT count(*)::bigint FROM bid.review_comments WHERE thread_id=sqlc.arg(thread_id);
-- name: ListReviewComments :many
SELECT id,thread_id,thread_version,comment_kind,body,created_by,created_at FROM bid.review_comments WHERE thread_id=sqlc.arg(thread_id) ORDER BY thread_version LIMIT 200;

-- name: InsertReviewAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'bid_review_thread',sqlc.arg(thread_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'thread_version',sqlc.arg(thread_version)::bigint),1,sqlc.arg(occurred_at));
-- name: InsertReviewOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'bid_review_thread',sqlc.arg(thread_id),sqlc.arg(thread_version),sqlc.arg(event_type),1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'thread_id',sqlc.arg(thread_id)::uuid,'thread_version',sqlc.arg(thread_version)::bigint),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
