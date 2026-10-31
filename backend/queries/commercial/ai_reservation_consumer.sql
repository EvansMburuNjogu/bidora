-- name: GetAIReservationRequestOutcome :one
SELECT id,event_id,checkpoint_id,tenant_id,source_operation_id,request_hash,assignment_generation,requester_user_id,disposition,reservation_id,denial_code,processed_at
FROM commercial.ai_reservation_request_outcomes WHERE event_id=sqlc.arg(event_id);

-- name: GetAIReservationRequestOutcomeByCheckpoint :one
SELECT id,event_id,checkpoint_id,tenant_id,source_operation_id,request_hash,assignment_generation,requester_user_id,disposition,reservation_id,denial_code,processed_at
FROM commercial.ai_reservation_request_outcomes WHERE checkpoint_id=sqlc.arg(checkpoint_id);

-- name: InsertAIReservationRequestOutcome :exec
INSERT INTO commercial.ai_reservation_request_outcomes(id,event_id,checkpoint_id,tenant_id,source_operation_id,request_hash,assignment_generation,requester_user_id,disposition,reservation_id,denial_code,processed_at)
VALUES(sqlc.arg(id),sqlc.arg(event_id),sqlc.arg(checkpoint_id),sqlc.arg(tenant_id),sqlc.arg(source_operation_id),sqlc.arg(request_hash),sqlc.arg(assignment_generation),sqlc.arg(requester_user_id),sqlc.arg(disposition),sqlc.narg(reservation_id),sqlc.narg(denial_code),sqlc.arg(processed_at));
