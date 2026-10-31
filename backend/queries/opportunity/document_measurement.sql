-- name: FindPendingDocumentMeasurementReservation :one
SELECT m.id AS measurement_id,m.job_id,m.document_version_id,m.requester_user_id,m.assignment_generation,m.correlation_id,m.unit,m.measured_units,m.usage_kind,m.estimated_credits
FROM ai.document_measurements m
JOIN jobs.jobs j ON j.id=m.job_id
LEFT JOIN ai.document_measurement_entitlements e ON e.measurement_id=m.id
WHERE j.kind='TENDER_ANALYSIS' AND j.state='PENDING_RESERVATION' AND e.measurement_id IS NULL
ORDER BY m.created_at,m.id
LIMIT 1;

-- name: RecoverExpiredDocumentMeasurementJob :one
WITH candidate AS (
 SELECT j.id,j.measurement_lease_id,j.measurement_attempts
 FROM jobs.jobs j
 WHERE j.kind='TENDER_ANALYSIS' AND j.state='MEASURING' AND j.measurement_lease_expires_at<=CURRENT_TIMESTAMP
 ORDER BY j.measurement_lease_expires_at,j.id FOR UPDATE SKIP LOCKED LIMIT 1
), evidence AS (
 INSERT INTO ai.document_measurement_attempt_outcomes(id,lease_id,outcome,failure_code,occurred_at,next_attempt_at)
 SELECT sqlc.arg(outcome_id),c.measurement_lease_id,'LEASE_EXPIRED','MEASUREMENT_WORKER_INTERRUPTED',CURRENT_TIMESTAMP,
        CASE WHEN c.measurement_attempts=1 THEN CURRENT_TIMESTAMP+INTERVAL '30 seconds' WHEN c.measurement_attempts=2 THEN CURRENT_TIMESTAMP+INTERVAL '120 seconds' ELSE NULL END
 FROM candidate c RETURNING lease_id
)
UPDATE jobs.jobs j SET
 state=CASE WHEN c.measurement_attempts>=3 THEN 'FAILED' ELSE 'QUEUED' END,
 started_at=NULL,completed_at=CASE WHEN c.measurement_attempts>=3 THEN CURRENT_TIMESTAMP ELSE NULL END,
 measurement_lease_id=NULL,measurement_lease_expires_at=NULL,
 measurement_next_attempt_at=CASE WHEN c.measurement_attempts=1 THEN CURRENT_TIMESTAMP+INTERVAL '30 seconds' WHEN c.measurement_attempts=2 THEN CURRENT_TIMESTAMP+INTERVAL '120 seconds' ELSE NULL END,
 measurement_failure_code=CASE WHEN c.measurement_attempts>=3 THEN 'MEASUREMENT_RETRY_EXHAUSTED' ELSE NULL END
FROM candidate c,evidence e WHERE j.id=c.id AND e.lease_id=c.measurement_lease_id
RETURNING j.id,j.state,j.measurement_attempts;

-- name: ClaimNextDocumentMeasurementJob :one
WITH candidate AS (
 SELECT j.id FROM jobs.jobs j JOIN documents.document_versions v ON v.id=j.document_version_id
 WHERE j.kind='TENDER_ANALYSIS' AND j.state='QUEUED' AND v.scan_state='CLEAN' AND j.measurement_attempts<3
   AND (j.measurement_next_attempt_at IS NULL OR j.measurement_next_attempt_at<=CURRENT_TIMESTAMP)
 ORDER BY COALESCE(j.measurement_next_attempt_at,j.created_at),j.id FOR UPDATE OF j SKIP LOCKED LIMIT 1
), claimed AS (
 UPDATE jobs.jobs j SET state='MEASURING',attempts=attempts+1,measurement_attempts=measurement_attempts+1,started_at=CURRENT_TIMESTAMP,
  measurement_lease_id=sqlc.arg(lease_id),measurement_lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '5 minutes',measurement_next_attempt_at=NULL,measurement_failure_code=NULL
 FROM candidate c WHERE j.id=c.id
 RETURNING j.id,j.document_version_id,j.requested_by,j.attempts,j.measurement_attempts,j.measurement_lease_id,j.measurement_lease_expires_at
), attempt AS (
 INSERT INTO ai.document_measurement_attempts(lease_id,job_id,document_version_id,attempt_number,parser_key,parser_version,semantics_version,profile_version,claimed_at,lease_expires_at)
 SELECT c.measurement_lease_id,c.id,c.document_version_id,c.measurement_attempts,sqlc.arg(parser_key),sqlc.arg(parser_version),
  CASE WHEN v.media_type='application/pdf' THEN sqlc.arg(pdf_semantics) ELSE sqlc.arg(docx_semantics) END,
  sqlc.arg(profile_version),CURRENT_TIMESTAMP,c.measurement_lease_expires_at
 FROM claimed c JOIN documents.document_versions v ON v.id=c.document_version_id RETURNING lease_id
)
SELECT c.id,c.document_version_id,c.requested_by,c.attempts,c.measurement_attempts,c.measurement_lease_id,c.measurement_lease_expires_at,
 v.object_key,v.object_version,v.size_bytes,v.sha256,v.media_type,d.opportunity_id,d.document_kind,
 sqlc.arg(parser_key)::text AS parser_key,sqlc.arg(parser_version)::text AS parser_version,
 CASE WHEN v.media_type='application/pdf' THEN sqlc.arg(pdf_semantics)::text ELSE sqlc.arg(docx_semantics)::text END AS semantics_version,
 sqlc.arg(profile_version)::text AS profile_version
FROM claimed c JOIN attempt a ON a.lease_id=c.measurement_lease_id
JOIN documents.document_versions v ON v.id=c.document_version_id JOIN documents.documents d ON d.id=v.document_id;

-- name: RetryDocumentMeasurementJob :execrows
UPDATE jobs.jobs SET state='QUEUED',started_at=NULL,measurement_lease_id=NULL,measurement_lease_expires_at=NULL,measurement_next_attempt_at=sqlc.arg(next_attempt_at)
WHERE id=sqlc.arg(id) AND kind='TENDER_ANALYSIS' AND state='MEASURING' AND measurement_lease_id=sqlc.arg(lease_id) AND measurement_lease_expires_at>CURRENT_TIMESTAMP;

-- name: FailDocumentMeasurementJob :execrows
UPDATE jobs.jobs SET state='FAILED',started_at=NULL,completed_at=sqlc.arg(occurred_at),measurement_lease_id=NULL,measurement_lease_expires_at=NULL,measurement_next_attempt_at=NULL,measurement_failure_code=sqlc.arg(failure_code)
WHERE id=sqlc.arg(id) AND kind='TENDER_ANALYSIS' AND state='MEASURING' AND measurement_lease_id=sqlc.arg(lease_id) AND measurement_lease_expires_at>CURRENT_TIMESTAMP;

-- name: CancelDocumentMeasurementJob :execrows
UPDATE jobs.jobs SET state='CANCELLED',started_at=NULL,completed_at=sqlc.arg(occurred_at),measurement_lease_id=NULL,measurement_lease_expires_at=NULL,measurement_next_attempt_at=NULL,measurement_failure_code='AUTHORIZATION_DENIED'
WHERE id=sqlc.arg(id) AND kind='TENDER_ANALYSIS' AND state='MEASURING' AND measurement_lease_id=sqlc.arg(lease_id) AND measurement_lease_expires_at>CURRENT_TIMESTAMP;

-- name: InsertDocumentMeasurementAttemptOutcome :exec
INSERT INTO ai.document_measurement_attempt_outcomes(id,lease_id,outcome,failure_code,occurred_at,next_attempt_at)
VALUES(sqlc.arg(id),sqlc.arg(lease_id),sqlc.arg(outcome),sqlc.narg(failure_code),sqlc.arg(occurred_at),sqlc.narg(next_attempt_at));

-- name: LockDocumentMeasurementJob :one
SELECT j.id,j.document_version_id,j.requested_by,v.media_type,v.size_bytes,v.sha256,v.scan_state
FROM jobs.jobs j JOIN documents.document_versions v ON v.id=j.document_version_id
WHERE j.id=sqlc.arg(id) AND j.kind='TENDER_ANALYSIS' AND j.state='MEASURING' AND j.measurement_lease_id=sqlc.arg(lease_id) AND j.measurement_lease_expires_at>CURRENT_TIMESTAMP
FOR UPDATE OF j,v;

-- name: InsertDocumentMeasurement :exec
INSERT INTO ai.document_measurements(id,job_id,document_version_id,requester_user_id,assignment_generation,correlation_id,media_type,byte_size,sha256,parser_key,parser_version,semantics_version,profile_version,unit,measured_units,applicability_hash,usage_kind,estimated_credits,created_at)
VALUES(sqlc.arg(id),sqlc.arg(job_id),sqlc.arg(document_version_id),sqlc.arg(requester_user_id),sqlc.arg(assignment_generation),sqlc.arg(correlation_id),sqlc.arg(media_type),sqlc.arg(byte_size),sqlc.arg(sha256),sqlc.arg(parser_key),sqlc.arg(parser_version),sqlc.arg(semantics_version),sqlc.arg(profile_version),sqlc.arg(unit),sqlc.arg(measured_units),sqlc.arg(applicability_hash),sqlc.arg(usage_kind),sqlc.arg(estimated_credits),sqlc.arg(created_at));

-- name: MarkDocumentMeasurementPendingReservation :execrows
UPDATE jobs.jobs SET state='PENDING_RESERVATION',blocked_reason='AI_ENTITLEMENT_PENDING',started_at=NULL,measurement_lease_id=NULL,measurement_lease_expires_at=NULL
WHERE id=sqlc.arg(id) AND kind='TENDER_ANALYSIS' AND state='MEASURING' AND measurement_lease_id=sqlc.arg(lease_id) AND measurement_lease_expires_at>CURRENT_TIMESTAMP;

-- name: InsertDocumentMeasurementAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'TenderDocumentMeasured','document_measurement',sqlc.arg(measurement_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('document_version_id',sqlc.arg(document_version_id)::uuid,'unit',sqlc.arg(unit)::text,'measured_units',sqlc.arg(measured_units)::bigint,'usage_kind',sqlc.arg(usage_kind)::text,'estimated_credits',sqlc.arg(estimated_credits)::bigint,'parser_key',sqlc.arg(parser_key)::text,'parser_version',sqlc.arg(parser_version)::text,'semantics_version',sqlc.arg(semantics_version)::text,'profile_version',sqlc.arg(profile_version)::text,'applicability_hash',encode(sqlc.arg(applicability_hash)::bytea,'hex')),1,sqlc.arg(occurred_at));

-- name: InsertDocumentMeasurementOutbox :exec
INSERT INTO bidora_eventing.outbox_events(id,actor_id,aggregate_type,aggregate_id,aggregate_version,event_type,schema_version,payload,occurred_at,next_attempt_at,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'document_measurement',sqlc.arg(measurement_id),1,'ai.document.measured.v1',1,jsonb_build_object('measurement_id',sqlc.arg(measurement_id)::uuid,'job_id',sqlc.arg(job_id)::uuid,'document_version_id',sqlc.arg(document_version_id)::uuid,'unit',sqlc.arg(unit)::text,'measured_units',sqlc.arg(measured_units)::bigint,'usage_kind',sqlc.arg(usage_kind)::text,'estimated_credits',sqlc.arg(estimated_credits)::bigint,'applicability_hash',encode(sqlc.arg(applicability_hash)::bytea,'hex')),sqlc.arg(occurred_at),sqlc.arg(occurred_at),sqlc.arg(occurred_at));

-- name: GetDocumentMeasurementEntitlement :one
SELECT checkpoint_id FROM ai.document_measurement_entitlements WHERE measurement_id=sqlc.arg(measurement_id);

-- name: DocumentMeasurementCheckpointMatches :one
SELECT EXISTS(
 SELECT 1
 FROM ai.document_measurements m
 JOIN jobs.jobs j ON j.id=m.job_id
 JOIN bidora_entitlement.ai_usage_checkpoints c ON c.id=sqlc.arg(checkpoint_id)
 WHERE m.id=sqlc.arg(measurement_id) AND m.job_id=sqlc.arg(job_id) AND m.requester_user_id=sqlc.arg(requester_user_id)
   AND m.assignment_generation=sqlc.arg(assignment_generation) AND m.usage_kind=sqlc.arg(usage_kind) AND m.estimated_credits=sqlc.arg(estimated_credits)
   AND j.state='PENDING_RESERVATION'
   AND c.tenant_id=sqlc.arg(tenant_id) AND c.target_type='TENDER_ANALYSIS' AND c.target_id=m.job_id
   AND c.source_operation_id=m.id AND c.requester_user_id=m.requester_user_id AND c.assignment_generation=m.assignment_generation
   AND c.usage_kind=m.usage_kind AND c.estimated_credits=m.estimated_credits AND c.state IN ('PENDING_RESERVATION','RESERVED','DENIED')
);

-- name: InsertDocumentMeasurementEntitlement :execrows
INSERT INTO ai.document_measurement_entitlements(measurement_id,checkpoint_id,linked_at)
VALUES(sqlc.arg(measurement_id),sqlc.arg(checkpoint_id),sqlc.arg(linked_at))
ON CONFLICT (measurement_id) DO NOTHING;

-- name: InsertDocumentMeasurementEntitlementAudit :exec
INSERT INTO bidora_audit.audit_events(id,actor_id,action,target_type,target_id,occurred_at,outcome,correlation_id,change_summary,policy_version,created_at)
VALUES(sqlc.arg(id),sqlc.arg(actor_id),'DocumentMeasurementEntitlementRequested','document_measurement',sqlc.arg(measurement_id),sqlc.arg(occurred_at),'SUCCEEDED',sqlc.arg(correlation_id),jsonb_build_object('checkpoint_id',sqlc.arg(checkpoint_id)::uuid),1,sqlc.arg(occurred_at));
