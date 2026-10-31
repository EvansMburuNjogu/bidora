-- name: GetBidArchivePreparation :one
SELECT w.id AS bid_id,w.state AS workspace_state,w.version AS workspace_version,w.archived_at,p.current_revision_id AS proposal_version_id,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id) AS requirement_count,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND r.status='OPEN') AS open_requirement_count,
 (SELECT count(*)::bigint FROM bid.review_threads t WHERE t.bid_id=w.id AND t.status='OPEN') AS open_review_thread_count,
 (SELECT count(*)::bigint FROM documents.upload_sessions u WHERE (u.opportunity_id=w.opportunity_id OR u.bid_id=w.id) AND u.state='INITIATED' AND u.expires_at>sqlc.arg(observed_at)) AS pending_upload_count,
 (SELECT count(*)::bigint FROM jobs.jobs j JOIN documents.document_versions dv ON dv.id=j.document_version_id JOIN documents.documents d ON d.id=dv.document_id WHERE (d.opportunity_id=w.opportunity_id OR d.bid_id=w.id) AND (j.state IN ('QUEUED','WAITING_FOR_CLEAN','MEASURING','PENDING_RESERVATION','READY_FOR_PROVIDER','RUNNING') OR EXISTS(SELECT 1 FROM ai.document_measurements m JOIN ai.document_measurement_entitlements e ON e.measurement_id=m.id JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=e.checkpoint_id WHERE m.job_id=j.id AND c.state IN('RESERVED','SETTLE_PENDING','RELEASE_PENDING')))) AS pending_job_count,
 (SELECT count(*)::bigint FROM bid.proposal_package_manifests m WHERE m.bid_id=w.id) AS package_manifest_count,
 (SELECT count(*)::bigint FROM bid.external_action_evidence e WHERE e.bid_id=w.id) AS external_report_count,
 (SELECT count(*)::bigint FROM bid.external_action_evidence e WHERE e.bid_id=w.id AND NOT EXISTS(SELECT 1 FROM bid.external_action_evidence s WHERE s.supersedes_evidence_id=e.id)) AS active_external_report_count,
 EXISTS(SELECT 1 FROM bid.submission_confirmations c WHERE c.bid_id=w.id) AS has_submission_confirmation,
 CAST(COALESCE((SELECT outcome::text FROM bid.reported_outcomes o WHERE o.bid_id=w.id AND NOT EXISTS(SELECT 1 FROM bid.reported_outcomes x WHERE x.supersedes_outcome_id=o.id) ORDER BY sequence DESC LIMIT 1),'') AS text) AS current_reported_outcome
FROM bid.workspaces w LEFT JOIN bid.proposals p ON p.bid_id=w.id
WHERE w.id=sqlc.arg(bid_id);
