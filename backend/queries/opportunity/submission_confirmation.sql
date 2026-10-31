-- name: LockExternalReportForProofUpload :one
SELECT e.id,e.bid_id,e.recorded_by FROM bid.external_action_evidence e JOIN bid.workspaces w ON w.id=e.bid_id
WHERE e.id=sqlc.arg(evidence_id) AND e.bid_id=sqlc.arg(bid_id) AND e.recorded_by=sqlc.arg(actor_id) AND w.state='BID' AND w.archived_at IS NULL
AND NOT EXISTS(SELECT 1 FROM bid.external_action_evidence x WHERE x.supersedes_evidence_id=e.id) FOR UPDATE OF w;
-- name: InsertSubmissionProofDocument :exec
INSERT INTO documents.documents(id,opportunity_id,title,state,created_by,created_at,updated_at,version,document_kind,bid_id,external_action_evidence_id)
VALUES(sqlc.arg(id),NULL,'External submission proof','UPLOADING',sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(created_at),1,'SUBMISSION_PROOF',sqlc.arg(bid_id),sqlc.arg(evidence_id));
-- name: InsertSubmissionProofUpload :exec
INSERT INTO documents.upload_sessions(id,document_id,opportunity_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at,created_by,created_at,bid_id,external_action_evidence_id)
VALUES(sqlc.arg(id),sqlc.arg(document_id),NULL,sqlc.arg(object_key),'application/pdf',sqlc.arg(size_bytes),sqlc.arg(sha256),'INITIATED',sqlc.arg(expires_at),sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(bid_id),sqlc.arg(evidence_id));
-- name: GetSubmissionProofUpload :one
SELECT id,document_id,bid_id,external_action_evidence_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at,completed_version_id
FROM documents.upload_sessions WHERE id=sqlc.arg(id) AND bid_id=sqlc.arg(bid_id) AND external_action_evidence_id=sqlc.arg(evidence_id) AND created_by=sqlc.arg(actor_id);
-- name: LockSubmissionProofUpload :one
SELECT id,document_id,bid_id,external_action_evidence_id,object_key,expected_media_type,expected_size_bytes,expected_sha256,state,expires_at
FROM documents.upload_sessions WHERE id=sqlc.arg(id) AND bid_id=sqlc.arg(bid_id) AND external_action_evidence_id=sqlc.arg(evidence_id) AND created_by=sqlc.arg(actor_id) FOR UPDATE;
-- name: GetSubmissionProofCompletion :one
SELECT u.id AS upload_id,u.document_id,u.completed_version_id AS version_id,j.id AS scan_job_id,v.scan_state
FROM documents.upload_sessions u JOIN documents.document_versions v ON v.id=u.completed_version_id JOIN jobs.jobs j ON j.document_version_id=v.id AND j.kind='MALWARE_SCAN'
WHERE u.id=sqlc.arg(id) AND u.bid_id=sqlc.arg(bid_id) AND u.external_action_evidence_id=sqlc.arg(evidence_id) AND u.created_by=sqlc.arg(actor_id);
-- name: InsertSubmissionProofAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),sqlc.arg(target_type),sqlc.arg(target_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'external_action_evidence_id',sqlc.arg(evidence_id)::uuid),1,sqlc.arg(occurred_at));
-- name: GetSubmissionConfirmationIdempotency :one
SELECT request_hash,state,response_status,response_ref,response_version FROM bidora_commands.idempotency_records WHERE actor_id=sqlc.arg(actor_id) AND operation='CONFIRM_EXTERNAL_SUBMISSION' AND key_hash=sqlc.arg(key_hash);
-- name: LockSubmissionConfirmationCandidate :one
WITH latest AS (SELECT DISTINCT ON (reviewer_id) reviewer_id,decision FROM bid.internal_review_decisions WHERE proposal_version_id=(SELECT proposal_version_id FROM bid.proposal_package_manifests WHERE id=sqlc.arg(manifest_id)) ORDER BY reviewer_id,reviewer_sequence DESC)
SELECT w.id AS bid_id,w.version AS workspace_version,e.id AS evidence_id,e.recorded_by,e.reported_action_at,e.recorded_at,e.package_manifest_id,
 m.proposal_version_id,m.readiness_ready,m.readiness_blockers,a.id AS approval_id,dv.id AS proof_version_id,dv.sha256,dv.size_bytes,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND r.mandatory AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence re JOIN documents.document_versions rv ON rv.id=re.document_version_id WHERE re.requirement_id=r.id AND (rv.expires_at IS NULL OR rv.expires_at>sqlc.arg(observed_at))))) AS mandatory_unsatisfied,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence re JOIN documents.document_versions rv ON rv.id=re.document_version_id WHERE re.requirement_id=r.id AND (rv.expires_at IS NULL OR rv.expires_at>sqlc.arg(observed_at)))) AND r.owner_id IS NULL) AS open_unassigned,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence re JOIN documents.document_versions rv ON rv.id=re.document_version_id WHERE re.requirement_id=r.id AND (rv.expires_at IS NULL OR rv.expires_at>sqlc.arg(observed_at)))) AND r.due_at IS NOT NULL AND r.due_at<sqlc.arg(observed_at)) AS open_overdue,
 (SELECT count(*)::bigint FROM bid.requirements r LEFT JOIN bid.response_sections s ON s.bid_id=r.bid_id AND s.requirement_id=r.id LEFT JOIN bid.response_versions rv ON rv.id=s.current_revision_id WHERE r.bid_id=w.id AND r.mandatory AND (s.id IS NULL OR rv.requirement_version_at_creation<>r.version)) AS mandatory_response_missing,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND r.status='SATISFIED' AND EXISTS(SELECT 1 FROM bid.requirement_evidence re JOIN documents.document_versions rv ON rv.id=re.document_version_id WHERE re.requirement_id=r.id AND rv.expires_at IS NOT NULL AND rv.expires_at<=sqlc.arg(observed_at)) AND NOT EXISTS(SELECT 1 FROM bid.requirement_evidence re JOIN documents.document_versions rv ON rv.id=re.document_version_id WHERE re.requirement_id=r.id AND (rv.expires_at IS NULL OR rv.expires_at>sqlc.arg(observed_at)))) AS expired_evidence_count
FROM bid.workspaces w JOIN bid.external_action_evidence e ON e.bid_id=w.id
JOIN bid.proposal_package_manifests m ON m.id=e.package_manifest_id
JOIN bid.proposals p ON p.bid_id=w.id AND p.current_revision_id=m.proposal_version_id
JOIN bid.proposal_approvals a ON a.package_manifest_id=m.id
JOIN documents.documents doc ON doc.external_action_evidence_id=e.id AND doc.document_kind='SUBMISSION_PROOF'
JOIN documents.document_versions dv ON dv.document_id=doc.id
WHERE w.id=sqlc.arg(bid_id) AND w.state='BID' AND w.archived_at IS NULL AND e.id=sqlc.arg(evidence_id) AND m.id=sqlc.arg(manifest_id)
AND dv.id=sqlc.arg(proof_version_id) AND dv.media_type='application/pdf' AND dv.size_bytes<=26214400 AND dv.scan_state='CLEAN' AND dv.expires_at IS NULL
AND e.recorded_by<>sqlc.arg(verifier_id) AND NOT EXISTS(SELECT 1 FROM bid.external_action_evidence x WHERE x.supersedes_evidence_id=e.id)
AND (SELECT count(*) FROM bid.external_action_evidence leaf WHERE leaf.bid_id=w.id AND NOT EXISTS(SELECT 1 FROM bid.external_action_evidence child WHERE child.supersedes_evidence_id=leaf.id))=1
AND NOT EXISTS(SELECT 1 FROM latest WHERE decision='CHANGES_REQUESTED')
AND NOT EXISTS(SELECT 1 FROM bid.review_threads t WHERE t.bid_id=w.id AND t.status='OPEN')
FOR UPDATE OF w,p;
-- name: InsertSubmissionConfirmation :exec
INSERT INTO bid.submission_confirmations(id,bid_id,external_action_evidence_id,package_manifest_id,proposal_approval_id,proof_document_version_id,reporter_id,verifier_id,reported_action_at,report_recorded_at,confirmed_at,rationale,submission_basis,policy_version,workspace_version)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(evidence_id),sqlc.arg(manifest_id),sqlc.arg(approval_id),sqlc.arg(proof_version_id),sqlc.arg(reporter_id),sqlc.arg(verifier_id),sqlc.arg(reported_action_at),sqlc.arg(report_recorded_at),sqlc.arg(confirmed_at),sqlc.arg(rationale),'HUMAN_CONFIRMED_EXTERNAL_ACTION',1,sqlc.arg(workspace_version));
-- name: MarkWorkspaceSubmitted :execrows
UPDATE bid.workspaces SET state='SUBMITTED',version=version+1,updated_at=sqlc.arg(updated_at) WHERE id=sqlc.arg(id) AND state='BID' AND version=sqlc.arg(expected_version);
-- name: GetSubmissionConfirmation :one
SELECT id,bid_id,external_action_evidence_id,package_manifest_id,proposal_approval_id,proof_document_version_id,reporter_id,verifier_id,reported_action_at,report_recorded_at,confirmed_at,rationale,submission_basis,policy_version,workspace_version FROM bid.submission_confirmations WHERE id=sqlc.arg(id);
-- name: GetSubmissionConfirmationByBid :one
SELECT id,bid_id,external_action_evidence_id,package_manifest_id,proposal_approval_id,proof_document_version_id,reporter_id,verifier_id,reported_action_at,report_recorded_at,confirmed_at,rationale,submission_basis,policy_version,workspace_version FROM bid.submission_confirmations WHERE bid_id=sqlc.arg(bid_id);
-- name: InsertSubmissionConfirmationAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at) VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ExternalSubmissionHumanConfirmed','submission_confirmation',sqlc.arg(confirmation_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'evidence_id',sqlc.arg(evidence_id)::uuid,'manifest_id',sqlc.arg(manifest_id)::uuid,'approval_id',sqlc.arg(approval_id)::uuid,'proof_version_id',sqlc.arg(proof_version_id)::uuid,'workspace_version',sqlc.arg(workspace_version)::bigint,'submission_basis','HUMAN_CONFIRMED_EXTERNAL_ACTION'),1,sqlc.arg(occurred_at));
-- name: InsertSubmissionConfirmationOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id) VALUES(sqlc.arg(id),'bid',sqlc.arg(bid_id),sqlc.arg(workspace_version),'bid.external_submission.human_confirmed.v1',1,jsonb_build_object('confirmation_id',sqlc.arg(confirmation_id)::uuid,'evidence_id',sqlc.arg(evidence_id)::uuid,'manifest_id',sqlc.arg(manifest_id)::uuid,'approval_id',sqlc.arg(approval_id)::uuid,'proof_version_id',sqlc.arg(proof_version_id)::uuid,'verifier_id',sqlc.arg(actor_id)::uuid,'workspace_version',sqlc.arg(workspace_version)::bigint,'submission_basis','HUMAN_CONFIRMED_EXTERNAL_ACTION','bidora_transmitted',false,'external_acceptance_confirmed',false,'on_time_submission_confirmed',false),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
