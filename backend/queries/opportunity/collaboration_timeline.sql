-- name: ListBidCollaborationTimeline :many
WITH target_bid AS (
 SELECT w.id FROM bid.workspaces w WHERE w.id=sqlc.arg(bid_id)
), events AS (
 SELECT t.id AS event_id,'REVIEW_THREAD_CREATED'::text AS event_type,t.created_by AS actor_id,t.created_at AS occurred_at,
        'review_thread'::text AS target_type,t.id AS target_id,1::bigint AS event_version,
        t.target_type::text AS detail_code,t.target_id::uuid AS related_id,NULL::timestamptz AS due_at
 FROM bid.review_threads t JOIN target_bid b ON b.id=t.bid_id
 UNION ALL
 SELECT c.id,CASE WHEN c.comment_kind='RESOLUTION' THEN 'REVIEW_THREAD_RESOLVED' ELSE 'REVIEW_COMMENT_ADDED' END,
        c.created_by,c.created_at,'review_thread',c.thread_id,c.thread_version,
        c.comment_kind::text,NULL::uuid,NULL::timestamptz
 FROM bid.review_comments c JOIN bid.review_threads t ON t.id=c.thread_id JOIN target_bid b ON b.id=t.bid_id
 WHERE c.comment_kind<>'INITIAL'
 UNION ALL
 SELECT a.id,'REVIEW_ASSIGNMENT_RECORDED',a.assigned_by,a.assigned_at,'review_thread',a.thread_id,a.thread_version,
        'ASSIGNED'::text,a.assigned_to::uuid,a.due_at
 FROM bid.review_assignments a JOIN bid.review_threads t ON t.id=a.thread_id JOIN target_bid b ON b.id=t.bid_id
 UNION ALL
 SELECT d.id,'INTERNAL_REVIEW_DECISION_RECORDED',d.reviewer_id,d.decided_at,'proposal_version',d.proposal_version_id,d.reviewer_sequence::bigint,
        d.decision::text,NULL::uuid,NULL::timestamptz
 FROM bid.internal_review_decisions d JOIN target_bid b ON b.id=d.bid_id
)
SELECT event_id,event_type,actor_id,occurred_at,target_type,target_id,event_version,detail_code,related_id,due_at
FROM events
WHERE sqlc.narg(cursor_at)::timestamptz IS NULL OR (occurred_at,event_id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid)
ORDER BY occurred_at DESC,event_id DESC
LIMIT sqlc.arg(page_limit);
