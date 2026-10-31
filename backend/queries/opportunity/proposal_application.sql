-- name: LockProposalSuggestionForApply :one
SELECT s.id AS suggestion_id,s.version AS suggestion_version,r.id AS run_id,r.bid_id,r.proposal_id,r.proposal_version_id,
 p.version AS proposal_version,p.current_revision_id,w.version AS workspace_version,
 current.title,current.recipient,current.letterhead_lines,current.organization_legal_name
FROM ai.proposal_draft_suggestions s JOIN ai.proposal_draft_runs r ON r.id=s.run_id
JOIN bid.proposals p ON p.id=r.proposal_id AND p.bid_id=r.bid_id
JOIN bid.proposal_versions current ON current.id=p.current_revision_id
JOIN bid.workspaces w ON w.id=p.bid_id AND w.state='BID' AND w.archived_at IS NULL
WHERE s.id=sqlc.arg(suggestion_id) AND s.review_state='PROPOSED_UNVERIFIED' AND s.version=sqlc.arg(expected_suggestion_version)
 AND r.bid_id=sqlc.arg(bid_id) AND p.current_revision_id=r.proposal_version_id
 AND p.version=sqlc.arg(expected_proposal_version) AND w.version=sqlc.arg(expected_workspace_version)
FOR UPDATE OF s,p,w;

-- name: InsertAppliedProposalVersion :exec
INSERT INTO bid.proposal_versions(id,proposal_id,revision,workspace_version_at_creation,organization_legal_name,title,recipient,letterhead_lines,introduction,closing,change_note,created_by,created_at,source_ai_suggestion_id)
VALUES(sqlc.arg(id),sqlc.arg(proposal_id),sqlc.arg(revision),sqlc.arg(workspace_version),sqlc.arg(organization_legal_name),sqlc.arg(title),sqlc.arg(recipient),sqlc.arg(letterhead_lines),sqlc.arg(introduction),sqlc.arg(closing),sqlc.arg(change_note),sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(suggestion_id));

-- name: ApplyProposalDraftSuggestion :execrows
UPDATE ai.proposal_draft_suggestions SET review_state='APPLIED',version=version+1,reviewed_by=sqlc.arg(actor_id),reviewed_at=sqlc.arg(reviewed_at),applied_proposal_version_id=sqlc.arg(proposal_version_id),review_note=sqlc.arg(review_note)
WHERE id=sqlc.arg(id) AND review_state='PROPOSED_UNVERIFIED' AND version=sqlc.arg(expected_version);

-- name: InsertProposalSuggestionAppliedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'AIProposalSuggestionApplied','ai_proposal_draft',sqlc.arg(suggestion_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid),1,sqlc.arg(occurred_at));
-- name: InsertProposalSuggestionAppliedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'ai_proposal_draft',sqlc.arg(suggestion_id),sqlc.arg(suggestion_version),'ai.proposal_draft.applied.v1',1,jsonb_build_object('suggestion_id',sqlc.arg(suggestion_id)::uuid,'bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));

-- name: GetProposalApprovalReadiness :one
SELECT w.id AS bid_id,w.state AS workspace_state,w.version AS workspace_version,p.id AS proposal_id,p.version AS proposal_version,p.current_revision_id,
 v.workspace_version_at_creation,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id) AS requirement_count,
 (SELECT count(*)::bigint FROM bid.review_threads t WHERE t.bid_id=w.id AND t.status='OPEN') AS open_review_thread_count,
 (SELECT count(*)::bigint FROM bid.review_threads t WHERE t.bid_id=w.id AND t.status='OPEN' AND t.assigned_to IS NULL) AS unassigned_review_thread_count,
 (SELECT count(*)::bigint FROM bid.review_threads t WHERE t.bid_id=w.id AND t.status='OPEN' AND t.due_at IS NOT NULL AND t.due_at<=sqlc.arg(observed_at)) AS overdue_review_thread_count,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND r.mandatory AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at))))) AS mandatory_unsatisfied,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at)))) AND r.owner_id IS NULL) AS open_unassigned,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND (r.status<>'SATISFIED' OR NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at)))) AND r.due_at IS NOT NULL AND r.due_at<sqlc.arg(observed_at)) AS open_overdue,
 (SELECT count(*)::bigint FROM bid.requirements r WHERE r.bid_id=w.id AND r.status='SATISFIED' AND EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND dv.expires_at IS NOT NULL AND dv.expires_at<=sqlc.arg(observed_at)) AND NOT EXISTS(SELECT 1 FROM bid.requirement_evidence e JOIN documents.document_versions dv ON dv.id=e.document_version_id WHERE e.requirement_id=r.id AND (dv.expires_at IS NULL OR dv.expires_at>sqlc.arg(observed_at)))) AS expired_evidence_count,
 (SELECT count(*)::bigint FROM bid.requirements r LEFT JOIN bid.response_sections s ON s.bid_id=r.bid_id AND s.requirement_id=r.id LEFT JOIN bid.response_versions rv ON rv.id=s.current_revision_id WHERE r.bid_id=w.id AND r.mandatory AND (s.id IS NULL OR rv.requirement_version_at_creation<>r.version)) AS mandatory_response_missing
FROM bid.workspaces w LEFT JOIN bid.proposals p ON p.bid_id=w.id LEFT JOIN bid.proposal_versions v ON v.id=p.current_revision_id
WHERE w.id=sqlc.arg(bid_id) AND w.archived_at IS NULL;
