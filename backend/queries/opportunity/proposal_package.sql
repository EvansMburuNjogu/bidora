-- name: ValidateProposalPackageBidRead :one
SELECT w.id FROM bid.workspaces w WHERE w.id=sqlc.arg(bid_id);

-- name: GetProposalPackageIdempotency :one
SELECT request_hash,state,response_status,response_ref,response_version FROM bidora_commands.idempotency_records
WHERE actor_id=sqlc.arg(actor_id) AND operation='PREPARE_PROPOSAL_PACKAGE' AND key_hash=sqlc.arg(key_hash);

-- name: LockProposalForPackagePreparation :one
SELECT p.id AS proposal_id,p.version AS proposal_version,p.current_revision_id,pv.revision,w.version AS workspace_version,pv.letterhead_version_id,pv.letterhead_version_number,pv.letterhead_template_key,pv.letterhead_content_hash
FROM bid.proposals p JOIN bid.proposal_versions pv ON pv.id=p.current_revision_id AND pv.proposal_id=p.id
JOIN bid.workspaces w ON w.id=p.bid_id
WHERE w.id=sqlc.arg(bid_id) AND w.archived_at IS NULL AND w.state='BID' AND pv.id=sqlc.arg(proposal_version_id)
FOR UPDATE OF p,w;

-- name: InsertProposalPackageManifest :exec
INSERT INTO bid.proposal_package_manifests(id,bid_id,proposal_id,proposal_version_id,proposal_revision,package_status,pdf_render_profile,pdf_sha256,pdf_size,docx_render_profile,docx_sha256,docx_size,readiness_observed_at,readiness_ready,readiness_blockers,manifest_sha256,manifest_version,created_by,created_at,letterhead_version_id,letterhead_version_number,letterhead_template_key,letterhead_content_hash)
VALUES(sqlc.arg(id),sqlc.arg(bid_id),sqlc.arg(proposal_id),sqlc.arg(proposal_version_id),sqlc.arg(proposal_revision),'UNAPPROVED_PREPARATION_ONLY','BIDORA_PDF_V1',sqlc.arg(pdf_sha256),sqlc.arg(pdf_size),'BIDORA_DOCX_V1',sqlc.arg(docx_sha256),sqlc.arg(docx_size),sqlc.arg(readiness_observed_at),sqlc.arg(readiness_ready),sqlc.arg(readiness_blockers),sqlc.arg(manifest_sha256),2,sqlc.arg(actor_id),sqlc.arg(created_at),sqlc.arg(letterhead_version_id),sqlc.arg(letterhead_version_number),sqlc.arg(letterhead_template_key),sqlc.arg(letterhead_content_hash));

-- name: GetProposalPackageManifest :one
SELECT id,bid_id,proposal_id,proposal_version_id,proposal_revision,package_status,pdf_render_profile,pdf_sha256,pdf_size,docx_render_profile,docx_sha256,docx_size,readiness_observed_at,readiness_ready,readiness_blockers,manifest_sha256,manifest_version,created_by,created_at,letterhead_version_id,letterhead_version_number,letterhead_template_key,letterhead_content_hash
FROM bid.proposal_package_manifests WHERE id=sqlc.arg(id) AND bid_id=sqlc.arg(bid_id);

-- name: ListProposalPackageManifests :many
SELECT id,bid_id,proposal_id,proposal_version_id,proposal_revision,package_status,pdf_render_profile,pdf_sha256,pdf_size,docx_render_profile,docx_sha256,docx_size,readiness_observed_at,readiness_ready,readiness_blockers,manifest_sha256,manifest_version,created_by,created_at,letterhead_version_id,letterhead_version_number,letterhead_template_key,letterhead_content_hash
FROM bid.proposal_package_manifests
WHERE bid_id=sqlc.arg(bid_id) AND (sqlc.narg(cursor_at)::timestamptz IS NULL OR (created_at,id)<(sqlc.narg(cursor_at)::timestamptz,sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: InsertProposalPackageAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'ProposalPackagePreparationRecorded','proposal_package_manifest',sqlc.arg(manifest_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'letterhead_version_id',sqlc.arg(letterhead_version_id)::uuid,'package_status','UNAPPROVED_PREPARATION_ONLY','manifest_version',2,'letterhead_version_number',sqlc.arg(letterhead_version_number)::bigint,'letterhead_template_key',sqlc.arg(letterhead_template_key)::text),1,sqlc.arg(occurred_at));

-- name: InsertProposalPackageOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at,actor_id)
VALUES(sqlc.arg(id),'proposal_package_manifest',sqlc.arg(manifest_id),1,'bid.proposal_package.prepared.v1',1,jsonb_build_object('bid_id',sqlc.arg(bid_id)::uuid,'proposal_version_id',sqlc.arg(proposal_version_id)::uuid,'letterhead_version_id',sqlc.arg(letterhead_version_id)::uuid,'package_status','UNAPPROVED_PREPARATION_ONLY','manifest_version',2,'letterhead_version_number',sqlc.arg(letterhead_version_number)::bigint,'letterhead_template_key',sqlc.arg(letterhead_template_key)::text),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(actor_id));
