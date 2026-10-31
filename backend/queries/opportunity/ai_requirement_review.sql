-- name: ListAIRequirementSuggestions :many
SELECT s.id,s.document_version_id,s.ordinal,s.requirement_text,s.category,s.mandatory,s.citation_page,s.citation_location,s.citation_quote,s.confidence,s.review_state,s.version,s.accepted_requirement_id,s.review_note,s.created_at,s.reviewed_by,s.reviewed_at
FROM ai.requirement_suggestions s
JOIN documents.document_versions v ON v.id=s.document_version_id
JOIN documents.documents d ON d.id=v.document_id
WHERE s.document_version_id=sqlc.arg(document_version_id) AND d.opportunity_id=sqlc.arg(opportunity_id)
ORDER BY s.ordinal LIMIT 500;

-- name: LockAISuggestionForAcceptance :one
SELECT s.id,s.document_version_id,s.requirement_text,s.category,s.mandatory,s.citation_page,s.citation_location,s.citation_quote,s.review_state,s.version,
       d.opportunity_id,w.id AS bid_id,w.source_version_id,w.version AS workspace_version,w.state AS workspace_state
FROM ai.requirement_suggestions s
JOIN documents.document_versions v ON v.id=s.document_version_id AND v.scan_state='CLEAN'
JOIN documents.documents d ON d.id=v.document_id
JOIN bid.workspaces w ON w.opportunity_id=d.opportunity_id AND w.archived_at IS NULL
WHERE s.id=sqlc.arg(suggestion_id) AND s.document_version_id=sqlc.arg(document_version_id)
  AND d.opportunity_id=sqlc.arg(opportunity_id) AND s.version=sqlc.arg(expected_suggestion_version)
  AND s.review_state='PROPOSED_UNVERIFIED' AND w.id=sqlc.arg(bid_id)
  AND w.source_version_id=sqlc.arg(source_version_id) AND w.version=sqlc.arg(expected_workspace_version) AND w.state='BID'
FOR UPDATE OF s,w;

-- name: InsertAcceptedAIRequirement :exec
INSERT INTO bid.requirements(id,bid_id,source_version_id,source_document_version_id,source_location,text,mandatory,category,status,owner_id,due_at,version,created_by,created_at,updated_at,current_content_revision_id)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(source_version_id),sqlc.arg(document_version_id),sqlc.arg(source_location),sqlc.arg(text),sqlc.arg(mandatory),sqlc.arg(category),'OPEN',sqlc.narg(owner_id),sqlc.narg(due_at),1,sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(created_at),sqlc.arg(id));

-- name: AcceptAIRequirementSuggestion :execrows
UPDATE ai.requirement_suggestions SET review_state='ACCEPTED',version=version+1,accepted_requirement_id=sqlc.arg(requirement_id),review_note=sqlc.narg(review_note),reviewed_by=sqlc.arg(actor_id),reviewed_at=sqlc.arg(reviewed_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND review_state='PROPOSED_UNVERIFIED';

-- name: LockAISuggestionForRejection :one
SELECT s.id,s.document_version_id,s.review_state,s.version,d.opportunity_id
FROM ai.requirement_suggestions s
JOIN documents.document_versions v ON v.id=s.document_version_id
JOIN documents.documents d ON d.id=v.document_id
WHERE s.id=sqlc.arg(suggestion_id) AND s.document_version_id=sqlc.arg(document_version_id)
 AND d.opportunity_id=sqlc.arg(opportunity_id) AND s.version=sqlc.arg(expected_version) AND s.review_state='PROPOSED_UNVERIFIED'
FOR UPDATE OF s;

-- name: RejectAIRequirementSuggestion :execrows
UPDATE ai.requirement_suggestions SET review_state='REJECTED',version=version+1,review_note=sqlc.arg(review_note),reviewed_by=sqlc.arg(actor_id),reviewed_at=sqlc.arg(reviewed_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND review_state='PROPOSED_UNVERIFIED';

-- name: GetAIRequirementSuggestion :one
SELECT id,document_version_id,ordinal,requirement_text,category,mandatory,citation_page,citation_location,citation_quote,confidence,review_state,version,accepted_requirement_id,review_note,created_at,reviewed_by,reviewed_at
FROM ai.requirement_suggestions WHERE id=sqlc.arg(id);

-- name: InsertAISuggestionReviewedAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),sqlc.arg(action),'ai_requirement_suggestion',sqlc.arg(suggestion_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_strip_nulls(jsonb_build_object('review_state',sqlc.arg(review_state)::text,'requirement_id',sqlc.narg(requirement_id)::uuid,'workspace_version',sqlc.narg(workspace_version)::bigint)),1,sqlc.arg(occurred_at));

-- name: InsertAISuggestionReviewedOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'ai_requirement_suggestion',sqlc.arg(suggestion_id),2,sqlc.arg(event_type),1,jsonb_strip_nulls(jsonb_build_object('suggestion_id',sqlc.arg(suggestion_id)::uuid,'document_version_id',sqlc.arg(document_version_id)::uuid,'review_state',sqlc.arg(review_state)::text,'requirement_id',sqlc.narg(requirement_id)::uuid)),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
