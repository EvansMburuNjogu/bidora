-- name: LockBidForProposal :one
SELECT w.id,w.version,w.state FROM bid.workspaces w
WHERE w.id=sqlc.arg(bid_id) AND w.archived_at IS NULL AND w.state='BID' AND w.version=sqlc.arg(expected_workspace_version)
FOR UPDATE;

-- name: GetProposalForUpdate :one
SELECT id,version,current_revision_id FROM bid.proposals WHERE bid_id=sqlc.arg(bid_id) FOR UPDATE;

-- name: ListCurrentResponseVersionsForProposal :many
SELECT r.id AS requirement_id,s.current_revision_id AS response_version_id,r.source_document_version_id,r.text AS requirement_text,r.category,r.mandatory
FROM bid.requirements r JOIN bid.response_sections s ON s.requirement_id=r.id AND s.bid_id=r.bid_id JOIN bid.response_versions rv ON rv.id=s.current_revision_id AND rv.requirement_version_at_creation=r.version
WHERE r.bid_id=sqlc.arg(bid_id)
ORDER BY r.mandatory DESC,r.category,r.id LIMIT 1001;

-- name: InsertProposal :exec
INSERT INTO bid.proposals(id,bid_id,current_revision_id,version,created_by,created_at,updated_by,updated_at)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),NULL,1,sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(actor_id),sqlc.arg(created_at));

-- name: InsertProposalVersion :exec
INSERT INTO bid.proposal_versions(id,proposal_id,revision,workspace_version_at_creation,organization_legal_name,title,recipient,letterhead_lines,introduction,closing,change_note,created_by,created_at)
SELECT sqlc.arg(id),sqlc.arg(proposal_id),sqlc.arg(revision),sqlc.arg(workspace_version),o.legal_name,sqlc.arg(title),sqlc.arg(recipient),sqlc.arg(letterhead_lines),sqlc.arg(introduction),sqlc.arg(closing),sqlc.arg(change_note),sqlc.arg(actor_id),sqlc.arg(created_at) FROM bidora_tenant.organization_profile o WHERE o.singleton_key=true;

-- name: InsertProposalResponseVersion :exec
INSERT INTO bid.proposal_response_versions(proposal_version_id,requirement_id,response_version_id,source_document_version_id,requirement_text,requirement_category,requirement_mandatory,ordinal)
VALUES(sqlc.arg(proposal_version_id),sqlc.arg(requirement_id),sqlc.arg(response_version_id),sqlc.narg(source_document_version_id),sqlc.arg(requirement_text),sqlc.arg(category),sqlc.arg(mandatory),sqlc.arg(ordinal));

-- name: AdvanceProposal :execrows
UPDATE bid.proposals SET current_revision_id=sqlc.arg(revision_id),version=sqlc.arg(new_version),updated_by=sqlc.arg(actor_id),updated_at=sqlc.arg(updated_at)
WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version);

-- name: AdvanceWorkspaceForProposal :execrows
UPDATE bid.workspaces SET version=version+1,updated_at=sqlc.arg(updated_at) WHERE id=sqlc.arg(id) AND version=sqlc.arg(expected_version) AND state='BID' AND archived_at IS NULL;

-- name: GetCurrentProposal :one
SELECT p.id,p.bid_id,p.version,p.current_revision_id,v.revision,v.title,v.recipient,v.letterhead_lines,v.introduction,v.closing,v.change_note,v.created_by,v.created_at,v.organization_legal_name AS legal_name,w.version AS workspace_version,v.letterhead_version_id,v.letterhead_version_number,v.letterhead_template_key,v.letterhead_content_hash
FROM bid.proposals p JOIN bid.proposal_versions v ON v.id=p.current_revision_id JOIN bid.workspaces w ON w.id=p.bid_id
WHERE p.bid_id=sqlc.arg(bid_id);

-- name: GetProposalVersion :one
SELECT p.id,p.bid_id,p.version,v.id AS proposal_version_id,v.revision,v.title,v.recipient,v.letterhead_lines,v.introduction,v.closing,v.change_note,v.created_by,v.created_at,v.organization_legal_name AS legal_name,w.version AS workspace_version,v.letterhead_version_id,v.letterhead_version_number,v.letterhead_template_key,v.letterhead_content_hash
FROM bid.proposals p JOIN bid.proposal_versions v ON v.proposal_id=p.id JOIN bid.workspaces w ON w.id=p.bid_id
WHERE p.bid_id=sqlc.arg(bid_id) AND v.id=sqlc.arg(proposal_version_id);

-- name: ListProposalSnapshotResponses :many
SELECT x.ordinal,x.requirement_id,x.requirement_text,x.requirement_category AS category,x.requirement_mandatory AS mandatory,v.id AS response_version_id,v.revision,v.content,v.created_at
FROM bid.proposal_response_versions x JOIN bid.response_versions v ON v.id=x.response_version_id
WHERE x.proposal_version_id=sqlc.arg(proposal_version_id) ORDER BY x.ordinal;

-- name: ListProposalHistory :many
SELECT v.id,v.revision,v.title,v.change_note,v.created_by,v.created_at FROM bid.proposals p JOIN bid.proposal_versions v ON v.proposal_id=p.id WHERE p.bid_id=sqlc.arg(bid_id) ORDER BY v.revision DESC LIMIT 100;

-- name: InsertProposalAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ProposalRevisionSaved','proposal',sqlc.arg(proposal_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'revision',sqlc.arg(revision)::bigint,'response_count',sqlc.arg(response_count)::integer,'letterhead_version_id',(SELECT letterhead_version_id FROM bid.proposal_versions WHERE id=sqlc.arg(proposal_version_id)::uuid),'letterhead_version_number',(SELECT letterhead_version_number FROM bid.proposal_versions WHERE id=sqlc.arg(proposal_version_id)::uuid),'letterhead_template_key',(SELECT letterhead_template_key FROM bid.proposal_versions WHERE id=sqlc.arg(proposal_version_id)::uuid)),1,sqlc.arg(occurred_at));

-- name: InsertProposalOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'proposal',sqlc.arg(proposal_id),sqlc.arg(revision),'bid.proposal.revised.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_id',sqlc.arg(proposal_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'revision',sqlc.arg(revision)::bigint,'letterhead_version_id',(SELECT letterhead_version_id FROM bid.proposal_versions WHERE id=sqlc.arg(proposal_version_id)::uuid),'letterhead_version_number',(SELECT letterhead_version_number FROM bid.proposal_versions WHERE id=sqlc.arg(proposal_version_id)::uuid),'letterhead_template_key',(SELECT letterhead_template_key FROM bid.proposal_versions WHERE id=sqlc.arg(proposal_version_id)::uuid)),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
