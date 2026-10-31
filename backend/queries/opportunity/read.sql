-- name: ListOpportunities :many
SELECT o.id,o.version,o.created_at,o.updated_at,s.id AS source_version_id,s.title,s.buyer_name,
       s.closes_at,s.source_timezone,s.source_date_text,s.verification_state
FROM bidora_opportunity.opportunities o
JOIN bidora_opportunity.source_versions s ON s.id=o.current_source_version_id AND s.opportunity_id=o.id
WHERE (sqlc.arg(cursor_created_at)::timestamptz IS NULL OR (o.created_at,o.id)<(sqlc.arg(cursor_created_at)::timestamptz,sqlc.arg(cursor_id)::uuid))
ORDER BY o.created_at DESC,o.id DESC
LIMIT sqlc.arg(max_rows);

-- name: GetOpportunityDetail :one
SELECT o.id,o.version,o.created_at,o.updated_at,s.id AS source_version_id,s.revision,s.title,s.buyer_name,
       s.source_uri,s.source_document_version_id,s.closes_at,s.source_timezone,s.source_date_text,
       s.verification_state,s.verified_by,s.verified_at
FROM bidora_opportunity.opportunities o
JOIN bidora_opportunity.source_versions s ON s.id=o.current_source_version_id AND s.opportunity_id=o.id
WHERE o.id=sqlc.arg(id);

-- name: ListPossibleDuplicateOpportunities :many
SELECT candidate.id
FROM bidora_opportunity.opportunities current_opportunity
JOIN bidora_opportunity.source_versions current_source ON current_source.id=current_opportunity.current_source_version_id
JOIN bidora_opportunity.opportunities candidate ON candidate.id<>current_opportunity.id
JOIN bidora_opportunity.source_versions candidate_source ON candidate_source.id=candidate.current_source_version_id
WHERE current_opportunity.id=sqlc.arg(opportunity_id)
  AND lower(candidate_source.title)=lower(current_source.title)
  AND lower(COALESCE(candidate_source.buyer_name,''))=lower(COALESCE(current_source.buyer_name,''))
ORDER BY candidate.created_at DESC,candidate.id DESC
LIMIT 21;

-- name: ListBidWorkspaces :many
SELECT w.id,w.opportunity_id,w.source_version_id,w.owner_id,w.state,w.version,w.archived_at,w.created_at,w.updated_at,
       s.title,s.buyer_name,s.closes_at,s.source_timezone
FROM bid.workspaces w
JOIN bidora_opportunity.source_versions s ON s.id=w.source_version_id AND s.opportunity_id=w.opportunity_id
WHERE ((sqlc.arg(archive_state)::text='ACTIVE' AND w.archived_at IS NULL) OR (sqlc.arg(archive_state)::text='ARCHIVED' AND w.archived_at IS NOT NULL))
  AND (sqlc.narg(state)::text IS NULL OR w.state=sqlc.narg(state))
  AND (sqlc.narg(owner_id)::uuid IS NULL OR w.owner_id=sqlc.narg(owner_id))
  AND (sqlc.arg(cursor_created_at)::timestamptz IS NULL OR (w.created_at,w.id)<(sqlc.arg(cursor_created_at)::timestamptz,sqlc.arg(cursor_id)::uuid))
ORDER BY w.created_at DESC,w.id DESC
LIMIT sqlc.arg(max_rows);

-- name: GetBidWorkspaceDetail :one
SELECT w.id,w.opportunity_id,w.source_version_id,w.owner_id,w.state,w.version,w.archived_at,w.archived_by,w.archive_reason,w.archive_policy_version,w.created_by,w.created_at,w.updated_at,
       s.revision AS source_revision,s.title,s.buyer_name,s.source_uri,s.source_document_version_id,
       s.closes_at,s.source_timezone,s.source_date_text,s.verification_state
FROM bid.workspaces w
JOIN bidora_opportunity.source_versions s ON s.id=w.source_version_id AND s.opportunity_id=w.opportunity_id
WHERE w.id=sqlc.arg(id);

-- name: ListBidDecisionHistory :many
SELECT id,sequence,workspace_version,decision,rationale,actor_id,decided_at
FROM bid.decisions
WHERE bid_id=sqlc.arg(bid_id)
ORDER BY sequence DESC,id DESC
LIMIT 100;
