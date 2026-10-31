-- name: GetProposalApprovalIdempotency :one
SELECT request_hash,state,response_status,response_ref,response_version FROM bidora_commands.idempotency_records
WHERE actor_id=sqlc.arg(actor_id) AND operation='APPROVE_PROPOSAL_PACKAGE' AND key_hash=sqlc.arg(key_hash);

-- name: LockProposalPackageForApproval :one
SELECT m.id AS manifest_id,m.bid_id,m.proposal_id,m.proposal_version_id,m.manifest_sha256,m.manifest_version,
 p.version AS proposal_version,p.current_revision_id,pv.created_by AS proposal_creator_id,w.version AS workspace_version,
 d.id AS review_decision_id,d.decision AS review_decision
FROM bid.proposal_package_manifests m
JOIN bid.proposals p ON p.id=m.proposal_id AND p.bid_id=m.bid_id
JOIN bid.proposal_versions pv ON pv.id=m.proposal_version_id AND pv.proposal_id=p.id
JOIN bid.workspaces w ON w.id=m.bid_id
JOIN LATERAL (
 SELECT x.id,x.decision FROM bid.internal_review_decisions x
 WHERE x.proposal_version_id=m.proposal_version_id AND x.reviewer_id=sqlc.arg(approver_id)
 ORDER BY x.reviewer_sequence DESC LIMIT 1
) d ON true
WHERE m.id=sqlc.arg(manifest_id) AND m.bid_id=sqlc.arg(bid_id) AND m.package_status='UNAPPROVED_PREPARATION_ONLY'
 AND w.archived_at IS NULL AND w.state='BID' AND p.current_revision_id=m.proposal_version_id
FOR UPDATE OF p,w;

-- name: GetProposalApprovalReviewBlockers :one
WITH latest AS (
 SELECT DISTINCT ON (reviewer_id) reviewer_id,decision
 FROM bid.internal_review_decisions WHERE proposal_version_id=sqlc.arg(proposal_version_id)
 ORDER BY reviewer_id,reviewer_sequence DESC
)
SELECT
 (SELECT count(*)::bigint FROM latest WHERE decision='CHANGES_REQUESTED') AS changes_requested_count,
 (SELECT count(*)::bigint FROM bid.review_threads rt WHERE rt.bid_id=sqlc.arg(bid_id) AND rt.status='OPEN') AS open_thread_count,
 (SELECT count(*)::bigint FROM bid.review_threads rt WHERE rt.bid_id=sqlc.arg(bid_id) AND rt.status='OPEN' AND rt.assigned_to IS NULL) AS unassigned_count,
 (SELECT count(*)::bigint FROM bid.review_threads rt WHERE rt.bid_id=sqlc.arg(bid_id) AND rt.status='OPEN' AND rt.due_at IS NOT NULL AND rt.due_at<=sqlc.arg(observed_at)) AS overdue_count,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=sqlc.arg(bid_id) AND r.mandatory AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at))))) AS mandatory_unsatisfied,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=sqlc.arg(bid_id) AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at)))) AND r.owner_id IS NULL) AS open_unassigned,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=sqlc.arg(bid_id) AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at)))) AND r.due_at IS NOT NULL AND r.due_at<sqlc.arg(observed_at)) AS open_overdue,
 (SELECT count(*)::bigint FROM bid.requirements r LEFT JOIN bid.response_sections s ON s.bid_id=r.bid_id AND s.requirement_id=r.id LEFT JOIN bid.response_versions rv ON rv.id=s.current_revision_id WHERE r.bid_id=sqlc.arg(bid_id) AND r.mandatory AND (s.id IS NULL OR rv.requirement_version_at_creation<>r.version)) AS mandatory_response_missing,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=sqlc.arg(bid_id) AND r.status='SATISFIED' AND EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND dv.expires_at IS NOT NULL AND dv.expires_at<=sqlc.arg(observed_at)) AND NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at)))) AS expired_evidence_count;

-- name: InsertProposalApproval :exec
INSERT INTO bid.proposal_approvals(id,bid_id,proposal_id,proposal_version_id,package_manifest_id,review_decision_id,approver_id,policy_version,rationale,approved_at)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(proposal_id),sqlc.arg(proposal_version_id),sqlc.arg(manifest_id),sqlc.arg(review_decision_id),sqlc.arg(approver_id),1,sqlc.narg(rationale),sqlc.arg(approved_at));

-- name: GetProposalApproval :one
SELECT id,bid_id,proposal_id,proposal_version_id,package_manifest_id,review_decision_id,approver_id,policy_version,rationale,approved_at
FROM bid.proposal_approvals WHERE id=sqlc.arg(id);

-- name: GetProposalApprovalByManifest :one
SELECT id,bid_id,proposal_id,proposal_version_id,package_manifest_id,review_decision_id,approver_id,policy_version,rationale,approved_at
FROM bid.proposal_approvals WHERE bid_id=sqlc.arg(bid_id) AND package_manifest_id=sqlc.arg(manifest_id);

-- name: InsertProposalApprovalAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ProposalPackageApproved','proposal_approval',sqlc.arg(approval_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'manifest_id',sqlc.arg(manifest_id)::uuid,'review_decision_id',sqlc.arg(review_decision_id)::uuid),1,sqlc.arg(occurred_at));

-- name: InsertProposalApprovalOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'proposal_approval',sqlc.arg(approval_id),1,'bid.proposal_package.approved.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'manifest_id',sqlc.arg(manifest_id)::uuid,'review_decision_id',sqlc.arg(review_decision_id)::uuid,'approver_id',sqlc.arg(actor_id)::uuid,'policy_version',1),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
