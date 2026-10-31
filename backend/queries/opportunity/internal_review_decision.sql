-- name: LockCurrentProposalVersionForInternalReview :one
SELECT pv.id AS proposal_version_id,p.id AS proposal_id,p.current_revision_id,w.id AS bid_id
FROM bid.proposal_versions pv
JOIN bid.proposals p ON p.id=pv.proposal_id
JOIN bid.workspaces w ON w.id=p.bid_id
WHERE w.id=sqlc.arg(bid_id) AND w.archived_at IS NULL AND w.state='BID' AND p.current_revision_id=pv.id AND pv.id=sqlc.arg(proposal_version_id)
FOR UPDATE OF p;

-- name: GetReviewerDecisionSequence :one
SELECT coalesce(max(reviewer_sequence),0)::integer FROM bid.internal_review_decisions
WHERE proposal_version_id=sqlc.arg(proposal_version_id) AND reviewer_id=sqlc.arg(reviewer_id);

-- name: InsertInternalReviewDecision :exec
INSERT INTO bid.internal_review_decisions(id,bid_id,proposal_id,proposal_version_id,reviewer_id,reviewer_sequence,decision,rationale,decided_at)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(proposal_id),sqlc.arg(proposal_version_id),sqlc.arg(reviewer_id),sqlc.arg(reviewer_sequence),sqlc.arg(decision),sqlc.arg(rationale),sqlc.arg(decided_at));

-- name: ValidateProposalVersionForInternalReviewRead :one
SELECT pv.id FROM bid.proposal_versions pv JOIN bid.proposals p ON p.id=pv.proposal_id JOIN bid.workspaces w ON w.id=p.bid_id
WHERE w.id=sqlc.arg(bid_id) AND pv.id=sqlc.arg(proposal_version_id);

-- name: GetInternalReviewDecision :one
SELECT id,bid_id,proposal_id,proposal_version_id,reviewer_id,reviewer_sequence,decision,rationale,decided_at
FROM bid.internal_review_decisions WHERE id=sqlc.arg(id);

-- name: ListInternalReviewDecisionHistory :many
SELECT id,bid_id,proposal_id,proposal_version_id,reviewer_id,reviewer_sequence,decision,rationale,decided_at
FROM bid.internal_review_decisions WHERE bid_id=sqlc.arg(bid_id) AND proposal_version_id=sqlc.arg(proposal_version_id)
ORDER BY decided_at DESC,id DESC LIMIT 201;

-- name: GetApprovalPolicyPreparation :one
WITH current_proposal AS (
 SELECT p.current_revision_id AS proposal_version_id
 FROM bid.workspaces w JOIN bid.proposals p ON p.bid_id=w.id
 WHERE w.id=sqlc.arg(bid_id) AND w.archived_at IS NULL AND w.state='BID'
), latest AS (
 SELECT DISTINCT ON (d.reviewer_id) d.reviewer_id,d.decision
 FROM bid.internal_review_decisions d JOIN current_proposal cp ON cp.proposal_version_id=d.proposal_version_id
 ORDER BY d.reviewer_id,d.reviewer_sequence DESC
)
SELECT cp.proposal_version_id,
 (SELECT count(*)::bigint FROM latest) AS reviewer_count,
 (SELECT count(*)::bigint FROM latest WHERE decision='REVIEW_COMPLETE') AS review_complete_count,
 (SELECT count(*)::bigint FROM latest WHERE decision='CHANGES_REQUESTED') AS changes_requested_count,
 (SELECT count(*)::bigint FROM bid.review_threads t WHERE t.bid_id=sqlc.arg(bid_id) AND t.status='OPEN') AS open_review_thread_count
FROM current_proposal cp;

-- name: InsertInternalReviewDecisionAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'InternalReviewDecisionRecorded','proposal_version',sqlc.arg(proposal_version_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'reviewer_sequence',sqlc.arg(reviewer_sequence)::integer,'decision',sqlc.arg(decision)::text),1,sqlc.arg(occurred_at));

-- name: InsertInternalReviewDecisionOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'internal_review_decision',sqlc.arg(decision_id),1,'bid.internal_review_decision.recorded.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'reviewer_id',sqlc.arg(reviewer_id)::uuid,'reviewer_sequence',sqlc.arg(reviewer_sequence)::integer,'decision',sqlc.arg(decision)::text),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
