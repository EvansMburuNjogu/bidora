-- name: ListOwnershipContinuityActivity :many
WITH activity AS (
 SELECT c.created_by AS user_id,max(c.created_at) AS last_activity_at FROM bid.review_comments c GROUP BY c.created_by
 UNION ALL SELECT a.assigned_by,max(a.assigned_at) FROM bid.review_assignments a GROUP BY a.assigned_by
 UNION ALL SELECT d.reviewer_id,max(d.decided_at) FROM bid.internal_review_decisions d GROUP BY d.reviewer_id
 UNION ALL SELECT p.created_by,max(p.created_at) FROM bid.proposal_package_manifests p GROUP BY p.created_by
 UNION ALL SELECT e.recorded_by,max(e.recorded_at) FROM bid.external_action_evidence e GROUP BY e.recorded_by
), latest_activity AS (SELECT user_id,max(last_activity_at)::timestamptz AS last_activity_at FROM activity GROUP BY user_id), identities AS (
 SELECT owner_id AS user_id FROM bid.workspaces WHERE owner_id IS NOT NULL AND archived_at IS NULL
 UNION SELECT r.owner_id FROM bid.requirements r JOIN bid.workspaces w ON w.id=r.bid_id AND w.archived_at IS NULL WHERE r.owner_id IS NOT NULL AND r.status='OPEN'
 UNION SELECT t.assigned_to FROM bid.review_threads t JOIN bid.workspaces w ON w.id=t.bid_id AND w.archived_at IS NULL WHERE t.assigned_to IS NOT NULL AND t.status='OPEN'
 UNION SELECT user_id FROM latest_activity
)
SELECT i.user_id,
 (SELECT count(*)::bigint FROM bid.workspaces w WHERE w.owner_id=i.user_id AND w.archived_at IS NULL) AS owned_bid_count,
 (SELECT count(*)::bigint FROM bid.requirements r JOIN bid.workspaces w ON w.id=r.bid_id AND w.archived_at IS NULL WHERE r.owner_id=i.user_id AND r.status='OPEN') AS open_requirement_count,
 (SELECT count(*)::bigint FROM bid.review_threads t JOIN bid.workspaces w ON w.id=t.bid_id AND w.archived_at IS NULL WHERE t.assigned_to=i.user_id AND t.status='OPEN') AS open_review_count,
 (SELECT count(*)::bigint FROM bid.review_threads t JOIN bid.workspaces w ON w.id=t.bid_id AND w.archived_at IS NULL WHERE t.assigned_to=i.user_id AND t.status='OPEN' AND t.due_at IS NOT NULL AND t.due_at<=sqlc.arg(observed_at)) AS overdue_review_count,
 a.last_activity_at
FROM identities i LEFT JOIN latest_activity a ON a.user_id=i.user_id
ORDER BY i.user_id LIMIT 501;
