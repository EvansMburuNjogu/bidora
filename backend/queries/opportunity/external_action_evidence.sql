-- name: LockPackageForExternalEvidence :one
SELECT m.id AS package_manifest_id,m.proposal_version_id,m.package_status,w.version AS workspace_version
FROM bid.proposal_package_manifests m JOIN bid.workspaces w ON w.id=m.bid_id
JOIN bid.proposal_approvals a ON a.package_manifest_id=m.id AND a.bid_id=m.bid_id AND a.proposal_version_id=m.proposal_version_id
WHERE m.id=sqlc.arg(package_manifest_id) AND m.bid_id=sqlc.arg(bid_id) AND w.archived_at IS NULL AND w.state='BID'
FOR UPDATE OF w;

-- name: ValidateSupersededExternalEvidence :one
SELECT id FROM bid.external_action_evidence WHERE id=sqlc.arg(id) AND bid_id=sqlc.arg(bid_id);

-- name: InsertExternalActionEvidence :exec
INSERT INTO bid.external_action_evidence(id,bid_id,package_manifest_id,report_type,verification_status,external_system_name,external_reference,reported_action_time_text,reported_action_at,report_note,supersedes_evidence_id,recorded_by,recorded_at)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(package_manifest_id),'EXTERNAL_ACTION_REPORTED','REPORTED_UNVERIFIED',sqlc.arg(external_system_name),sqlc.narg(external_reference),sqlc.arg(reported_action_time_text),sqlc.arg(reported_action_at),sqlc.arg(report_note),sqlc.narg(supersedes_evidence_id),sqlc.arg(actor_id),sqlc.arg(recorded_at));

-- name: GetExternalActionEvidence :one
SELECT id,bid_id,package_manifest_id,report_type,verification_status,external_system_name,external_reference,reported_action_time_text,reported_action_at,report_note,supersedes_evidence_id,recorded_by,recorded_at
FROM bid.external_action_evidence WHERE id=sqlc.arg(id) AND bid_id=sqlc.arg(bid_id);

-- name: ListExternalActionEvidence :many
SELECT id,bid_id,package_manifest_id,report_type,verification_status,external_system_name,external_reference,reported_action_time_text,reported_action_at,report_note,supersedes_evidence_id,recorded_by,recorded_at
FROM bid.external_action_evidence
WHERE bid_id=sqlc.arg(bid_id) AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (recorded_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY recorded_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: GetExternalEvidenceReconciliationCounts :one
SELECT w.id AS bid_id,p.current_revision_id,
 (SELECT count(*)::bigint FROM bid.external_action_evidence e WHERE e.bid_id=w.id) AS report_count,
 (SELECT count(*)::bigint FROM bid.external_action_evidence e WHERE e.bid_id=w.id AND NOT EXISTS(SELECT 1 FROM bid.external_action_evidence s WHERE s.supersedes_evidence_id=e.id)) AS active_report_count,
 EXISTS(SELECT 1 FROM bid.submission_confirmations c WHERE c.bid_id=w.id) AS submitted
FROM bid.workspaces w LEFT JOIN bid.proposals p ON p.bid_id=w.id
WHERE w.id=sqlc.arg(bid_id);

-- name: ListActiveExternalEvidenceReconciliation :many
WITH RECURSIVE leaves AS (
 SELECT e.* FROM bid.external_action_evidence e
 WHERE e.bid_id=sqlc.arg(bid_id) AND NOT EXISTS(SELECT 1 FROM bid.external_action_evidence s WHERE s.supersedes_evidence_id=e.id)
), lineage AS (
 SELECT l.id AS leaf_id,l.id AS node_id,l.supersedes_evidence_id,1::integer AS depth FROM leaves l
 UNION ALL
 SELECT x.leaf_id,p.id,p.supersedes_evidence_id,x.depth+1 FROM lineage x
 JOIN bid.external_action_evidence p ON p.id=x.supersedes_evidence_id
 WHERE x.depth<100
), depths AS (
 SELECT x.leaf_id,max(x.depth)::integer AS lineage_depth,bool_or(x.depth=100 AND x.supersedes_evidence_id IS NOT NULL) AS lineage_truncated
 FROM lineage x GROUP BY x.leaf_id
)
SELECT l.id AS evidence_id,l.package_manifest_id,l.reported_action_at,l.recorded_at,l.verification_status,l.supersedes_evidence_id,
 m.proposal_version_id,m.package_status,m.readiness_ready,m.readiness_observed_at,d.lineage_depth,d.lineage_truncated
FROM leaves l JOIN depths d ON d.leaf_id=l.id JOIN bid.proposal_package_manifests m ON m.id=l.package_manifest_id AND m.bid_id=l.bid_id
ORDER BY l.recorded_at DESC,l.id DESC LIMIT 101;

-- name: InsertExternalActionEvidenceAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ExternalActionEvidenceReported','external_action_evidence',sqlc.arg(evidence_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'package_manifest_id',sqlc.arg(package_manifest_id)::uuid,'verification_status','REPORTED_UNVERIFIED','submitted_state_changed',false),1,sqlc.arg(occurred_at));

-- name: InsertExternalActionEvidenceOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'external_action_evidence',sqlc.arg(evidence_id),1,'bid.external_action_evidence.reported.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'package_manifest_id',sqlc.arg(package_manifest_id)::uuid,'verification_status','REPORTED_UNVERIFIED','submitted_state_changed',false),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
